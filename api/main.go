// Command ragdesk-api is the Go core service: it owns tenancy, documents,
// billing and metering, and fronts the Python AI service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/thefcan/ragdesk/api/internal/ai"
	"github.com/thefcan/ragdesk/api/internal/auth"
	"github.com/thefcan/ragdesk/api/internal/billing"
	"github.com/thefcan/ragdesk/api/internal/config"
	"github.com/thefcan/ragdesk/api/internal/ingest"
	"github.com/thefcan/ragdesk/api/internal/server"
	"github.com/thefcan/ragdesk/api/internal/store"
	"github.com/thefcan/ragdesk/api/internal/telemetry"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", slog.Any("err", err))
		os.Exit(1)
	}

	if cfg.JWTSecret == config.DefaultJWTSecret {
		log.Warn("using the default JWT secret; set JWT_SECRET outside development")
	}

	// Optional distributed tracing (no-op unless OTEL_EXPORTER_OTLP_ENDPOINT set).
	shutdownTracing, err := telemetry.Init(context.Background(), "ragdesk-api", server.Version)
	if err != nil {
		log.Warn("telemetry init", slog.Any("err", err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(ctx)
	}()
	if telemetry.Enabled() {
		log.Info("opentelemetry tracing enabled")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres config", slog.Any("err", err))
		os.Exit(1)
	}
	// Trace SQL queries; a no-op without a configured tracer provider.
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()
	db, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		log.Error("postgres pool", slog.Any("err", err))
		os.Exit(1)
	}
	defer db.Close()

	st := store.New(db)
	{
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := st.Migrate(ctx)
		cancel()
		if err != nil {
			log.Error("migrate", slog.Any("err", err))
			os.Exit(1)
		}
		log.Info("migrations applied")
	}

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Error("redis url", slog.Any("err", err))
		os.Exit(1)
	}
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()

	issuer := auth.NewIssuer(cfg.JWTSecret, cfg.JWTTTL)

	// Billing provider: real Stripe when configured, otherwise the $0 dev path.
	var billingProvider billing.Provider = billing.Noop{}
	if cfg.StripeSecretKey != "" {
		billingProvider = billing.NewStripe(cfg.StripeSecretKey, cfg.StripePriceProID, cfg.StripeWebhookSecret)
		log.Info("stripe billing enabled")
	} else {
		log.Info("stripe not configured; billing runs in dev mode (no charges)")
	}

	// Async ingestion worker.
	aiClient := ai.NewClient(cfg.AIServiceURL, cfg.AIInternalToken)
	worker := ingest.NewWorker(rdb, st, aiClient, log)
	workerCtx, stopWorker := context.WithCancel(context.Background())
	go worker.Run(workerCtx)

	handler := server.New(st, rdb, issuer, aiClient, billingProvider, cfg.CORSAllowedOrigins, cfg.WebBaseURL, log).Handler()
	// Server span per request; trace context is propagated from inbound headers.
	handler = otelhttp.NewHandler(handler, "ragdesk-api")
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("api starting", slog.String("addr", srv.Addr), slog.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen", slog.Any("err", err))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info("api shutting down")
	stopWorker()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown", slog.Any("err", err))
	}
	log.Info("api stopped")
}

func healthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:" + port + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

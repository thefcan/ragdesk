// Command ragdesk-api is the Go core service: it owns tenancy, billing and
// metering, and fronts the Python AI service.
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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/thefcan/ragdesk/api/internal/auth"
	"github.com/thefcan/ragdesk/api/internal/config"
	"github.com/thefcan/ragdesk/api/internal/server"
	"github.com/thefcan/ragdesk/api/internal/store"
)

func main() {
	// Self health-check mode used by the container HEALTHCHECK: the distroless
	// image ships no shell or curl, so the binary probes itself.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", slog.Any("err", err))
		os.Exit(1)
	}

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
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

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.New(st, rdb, issuer, log).Handler(),
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown", slog.Any("err", err))
	}
	log.Info("api stopped")
}

// healthcheck performs an HTTP probe against the local liveness endpoint and
// returns a process exit code (0 healthy, 1 unhealthy).
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

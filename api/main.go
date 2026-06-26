// Command ragdesk-api is the Go core service: it owns tenancy, billing and
// metering, and fronts the Python AI service. Phase 0 wires the server,
// dependencies, health probes and graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/thefcan/ragdesk/api/internal/config"
	"github.com/thefcan/ragdesk/api/internal/server"
)

func main() {
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

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Error("redis url", slog.Any("err", err))
		os.Exit(1)
	}
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.New(db, rdb, log).Handler(),
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

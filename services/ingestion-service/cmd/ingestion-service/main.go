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

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/redis/go-redis/v9"

	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/api"
	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/eventbus"
	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/healthz"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
	redisPingTimeout  = 5 * time.Second
	defaultRedisURL   = "redis://localhost:6379"
	defaultPort       = "8081"
)

func main() {
	logger := slog.Default()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = defaultRedisURL
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("invalid REDIS_URL", "error", err)
		os.Exit(1)
	}
	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()

	pingCtx, pingCancel := context.WithTimeout(ctx, redisPingTimeout)
	defer pingCancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Error("cannot reach Redis", "error", err, "redis_url", redisURL)
		os.Exit(1)
	}

	publisher := eventbus.NewPublisher(client)

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz.Handler())

	humaAPI := humago.New(mux, huma.DefaultConfig("Ingestion Service", "0.1.0"))
	api.Register(humaAPI, publisher)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "error", err)
		}
	}()

	logger.Info("ingestion-service listening", "port", port, "redis_url", redisURL)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

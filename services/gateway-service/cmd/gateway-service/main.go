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

	"github.com/AndoNorth/match-flow/services/gateway-service/internal/api"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/cors"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/healthz"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/matchclient"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/realtime"
)

const (
	readHeaderTimeout    = 5 * time.Second
	shutdownTimeout      = 5 * time.Second
	redisPingTimeout     = 5 * time.Second
	defaultRedisURL      = "redis://localhost:6379"
	defaultPort          = "8083"
	defaultMatchAddr     = "localhost:8082"
	defaultAllowedOrigin = "http://localhost:3000"
	matchflowEventsChan  = "matchflow:events"
)

func main() {
	logger := slog.Default()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	redisURL := envOr("REDIS_URL", defaultRedisURL)
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("invalid REDIS_URL", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(opts)
	defer func() { _ = redisClient.Close() }()

	pingCtx, pingCancel := context.WithTimeout(ctx, redisPingTimeout)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		logger.Error("cannot reach Redis", "error", err, "redis_url", redisURL)
		os.Exit(1)
	}

	matchAddr := envOr("MATCH_SERVICE_ADDR", defaultMatchAddr)
	client := matchclient.New("http://"+matchAddr, http.DefaultClient)

	registry := realtime.NewRegistry()
	go realtime.Subscribe(ctx, redisClient, matchflowEventsChan, registry, logger)

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz.Handler())
	mux.Handle("GET /events", realtime.Handler(registry, client))

	humaAPI := humago.New(mux, huma.DefaultConfig("Gateway Service", "0.1.0"))
	api.Register(humaAPI, client)

	allowedOrigin := envOr("GATEWAY_ALLOWED_ORIGIN", defaultAllowedOrigin)

	port := envOr("PORT", defaultPort)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           cors.Middleware(allowedOrigin, mux),
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

	logger.Info("gateway-service listening",
		"port", port, "redis_url", redisURL, "match_service_addr", matchAddr, "allowed_origin", allowedOrigin)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

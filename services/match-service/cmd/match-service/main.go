package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/AndoNorth/match-flow/services/match-service/internal/api"
	"github.com/AndoNorth/match-flow/services/match-service/internal/eventstream"
	"github.com/AndoNorth/match-flow/services/match-service/internal/healthz"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
	redisPingTimeout  = 5 * time.Second
	defaultRedisURL   = "redis://localhost:6379"
	//nolint:gosec // local dev default, not a real credential
	defaultPostgresDSN = "postgres://matchflow:matchflow@localhost:5432/matchflow?sslmode=disable"
	defaultPort        = "8082"
	defaultWorkers     = 4
	defaultSport       = "football"
)

func main() {
	logger := slog.Default()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dsn := envOr("POSTGRES_DSN", defaultPostgresDSN)
	if err := matchstate.Migrate(ctx, dsn); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}

	dbPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("cannot connect to Postgres", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

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

	sport := envOr("MATCH_SERVICE_DEFAULT_SPORT", defaultSport)
	store := matchstate.NewStore(dbPool, sport)

	numWorkers, err := workerCount()
	if err != nil {
		logger.Error("invalid MATCH_SERVICE_WORKERS", "error", err)
		os.Exit(1)
	}
	pool := matchstate.NewPool(numWorkers, store, logger)

	runDone := make(chan struct{})
	go func() {
		eventstream.Run(ctx, redisClient, pool, logger)
		close(runDone)
	}()

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz.Handler())

	humaAPI := humago.New(mux, huma.DefaultConfig("Match Service", "0.1.0"))
	api.Register(humaAPI, store)

	port := envOr("PORT", defaultPort)
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

	logger.Info("match-service listening", "port", port, "redis_url", redisURL, "workers", numWorkers)
	serveErr := server.ListenAndServe()
	<-runDone // wait for the worker pool to finish draining before exiting
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		logger.Error("server failed", "error", serveErr)
		os.Exit(1)
	}
}

func workerCount() (int, error) {
	v := os.Getenv("MATCH_SERVICE_WORKERS")
	if v == "" {
		return defaultWorkers, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err //nolint:wrapcheck // top-level main, error is logged directly by the caller
	}
	if n < 1 {
		return 0, errors.New("must be at least 1")
	}
	return n, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

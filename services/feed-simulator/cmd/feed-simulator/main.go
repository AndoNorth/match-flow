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

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/healthz"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/football"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/providers"
)

const (
	readHeaderTimeout   = 5 * time.Second
	shutdownTimeout     = 5 * time.Second
	defaultIngestionURL = "http://localhost:8081"
	defaultPort         = "8080"
)

func main() {
	logger := slog.Default()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	sport := football.New(time.Now().UnixNano())
	// One tick per second, each tick advances the simulated clock by
	// one minute - a full 90-minute match plays out in ~90 seconds.
	ticker := domain.NewRealTicker(1 * time.Second)
	engine := domain.NewMatchEngine(sport, ticker, "match-1")

	ingestionURL := os.Getenv("INGESTION_URL")
	if ingestionURL == "" {
		ingestionURL = defaultIngestionURL
	}
	submitter := simulator.NewHTTPSubmitter(ingestionURL)

	routes := []simulator.ProviderRoute{
		{Encode: providers.EncodeProviderA, Route: "/events/provider-a"},
		{Encode: providers.EncodeProviderB, Route: "/events/provider-b"},
	}
	runner := simulator.NewRunner(engine, routes, submitter, logger)

	go runner.Run(ctx)

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz.Handler())

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

	logger.Info("feed-simulator listening", "port", port, "ingestion_url", ingestionURL)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

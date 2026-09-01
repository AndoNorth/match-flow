package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	simulatorapi "github.com/AndoNorth/match-flow/services/feed-simulator/internal/api"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/healthz"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/manager"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/providers"
)

const (
	readHeaderTimeout   = 5 * time.Second
	shutdownTimeout     = 5 * time.Second
	tickInterval        = 1 * time.Second
	defaultIngestionURL = "http://localhost:8081"
	defaultPort         = "8080"
	// Air's cwd is services/feed-simulator/cmd/feed-simulator (see
	// .air.toml), so this default has to climb back to the service
	// root's templates/ directory from there - TEMPLATES_DIR overrides
	// it for any other invocation context (e.g. running the built
	// binary from the repo root).
	defaultTemplatesDir = "../../templates"
)

func main() {
	logger := slog.Default()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ingestionURL := os.Getenv("INGESTION_URL")
	if ingestionURL == "" {
		ingestionURL = defaultIngestionURL
	}
	submitter := simulator.NewHTTPSubmitter(ingestionURL)

	routes := []simulator.ProviderRoute{
		{Encode: providers.EncodeProviderA, Route: "/events/provider-a"},
		{Encode: providers.EncodeProviderB, Route: "/events/provider-b"},
	}

	templatesDir := os.Getenv("TEMPLATES_DIR")
	if templatesDir == "" {
		templatesDir = defaultTemplatesDir
	}
	absTemplatesDir, err := filepath.Abs(templatesDir)
	if err != nil {
		logger.Error("resolve templates dir failed", "error", err)
		os.Exit(1)
	}

	// ctx (tied to process signals, not any one HTTP request) is the
	// base every spawned match's lifetime derives from - see
	// manager.New's doc comment for why that distinction matters.
	m := manager.New(ctx, routes, submitter, logger, absTemplatesDir, tickInterval)

	// Feeds live matches continuously by default, exactly as before
	// this endpoint set existed - POST /control/stop turns it off,
	// POST /control/start (optionally with a different template) turns
	// it back on.
	if _, err := m.Start(""); err != nil {
		logger.Error("initial match start failed", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz.Handler())

	humaAPI := humago.New(mux, huma.DefaultConfig("Feed Simulator", "0.1.0"))
	simulatorapi.Register(humaAPI, m)

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

	logger.Info("feed-simulator listening",
		"port", port, "ingestion_url", ingestionURL, "templates_dir", absTemplatesDir)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

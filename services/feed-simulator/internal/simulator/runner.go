package simulator

import (
	"context"
	"log/slog"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/providers"
)

// ProviderRoute pairs one provider's encoder with the Ingestion Service
// route that decodes that same shape.
type ProviderRoute struct {
	Encode providers.Encoder
	Route  string
}

// Runner wires a MatchEngine's event stream through alternating
// provider encoders, logs each payload, and submits it to Ingestion
// Service over HTTP. This is where the actual generator/encoder/
// logging/submission behavior lives - main.go only constructs a Runner
// and calls Run.
type Runner struct {
	engine       *domain.MatchEngine
	logger       *slog.Logger
	submitter    Submitter
	routes       []ProviderRoute
	emittedCount int
}

// NewRunner builds a Runner. submitter may be nil - Run then logs each
// event without submitting it anywhere (useful for tests that only
// care about encoding/logging behavior).
func NewRunner(engine *domain.MatchEngine, routes []ProviderRoute, submitter Submitter, logger *slog.Logger) *Runner {
	return &Runner{engine: engine, routes: routes, submitter: submitter, logger: logger}
}

// Run blocks until the engine's event channel closes (the Sport
// reported done, or ctx was canceled), then logs one match_complete
// line so "the match finished" is never ambiguous with "the generator
// stalled" from the logs alone.
func (r *Runner) Run(ctx context.Context) {
	events := r.engine.Run(ctx)

	for event := range events {
		route := r.routes[r.emittedCount%len(r.routes)]
		r.emittedCount++

		payload, err := route.Encode(event)
		if err != nil {
			r.logger.Error("encode event failed", "error", err)
			continue
		}

		r.logger.Info("event", "provider_payload", string(payload))

		if r.submitter != nil {
			if err := r.submitter.Submit(ctx, route.Route, payload); err != nil {
				r.logger.Error("submit event failed", "error", err, "route", route.Route)
			}
		}
	}

	if ctx.Err() != nil {
		r.logger.Info("generator canceled")
		return
	}
	r.logger.Info("match_complete")
}

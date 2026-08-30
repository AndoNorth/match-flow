package simulator

import (
	"context"
	"log/slog"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/providers"
)

// Runner wires a MatchEngine's event stream through alternating
// provider encoders and into a logger. This is where the actual
// generator/encoder/logging behavior lives - main.go only constructs a
// Runner and calls Run.
type Runner struct {
	engine       *domain.MatchEngine
	logger       *slog.Logger
	encoders     []providers.Encoder
	emittedCount int
}

func NewRunner(engine *domain.MatchEngine, encoders []providers.Encoder, logger *slog.Logger) *Runner {
	return &Runner{engine: engine, encoders: encoders, logger: logger}
}

// Run blocks until the engine's event channel closes (the Sport
// reported done, or ctx was canceled), then logs one match_complete
// line so "the match finished" is never ambiguous with "the generator
// stalled" from the logs alone.
func (r *Runner) Run(ctx context.Context) {
	events := r.engine.Run(ctx)

	for event := range events {
		encoder := r.encoders[r.emittedCount%len(r.encoders)]
		r.emittedCount++

		payload, err := encoder(event)
		if err != nil {
			r.logger.Error("encode event failed", "error", err)
			continue
		}

		r.logger.Info("event", "provider_payload", string(payload))
	}

	if ctx.Err() != nil {
		r.logger.Info("generator canceled")
		return
	}
	r.logger.Info("match_complete")
}

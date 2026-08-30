// Package api registers Ingestion Service's two Huma routes - one per
// Feed Simulator provider wire shape - and converts each into
// internal/normalize's provider-agnostic Input before publishing.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/normalize"
)

// eventPublisher is the subset of eventbus.Publisher the handlers need -
// letting tests inject a fake instead of a real Redis connection.
type eventPublisher interface {
	Publish(ctx context.Context, event normalize.CanonicalEvent) error
}

// acceptedOutput is returned by both routes on success.
type acceptedOutput struct {
	Body struct {
		Status string `json:"status" example:"accepted" doc:"Always \"accepted\" on success."`
	}
}

// Register wires both provider routes onto api.
func Register(api huma.API, pub eventPublisher) {
	huma.Register(api, huma.Operation{
		OperationID: "submit-provider-a-event",
		Method:      http.MethodPost,
		Path:        "/events/provider-a",
		Summary:     "Submit a ProviderA-shaped match event",
	}, providerAHandler(pub))

	huma.Register(api, huma.Operation{
		OperationID: "submit-provider-b-event",
		Method:      http.MethodPost,
		Path:        "/events/provider-b",
		Summary:     "Submit a ProviderB-shaped match event",
	}, providerBHandler(pub))
}

// --- ProviderA: nested/abbreviated shape ---

type providerAInput struct {
	Body struct {
		Data struct {
			MatchID  string         `json:"mid" required:"true" minLength:"1"`
			Sequence int            `json:"seq" required:"true" minimum:"0"`
			Type     string         `json:"typ" required:"true" minLength:"1"`
			TS       int64          `json:"ts" required:"true"`
			Payload  map[string]any `json:"pl,omitempty"`
		} `json:"data" required:"true"`
	}
}

func providerAHandler(pub eventPublisher) func(context.Context, *providerAInput) (*acceptedOutput, error) {
	return func(ctx context.Context, in *providerAInput) (*acceptedOutput, error) {
		input := normalize.Input{
			MatchID:   in.Body.Data.MatchID,
			Sequence:  in.Body.Data.Sequence,
			Type:      in.Body.Data.Type,
			Timestamp: time.Unix(in.Body.Data.TS, 0),
			Payload:   in.Body.Data.Payload,
		}
		return publish(ctx, pub, input, "provider-a")
	}
}

// --- ProviderB: flat shape ---

type providerBInput struct {
	Body struct {
		MatchID    string         `json:"match_id" required:"true" minLength:"1"`
		Sequence   int            `json:"sequence" required:"true" minimum:"0"`
		EventType  string         `json:"event_type" required:"true" minLength:"1"`
		OccurredAt string         `json:"occurred_at" required:"true"`
		Details    map[string]any `json:"details,omitempty"`
	}
}

func providerBHandler(pub eventPublisher) func(context.Context, *providerBInput) (*acceptedOutput, error) {
	return func(ctx context.Context, in *providerBInput) (*acceptedOutput, error) {
		ts, err := time.Parse(time.RFC3339, in.Body.OccurredAt)
		if err != nil {
			return nil, huma.Error400BadRequest("occurred_at must be RFC3339", err)
		}
		input := normalize.Input{
			MatchID:   in.Body.MatchID,
			Sequence:  in.Body.Sequence,
			Type:      in.Body.EventType,
			Timestamp: ts,
			Payload:   in.Body.Details,
		}
		return publish(ctx, pub, input, "provider-b")
	}
}

func publish(
	ctx context.Context,
	pub eventPublisher,
	input normalize.Input,
	provider string,
) (*acceptedOutput, error) {
	event := normalize.Build(input, provider, time.Now())
	if err := pub.Publish(ctx, event); err != nil {
		return nil, huma.Error500InternalServerError("failed to publish event", err)
	}
	out := &acceptedOutput{}
	out.Body.Status = "accepted"
	return out, nil
}

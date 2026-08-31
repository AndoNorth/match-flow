// Package api registers the Gateway's three read-only Huma routes,
// mirroring Match Service's own routes but backed by a gRPC client
// through internal/resolver instead of a direct store.
package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/resolver"
)

// client is the subset of matchclient.Client the routes need - letting
// tests inject a fake instead of a real network call.
type client interface {
	ListMatches(ctx context.Context, status string) (*matchservicev1.ListMatchesResponse, error)
	GetMatch(ctx context.Context, matchID string) (*matchservicev1.Match, error)
	ListMatchEvents(ctx context.Context, matchID string) (*matchservicev1.ListMatchEventsResponse, error)
}

type matchOutput struct {
	Body resolver.MatchBody
}

type matchListOutput struct {
	Body struct {
		Matches []resolver.MatchBody `json:"matches"`
	}
}

type eventListOutput struct {
	Body struct {
		Events []resolver.EventBody `json:"events"`
	}
}

type listMatchesInput struct {
	Status string `query:"status"`
}

type matchIDInput struct {
	MatchID string `path:"id"`
}

// Register wires all three routes onto api.
func Register(api huma.API, c client) {
	huma.Register(api, huma.Operation{
		OperationID: "list-matches",
		Method:      http.MethodGet,
		Path:        "/matches",
		Summary:     "List matches, optionally filtered by status",
	}, func(ctx context.Context, in *listMatchesInput) (*matchListOutput, error) {
		resp, err := c.ListMatches(ctx, in.Status)
		if err != nil {
			return nil, resolver.HTTPError(err)
		}
		out := &matchListOutput{}
		out.Body.Matches = resolveMatches(resp.GetMatches())
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-match",
		Method:      http.MethodGet,
		Path:        "/matches/{id}",
		Summary:     "Get a single match's current state",
	}, func(ctx context.Context, in *matchIDInput) (*matchOutput, error) {
		m, err := c.GetMatch(ctx, in.MatchID)
		if err != nil {
			return nil, resolver.HTTPError(err)
		}
		return &matchOutput{Body: resolver.Match(m)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-match-events",
		Method:      http.MethodGet,
		Path:        "/matches/{id}/events",
		Summary:     "List a match's event timeline, ordered by sequence",
	}, func(ctx context.Context, in *matchIDInput) (*eventListOutput, error) {
		resp, err := c.ListMatchEvents(ctx, in.MatchID)
		if err != nil {
			return nil, resolver.HTTPError(err)
		}
		out := &eventListOutput{}
		out.Body.Events = resolver.Events(resp.GetEvents())
		return out, nil
	})
}

func resolveMatches(matches []*matchservicev1.Match) []resolver.MatchBody {
	out := make([]resolver.MatchBody, 0, len(matches))
	for _, m := range matches {
		out = append(out, resolver.Match(m))
	}
	return out
}

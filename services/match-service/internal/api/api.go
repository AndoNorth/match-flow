// services/match-service/internal/api/api.go

// Package api registers Match Service's three read-only Huma routes.
// Nothing here creates or mutates a match - only internal/eventstream
// does that, via the Redis event stream.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

// reader is the subset of matchstate.Store the routes need - letting
// tests inject a fake instead of a real Postgres connection.
type reader interface {
	ListMatches(ctx context.Context, status string) ([]matchstate.MatchRecord, error)
	GetMatch(ctx context.Context, matchID string) (matchstate.MatchRecord, error)
	ListEvents(ctx context.Context, matchID string) ([]matchstate.EventRecord, error)
}

type matchBody struct {
	MatchID   string `json:"match_id"`
	Sport     string `json:"sport"`
	Status    string `json:"status"`
	HomeScore int    `json:"home_score"`
	AwayScore int    `json:"away_score"`
	ClockMins int    `json:"clock_mins"`
}

type matchOutput struct {
	Body matchBody
}

type matchListOutput struct {
	Body struct {
		Matches []matchBody `json:"matches"`
	}
}

type eventBody struct {
	Payload  map[string]any `json:"payload"`
	Type     string         `json:"type"`
	Sequence int            `json:"sequence"`
}

type eventListOutput struct {
	Body struct {
		Events []eventBody `json:"events"`
	}
}

type listMatchesInput struct {
	Status string `query:"status"`
}

type matchIDInput struct {
	MatchID string `path:"id"`
}

// Register wires all three routes onto api.
func Register(api huma.API, store reader) {
	huma.Register(api, huma.Operation{
		OperationID: "list-matches",
		Method:      http.MethodGet,
		Path:        "/matches",
		Summary:     "List matches, optionally filtered by status",
	}, func(ctx context.Context, in *listMatchesInput) (*matchListOutput, error) {
		records, err := store.ListMatches(ctx, in.Status)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list matches", err)
		}
		out := &matchListOutput{}
		out.Body.Matches = make([]matchBody, 0, len(records))
		for _, r := range records {
			out.Body.Matches = append(out.Body.Matches, toMatchBody(r))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-match",
		Method:      http.MethodGet,
		Path:        "/matches/{id}",
		Summary:     "Get a single match's current state",
	}, func(ctx context.Context, in *matchIDInput) (*matchOutput, error) {
		record, err := store.GetMatch(ctx, in.MatchID)
		if errors.Is(err, matchstate.ErrNotFound) {
			return nil, huma.Error404NotFound("no such match")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to get match", err)
		}
		return &matchOutput{Body: toMatchBody(record)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-match-events",
		Method:      http.MethodGet,
		Path:        "/matches/{id}/events",
		Summary:     "List a match's event timeline, ordered by sequence",
	}, func(ctx context.Context, in *matchIDInput) (*eventListOutput, error) {
		records, err := store.ListEvents(ctx, in.MatchID)
		if errors.Is(err, matchstate.ErrNotFound) {
			return nil, huma.Error404NotFound("no such match")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list events", err)
		}
		out := &eventListOutput{}
		out.Body.Events = make([]eventBody, 0, len(records))
		for _, r := range records {
			out.Body.Events = append(out.Body.Events, eventBody{
				Type: r.Type, Payload: r.Payload, Sequence: r.Sequence,
			})
		}
		return out, nil
	})
}

func toMatchBody(r matchstate.MatchRecord) matchBody {
	return matchBody{
		MatchID: r.MatchID, Sport: r.Sport, Status: r.Status,
		HomeScore: r.HomeScore, AwayScore: r.AwayScore, ClockMins: r.ClockMins,
	}
}

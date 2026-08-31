// Package resolver is the only place in the Gateway that knows both
// the REST/JSON shapes route handlers use and Match Service's
// protobuf message shapes - route handlers and the SSE handler never
// import generated protobuf types directly, they call this package.
package resolver

import (
	"connectrpc.com/connect"
	"github.com/danielgtaylor/huma/v2"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
)

// MatchBody is a match's current state, as the Gateway's REST API and
// SSE snapshot both return it.
type MatchBody struct {
	MatchID   string `json:"match_id"`
	Sport     string `json:"sport"`
	Status    string `json:"status"`
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeScore int    `json:"home_score"`
	AwayScore int    `json:"away_score"`
	ClockMins int    `json:"clock_mins"`
}

// EventBody is one event in a match's timeline, as the Gateway's REST
// API and SSE updates both return it. MatchID is empty (omitted) for
// the REST path (already scoped by the request's match ID in the
// URL) and populated for the SSE path, where an unscoped subscriber
// needs it to attribute an update to a match.
type EventBody struct {
	Payload  map[string]any `json:"payload"`
	Type     string         `json:"type"`
	MatchID  string         `json:"match_id,omitempty"`
	Sequence int            `json:"sequence"`
}

// Match converts a protobuf Match to the REST/SSE shape.
func Match(m *matchservicev1.Match) MatchBody {
	return MatchBody{
		MatchID:   m.GetMatchId(),
		Sport:     m.GetSport(),
		Status:    m.GetStatus(),
		HomeTeam:  m.GetHomeTeam(),
		AwayTeam:  m.GetAwayTeam(),
		HomeScore: int(m.GetHomeScore()),
		AwayScore: int(m.GetAwayScore()),
		ClockMins: int(m.GetClockMins()),
	}
}

// Events converts a slice of protobuf MatchEvents to the REST/SSE
// shape, always returning a non-nil (possibly empty) slice.
func Events(events []*matchservicev1.MatchEvent) []EventBody {
	out := make([]EventBody, 0, len(events))
	for _, e := range events {
		var payload map[string]any
		if e.GetPayload() != nil {
			payload = e.GetPayload().AsMap()
		}
		out = append(out, EventBody{Type: e.GetType(), Sequence: int(e.GetSequence()), Payload: payload})
	}
	return out
}

// HTTPError maps a gRPC error (from matchclient) to the HTTP status
// the Gateway returns - the one place this mapping happens, so no
// route hand-rolls its own.
func HTTPError(err error) error {
	switch connect.CodeOf(err) {
	case connect.CodeNotFound:
		return huma.Error404NotFound("no such match", err)
	case connect.CodeInvalidArgument:
		return huma.Error400BadRequest("invalid request", err)
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded:
		return huma.Error503ServiceUnavailable("match service unavailable", err)
	default:
		return huma.Error500InternalServerError("unexpected error", err)
	}
}

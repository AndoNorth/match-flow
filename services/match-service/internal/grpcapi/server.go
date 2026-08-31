// Package grpcapi implements Match Service's gRPC (connect-go) reads,
// wrapping the same matchstate.Store internal/api's REST routes use -
// no new data-access code, just a second protocol over the same reads.
package grpcapi

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

// reader is the subset of matchstate.Store the RPCs need - same shape
// as internal/api's reader interface, letting tests inject a fake.
type reader interface {
	ListMatches(ctx context.Context, status string) ([]matchstate.MatchRecord, error)
	GetMatch(ctx context.Context, matchID string) (matchstate.MatchRecord, error)
	ListEvents(ctx context.Context, matchID string) ([]matchstate.EventRecord, error)
}

// Server implements matchservicev1connect.MatchServiceHandler.
type Server struct {
	store reader
}

// NewServer builds a Server backed by store.
func NewServer(store reader) *Server {
	return &Server{store: store}
}

// ListMatches lists every match, or only those with the given status.
func (s *Server) ListMatches(
	ctx context.Context,
	req *connect.Request[matchservicev1.ListMatchesRequest],
) (*connect.Response[matchservicev1.ListMatchesResponse], error) {
	records, err := s.store.ListMatches(ctx, req.Msg.GetStatus())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	matches := make([]*matchservicev1.Match, 0, len(records))
	for _, r := range records {
		matches = append(matches, toProtoMatch(r))
	}
	return connect.NewResponse(&matchservicev1.ListMatchesResponse{Matches: matches}), nil
}

// GetMatch returns a single match's current state.
func (s *Server) GetMatch(
	ctx context.Context,
	req *connect.Request[matchservicev1.GetMatchRequest],
) (*connect.Response[matchservicev1.Match], error) {
	record, err := s.store.GetMatch(ctx, req.Msg.GetMatchId())
	if errors.Is(err, matchstate.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(toProtoMatch(record)), nil
}

// ListMatchEvents returns a match's event timeline, ordered by sequence.
func (s *Server) ListMatchEvents(
	ctx context.Context,
	req *connect.Request[matchservicev1.ListMatchEventsRequest],
) (*connect.Response[matchservicev1.ListMatchEventsResponse], error) {
	records, err := s.store.ListEvents(ctx, req.Msg.GetMatchId())
	if errors.Is(err, matchstate.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	events := make([]*matchservicev1.MatchEvent, 0, len(records))
	for _, r := range records {
		payload, err := structpb.NewStruct(r.Payload)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		events = append(events, &matchservicev1.MatchEvent{
			Type:     r.Type,
			Sequence: int32(r.Sequence), //nolint:gosec // sequence is always small and non-negative
			Payload:  payload,
		})
	}
	return connect.NewResponse(&matchservicev1.ListMatchEventsResponse{Events: events}), nil
}

func toProtoMatch(r matchstate.MatchRecord) *matchservicev1.Match {
	return &matchservicev1.Match{
		MatchId:   r.MatchID,
		Sport:     r.Sport,
		Status:    r.Status,
		HomeTeam:  r.HomeTeam,
		AwayTeam:  r.AwayTeam,
		HomeScore: int32(r.HomeScore), //nolint:gosec // score is always small and non-negative
		AwayScore: int32(r.AwayScore), //nolint:gosec // score is always small and non-negative
		ClockMins: int32(r.ClockMins), //nolint:gosec // clock minutes is always small and non-negative
		CreatedAt: timestamppb.New(r.CreatedAt),
	}
}

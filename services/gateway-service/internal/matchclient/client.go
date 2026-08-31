// Package matchclient wraps the generated connect-go client for Match
// Service's gRPC API into a plain Go interface - internal/resolver and
// internal/api depend on this, not on connect-go or protobuf types
// directly beyond what a Client method returns.
package matchclient

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1/matchservicev1connect"
)

// Client calls Match Service's gRPC API.
type Client struct {
	inner matchservicev1connect.MatchServiceClient
}

// New builds a Client targeting baseURL (e.g. "http://localhost:8082").
func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{inner: matchservicev1connect.NewMatchServiceClient(httpClient, baseURL)}
}

// ListMatches lists every match, or only those with the given status.
func (c *Client) ListMatches(ctx context.Context, status string) (*matchservicev1.ListMatchesResponse, error) {
	resp, err := c.inner.ListMatches(ctx, connect.NewRequest(&matchservicev1.ListMatchesRequest{Status: status}))
	if err != nil {
		return nil, err //nolint:wrapcheck // connect.Error already carries a code; resolver.HTTPError inspects it directly
	}
	return resp.Msg, nil
}

// GetMatch returns a single match's current state.
func (c *Client) GetMatch(ctx context.Context, matchID string) (*matchservicev1.Match, error) {
	resp, err := c.inner.GetMatch(ctx, connect.NewRequest(&matchservicev1.GetMatchRequest{MatchId: matchID}))
	if err != nil {
		return nil, err //nolint:wrapcheck // connect.Error already carries a code; resolver.HTTPError inspects it directly
	}
	return resp.Msg, nil
}

// ListMatchEvents returns a match's event timeline, ordered by sequence.
func (c *Client) ListMatchEvents(ctx context.Context, matchID string) (*matchservicev1.ListMatchEventsResponse, error) {
	resp, err := c.inner.ListMatchEvents(
		ctx,
		connect.NewRequest(&matchservicev1.ListMatchEventsRequest{MatchId: matchID}),
	)
	if err != nil {
		return nil, err //nolint:wrapcheck // connect.Error already carries a code; resolver.HTTPError inspects it directly
	}
	return resp.Msg, nil
}

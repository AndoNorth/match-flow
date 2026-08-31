package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/resolver"
)

// matchGetter is the subset of matchclient.Client the SSE handler
// needs for its initial snapshot.
type matchGetter interface {
	GetMatch(ctx context.Context, matchID string) (*matchservicev1.Match, error)
	ListMatches(ctx context.Context, status string) (*matchservicev1.ListMatchesResponse, error)
}

// Handler serves GET /events: registers the caller with registry for
// the requested ?match_id= (or every match, if none given), writes an
// initial "snapshot" frame via client, then streams every subsequent
// broadcast for that match as an "update" frame until the client
// disconnects.
func Handler(registry *Registry, client matchGetter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		matchID := r.URL.Query().Get("match_id")

		ch, unregister := registry.Register(matchID)
		defer unregister()

		if err := writeSnapshot(r.Context(), w, client, matchID); err != nil {
			status := http.StatusInternalServerError
			var statusErr huma.StatusError
			if errors.As(resolver.HTTPError(err), &statusErr) {
				status = statusErr.GetStatus()
			}
			http.Error(w, "failed to load match snapshot", status)
			return
		}
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case payload, ok := <-ch:
				if !ok {
					return
				}
				if _, err := fmt.Fprintf(w, "event: update\ndata: %s\n\n", payload); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}

func writeSnapshot(ctx context.Context, w http.ResponseWriter, client matchGetter, matchID string) error {
	var body any
	if matchID == "" {
		resp, err := client.ListMatches(ctx, "")
		if err != nil {
			return err //nolint:wrapcheck // caller writes a plain HTTP error, no further wrapping needed
		}
		matches := make([]resolver.MatchBody, 0, len(resp.GetMatches()))
		for _, m := range resp.GetMatches() {
			matches = append(matches, resolver.Match(m))
		}
		body = struct {
			Matches []resolver.MatchBody `json:"matches"`
		}{Matches: matches}
	} else {
		m, err := client.GetMatch(ctx, matchID)
		if err != nil {
			return err //nolint:wrapcheck // caller writes a plain HTTP error, no further wrapping needed
		}
		body = resolver.Match(m)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return err //nolint:wrapcheck // caller writes a plain HTTP error, no further wrapping needed
	}
	_, err = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", encoded)
	return err //nolint:wrapcheck // caller writes a plain HTTP error, no further wrapping needed
}

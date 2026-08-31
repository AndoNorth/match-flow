// services/match-service/internal/matchstate/persist.go
package matchstate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store applies WorkItems to Postgres and serves the read queries in
// query.go. sport is written once, at match creation, from
// MATCH_SERVICE_DEFAULT_SPORT - see NewStore.
type Store struct {
	pool  *pgxpool.Pool
	sport string
}

// NewStore wraps an already-connected pool. sport is the value every
// newly-seen match is created with (see Apply) - the only place a
// match's sport is ever set.
func NewStore(pool *pgxpool.Pool, sport string) *Store {
	return &Store{pool: pool, sport: sport}
}

// Apply loads match's current state (creating the row with s.sport if
// this is the first event seen for it), runs Reduce, and - unless
// Reduce reports the event was a stale/duplicate replay - writes the
// new state and a match_events row, all inside one transaction.
func (s *Store) Apply(ctx context.Context, item WorkItem) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful Commit

	var state State
	err = tx.QueryRow(ctx, `
		INSERT INTO matches (match_id, sport)
		VALUES ($1, $2)
		ON CONFLICT (match_id) DO UPDATE SET match_id = matches.match_id
		RETURNING status, home_score, away_score, clock_mins, last_sequence
	`, item.Event.MatchID, s.sport).Scan(
		&state.Status, &state.HomeScore, &state.AwayScore, &state.ClockMins, &state.LastSequence,
	)
	if err != nil {
		return fmt.Errorf("load or create match state: %w", err)
	}

	next, ok := Reduce(state, item.Rule, item.Event)
	if !ok {
		return nil // stale/duplicate Sequence - transaction rolls back, nothing written
	}

	if _, err := tx.Exec(ctx, `
		UPDATE matches
		SET status = $1, home_score = $2, away_score = $3, clock_mins = $4,
		    last_sequence = $5, updated_at = now()
		WHERE match_id = $6
	`, next.Status, next.HomeScore, next.AwayScore, next.ClockMins, next.LastSequence, item.Event.MatchID); err != nil {
		return fmt.Errorf("update match state: %w", err)
	}

	payload := item.Event.Payload
	if payload == nil {
		payload = map[string]any{} // payload is NOT NULL; a nil map would encode as SQL NULL
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO match_events (match_id, sequence, type, payload, occurred_at, ingested_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, item.Event.MatchID, item.Event.Sequence, item.Event.Type, payload,
		item.Event.OccurredAt, item.Event.IngestedAt); err != nil {
		return fmt.Errorf("insert match event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

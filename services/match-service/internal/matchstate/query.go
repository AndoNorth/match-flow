// services/match-service/internal/matchstate/query.go
package matchstate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned by GetMatch and ListEvents for an unknown
// MatchID - internal/api maps it to a 404.
var ErrNotFound = errors.New("match not found")

// MatchRecord is a match's current state, as read from the matches
// table.
type MatchRecord struct {
	MatchID   string
	Sport     string
	Status    string
	HomeScore int
	AwayScore int
	ClockMins int
}

// EventRecord is one row from a match's event timeline.
type EventRecord struct {
	OccurredAt time.Time
	Payload    map[string]any
	Type       string
	Sequence   int
}

// ListMatches returns every match, or only those with the given
// status if it's non-empty.
func (s *Store) ListMatches(ctx context.Context, status string) ([]MatchRecord, error) {
	query := `SELECT match_id, sport, status, home_score, away_score, clock_mins FROM matches`
	args := []any{}
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY match_id`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query matches: %w", err)
	}
	defer rows.Close()

	var records []MatchRecord
	for rows.Next() {
		var r MatchRecord
		if err := rows.Scan(&r.MatchID, &r.Sport, &r.Status, &r.HomeScore, &r.AwayScore, &r.ClockMins); err != nil {
			return nil, fmt.Errorf("scan match row: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate match rows: %w", err)
	}
	return records, nil
}

// GetMatch returns ErrNotFound if matchID doesn't exist.
func (s *Store) GetMatch(ctx context.Context, matchID string) (MatchRecord, error) {
	var r MatchRecord
	err := s.pool.QueryRow(ctx, `
		SELECT match_id, sport, status, home_score, away_score, clock_mins
		FROM matches WHERE match_id = $1
	`, matchID).Scan(&r.MatchID, &r.Sport, &r.Status, &r.HomeScore, &r.AwayScore, &r.ClockMins)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchRecord{}, ErrNotFound
	}
	if err != nil {
		return MatchRecord{}, fmt.Errorf("query match: %w", err)
	}
	return r, nil
}

// ListEvents returns ErrNotFound if matchID doesn't exist (checked
// before querying match_events, so an unknown match and a match with
// zero events - which shouldn't happen in practice, since a match row
// is only ever created by the same transaction that inserts its first
// event - are distinguishable).
func (s *Store) ListEvents(ctx context.Context, matchID string) ([]EventRecord, error) {
	if _, err := s.GetMatch(ctx, matchID); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT type, payload, occurred_at, sequence
		FROM match_events WHERE match_id = $1
		ORDER BY sequence
	`, matchID)
	if err != nil {
		return nil, fmt.Errorf("query match events: %w", err)
	}
	defer rows.Close()

	var records []EventRecord
	for rows.Next() {
		var r EventRecord
		if err := rows.Scan(&r.Type, &r.Payload, &r.OccurredAt, &r.Sequence); err != nil {
			return nil, fmt.Errorf("scan match event row: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate match event rows: %w", err)
	}
	return records, nil
}

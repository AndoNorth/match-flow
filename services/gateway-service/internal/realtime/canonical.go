package realtime

import "time"

// CanonicalEvent mirrors ingestion-service's
// internal/normalize.CanonicalEvent (and match-service's
// internal/eventstream.CanonicalEvent) field-for-field. Duplicated
// rather than imported - see canonical_contract_test.go for the
// drift guard.
type CanonicalEvent struct {
	Payload    map[string]any `json:"payload"`
	Timestamp  time.Time      `json:"timestamp"`
	IngestedAt time.Time      `json:"ingested_at"`
	MatchID    string         `json:"match_id"`
	Type       string         `json:"type"`
	Provider   string         `json:"provider"`
	Sequence   int            `json:"sequence"`
}

package eventstream

import "time"

// CanonicalEvent mirrors ingestion-service's
// internal/normalize.CanonicalEvent field-for-field. Duplicated rather
// than imported - Go's internal-import rule means match-service can't
// import a package under ingestion-service's internal/ tree even
// though both live in the same module, and the design spec's Non-Goal
// declines to introduce a shared module for it. See
// canonical_contract_test.go for the drift guard.
type CanonicalEvent struct {
	Payload    map[string]any `json:"payload"`
	Timestamp  time.Time      `json:"timestamp"`
	IngestedAt time.Time      `json:"ingested_at"`
	MatchID    string         `json:"match_id"`
	Type       string         `json:"type"`
	Provider   string         `json:"provider"`
	Sequence   int            `json:"sequence"`
}

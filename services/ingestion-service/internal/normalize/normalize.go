// Package normalize builds the canonical event MatchFlow publishes to
// Redis. It performs no validation and no I/O - by the time a value
// reaches Build, Huma's route-level schema validation and timestamp
// conversion have already run.
package normalize

import "time"

// Input is the provider-agnostic shape both route handlers in
// internal/api build after decoding their own provider's wire format
// and converting its timestamp to time.Time.
type Input struct {
	Payload   map[string]any
	Timestamp time.Time
	MatchID   string
	Type      string
	Sequence  int
}

// CanonicalEvent is what gets published to Redis.
type CanonicalEvent struct {
	Payload    map[string]any `json:"payload"`
	Timestamp  time.Time      `json:"timestamp"`
	IngestedAt time.Time      `json:"ingested_at"`
	MatchID    string         `json:"match_id"`
	Type       string         `json:"type"`
	Provider   string         `json:"provider"`
	Sequence   int            `json:"sequence"`
}

// Build attaches provenance to input, producing the canonical event.
func Build(input Input, provider string, ingestedAt time.Time) CanonicalEvent {
	return CanonicalEvent{
		MatchID:    input.MatchID,
		Sequence:   input.Sequence,
		Type:       input.Type,
		Timestamp:  input.Timestamp,
		Payload:    input.Payload,
		Provider:   provider,
		IngestedAt: ingestedAt,
	}
}

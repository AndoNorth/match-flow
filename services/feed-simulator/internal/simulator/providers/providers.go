package providers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
)

// Encoder shapes a canonical DomainEvent into one provider's wire
// format. Two encoders exist so the same real-world event goes out in
// two structurally different payloads - simulating competing odds
// feeds - giving Ingestion Service's future normalization step a real
// gap to close.
type Encoder func(domain.DomainEvent) ([]byte, error)

// EncodeProviderA nests everything under "data" with abbreviated keys.
func EncodeProviderA(e domain.DomainEvent) ([]byte, error) {
	body := map[string]any{
		"data": map[string]any{
			"mid": e.MatchID,
			"seq": e.Sequence,
			"typ": string(e.Type),
			"ts":  e.Timestamp.Unix(),
			"pl":  e.Payload,
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode provider A: %w", err)
	}
	return data, nil
}

func DecodeProviderA(data []byte) (domain.DomainEvent, error) {
	var wrapper struct {
		Data struct {
			MID string         `json:"mid"`
			Seq int            `json:"seq"`
			Typ string         `json:"typ"`
			TS  int64          `json:"ts"`
			PL  map[string]any `json:"pl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return domain.DomainEvent{}, fmt.Errorf("decode provider A: %w", err)
	}
	return domain.DomainEvent{
		MatchID:   wrapper.Data.MID,
		Sequence:  wrapper.Data.Seq,
		Type:      domain.EventType(wrapper.Data.Typ),
		Timestamp: time.Unix(wrapper.Data.TS, 0),
		Payload:   wrapper.Data.PL,
	}, nil
}

// EncodeProviderB is flat with full field names - structurally
// different from ProviderA for the same event.
func EncodeProviderB(e domain.DomainEvent) ([]byte, error) {
	body := map[string]any{
		"match_id":    e.MatchID,
		"sequence":    e.Sequence,
		"event_type":  string(e.Type),
		"occurred_at": e.Timestamp.Format(time.RFC3339),
		"details":     e.Payload,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode provider B: %w", err)
	}
	return data, nil
}

func DecodeProviderB(data []byte) (domain.DomainEvent, error) {
	var wrapper struct {
		MatchID    string         `json:"match_id"`
		Sequence   int            `json:"sequence"`
		EventType  string         `json:"event_type"`
		OccurredAt string         `json:"occurred_at"`
		Details    map[string]any `json:"details"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return domain.DomainEvent{}, fmt.Errorf("decode provider B: %w", err)
	}
	ts, err := time.Parse(time.RFC3339, wrapper.OccurredAt)
	if err != nil {
		return domain.DomainEvent{}, fmt.Errorf("parse provider B timestamp: %w", err)
	}
	return domain.DomainEvent{
		MatchID:   wrapper.MatchID,
		Sequence:  wrapper.Sequence,
		Type:      domain.EventType(wrapper.EventType),
		Timestamp: ts,
		Payload:   wrapper.Details,
	}, nil
}

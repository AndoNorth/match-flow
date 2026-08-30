// Package eventbus publishes canonical events to Redis pub/sub. It has
// no rejection logic of its own - a Publish failure surfaces as a 500
// from the calling handler, since Redis is Ingestion's only output.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/normalize"
)

// Channel is the single Redis pub/sub channel every canonical event is
// published to - a fixed constant, not configurable (see the design
// spec's Config section).
const Channel = "matchflow:events"

// Publisher publishes canonical events to Redis.
type Publisher struct {
	client *redis.Client
}

// NewPublisher wraps an already-connected Redis client.
func NewPublisher(client *redis.Client) *Publisher {
	return &Publisher{client: client}
}

// Publish JSON-encodes event and PUBLISHes it to Channel.
func (p *Publisher) Publish(ctx context.Context, event normalize.CanonicalEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal canonical event: %w", err)
	}
	if err := p.client.Publish(ctx, Channel, data).Err(); err != nil {
		return fmt.Errorf("publish canonical event: %w", err)
	}
	return nil
}

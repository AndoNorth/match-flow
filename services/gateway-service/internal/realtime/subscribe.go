package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"github.com/AndoNorth/match-flow/services/gateway-service/internal/resolver"
)

// Subscribe reads channel from rdb until ctx is cancelled, decoding
// each message as a CanonicalEvent and broadcasting its type/sequence/
// payload (the same shape internal/api's REST routes return) to
// registry, keyed by the event's match ID. A malformed message is
// logged and dropped, matching match-service's eventstream.Decode
// convention - one bad message never brings down the subscription.
func Subscribe(ctx context.Context, rdb *redis.Client, channel string, registry *Registry, logger *slog.Logger) {
	pubsub := rdb.Subscribe(ctx, channel)
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event CanonicalEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				logger.Warn("dropped malformed event", "error", err)
				continue
			}
			payload, err := EventPayload(event)
			if err != nil {
				logger.Warn("dropped unmarshalable event", "error", err, "match_id", event.MatchID)
				continue
			}
			registry.Broadcast(event.MatchID, payload)
		}
	}
}

// EventPayload marshals event into the JSON shape a browser SSE
// client receives for an "update" frame - resolver.EventBody plus
// the event's match ID, so an unscoped (list-wide) subscriber can
// tell which match an update is for.
func EventPayload(event CanonicalEvent) ([]byte, error) {
	payload, err := json.Marshal(resolver.EventBody{
		Type:     event.Type,
		MatchID:  event.MatchID,
		Sequence: event.Sequence,
		Payload:  event.Payload,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal event body: %w", err)
	}
	return payload, nil
}

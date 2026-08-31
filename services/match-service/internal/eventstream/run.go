// services/match-service/internal/eventstream/run.go
package eventstream

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

// Run subscribes to Channel and routes every decoded event to pool
// until ctx is cancelled or the subscription's channel closes, then
// closes pool (draining whatever was already buffered - see
// matchstate.Pool.Close) before returning. Intended to run in its own
// goroutine; the caller waits for Run to return before treating
// shutdown as complete - see cmd/match-service/main.go.
func Run(ctx context.Context, client *redis.Client, pool *matchstate.Pool, logger *slog.Logger) {
	sub := client.Subscribe(ctx, Channel)
	defer func() { _ = sub.Close() }()
	msgs := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			// Close the subscription first so its read goroutine hits
			// pool.ErrClosed and closes msgs - only then do we know
			// ranging over msgs will terminate. Anything already
			// buffered in msgs at that point (redis-go's internal
			// channel holds up to 100 messages, decoupled from ctx -
			// see go-redis PubSub.initMsgChan) is drained and applied
			// before the worker pool is closed, so no already-received
			// event is lost to cancellation.
			_ = sub.Close()
			for msg := range msgs {
				handle(msg.Payload, pool, logger)
			}
			pool.Close()
			return
		case msg, open := <-msgs:
			if !open {
				pool.Close()
				return
			}
			handle(msg.Payload, pool, logger)
		}
	}
}

func handle(payload string, pool *matchstate.Pool, logger *slog.Logger) {
	event, ok := Decode([]byte(payload))
	if !ok {
		logger.Warn("dropping malformed event", "payload", payload)
		return
	}
	item, idx, route := Route(event, pool.NumWorkers())
	if !route {
		return
	}
	pool.Send(idx, item)
}

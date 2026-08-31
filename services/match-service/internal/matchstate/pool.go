// services/match-service/internal/matchstate/pool.go
package matchstate

import (
	"context"
	"log/slog"
	"sync"
)

// channelBuffer is each worker's channel depth. One tuning knob
// (pool size, via NewPool's n) is enough for this service - see the
// design spec's Non-Goal on a separate buffer-depth env var.
const channelBuffer = 32

// Pool is a hash-sharded worker pool: every WorkItem sent to the same
// index (see internal/eventstream.WorkerIndex, which every event for
// one match always resolves to) is applied by the same worker,
// sequentially - what guarantees per-match ordering while different
// matches process in parallel across workers.
type Pool struct {
	chans []chan WorkItem
	wg    sync.WaitGroup
}

// NewPool starts n worker goroutines applying to store and returns
// immediately - the workers run until Close is called.
func NewPool(n int, store *Store, logger *slog.Logger) *Pool {
	p := &Pool{chans: make([]chan WorkItem, n)}
	for i := range p.chans {
		p.chans[i] = make(chan WorkItem, channelBuffer)
		p.wg.Add(1)
		go func(ch <-chan WorkItem) {
			defer p.wg.Done()
			for item := range ch {
				// A background context, not a cancellable one: Close
				// must never abort a transaction already in flight or
				// still buffered - every drained item runs to completion.
				if err := store.Apply(context.Background(), item); err != nil {
					logger.Error("apply failed", "error", err,
						"match_id", item.Event.MatchID, "sequence", item.Event.Sequence)
				}
			}
		}(p.chans[i])
	}
	return p
}

// Send routes item to the worker at idx, blocking (backpressure, not
// drop) if that worker's channel is full. idx must be in
// [0, NumWorkers()) - see internal/eventstream.WorkerIndex.
func (p *Pool) Send(idx int, item WorkItem) {
	p.chans[idx] <- item
}

// NumWorkers reports how many workers this pool has, for callers
// computing a worker index.
func (p *Pool) NumWorkers() int {
	return len(p.chans)
}

// Close stops accepting new work by closing every worker channel, then
// blocks until every worker has drained whatever was already buffered
// and exited. Call once, after nothing will call Send again.
func (p *Pool) Close() {
	for _, ch := range p.chans {
		close(ch)
	}
	p.wg.Wait()
}

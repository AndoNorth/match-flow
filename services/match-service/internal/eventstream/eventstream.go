// Package eventstream is the only package in this service that
// touches Redis or JSON wire format. It decodes matchflow:events
// messages and converts each into a matchstate.WorkItem, looking up
// its Rule via internal/football's registry - matchstate itself never
// imports football or knows an event's original type string.
package eventstream

import (
	"encoding/json"
	"hash/fnv"

	"github.com/AndoNorth/match-flow/services/match-service/internal/football"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

// Channel is the Redis pub/sub channel this service subscribes to -
// must match ingestion-service's internal/eventbus.Channel exactly.
const Channel = "matchflow:events"

const oddsUpdateType = "odds_update"

// Decode parses a raw Redis message into a CanonicalEvent. ok is false
// for malformed JSON - the caller logs and drops it, see run.go.
func Decode(data []byte) (event CanonicalEvent, ok bool) {
	if err := json.Unmarshal(data, &event); err != nil {
		return CanonicalEvent{}, false
	}
	return event, true
}

// Route converts a decoded CanonicalEvent into a matchstate.WorkItem
// and the index of the worker (of numWorkers) it must be sent to.
// ok is false only for odds_update, which this service reserves for a
// future Odds Service and never routes to a worker or stores.
func Route(event CanonicalEvent, numWorkers int) (item matchstate.WorkItem, workerIdx int, ok bool) {
	if event.Type == oddsUpdateType {
		return matchstate.WorkItem{}, 0, false
	}

	item = matchstate.WorkItem{
		Event: matchstate.Event{
			MatchID:    event.MatchID,
			Sequence:   event.Sequence,
			Type:       event.Type,
			Payload:    event.Payload,
			OccurredAt: event.Timestamp,
			IngestedAt: event.IngestedAt,
		},
		Rule: football.Registry[event.Type], // zero value (Unknown) if absent
	}
	return item, WorkerIndex(event.MatchID, numWorkers), true
}

// WorkerIndex hashes matchID (FNV-32a) to a worker index in
// [0, numWorkers). Every event for the same match always resolves to
// the same index - what gives per-match ordering across the worker
// pool in internal/matchstate/pool.go.
func WorkerIndex(matchID string, numWorkers int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(matchID))       // hash.Hash.Write never errors
	idx := h.Sum32() % uint32(numWorkers) //nolint:gosec // numWorkers is always positive
	return int(idx)
}

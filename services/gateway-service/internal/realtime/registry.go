// Package realtime owns the Gateway's one Redis subscription to
// Ingestion Service's event channel and fans each message out to
// every SSE client registered for that match - or every client
// registered for no specific match ("all matches").
package realtime

import "sync/atomic"

// clientBufferSize is how many pending messages a slow client can
// accumulate before Broadcast starts dropping messages for it rather
// than blocking delivery to every other client.
const clientBufferSize = 16

type registryClient struct {
	ch      chan []byte
	matchID string
}

// Registry is a concurrency-safe set of registered SSE clients, keyed
// by an internal id, each holding the match_id it requested.
type Registry struct {
	clients atomic.Pointer[map[int64]*registryClient]
	nextID  atomic.Int64
}

// NewRegistry builds an empty Registry.
func NewRegistry() *Registry {
	r := &Registry{}
	empty := map[int64]*registryClient{}
	r.clients.Store(&empty)
	return r
}

// Register adds a client interested in matchID ("" means every
// match). The returned channel receives every subsequent Broadcast
// call matching matchID until unregister is called.
func (r *Registry) Register(matchID string) (<-chan []byte, func()) {
	id := r.nextID.Add(1)
	client := &registryClient{ch: make(chan []byte, clientBufferSize), matchID: matchID}

	for {
		old := r.clients.Load()
		next := make(map[int64]*registryClient, len(*old)+1)
		for k, v := range *old {
			next[k] = v
		}
		next[id] = client
		if r.clients.CompareAndSwap(old, &next) {
			break
		}
	}

	unregister := func() {
		for {
			old := r.clients.Load()
			if _, ok := (*old)[id]; !ok {
				return
			}
			next := make(map[int64]*registryClient, len(*old)-1)
			for k, v := range *old {
				if k != id {
					next[k] = v
				}
			}
			if r.clients.CompareAndSwap(old, &next) {
				return
			}
		}
	}
	return client.ch, unregister
}

// Broadcast sends payload to every client registered for matchID, and
// to every client registered for no specific match. A client whose
// channel is full has this message dropped for it rather than
// blocking delivery to any other client.
func (r *Registry) Broadcast(matchID string, payload []byte) {
	clients := *r.clients.Load()
	for _, c := range clients {
		if c.matchID != "" && c.matchID != matchID {
			continue
		}
		select {
		case c.ch <- payload:
		default:
		}
	}
}

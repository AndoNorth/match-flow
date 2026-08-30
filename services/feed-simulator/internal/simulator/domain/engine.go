package domain

import (
	"context"
	"time"
)

// MatchEngine is sport-agnostic: it knows nothing about football or any
// other sport. It owns MatchState and advances ClockMins/Sequence every
// tick, asking the injected Sport what happens next.
type MatchEngine struct {
	sport   Sport
	ticker  Ticker
	matchID string
}

func NewMatchEngine(sport Sport, ticker Ticker, matchID string) *MatchEngine {
	return &MatchEngine{sport: sport, ticker: ticker, matchID: matchID}
}

// Run starts the tick loop and returns a channel of emitted events. The
// channel closes when the Sport reports done, or when ctx is canceled.
func (e *MatchEngine) Run(ctx context.Context) <-chan DomainEvent {
	out := make(chan DomainEvent)

	go func() {
		defer close(out)
		defer e.ticker.Stop()

		state := MatchState{MatchID: e.matchID}

		for {
			select {
			case <-ctx.Done():
				return
			case <-e.ticker.Chan():
				state.ClockMins++
				state.Sequence++

				event, hasEvent, done := e.sport.NextEvent(state)
				if hasEvent {
					event.MatchID = state.MatchID
					event.Sequence = state.Sequence
					event.Timestamp = time.Now()
					out <- event
				}
				if done {
					return
				}
			}
		}
	}()

	return out
}

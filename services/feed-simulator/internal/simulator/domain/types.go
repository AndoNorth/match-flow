package domain

import "time"

// EventType is deliberately an open string type with no constants here -
// a sport's own package (e.g. football) declares its event vocabulary.
// Adding a new sport never requires touching this package.
type EventType string

// DomainEvent is the canonical, sport-agnostic shape MatchEngine emits.
// MatchID, Sequence, and Timestamp are filled by MatchEngine, never by
// a Sport implementation - Sport only ever sets Type and Payload.
type DomainEvent struct {
	Timestamp time.Time
	Payload   map[string]any
	MatchID   string
	Type      EventType
	Sequence  int
}

// MatchState is intentionally thin and sport-agnostic: no score, no
// cards, nothing sport-specific. A Sport implementation tracks any
// sport-specific state internally, in its own struct.
type MatchState struct {
	MatchID   string
	ClockMins int
	Sequence  int
}

// Sport advances one tick. hasEvent is true when an event was produced
// this tick (MatchEngine will emit it); false is a quiet tick, no event
// to emit. done is true when the match is over - MatchEngine stops
// ticking after this call regardless of hasEvent. MatchEngine never
// inspects Type to decide when to stop; only done decides that.
type Sport interface {
	NextEvent(state MatchState) (event DomainEvent, hasEvent bool, done bool)
}

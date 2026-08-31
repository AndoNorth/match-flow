package matchstate

import "time"

// Category is what a sport's registry (see internal/football) maps
// each event type to. Reduce branches only on Category, never on the
// original event-type string - Status carries whatever literal value
// a status-changing Category needs, chosen by the registry, not
// compared against here.
type Category int

const (
	Unknown Category = iota
	MatchStart
	PeriodBoundary
	MatchEnd
	ScoreEvent
)

// Rule is what a sport's registry maps each event type to. Status is
// read only for MatchStart/PeriodBoundary/MatchEnd; ScoreEvent and
// Unknown ignore it.
type Rule struct {
	Status   string
	Category Category
}

// Event is the domain event Reduce and Store operate on - built by
// internal/eventstream from the wire-level CanonicalEvent it decodes
// off Redis. Kept separate from that wire type so matchstate has no
// JSON/wire-format concerns of its own.
type Event struct {
	OccurredAt time.Time
	IngestedAt time.Time
	Payload    map[string]any
	MatchID    string
	Type       string
	Sequence   int
}

// State is a match's current derived state - what Reduce transforms
// and what Store persists to the matches table.
type State struct {
	Status       string
	HomeScore    int
	AwayScore    int
	ClockMins    int
	LastSequence int
}

// WorkItem pairs a decoded Event with the Rule its type resolved to -
// carried together on a worker's channel (see pool.go) so a worker
// never has to re-derive Rule, and matchstate never imports a sport
// package to do so.
type WorkItem struct {
	Event Event
	Rule  Rule
}

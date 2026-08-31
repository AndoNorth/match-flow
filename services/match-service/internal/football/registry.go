// Package football is the football sport's event vocabulary - the
// only package in this service that knows what "kickoff" or "goal"
// mean. Adding a second sport means a new sibling package with its
// own Registry, never a change to matchstate or eventstream.
package football

import "github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"

// Registry maps football's event types (matching the vocabulary in
// services/feed-simulator/internal/simulator/football/football.go) to
// the Rule matchstate.Reduce needs. odds_update is deliberately absent
// - internal/eventstream drops it before a Rule lookup ever happens.
// A type absent from this map (including a genuinely unrecognized
// one) resolves to the zero-value Rule{Category: Unknown}, same
// handling as card.
var Registry = map[string]matchstate.Rule{
	"kickoff":   {Category: matchstate.MatchStart, Status: "live"},
	"half_time": {Category: matchstate.PeriodBoundary, Status: "half_time"},
	"full_time": {Category: matchstate.MatchEnd, Status: "full_time"},
	"goal":      {Category: matchstate.ScoreEvent},
	"card":      {Category: matchstate.Unknown},
}

package matchstate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Reduce", func() {
	baseState := matchstate.State{Status: "scheduled", LastSequence: 0}

	It("sets status for MatchStart", func() {
		rule := matchstate.Rule{Category: matchstate.MatchStart, Status: "live"}
		event := matchstate.Event{Sequence: 1}

		next, ok := matchstate.Reduce(baseState, rule, event)

		Expect(ok).To(BeTrue())
		Expect(next.Status).To(Equal("live"))
		Expect(next.LastSequence).To(Equal(1))
	})

	It("sets status for PeriodBoundary", func() {
		rule := matchstate.Rule{Category: matchstate.PeriodBoundary, Status: "half_time"}
		event := matchstate.Event{Sequence: 2}

		next, ok := matchstate.Reduce(baseState, rule, event)

		Expect(ok).To(BeTrue())
		Expect(next.Status).To(Equal("half_time"))
	})

	It("sets status for MatchEnd", func() {
		rule := matchstate.Rule{Category: matchstate.MatchEnd, Status: "full_time"}
		event := matchstate.Event{Sequence: 3}

		next, ok := matchstate.Reduce(baseState, rule, event)

		Expect(ok).To(BeTrue())
		Expect(next.Status).To(Equal("full_time"))
	})

	It("increments the home score and sets the clock for a ScoreEvent", func() {
		rule := matchstate.Rule{Category: matchstate.ScoreEvent}
		event := matchstate.Event{
			Sequence: 4,
			Payload:  map[string]any{"team": "home", "minute": 23},
		}

		next, ok := matchstate.Reduce(baseState, rule, event)

		Expect(ok).To(BeTrue())
		Expect(next.HomeScore).To(Equal(1))
		Expect(next.AwayScore).To(Equal(0))
		Expect(next.ClockMins).To(Equal(23))
	})

	It("increments the away score for a ScoreEvent", func() {
		rule := matchstate.Rule{Category: matchstate.ScoreEvent}
		event := matchstate.Event{
			Sequence: 5,
			Payload:  map[string]any{"team": "away", "minute": 40},
		}

		next, ok := matchstate.Reduce(baseState, rule, event)

		Expect(ok).To(BeTrue())
		Expect(next.AwayScore).To(Equal(1))
	})

	It("handles a JSON-decoded float64 minute the same as an int", func() {
		rule := matchstate.Rule{Category: matchstate.ScoreEvent}
		event := matchstate.Event{
			Sequence: 6,
			Payload:  map[string]any{"team": "home", "minute": float64(55)},
		}

		next, ok := matchstate.Reduce(baseState, rule, event)

		Expect(ok).To(BeTrue())
		Expect(next.ClockMins).To(Equal(55))
	})

	It("causes no state change for Unknown (e.g. card)", func() {
		rule := matchstate.Rule{Category: matchstate.Unknown}
		event := matchstate.Event{
			Sequence: 7,
			Payload:  map[string]any{"team": "home", "minute": 60},
		}
		state := matchstate.State{Status: "live", HomeScore: 1, ClockMins: 23, LastSequence: 6}

		next, ok := matchstate.Reduce(state, rule, event)

		Expect(ok).To(BeTrue())
		Expect(next.Status).To(Equal("live"))
		Expect(next.HomeScore).To(Equal(1))
		Expect(next.ClockMins).To(Equal(23)) // unchanged - card never touches clock_mins
		Expect(next.LastSequence).To(Equal(7))
	})

	It("skips an event whose Sequence is not greater than LastSequence", func() {
		rule := matchstate.Rule{Category: matchstate.ScoreEvent}
		event := matchstate.Event{Sequence: 3, Payload: map[string]any{"team": "home"}}
		state := matchstate.State{HomeScore: 2, LastSequence: 5}

		next, ok := matchstate.Reduce(state, rule, event)

		Expect(ok).To(BeFalse())
		Expect(next).To(Equal(state)) // unchanged
	})
})

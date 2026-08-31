package football_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/match-service/internal/football"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Registry", func() {
	It("maps kickoff to MatchStart/live", func() {
		Expect(football.Registry["kickoff"]).To(Equal(
			matchstate.Rule{Category: matchstate.MatchStart, Status: "live"},
		))
	})

	It("maps half_time to PeriodBoundary/half_time", func() {
		Expect(football.Registry["half_time"]).To(Equal(
			matchstate.Rule{Category: matchstate.PeriodBoundary, Status: "half_time"},
		))
	})

	It("maps full_time to MatchEnd/full_time", func() {
		Expect(football.Registry["full_time"]).To(Equal(
			matchstate.Rule{Category: matchstate.MatchEnd, Status: "full_time"},
		))
	})

	It("maps goal to ScoreEvent", func() {
		Expect(football.Registry["goal"]).To(Equal(
			matchstate.Rule{Category: matchstate.ScoreEvent},
		))
	})

	It("maps card to Unknown", func() {
		Expect(football.Registry["card"]).To(Equal(
			matchstate.Rule{Category: matchstate.Unknown},
		))
	})

	It("resolves an unrecognized type to the zero-value Rule (Unknown)", func() {
		rule := football.Registry["some_future_event_type"]
		Expect(rule).To(Equal(matchstate.Rule{}))
		Expect(rule.Category).To(Equal(matchstate.Unknown))
	})
})

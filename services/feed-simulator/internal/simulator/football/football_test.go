package football_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/football"
)

var _ = Describe("Football", func() {
	It("emits kickoff on the very first tick", func() {
		f := football.New(1)
		event, hasEvent, done := f.NextEvent(domain.MatchState{ClockMins: 1})

		Expect(hasEvent).To(BeTrue())
		Expect(done).To(BeFalse())
		Expect(event.Type).To(Equal(football.EventKickoff))
	})

	It("emits half_time exactly once, at clock minute 45", func() {
		f := football.New(2)
		f.NextEvent(domain.MatchState{ClockMins: 1}) // consume kickoff

		var halfTimeCount int
		for clock := 2; clock <= 44; clock++ {
			event, hasEvent, _ := f.NextEvent(domain.MatchState{ClockMins: clock})
			if hasEvent && event.Type == football.EventHalfTime {
				halfTimeCount++
			}
		}

		event, hasEvent, done := f.NextEvent(domain.MatchState{ClockMins: 45})
		Expect(hasEvent).To(BeTrue())
		Expect(done).To(BeFalse())
		Expect(event.Type).To(Equal(football.EventHalfTime))
		halfTimeCount++

		// half_time never fires again after minute 45
		for clock := 46; clock <= 89; clock++ {
			event, hasEvent, _ := f.NextEvent(domain.MatchState{ClockMins: clock})
			if hasEvent && event.Type == football.EventHalfTime {
				halfTimeCount++
			}
		}

		Expect(halfTimeCount).To(Equal(1))
	})

	It("emits full_time with done=true at clock minute 90, and never marks done before then", func() {
		f := football.New(3)
		f.NextEvent(domain.MatchState{ClockMins: 1}) // consume kickoff

		for clock := 2; clock <= 89; clock++ {
			_, _, done := f.NextEvent(domain.MatchState{ClockMins: clock})
			Expect(done).To(BeFalse(), "must not be done before minute 90")
		}

		event, hasEvent, done := f.NextEvent(domain.MatchState{ClockMins: 90})
		Expect(hasEvent).To(BeTrue())
		Expect(done).To(BeTrue())
		Expect(event.Type).To(Equal(football.EventFullTime))
	})

	It("odds_update payload carries market/selection/price", func() {
		f := football.New(5)
		f.NextEvent(domain.MatchState{ClockMins: 1}) // consume kickoff

		var found bool
		for clock := 2; clock <= 44; clock++ {
			event, hasEvent, _ := f.NextEvent(domain.MatchState{ClockMins: clock})
			if hasEvent && event.Type == football.EventOddsUpdate {
				Expect(event.Payload).To(HaveKey("market"))
				Expect(event.Payload).To(HaveKey("selection"))
				Expect(event.Payload).To(HaveKey("price"))
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "expected at least one odds_update in 44 ticks with seed 5")
	})
})

package football_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/football"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/template"
)

// playOut ticks f from minute 1 through 90 (or until done), collecting
// every emitted event in order.
func playOut(f *football.Football) []domain.DomainEvent {
	var events []domain.DomainEvent
	for clock := 1; clock <= 90; clock++ {
		event, hasEvent, done := f.NextEvent(domain.MatchState{ClockMins: clock})
		if hasEvent {
			events = append(events, event)
		}
		if done {
			break
		}
	}
	return events
}

func countByType(events []domain.DomainEvent, eventType domain.EventType) int {
	count := 0
	for _, e := range events {
		if e.Type == eventType {
			count++
		}
	}
	return count
}

var _ = Describe("Football", func() {
	It("emits kickoff on the very first tick", func() {
		f := football.New(1)
		event, hasEvent, done := f.NextEvent(domain.MatchState{ClockMins: 1})

		Expect(hasEvent).To(BeTrue())
		Expect(done).To(BeFalse())
		Expect(event.Type).To(Equal(football.EventKickoff))
	})

	It("kickoff payload carries two distinct, non-empty team names", func() {
		f := football.New(4)
		event, _, _ := f.NextEvent(domain.MatchState{ClockMins: 1})

		homeTeam, _ := event.Payload["home_team"].(string)
		awayTeam, _ := event.Payload["away_team"].(string)
		Expect(homeTeam).NotTo(BeEmpty())
		Expect(awayTeam).NotTo(BeEmpty())
		Expect(homeTeam).NotTo(Equal(awayTeam))
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

var _ = Describe("NewFromTemplate", func() {
	It("produces the exact goal/card counts a bounded template specifies", func() {
		tmpl := template.Template{
			Name: "high_scoring_chaos", Kind: template.KindBounded,
			HomeGoals:   template.Range{Min: 5, Max: 5},
			AwayGoals:   template.Range{Min: 5, Max: 5},
			YellowCards: template.Range{Min: 2, Max: 2},
			RedCards:    template.Range{Min: 3, Max: 3},
		}
		f, err := football.NewFromTemplate(1, tmpl)
		Expect(err).NotTo(HaveOccurred())

		events := playOut(f)
		Expect(countByType(events, football.EventGoal)).To(Equal(10))
		Expect(countByType(events, football.EventHalfTime)).To(Equal(1))
		Expect(countByType(events, football.EventFullTime)).To(Equal(1))

		var yellow, red int
		for _, e := range events {
			if e.Type != football.EventCard {
				continue
			}
			switch e.Payload["card_type"] {
			case "yellow":
				yellow++
			case "red":
				red++
			}
		}
		Expect(yellow).To(Equal(2))
		Expect(red).To(Equal(3))
	})

	It("produces zero goals/cards for an all-zero bounded template", func() {
		tmpl := template.Template{Name: "goalless_draw", Kind: template.KindBounded}
		f, err := football.NewFromTemplate(1, tmpl)
		Expect(err).NotTo(HaveOccurred())

		events := playOut(f)
		Expect(countByType(events, football.EventGoal)).To(Equal(0))
		Expect(countByType(events, football.EventCard)).To(Equal(0))
		Expect(countByType(events, football.EventHalfTime)).To(Equal(1))
		Expect(countByType(events, football.EventFullTime)).To(Equal(1))
	})

	It("plays a literal template's scripted events in minute order", func() {
		tmpl := template.Template{
			Name: "scripted_demo", Kind: template.KindLiteral,
			Events: []template.ScriptedEvent{
				{Type: "full_time", Minute: 90},
				{Type: "goal", Team: "home", Minute: 10},
				{Type: "half_time", Minute: 45},
			},
		}
		f, err := football.NewFromTemplate(1, tmpl)
		Expect(err).NotTo(HaveOccurred())

		events := playOut(f)
		// kickoff is always first and automatic, then the scripted
		// events in minute order regardless of the order they were
		// declared in the template.
		Expect(events).To(HaveLen(4))
		Expect(events[0].Type).To(Equal(football.EventKickoff))
		Expect(events[1].Type).To(Equal(football.EventGoal))
		Expect(events[2].Type).To(Equal(football.EventHalfTime))
		Expect(events[3].Type).To(Equal(football.EventFullTime))
	})

	It("rejects a template with an unsupported scripted event type", func() {
		tmpl := template.Template{
			Name: "bad", Kind: template.KindLiteral,
			Events: []template.ScriptedEvent{{Type: "own_goal", Minute: 10}},
		}
		_, err := football.NewFromTemplate(1, tmpl)
		Expect(err).To(HaveOccurred())
	})
})

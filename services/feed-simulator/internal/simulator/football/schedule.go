package football

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/template"
)

// randomEventMinute is any minute a randomly-timed goal/card can land
// on - never 0 (kickoff owns it, emitted separately) and never
// halfTimeMinute/fullTimeMinute (the fixed anchors every bounded
// schedule appends below).
const minRandomEventMinute = 1

// maxRandomEventMinute is exclusive of fullTimeMinute itself.
const maxRandomEventMinute = fullTimeMinute - 1

// buildSchedule turns tmpl into a minute-ordered list of events for
// nextScheduledEvent to walk. A literal template's Events convert
// directly; a bounded template rolls a count in each Range and gives
// each rolled event a random minute, then always appends the
// half_time/full_time anchors - matching the default random mode's
// existing "these two always happen" behavior.
func buildSchedule(rng *rand.Rand, tmpl template.Template) ([]scheduledEvent, error) {
	var schedule []scheduledEvent

	switch tmpl.Kind {
	case template.KindLiteral:
		for _, se := range tmpl.Events {
			event, err := scriptedEventToDomain(se)
			if err != nil {
				return nil, err
			}
			schedule = append(schedule, scheduledEvent{minute: se.Minute, event: event})
		}

	case template.KindBounded:
		schedule = append(schedule, randomGoals(rng, "home", tmpl.HomeGoals)...)
		schedule = append(schedule, randomGoals(rng, "away", tmpl.AwayGoals)...)
		schedule = append(schedule, randomCards(rng, "yellow", tmpl.YellowCards)...)
		schedule = append(schedule, randomCards(rng, "red", tmpl.RedCards)...)
		schedule = append(schedule,
			scheduledEvent{minute: halfTimeMinute, event: domain.DomainEvent{Type: EventHalfTime}},
			scheduledEvent{minute: fullTimeMinute, event: domain.DomainEvent{Type: EventFullTime}},
		)

	default:
		return nil, fmt.Errorf("build schedule: unknown template kind %q", tmpl.Kind)
	}

	sort.Slice(schedule, func(i, j int) bool { return schedule[i].minute < schedule[j].minute })
	return schedule, nil
}

func scriptedEventToDomain(se template.ScriptedEvent) (domain.DomainEvent, error) {
	event := domain.DomainEvent{Type: domain.EventType(se.Type)}
	switch domain.EventType(se.Type) {
	case EventGoal:
		event.Payload = map[string]any{"team": se.Team, "minute": se.Minute}
	case EventCard:
		event.Payload = map[string]any{"team": se.Team, "minute": se.Minute, "card_type": se.CardType}
	case EventHalfTime, EventFullTime:
		// no payload
	default:
		return domain.DomainEvent{}, fmt.Errorf("scripted event has unsupported type %q", se.Type)
	}
	return event, nil
}

func randomGoals(rng *rand.Rand, team string, r template.Range) []scheduledEvent {
	count := rollCount(rng, r)
	events := make([]scheduledEvent, 0, count)
	for range count {
		minute := randomEventMinute(rng)
		events = append(events, scheduledEvent{
			minute: minute,
			event:  domain.DomainEvent{Type: EventGoal, Payload: map[string]any{"team": team, "minute": minute}},
		})
	}
	return events
}

func randomCards(rng *rand.Rand, cardType string, r template.Range) []scheduledEvent {
	count := rollCount(rng, r)
	events := make([]scheduledEvent, 0, count)
	for range count {
		minute := randomEventMinute(rng)
		team := "home"
		if rng.Float64() < 0.5 { //nolint:mnd // fair coin flip between home and away
			team = "away"
		}
		events = append(events, scheduledEvent{
			minute: minute,
			event: domain.DomainEvent{
				Type: EventCard,
				Payload: map[string]any{
					"team": team, "minute": minute, "card_type": cardType,
				},
			},
		})
	}
	return events
}

// rollCount picks a count in [r.Min, r.Max] - a fixed range (Min ==
// Max) always returns exactly that count, no randomness involved.
func rollCount(rng *rand.Rand, r template.Range) int {
	if r.Max <= r.Min {
		return r.Min
	}
	return r.Min + rng.Intn(r.Max-r.Min+1)
}

func randomEventMinute(rng *rand.Rand) int {
	return minRandomEventMinute + rng.Intn(maxRandomEventMinute-minRandomEventMinute+1)
}

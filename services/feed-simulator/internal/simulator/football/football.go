package football

import (
	"math/rand"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/template"
)

// Football's event vocabulary - declared here, not in domain, so a
// second sport never has to touch the domain package to add its own.
const (
	EventKickoff    domain.EventType = "kickoff"
	EventGoal       domain.EventType = "goal"
	EventCard       domain.EventType = "card"
	EventOddsUpdate domain.EventType = "odds_update"
	EventHalfTime   domain.EventType = "half_time"
	EventFullTime   domain.EventType = "full_time"
)

const (
	halfTimeMinute = 45
	fullTimeMinute = 90
)

// scheduledEvent is one pre-computed (minute, event) pair - built once
// at construction from a template.Template, then walked minute-by-
// minute by NextEvent. Building a schedule up front (rather than
// rolling dice per tick like the default random mode) is what lets a
// template pin exact or bounded counts: the count is decided once,
// here, not re-rolled every tick.
type scheduledEvent struct {
	event  domain.DomainEvent
	minute int
}

// Football implements domain.Sport. All sport-specific state (whether
// kickoff/half_time have already fired) lives here, never in
// domain.MatchState.
type Football struct {
	rng             *rand.Rand
	homeTeam        string
	awayTeam        string
	schedule        []scheduledEvent // nil = default unbounded random mode
	scheduleIdx     int
	kickoffEmitted  bool
	halfTimeEmitted bool
}

func New(seed int64) *Football {
	//nolint:gosec // seeded RNG for reproducible simulation
	rng := rand.New(rand.NewSource(seed))
	homeTeam, awayTeam := randomTeamNames(rng)
	return &Football{rng: rng, homeTeam: homeTeam, awayTeam: awayTeam}
}

// NewFromTemplate builds a Football whose goal/card events follow
// tmpl instead of the default per-tick random chances - kickoff always
// fires first regardless (team names are generated the same way
// either mode), and every schedule always ends with a full_time entry.
func NewFromTemplate(seed int64, tmpl template.Template) (*Football, error) {
	//nolint:gosec // seeded RNG for reproducible simulation
	rng := rand.New(rand.NewSource(seed))
	homeTeam, awayTeam := randomTeamNames(rng)
	f := &Football{rng: rng, homeTeam: homeTeam, awayTeam: awayTeam}

	schedule, err := buildSchedule(rng, tmpl)
	if err != nil {
		return nil, err
	}
	f.schedule = schedule
	return f, nil
}

func (f *Football) NextEvent(state domain.MatchState) (domain.DomainEvent, bool, bool) {
	switch {
	case !f.kickoffEmitted:
		f.kickoffEmitted = true
		return domain.DomainEvent{
			Type: EventKickoff,
			Payload: map[string]any{
				"home_team": f.homeTeam,
				"away_team": f.awayTeam,
			},
		}, true, false

	case f.schedule != nil:
		return f.nextScheduledEvent(state)

	case state.ClockMins == halfTimeMinute && !f.halfTimeEmitted:
		f.halfTimeEmitted = true
		return domain.DomainEvent{Type: EventHalfTime}, true, false

	case state.ClockMins >= fullTimeMinute:
		return domain.DomainEvent{Type: EventFullTime}, true, true

	case f.rng.Float64() < 0.05: //nolint:mnd // 5% odds_update probability per tick
		return domain.DomainEvent{
			Type: EventOddsUpdate,
			Payload: map[string]any{
				"market":    "match_winner",
				"selection": "home",
				"price":     f.rng.Float64()*3 + 1.1, //nolint:mnd // odds price range
			},
		}, true, false

	case f.rng.Float64() < 0.03: //nolint:mnd // 3% goal probability per tick
		return domain.DomainEvent{
			Type: EventGoal,
			Payload: map[string]any{
				"team":   f.pickTeam(),
				"minute": state.ClockMins,
			},
		}, true, false

	case f.rng.Float64() < 0.01: //nolint:mnd // 1% card probability per tick
		return domain.DomainEvent{
			Type: EventCard,
			Payload: map[string]any{
				"team":   f.pickTeam(),
				"minute": state.ClockMins,
			},
		}, true, false

	default:
		return domain.DomainEvent{}, false, false
	}
}

// nextScheduledEvent walks f.schedule in order, holding back an entry
// until the clock actually reaches its minute - a quiet tick (no event
// due yet) is a normal, expected result here, not an error.
func (f *Football) nextScheduledEvent(state domain.MatchState) (domain.DomainEvent, bool, bool) {
	if f.scheduleIdx >= len(f.schedule) {
		// Every schedule built by buildSchedule ends with full_time,
		// which reports done=true and stops the engine before this
		// branch is ever reached in practice - reaching it anyway
		// (e.g. a hand-built schedule with no full_time) just ends the
		// match quietly rather than panicking on an out-of-range index.
		return domain.DomainEvent{}, false, true
	}

	next := f.schedule[f.scheduleIdx]
	if state.ClockMins < next.minute {
		return domain.DomainEvent{}, false, false
	}

	f.scheduleIdx++
	done := next.event.Type == EventFullTime
	return next.event, true, done
}

func (f *Football) pickTeam() string {
	if f.rng.Float64() < 0.5 { //nolint:mnd // fair coin flip between home and away
		return "home"
	}
	return "away"
}

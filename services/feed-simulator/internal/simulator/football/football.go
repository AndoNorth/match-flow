package football

import (
	"math/rand"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
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

// Football implements domain.Sport. All sport-specific state (whether
// kickoff/half_time have already fired) lives here, never in
// domain.MatchState.
type Football struct {
	rng             *rand.Rand
	kickoffEmitted  bool
	halfTimeEmitted bool
}

func New(seed int64) *Football {
	//nolint:gosec // seeded RNG for reproducible simulation
	return &Football{rng: rand.New(rand.NewSource(seed))}
}

func (f *Football) NextEvent(state domain.MatchState) (domain.DomainEvent, bool, bool) {
	switch {
	case !f.kickoffEmitted:
		f.kickoffEmitted = true
		return domain.DomainEvent{Type: EventKickoff}, true, false

	case state.ClockMins == 45 && !f.halfTimeEmitted: //nolint:mnd // half-time at minute 45
		f.halfTimeEmitted = true
		return domain.DomainEvent{Type: EventHalfTime}, true, false

	case state.ClockMins >= 90: //nolint:mnd // full-time at minute 90
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

	default:
		return domain.DomainEvent{}, false, false
	}
}

func (f *Football) pickTeam() string {
	if f.rng.Float64() < 0.5 { //nolint:mnd // fair coin flip between home and away
		return "home"
	}
	return "away"
}

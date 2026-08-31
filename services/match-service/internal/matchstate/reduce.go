package matchstate

// Reduce computes the next State for a match after applying event
// under rule. ok is false when event.Sequence is not greater than
// state.LastSequence - a duplicate or late/out-of-order replay - in
// which case state is returned unchanged and the caller must not
// persist a match_events row for it.
func Reduce(state State, rule Rule, event Event) (next State, ok bool) {
	if event.Sequence <= state.LastSequence {
		return state, false
	}

	next = state
	next.LastSequence = event.Sequence

	switch rule.Category {
	case MatchStart, PeriodBoundary, MatchEnd:
		next.Status = rule.Status
	case ScoreEvent:
		applyScoreEvent(&next, event.Payload)
	case Unknown:
		// No state change - the event is still recorded by the caller,
		// see internal/matchstate/persist.go.
	}

	return next, true
}

func applyScoreEvent(state *State, payload map[string]any) {
	switch payload["team"] {
	case "home":
		state.HomeScore++
	case "away":
		state.AwayScore++
	}
	if minute, ok := minuteFromPayload(payload); ok {
		state.ClockMins = minute
	}
}

// minuteFromPayload reads payload["minute"], which arrives as int in
// unit tests and as float64 once it's round-tripped through JSON
// (encoding/json decodes all JSON numbers into float64).
func minuteFromPayload(payload map[string]any) (int, bool) {
	switch v := payload["minute"].(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

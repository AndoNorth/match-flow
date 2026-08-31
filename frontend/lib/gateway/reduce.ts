import type { EventBody, MatchBody, MatchStatus } from "./types";

type Rule =
  | { category: "status"; status: MatchStatus }
  | { category: "score" }
  | { category: "unknown" };

// Mirrors services/match-service/internal/football/registry.go's
// event-type-to-rule mapping and
// services/match-service/internal/matchstate/reduce.go's Reduce
// function - the Gateway's update frames carry a raw event, not a
// computed delta, so this logic has to live in the frontend too.
const RULES: Record<string, Rule> = {
  kickoff: { category: "status", status: "live" },
  half_time: { category: "status", status: "half_time" },
  full_time: { category: "status", status: "full_time" },
  goal: { category: "score" },
  card: { category: "unknown" },
};

// The deterministic in-match minute for event types that don't carry a
// payload.minute (kickoff/half_time/full_time only ever fire at these
// points, per football's Sport.NextEvent - see
// services/feed-simulator/internal/simulator/football/football.go).
const FIXED_MINUTE: Record<string, number> = {
  kickoff: 0,
  half_time: 45,
  full_time: 90,
};

// eventMinute returns the in-match minute an event happened at, for
// display - the Gateway's wire format carries no timestamp field, so
// this is the only clock reference a UI has for a timeline row.
export function eventMinute(event: EventBody): number | undefined {
  if (typeof event.payload?.minute === "number") {
    return event.payload.minute;
  }
  return FIXED_MINUTE[event.type];
}

export function reduce(state: MatchBody, event: EventBody): MatchBody {
  const rule = RULES[event.type] ?? { category: "unknown" as const };

  switch (rule.category) {
    case "status":
      return { ...state, status: rule.status };
    case "score":
      return applyScoreEvent(state, event.payload ?? {});
    default:
      return state;
  }
}

function applyScoreEvent(
  state: MatchBody,
  payload: Record<string, unknown>,
): MatchBody {
  const next = { ...state };
  if (payload.team === "home") {
    next.home_score += 1;
  } else if (payload.team === "away") {
    next.away_score += 1;
  }
  if (typeof payload.minute === "number") {
    next.clock_mins = payload.minute;
  }
  return next;
}

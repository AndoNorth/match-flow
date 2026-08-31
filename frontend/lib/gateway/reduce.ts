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

import { describe, expect, it } from "vitest";
import { reduce } from "./reduce";
import type { MatchBody } from "./types";

const base: MatchBody = {
  match_id: "m1",
  sport: "football",
  status: "scheduled",
  home_score: 0,
  away_score: 0,
  clock_mins: 0,
};

describe("reduce", () => {
  it("kickoff sets status to live, no score change", () => {
    const next = reduce(base, { type: "kickoff", sequence: 1, payload: {} });
    expect(next).toEqual({ ...base, status: "live" });
  });

  it("half_time sets status to half_time", () => {
    const next = reduce(
      { ...base, status: "live" },
      { type: "half_time", sequence: 5, payload: {} },
    );
    expect(next.status).toBe("half_time");
  });

  it("full_time sets status to full_time", () => {
    const next = reduce(
      { ...base, status: "live" },
      { type: "full_time", sequence: 10, payload: {} },
    );
    expect(next.status).toBe("full_time");
  });

  it("a home goal increments home_score and sets clock_mins from the payload", () => {
    const live = { ...base, status: "live" as const };
    const next = reduce(live, {
      type: "goal",
      sequence: 3,
      payload: { team: "home", minute: 23 },
    });
    expect(next).toEqual({ ...live, home_score: 1, clock_mins: 23 });
  });

  it("an away goal increments away_score", () => {
    const live = { ...base, status: "live" as const };
    const next = reduce(live, {
      type: "goal",
      sequence: 3,
      payload: { team: "away", minute: 40 },
    });
    expect(next.away_score).toBe(1);
    expect(next.home_score).toBe(0);
  });

  it("a card event changes nothing on the match state", () => {
    const live = { ...base, status: "live" as const };
    const next = reduce(live, {
      type: "card",
      sequence: 4,
      payload: { team: "home", minute: 30 },
    });
    expect(next).toEqual(live);
  });

  it("an unrecognized event type changes nothing", () => {
    const live = { ...base, status: "live" as const };
    const next = reduce(live, {
      type: "something_new",
      sequence: 4,
      payload: {},
    });
    expect(next).toEqual(live);
  });
});

import { describe, expect, it } from "vitest";
import { liveClockMins, reduce } from "./reduce";
import type { MatchBody } from "./types";

const base: MatchBody = {
  match_id: "m1",
  sport: "football",
  status: "scheduled",
  home_team: "Ashford United",
  away_team: "Denbury City",
  home_score: 0,
  away_score: 0,
  clock_mins: 0,
  created_at: "2026-08-31T12:00:00Z",
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

describe("liveClockMins", () => {
  const kickoffAt = new Date(base.created_at).getTime();

  it("stays at 0 for a scheduled match regardless of elapsed time", () => {
    const scheduled = { ...base, status: "scheduled" as const };
    expect(liveClockMins(scheduled, kickoffAt + 60_000)).toBe(0);
  });

  it("infers the elapsed minute for a live match, one minute per second", () => {
    const live = { ...base, status: "live" as const, clock_mins: 0 };
    expect(liveClockMins(live, kickoffAt + 30_000)).toBe(30);
  });

  it("never infers below the last known authoritative clock_mins", () => {
    const live = { ...base, status: "live" as const, clock_mins: 50 };
    expect(liveClockMins(live, kickoffAt + 10_000)).toBe(50);
  });

  it("caps inference at 90 minutes for a still-live match", () => {
    const live = { ...base, status: "live" as const, clock_mins: 0 };
    expect(liveClockMins(live, kickoffAt + 200_000)).toBe(90);
  });

  it("freezes at clock_mins once full_time, ignoring elapsed time", () => {
    const finished = { ...base, status: "full_time" as const, clock_mins: 90 };
    expect(liveClockMins(finished, kickoffAt + 500_000)).toBe(90);
  });
});

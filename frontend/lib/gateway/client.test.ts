import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GatewayError, getMatch, listMatchEvents, listMatches } from "./client";

function mockFetchOnce(status: number, body: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: status >= 200 && status < 300,
      status,
      json: () => Promise.resolve(body),
    }),
  );
}

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_GATEWAY_URL", "http://gateway.test");
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe("listMatches", () => {
  it("requests /matches with no query when status is omitted", async () => {
    mockFetchOnce(200, { matches: [] });
    await listMatches();
    expect(fetch).toHaveBeenCalledWith("http://gateway.test/matches");
  });

  it("requests /matches?status=live when a status is given", async () => {
    mockFetchOnce(200, { matches: [] });
    await listMatches("live");
    expect(fetch).toHaveBeenCalledWith(
      "http://gateway.test/matches?status=live",
    );
  });

  it("returns the matches array from the response body", async () => {
    const matches = [
      {
        match_id: "m1",
        sport: "football",
        status: "live",
        home_score: 1,
        away_score: 0,
        clock_mins: 10,
      },
    ];
    mockFetchOnce(200, { matches });
    await expect(listMatches()).resolves.toEqual(matches);
  });
});

describe("getMatch", () => {
  it("requests /matches/{id}", async () => {
    mockFetchOnce(200, {
      match_id: "m1",
      sport: "football",
      status: "live",
      home_score: 0,
      away_score: 0,
      clock_mins: 0,
    });
    await getMatch("m1");
    expect(fetch).toHaveBeenCalledWith("http://gateway.test/matches/m1");
  });

  it("throws a GatewayError carrying the status on a 404", async () => {
    mockFetchOnce(404, {});
    await expect(getMatch("missing")).rejects.toMatchObject({ status: 404 });
    await expect(getMatch("missing")).rejects.toBeInstanceOf(GatewayError);
  });
});

describe("listMatchEvents", () => {
  it("requests /matches/{id}/events and returns the events array", async () => {
    const events = [{ type: "kickoff", sequence: 1, payload: {} }];
    mockFetchOnce(200, { events });
    await expect(listMatchEvents("m1")).resolves.toEqual(events);
    expect(fetch).toHaveBeenCalledWith("http://gateway.test/matches/m1/events");
  });
});

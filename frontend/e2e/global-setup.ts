import Redis from "ioredis";

const CHANNEL = "matchflow:events";

function canonicalEvent(
  matchId: string,
  type: string,
  sequence: number,
  payload: Record<string, unknown> = {},
) {
  const now = new Date().toISOString();
  return JSON.stringify({
    match_id: matchId,
    type,
    sequence,
    payload,
    timestamp: now,
    ingested_at: now,
    provider: "e2e-fixture",
  });
}

export default async function globalSetup() {
  // Unique per run so a replayed test against the same Postgres data
  // doesn't collide with a lower sequence number from a prior run -
  // Match Service's Reduce drops any event whose sequence isn't higher
  // than what it already has for a given match_id.
  const fixtureMatchId = `e2e-fixture-${Date.now()}`;
  process.env.E2E_MATCH_ID = fixtureMatchId;

  const redis = new Redis(process.env.REDIS_URL ?? "redis://localhost:6379");
  await redis.publish(CHANNEL, canonicalEvent(fixtureMatchId, "kickoff", 1));
  await redis.quit();
  // Give Match Service a moment to consume and persist the event
  // before any test's initial page load fetches it.
  await new Promise((resolve) => setTimeout(resolve, 500));
}

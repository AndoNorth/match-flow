import Redis from "ioredis";

export const FIXTURE_MATCH_ID = "e2e-fixture-match";
const CHANNEL = "matchflow:events";

function canonicalEvent(
  type: string,
  sequence: number,
  payload: Record<string, unknown> = {},
) {
  const now = new Date().toISOString();
  return JSON.stringify({
    match_id: FIXTURE_MATCH_ID,
    type,
    sequence,
    payload,
    timestamp: now,
    ingested_at: now,
    provider: "e2e-fixture",
  });
}

export default async function globalSetup() {
  const redis = new Redis(process.env.REDIS_URL ?? "redis://localhost:6379");
  await redis.publish(CHANNEL, canonicalEvent("kickoff", 1));
  await redis.quit();
  // Give Match Service a moment to consume and persist the event
  // before any test's initial page load fetches it.
  await new Promise((resolve) => setTimeout(resolve, 500));
}

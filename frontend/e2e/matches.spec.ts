import { expect, test } from "@playwright/test";
import Redis from "ioredis";
import { FIXTURE_MATCH_ID } from "./global-setup";

test("match list shows the seeded fixture match", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText(FIXTURE_MATCH_ID)).toBeVisible();
});

test("a live update reaches the detail page without a reload", async ({
  page,
}) => {
  await page.goto(`/matches/${FIXTURE_MATCH_ID}`);
  await expect(page.getByText("0 - 0")).toBeVisible();

  const redis = new Redis(process.env.REDIS_URL ?? "redis://localhost:6379");
  const now = new Date().toISOString();
  await redis.publish(
    "matchflow:events",
    JSON.stringify({
      match_id: FIXTURE_MATCH_ID,
      type: "goal",
      sequence: 2,
      payload: { team: "home", minute: 5 },
      timestamp: now,
      ingested_at: now,
      provider: "e2e-fixture",
    }),
  );
  await redis.quit();

  await expect(page.getByText("1 - 0")).toBeVisible();
});

test("an unknown match ID renders a real 404", async ({ page }) => {
  const response = await page.goto("/matches/does-not-exist");
  expect(response?.status()).toBe(404);
  await expect(page.getByText("Match not found.")).toBeVisible();
});

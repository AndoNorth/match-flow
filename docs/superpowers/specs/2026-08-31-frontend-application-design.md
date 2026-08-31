---
status: draft
date: 2026-08-31
updated: 2026-08-31
issues: [5]
scope: [frontend]
---

# Frontend Application

## Problem

The Gateway Service (#4, #10) exposes REST reads and an SSE realtime stream, but nothing
consumes them - MatchFlow has no human-facing client, so the PoC isn't actually visible
end-to-end. Match state and live events exist and update correctly in Postgres and over Redis,
but there is no way to see any of it without `curl`.

## Goals

- A Next.js (App Router) application at `frontend/`, the repo's already-locked location for it
  (see [README.md](../../../README.md#repository-layout)).
- A match list page showing every match, grouped into Scheduled / Live / Finished tabs, updating
  live as scores/status change - no reload needed.
- A match detail page for one match: score, clock, status, and its full event timeline, also
  updating live.
- One thin Gateway client module - all REST and SSE calls to the Gateway go through it, per the
  "thin API/client boundary" [ARCHITECTURE.md](../../ARCHITECTURE.md#frontend-application)
  already calls for.
- A real 404 for an unknown match ID, and a generic error boundary for any other Gateway/server
  failure.
- Vitest coverage of the Gateway client module and the two live-update components; Playwright
  coverage of the two pages against a running Gateway.
- `tsc --noEmit` and `fallow audit` wired into this repo's pre-commit hook chain, the same way
  Biome already is, now that `frontend/package.json` exists to give them something to check
  (`DEVELOPMENT.md` already named both as planned, waiting on exactly this).
- A locked visual theme (DaisyUI `night`, see [Theme](#theme)) so implementation doesn't stall on
  a styling decision.

## Non-Goals

- Authentication, user accounts, or any multi-tenant concept - per
  [GOALS.md](../../GOALS.md#non-goals), the frontend is a read-only client of live match data.
- An odds/betting engine or any odds UI. `odds_update` events exist in the Feed Simulator's
  model but are deliberately dropped before persistence
  (`services/match-service/internal/football/registry.go`), and the Gateway exposes no odds
  endpoint. Surfaced during the visual design pass and explicitly deferred - a real odds engine
  is a new backend subsystem (Match Service schema + API, Ingestion no longer dropping the
  event) and its own spec, not a frontend decision.
- Docker Compose / K8s dev-loop wiring for the frontend, and any `.github/workflows/ci.yml`.
  Both are repo-wide, cross-service milestones
  ([ROADMAP.md](../../ROADMAP.md#phase-7---containerization), Phase 1's CI note) done once for
  every service together, not scoped into this spec.
- A client-side data-fetching/caching library (TanStack Query, SWR, ...). The Gateway's SSE
  stream already pushes updates; there is no client-side cache-invalidation problem for a
  library like that to solve here.
- WebSocket support - matches the Gateway's own SSE-only decision
  ([ARCHITECTURE.md](../../ARCHITECTURE.md#gateway-service)).

## Architecture / Design

### Routes

- `frontend/app/page.tsx` - match list. A Server Component fetches `GET /matches?status=` per
  tab (Scheduled / Live / Finished) for the initial render; a client component
  (`<LiveMatchList>`) then opens the Gateway's SSE stream (`GET /events`, no `match_id`) and
  patches matches into local state by `match_id` as `update`/`snapshot` frames arrive, moving a
  match between tabs as its status changes. Match Service's four raw statuses collapse into
  three tabs: `live` and `half_time` both group under **Live** (both mean "in progress" to a
  viewer), `full_time` maps to **Finished**, `scheduled` maps to **Scheduled** - the tab mapping
  lives in the frontend, the Gateway/Match Service statuses stay as-is.
- `frontend/app/matches/[id]/page.tsx` - one match's score, clock, status, and event timeline. A
  Server Component fetches `GET /matches/{id}` and `GET /matches/{id}/events`; a 404 response
  calls Next.js's `notFound()`. A client component (`<LiveMatchDetail>`) then opens
  `GET /events?match_id=<id>` for live updates.
- No other routes - no auth, no search, per Non-Goals above.

### Gateway client module

The Gateway's REST/JSON shapes match `resolver.MatchBody` and `resolver.EventBody`
(`services/gateway-service/internal/resolver/resolver.go`) exactly - no team names, only
`match_id, sport, status, home_score, away_score, clock_mins` for a match, and
`type, sequence, payload` per event.

- `frontend/lib/gateway/client.ts` - `listMatches(status?)`, `getMatch(id)`,
  `listMatchEvents(id)`: plain functions wrapping `fetch(GATEWAY_URL + path)`, used by Server
  Components. Throw on a non-2xx response; the `getMatch` 404 case is caught by the route to
  call `notFound()`.
- `frontend/lib/gateway/realtime.ts` - `subscribeToMatches(matchId?, onSnapshot, onUpdate)`:
  wraps the browser's native `EventSource` (no header/auth need, per
  [GOALS.md](../../GOALS.md#non-goals) - no auth to carry), parsing `snapshot`/`update` frames.
  Used by `<LiveMatchList>` and `<LiveMatchDetail>`.
- `NEXT_PUBLIC_GATEWAY_URL` env var - needs the `NEXT_PUBLIC_` prefix since the browser-side
  `EventSource` call reads it too, not only the server-rendered `fetch` calls.

### Data flow & error handling

- SSE connection errors rely on `EventSource`'s built-in auto-reconnect; no custom retry/backoff.
  A small "reconnecting..." indicator shows on `onerror`, clears on the next message/`onopen` -
  the only connection-health UI needed.
- Known accepted gap: the Gateway sends no heartbeat comment frame, so a silently-dead
  connection has no watchdog beyond the browser's own reconnect-on-error. Not building one now -
  YAGNI until it's an observed problem in this PoC.
- `frontend/app/matches/[id]/not-found.tsx` - unknown match ID. Plain message, link back to the
  list.
- `frontend/app/error.tsx` - root error boundary for any other server-side failure (Gateway
  unreachable, unexpected 5xx). Plain message + Next's built-in `reset()`.
- No custom loading skeletons/suspense boundaries beyond Next's defaults.

### Theme

Locked via a `/lavish` visual review (four DaisyUI themes mocked against the real match-list +
match-detail shape): **DaisyUI `night`** - `<html data-theme="night">`. Closest to a
broadcast-scoreboard feel; high contrast for live badges and score numbers. The selected match
card in the list carries a `ring-2 ring-primary` outline tying it to the expanded detail below.

`tailwindcss` and `daisyui` are installed as real npm dependencies of `frontend/` (the CDN
`<script>`/`<link>` tags used in the throwaway `.lavish/` mockup are a Lavish-artifact
convenience only, not how the shipped app loads either library).

### Tooling

- `frontend/package.json` gets a `typecheck` script (`tsc --noEmit`), run via
  `make lint SVC=frontend` alongside Biome.
- `fallow` added as a devDependency; audited the same way, via `make lint SVC=frontend`.
- Both wired into `flake.nix`'s git-hooks.nix `pre-commit` stage, matching this repo's existing
  hook stage convention (no `pre-push` stage exists here, unlike the reference setup these were
  pulled from).

## Validation

- **Vitest**, colocated with the code it tests:
  - `gateway/client.test.ts` - each function hits the right path/query, parses the response
    shape, throws on non-2xx.
  - `gateway/realtime.test.ts` - `subscribeToMatches` routes `snapshot` to the initial callback
    and `update` to the patch callback, and ignores frames for an unrelated `match_id` when
    scoped.
  - Component tests for `<LiveMatchList>` / `<LiveMatchDetail>` - mock the gateway client
    module (per [DEVELOPMENT.md](../../DEVELOPMENT.md#testing-conventions)'s existing
    convention: mock at the module boundary the component imports, not `fetch`/`EventSource`
    directly): an `update` event patches rendered state (score/status change), and the
    reconnect indicator toggles on `onerror`/`onopen`.
- **Playwright** (`frontend/e2e/`):
  - Load the match list -> see seeded matches.
  - Load a match detail page -> a live update arrives (driven via Feed Simulator/Ingestion, or a
    seeded Gateway) -> score/timeline updates without a reload.
  - Unknown match ID -> a real 404 page.

## References

- [ARCHITECTURE.md - Frontend Application](../../ARCHITECTURE.md#frontend-application)
- [GOALS.md - Non-Goals](../../GOALS.md#non-goals)
- [DEVELOPMENT.md - Testing Conventions](../../DEVELOPMENT.md#testing-conventions)
- [README.md - Repository Layout](../../../README.md#repository-layout)
- [services/gateway-service/README.md](../../../services/gateway-service/README.md) - Gateway
  endpoints this client consumes
- `services/gateway-service/internal/resolver/resolver.go` - `MatchBody`/`EventBody` shapes
- `services/gateway-service/internal/realtime/sse.go` - SSE frame format (`snapshot`/`update`)
- `services/match-service/internal/football/registry.go` - where `odds_update` is dropped today
- [2026-08-31-gateway-service-design.md](2026-08-31-gateway-service-design.md) - names the
  Frontend Application as #5 and the Odds Service as a separate future effort

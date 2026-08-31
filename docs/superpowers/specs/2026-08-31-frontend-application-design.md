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
  Biome already is - `fallow` is DEVELOPMENT.md's already-planned tool waiting on exactly this
  (`frontend/package.json` existing); `tsc --noEmit` is new, this spec introduces it.
- A locked visual theme (DaisyUI `night`, see [Theme](#theme)) so implementation doesn't stall on
  a styling decision.

## Non-Goals

- Authentication, user accounts, or any multi-tenant concept - per
  [GOALS.md](../../GOALS.md#non-goals), the frontend is a read-only client of live match data.
- An odds/betting engine or any odds UI. `odds_update` events exist in the Feed Simulator's
  model but are deliberately dropped by Match Service before persistence
  (`services/match-service/internal/eventstream/eventstream.go`'s `Route` function - see
  [References](#references)), and the Gateway exposes no odds endpoint. Surfaced during the
  visual design pass and explicitly deferred - a real odds engine is a new backend subsystem
  (Match Service would need to stop dropping the event and add schema/API for it) and its own
  spec, not a frontend decision.
- Docker Compose / K8s dev-loop wiring for the frontend, and any `.github/workflows/ci.yml`.
  Both are repo-wide, cross-service milestones
  ([ROADMAP.md](../../ROADMAP.md#phase-7---containerization), Phase 1's CI note) done once for
  every service together, not scoped into this spec. `make run SVC=frontend` (plain `next dev`,
  no container) is in scope - see [Tooling](#tooling).
- A client-side data-fetching/caching library (TanStack Query, SWR, ...). The Gateway's SSE
  stream already pushes updates; there is no client-side cache-invalidation problem for a
  library like that to solve here.
- WebSocket support - matches the Gateway's own SSE-only decision
  ([ARCHITECTURE.md](../../ARCHITECTURE.md#gateway-service)).

## Architecture / Design

### Routes

- `frontend/app/page.tsx` - match list. A Server Component fetches all matches with one
  unfiltered `GET /matches` call for the initial render (not one request per tab - the Gateway's
  `status` query filters to a single raw status, which can't express the tab grouping below in
  fewer than four calls, so the frontend fetches once and groups client-side). A client component
  (`<LiveMatchList>`) then opens the Gateway's SSE stream (`GET /events`, no `match_id`) and
  applies subsequent frames via the [reducer](#client-side-match-state-reducer), moving a match
  between tabs as its status changes. Match Service's four raw statuses collapse into three tabs:
  `live` and `half_time` both group under **Live** (both mean "in progress" to a viewer),
  `full_time` maps to **Finished**, `scheduled` maps to **Scheduled** - the tab mapping lives in
  the frontend, the Gateway/Match Service statuses stay as-is. `listMatches(status?)` keeps its
  `status` parameter for completeness against the Gateway's actual API and for direct unit
  testing, even though the list page itself calls it unfiltered.
- `frontend/app/matches/[id]/page.tsx` - one match's score, clock, status, and event timeline. A
  Server Component fetches `GET /matches/{id}` and `GET /matches/{id}/events`; a 404 response
  calls Next.js's `notFound()` (see [Gateway client module](#gateway-client-module) for how the
  client surfaces that distinctly). A client component (`<LiveMatchDetail>`) then opens
  `GET /events?match_id=<id>` for live updates, applied via the same reducer.
- No other routes - no auth, no search, per Non-Goals above.

### Gateway client module

The Gateway's REST/JSON shapes match `resolver.MatchBody` and `resolver.EventBody`
(`services/gateway-service/internal/resolver/resolver.go`) exactly - no team names, only
`match_id, sport, status, home_score, away_score, clock_mins` for a match, and
`type, sequence, payload` per event.

- `frontend/lib/gateway/client.ts` - `listMatches(status?)`, `getMatch(id)`,
  `listMatchEvents(id)`: plain functions wrapping `fetch(NEXT_PUBLIC_GATEWAY_URL + path)`, used
  by Server Components. On a non-2xx response, throw a `GatewayError` carrying the HTTP
  `status` code (not a bare `Error`) - the `matches/[id]` route checks
  `error instanceof GatewayError && error.status === 404` to call `notFound()`; every other
  error (including a non-404 `GatewayError` and a network failure) propagates to `error.tsx`.
  This is the one place that distinction is made, so no route re-derives it.
- `frontend/lib/gateway/realtime.ts` - `subscribeToMatches(matchId?, onSnapshot, onUpdate)`:
  wraps the browser's native `EventSource` (no header/auth need, per
  [GOALS.md](../../GOALS.md#non-goals) - no auth to carry). `onSnapshot` fires once, on the
  `snapshot` frame the Gateway sends right after registering
  (`services/gateway-service/internal/realtime/sse.go`'s `writeSnapshot`) - it carries a full
  `MatchBody` (or list of them), and the caller replaces its local state with it wholesale.
  `onUpdate` fires per subsequent `update` frame - it carries a raw `EventBody`
  (`type`/`sequence`/`payload`, **not** a score/status delta - confirmed against
  `services/gateway-service/internal/realtime/subscribe.go`, which broadcasts the decoded event
  itself, not computed match state), which the caller runs through the
  [reducer](#client-side-match-state-reducer) to produce the next match state. Used by
  `<LiveMatchList>` and `<LiveMatchDetail>`.
- `NEXT_PUBLIC_GATEWAY_URL` env var, one name, used by both the server-rendered `fetch` calls and
  the browser's `EventSource` (needs the `NEXT_PUBLIC_` prefix for the latter; using it
  everywhere avoids a second server-only variable naming the same host). Defaults to
  `http://localhost:8083` - matching this repo's existing convention that every cross-service
  address env var defaults to `localhost:<port>` until a real deployment overrides it
  ([ARCHITECTURE.md](../../ARCHITECTURE.md#multi-service-realtime-fan-out)), and `8083` is the
  Gateway's own default port ([services/gateway-service/README.md](../../../services/gateway-service/README.md)).

### Client-side match-state reducer

Because `update` frames carry a raw event, not a score/status delta, `<LiveMatchList>` and
`<LiveMatchDetail>` need to turn "a `goal` event happened for the home team at minute 23" into
"home_score + 1, clock_mins = 23" themselves - `ARCHITECTURE.md`'s
[Frontend Application](../../ARCHITECTURE.md#frontend-application) section already commits to
applying event updates client-side rather than re-fetching, so this logic has to live somewhere
in the frontend regardless.

`frontend/lib/gateway/reduce.ts` mirrors `services/match-service/internal/matchstate/reduce.go`'s
`Reduce` function exactly, using the same category-per-event-type mapping as
`services/match-service/internal/football/registry.go`:

- `kickoff` -> `status: "live"`; `half_time` -> `status: "half_time"`; `full_time` ->
  `status: "full_time"` (no score change).
- `goal` -> increment `home_score` or `away_score` per `payload.team`, and set `clock_mins` from
  `payload.minute`.
- Anything else (`card`, an unrecognized type) -> no score/status/clock change; still appended to
  the event timeline.

This is a deliberate, minimal mirror of an existing backend rule set, not a reinterpretation of
it - see [Open Questions](#open-questions) for the duplication this creates.

### Error handling & connection health

- `frontend/app/matches/[id]/not-found.tsx` - unknown match ID. Plain message, link back to the
  list.
- `frontend/app/error.tsx` - root error boundary for any other server-side failure (Gateway
  unreachable, unexpected 5xx). Plain message + Next's built-in `reset()`.
- SSE connection errors rely on `EventSource`'s built-in auto-reconnect; no custom retry/backoff.
  A small "reconnecting..." indicator shows on `onerror`, clears on the next message/`onopen` -
  the only connection-health UI needed.
- Known accepted gap: the Gateway sends no heartbeat comment frame, so a silently-dead
  connection has no watchdog beyond the browser's own reconnect-on-error. Not building one now -
  YAGNI until it's an observed problem in this PoC.
- No custom loading skeletons/suspense boundaries beyond Next's defaults.

### Theme

Locked via a `/lavish` visual review (four DaisyUI themes mocked against the real match-list +
match-detail shape): **DaisyUI `night`** - `<html data-theme="night">`. Closest to a
broadcast-scoreboard feel; high contrast for live badges and score numbers. That review's mockup
showed the list and an expanded detail panel side-by-side with a `ring-2 ring-primary` outline
tying a "selected" card to its detail below purely to compare themes at a glance - the shipped
app has no such inline expansion (see [Routes](#routes): list and detail are two separate pages),
so that outline pattern doesn't carry over; it was a mockup-comparison device, not a UI decision.

`tailwindcss` and `daisyui` are installed as real npm dependencies of `frontend/` (the CDN
`<script>`/`<link>` tags used in the throwaway `.lavish/` mockup are a Lavish-artifact
convenience only, not how the shipped app loads either library).

### Tooling

- `frontend/package.json` gets a `typecheck` script (`tsc --noEmit`) and `fallow` as a
  devDependency.
- `Makefile`'s `lint` target is currently `golangci-lint run ./services/$(SVC)/...` only - no
  Biome branch, and `frontend/` isn't under `services/` anyway. This spec adds a frontend case to
  `lint` (and to `build`/`test`) that runs Biome, `tsc --noEmit`, and `fallow audit` instead of
  `golangci-lint`, keeping `make lint SVC=frontend` as the one command, consistent with every
  Go service.
- `make run SVC=frontend` gets a matching case running `next dev` (default port `3000`, Next's
  own default - distinct from the Gateway's `8083`). This is the port Playwright's tests target.
- `tsc --noEmit`/`fallow audit` are also wired into `flake.nix`'s git-hooks.nix `pre-commit`
  stage, matching this repo's existing hook stage convention (no `pre-push` stage exists here).

## Validation

- **Vitest**, colocated with the code it tests. `gateway/client.test.ts` covers `client.ts`'s
  request construction and its `GatewayError`/status-code behavior on non-2xx responses.
  `gateway/realtime.test.ts` and `gateway/reduce.test.ts` cover `subscribeToMatches`'s
  frame-to-callback routing and the reducer's per-event-type rules, respectively. Component tests
  for `<LiveMatchList>`/`<LiveMatchDetail>` mock the gateway client module - per
  [DEVELOPMENT.md](../../DEVELOPMENT.md#testing-conventions)'s existing convention: mock at the
  module boundary the component imports, not `fetch`/`EventSource` directly - and assert that a
  reduced update patches rendered state and that the connection-health indicator tracks
  `onerror`/`onopen`.
- **Playwright** (`frontend/e2e/`) covers the two pages against a running Gateway seeded with
  fixture match data, injecting the "live update" as a direct `matchflow:events` Redis publish
  (`redis-cli PUBLISH` or an equivalent test helper) rather than running the full
  Feed-Simulator-through-Ingestion pipeline - lighter infra, and consistent with this spec's
  Non-Goal of not standing up new dev-loop wiring. Covers: the list rendering seeded matches, a
  live update reaching the detail page without a reload, and an unknown match ID rendering a real
  404.

## Open Questions

- The client-side reducer ([Client-side match-state reducer](#client-side-match-state-reducer))
  duplicates `matchstate.Reduce`'s rules in TypeScript. Acceptable for one sport (football) with
  a small, stable rule set; revisit (e.g. the Gateway exposing the *result* of a rule application
  rather than the raw event) if a second sport multiplies the rule set this has to mirror.

## References

- [ARCHITECTURE.md - Frontend Application](../../ARCHITECTURE.md#frontend-application)
- [ARCHITECTURE.md - Multi-service realtime fan-out](../../ARCHITECTURE.md#multi-service-realtime-fan-out) -
  the `localhost:<port>` default convention for cross-service address env vars
- [GOALS.md - Non-Goals](../../GOALS.md#non-goals)
- [DEVELOPMENT.md - Testing Conventions](../../DEVELOPMENT.md#testing-conventions)
- [README.md - Repository Layout](../../../README.md#repository-layout)
- [services/gateway-service/README.md](../../../services/gateway-service/README.md) - Gateway
  endpoints this client consumes, and its `8083` default port
- `services/gateway-service/internal/resolver/resolver.go` - `MatchBody`/`EventBody` shapes
- `services/gateway-service/internal/realtime/sse.go` - SSE frame format (`snapshot`/`update`)
- `services/gateway-service/internal/realtime/subscribe.go` - confirms `update` frames carry a
  raw `EventBody`, not computed match state
- `services/match-service/internal/matchstate/reduce.go` - the exact rules
  [reduce.ts](#client-side-match-state-reducer) mirrors
- `services/match-service/internal/football/registry.go` - event-type-to-rule mapping
- `services/match-service/internal/eventstream/eventstream.go` - where `odds_update` is actually
  dropped today (`Route`'s `oddsUpdateType` check)
- [2026-08-31-gateway-service-design.md](2026-08-31-gateway-service-design.md) - names the
  Frontend Application as #5 and the Odds Service as a separate future effort

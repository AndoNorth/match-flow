# Feed Simulator PoC & Go Service Dev-Loop Design

Status: approved
Date: 2026-08-30
Scope: Issue #1 (Feed Simulator), with dev-loop conventions carried forward
to issues #2-#4 (Ingestion, Match, Gateway services)

## Context

Feed Simulator is the first Go service scaffolded (Phase 3, see
docs/ROADMAP.md). Because it's first, it also has to establish the
conventions every later backend service reuses: how a service is run
locally, how it's tested, how its ports are managed, and how it's
documented. This spec covers both the feed-simulator PoC itself and
those shared conventions.

## Feed Simulator PoC

### Domain design

Generic on two axes, deliberately:

1. **Sport-agnostic engine.** `internal/simulator/domain` defines a
   `Sport` interface (`NextEvent(state MatchState) (DomainEvent, bool)`)
   and a `MatchEngine` that drives it on a ticker. `football.go` is the
   only implementation today. A second sport is a new type implementing
   `Sport`, not a rewrite of the engine.
2. **Multiple provider payload shapes for the same event.** Real odds
   feeds from different providers describe the same event differently
   (field names, nesting, casing). `internal/simulator/providers` holds
   two encoders (`EncodeProviderA`, `EncodeProviderB`) that take one
   canonical `DomainEvent` and produce two structurally different JSON
   payloads. This exists so Ingestion Service's normalization step
   (Phase 4) has a real, non-trivial gap to close instead of an assumed
   one.

Event vocabulary (football): `kickoff`, `goal`, `card`, `odds_update`,
`half_time`, `full_time`. `odds_update` carries `market`/`selection`/
`price` fields - the specific case driving the multi-provider-shape
design.

Canonical event shape:

```go
type DomainEvent struct {
    MatchID   string
    Sequence  int
    Type      EventType
    Timestamp time.Time
    Payload   map[string]any
}
```

### Runtime shape

`cmd/feed-simulator/main.go` runs two things concurrently:

- A generator goroutine: `MatchEngine` emits canonical events, an
  encoder (alternating providers) shapes each one, `slog` logs the
  **encoded** payload (not the canonical struct - that's what a real
  feed would actually put on the wire).
- A stdlib `net/http` server exposing `GET /healthz`, proving the
  Makefile/Air run-loop independent of the generator.

### Where events land (and don't, yet)

Events are logged only. Ingestion Service doesn't exist yet, so there's
no real delivery target. This is a deliberate interim state, not a gap
left open indefinitely: **issue #2 (Ingestion) adds a line item** to
update feed-simulator's generator to POST events to Ingestion's new
receiving endpoint once it exists, replacing (or joining) the log sink.
Proves the real Feed Simulator -> Ingestion hop end-to-end in that PR.

Per docs/ARCHITECTURE.md, that hop is plain REST (via Huma, matching
TECH_STACK.md's stated default for simple service-to-service calls),
not gRPC and not a message broker - Redis pub/sub only starts one hop
later, between Ingestion and Match Service.

### Testing

Ginkgo + Gomega (docs/TECH_STACK.md), unit-only for this issue:

- `MatchEngine` produces a valid, correctly-ordered event sequence for
  one full simulated match - deterministic (seeded RNG), not
  wall-clock-timed.
- Both provider encoders round-trip-decodable (encode then decode
  recovers the same logical fields).

No testcontainers here - feed-simulator touches neither Redis nor
Postgres. testcontainers-go enters `go.mod` at the first real
consumer: Ingestion (#2, Redis) or Match (#3, Postgres).

## Shared Go service dev-loop conventions

These apply to every backend service from #1 onward.

### Directory & Makefile pattern

- `services/<name>/cmd/<name>/main.go` + `services/<name>/internal/`
  (README.md#repository-layout)
- Generic per-service Makefile targets, parameterized by `SVC`:
  `make build SVC=<name>`, `make run SVC=<name>` (Air hot-reload),
  `make test SVC=<name>` (unit only), `make test-integration
  SVC=<name>` (adds `-tags=integration`), `make lint SVC=<name>`
- One root `.air.toml`, generic across services (mirrors
  syntrix/crazy-train-code: Air's cwd is `services/<name>/cmd/<name>`
  when launched, so `go build -o ./tmp/main .` always builds the
  right thing)
- Air itself is provided by the Nix devShell already (CLAUDE.md) - the
  Makefile only wires the invocation, Nix's job stops at "air exists
  in PATH"

### Unit vs integration test split

Integration specs live behind a `//go:build integration` tag (e.g.
`internal/foo/foo_integration_test.go`). `make test` stays fast by
default; `make test-integration` opts in. Integration tests always
spin fresh testcontainers-go containers - random host ports, no
coupling to whether `make dev-infra` happens to be running. Same
behavior locally and in CI.

### Port-squatter registry

Ported from syntrix/crazy-train-code's `scripts/lib/dev-ports.sh` +
`scripts/check-dev-ports.sh` pattern (separate from the existing
`scripts/check-infra-ports.sh`, which covers Redis/Postgres host
ports and is unchanged):

- `scripts/lib/dev-ports.sh`: single array of `"service:base_port"`
  entries. Starts with `feed-simulator:8080` (port picked arbitrarily -
  no doc precedent, open to change). New services append one line.
- `scripts/check-dev-ports.sh`: pre-flight check sourcing that array,
  wired into `make run SVC=<name>` the same way
  `check-infra-ports.sh` is wired into `make dev-infra`. Warns/fails
  (same `PORT_CHECK_MODE` convention) if a non-Go/non-Air process
  already holds a service's port.

### Per-service README

- `services/README.md`: shared conventions once - Huma auto-serves
  `GET /openapi.json` for any service using it (no separate
  generation step; deferred: a checked-in generated-OAS snapshot a la
  crazy-train's `api/openapi/generated/<svc>.json` drift-check, until
  something actually consumes the spec offline), common env vars
  (e.g. `PORT`), common flags if any.
- `services/<name>/README.md`: only what's specific to that service
  (its env vars, its flags), with a one-line link back to
  `services/README.md` for the shared bits.
- feed-simulator has no Huma/OAS surface (stdlib `/healthz` only), so
  its README is minimal: just its own env vars (`PORT`, default
  8080).

### OAS generation - explicitly deferred scope

Crazy-train also bundles every service's OAS into one master spec to
generate a UI TypeScript client. That doesn't apply here: architecturally,
only the Gateway Service's REST contract ever reaches the frontend
(hard constraint, docs/ARCHITECTURE.md) - Ingestion and Match Service
OAS specs have no frontend consumer to bundle for. Not building
multi-service OAS bundling; each Huma-using service just serves its
own live spec.

## Out of scope (this spec)

- Real Feed Simulator -> Ingestion delivery (tracked in issue #2)
- Any Ingestion/Match/Gateway business logic
- OpenTelemetry instrumentation (Phase 6)
- Containerization (Phase 7)

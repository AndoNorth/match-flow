---
status: approved
date: 2026-08-30
issues: [1]
---

# Feed Simulator PoC & Go Service Dev-Loop Design

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

1. **Sport-agnostic engine.** `services/feed-simulator/internal/simulator/domain`
   defines a `Sport` interface and a `MatchEngine` that drives it on a
   tick source. `services/feed-simulator/internal/simulator/football`
   holds the only implementation today. A second sport is a new package
   implementing `Sport`, not a rewrite of the engine.
2. **Multiple provider payload shapes for the same event.** Real odds
   feeds from different providers describe the same event differently
   (field names, nesting, casing).
   `services/feed-simulator/internal/simulator/providers` holds two
   encoders (`EncodeProviderA`, `EncodeProviderB`) that take one
   canonical `DomainEvent` and produce two structurally different JSON
   payloads. This exists so Ingestion Service's normalization step
   (Phase 4) has a real, non-trivial gap to close instead of an assumed
   one.

`Sport` interface:

```go
type Sport interface {
    // NextEvent advances one tick. hasEvent is true when an event was
    // produced this tick (log it); false is a quiet tick, no event to
    // log. done is true when the match is over - the engine stops
    // ticking after this call regardless of hasEvent. The engine never
    // inspects Type to decide when to stop; only done decides that.
    NextEvent(state MatchState) (event DomainEvent, hasEvent bool, done bool)
}
```

`MatchState` is intentionally thin and generic - `MatchID`, `ClockMins`,
`Sequence` only. It carries no sport-specific fields (no score, no
cards). `MatchEngine` owns and advances `MatchState` between ticks;
each `Sport` implementation tracks any sport-specific state (e.g.
football's own score/cards) internally, inside its own struct, not in
`MatchState`. This is what keeps the engine sport-agnostic: it never
needs to interpret a sport's payload to know what to do next.

`EventType` is an open string type declared in `domain` - the package
defines the type but no constants. Football's six event names
(`kickoff`, `goal`, `card`, `odds_update`, `half_time`, `full_time`) are
declared in the `football` package, not `domain`. `odds_update` carries
`market`/`selection`/`price` payload fields - the specific case driving
the multi-provider-shape design. A second sport adding its own event
vocabulary never touches `domain`.

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

`services/feed-simulator/cmd/feed-simulator/main.go` is wiring only, no
logic (per README.md#repository-layout): it constructs a
`simulator.Runner` (in `internal/simulator`) and an HTTP server, then
starts both concurrently. All generator/encoder/logging behavior lives
in `internal/simulator`, not in `main.go`.

- `simulator.Runner.Run(ctx)`: `MatchEngine` emits canonical events, an
  encoder (alternating providers) shapes each one, `slog` logs the
  **encoded** payload (not the canonical struct - that's what a real
  feed would actually put on the wire). `MatchEngine`'s tick source is
  injected (a small `Ticker` interface) - production wiring uses a real
  `time.Ticker`; tests inject a manual/fake tick source so the engine
  can be stepped synchronously with no wall-clock wait. When `Sport`
  reports `done`, `Run` logs one explicit `match_complete` line and
  returns - so "the match finished" is never ambiguous with "the
  generator stalled" from the logs alone.
- A stdlib `net/http` server exposing `GET /healthz`, proving the
  Makefile/Air run-loop independent of the generator.

`football.New(seed int64)` takes an explicit RNG seed - production
wiring in `main.go` seeds from `time.Now().UnixNano()` (so `make run`
output is intentionally non-reproducible run to run, which is fine for
a PoC), tests pass a fixed seed for determinism. The seed is a
constructor argument, not an env var - it does not appear in
feed-simulator's README env-var list.

### Where events land (and don't, yet)

Events are logged only. Ingestion Service doesn't exist yet, so there's
no real delivery target. This is a deliberate interim state, not a gap
left open indefinitely: once Ingestion Service's receiving endpoint
exists, feed-simulator's generator gets updated to POST events there,
replacing (or joining) the log sink - proving the real Feed Simulator
-> Ingestion hop end-to-end.

docs/ARCHITECTURE.md leaves that hop's protocol open ("REST or gRPC,
per implementation"). This spec decides REST (via Huma, matching
TECH_STACK.md's stated default for simple service-to-service calls)
over gRPC and over a message broker - Redis pub/sub only starts one
hop later, between Ingestion and Match Service.

### Testing

Ginkgo + Gomega (docs/TECH_STACK.md), unit-only for feed-simulator:

- `MatchEngine` produces a valid, correctly-ordered event sequence for
  one full simulated match, driven via the injected fake tick source -
  deterministic (seeded RNG), not wall-clock-timed.
- Both provider encoders round-trip-decodable (encode then decode
  recovers the same logical fields).
- `GET /healthz` returns 200 (boundary/API test, per
  docs/DEVELOPMENT.md's "cover a service's behavior at its
  boundaries... not just individual functions") - this is the only
  externally-observable API surface this service has.

No testcontainers here - feed-simulator touches neither Redis nor
Postgres. testcontainers-go enters `go.mod` at the first real
consumer: Ingestion Service (Redis) or Match Service (Postgres).
`make test-integration SVC=feed-simulator` is a deliberate no-op
scaffold for this landing - same `check-docker` gate as every other
service (consistent behavior, set as the precedent now rather than
introduced later), then `go test -tags=integration ./...` against zero
matching files (Go reports "no test files", not an error). Proves the
target and its Docker preflight are wired correctly ahead of the
service that actually needs testcontainers-go; a machine without
Docker gets the same clear `check-docker` message as every other
Docker-touching target, not a false "it works."

## Shared Go service dev-loop conventions

These apply to every backend service, starting with feed-simulator.

### Directory & Makefile pattern

- `services/<name>/cmd/<name>/main.go` + `services/<name>/internal/`
  (README.md#repository-layout)
- Generic per-service Makefile targets, parameterized by `SVC`:
  `make build SVC=<name>`, `make run SVC=<name>` (Air hot-reload),
  `make test SVC=<name>` (unit only), `make test-integration
  SVC=<name>` (adds `-tags=integration`), `make lint SVC=<name>`
- One root `.air.toml`, generic across services: Air's cwd is
  `services/<name>/cmd/<name>` when launched, so `go build -o
  ./tmp/main .` always builds the right thing regardless of which
  service invoked it
- Air itself is provided by the Nix devShell already (CLAUDE.md) - the
  Makefile only wires the invocation, Nix's job stops at "air exists
  in PATH"

These artifacts (the `SVC=` targets, root `.air.toml`, port check) are
verified the same way as any other part of issue #1: its own acceptance
criteria (`make build/run/test/lint SVC=feed-simulator` all working)
prove the pattern end-to-end - there's no separate test suite for
Makefile/Air plumbing itself.

### Unit vs integration test split

Integration specs live behind a `//go:build integration` tag (e.g.
`internal/foo/foo_integration_test.go`). `make test` stays fast by
default; `make test-integration` opts in and depends on `check-docker`
(same preflight `dev-infra`/`dev-k8s` already require) before
testcontainers-go touches Docker - a missing/unreachable daemon gets
the same clear message as every other Docker-touching target, not a
raw connection error. Integration tests always spin fresh
testcontainers-go containers - random host ports, no coupling to
whether `make dev-infra` happens to be running. Same behavior locally
and in CI.

### Port-squatter registry

Extends the existing `scripts/check-infra-ports.sh` rather than adding
new files - no `scripts/lib/dev-ports.sh` or second check script:

- `check-infra-ports.sh` gains an optional ports argument (falls back
  to its current `PORTS=(6379 5432)` default when none is given), and
  keeps its existing `PORT_CHECK_MODE`/`SKIP_INFRA_PORT_CHECK`
  behavior unchanged.
- Each service's dev port is declared once as a Makefile variable next
  to the rest of the `SVC=` plumbing (e.g. `PORT_feed-simulator = 8080`
  - picked arbitrarily, no doc precedent, open to change). `make run
  SVC=<name>` calls `check-infra-ports.sh $(PORT_<name>)` with only
  that one service's port before starting Air - never the whole
  registry, so one service's `make run` can't false-positive on a
  different, unrelated service's port.
- No Go/Air-vs-everything-else differentiation: this reuses exactly
  the same "any listener on the port is flagged" semantics already
  accepted for Redis/Postgres. Air's own `kill_delay` (2s) tears down
  the previous binary before rebuilding, so a normal hot-reload cycle
  has released the port by the time the check runs - no filter needed.

### Per-service README

- `services/feed-simulator/README.md` only, for this landing: its env
  vars (`PORT`, defaults to 8080 - picked arbitrarily, no doc
  precedent, open to change, same value as the Makefile's port-check
  variable above) and `make run SVC=feed-simulator`.
- No `services/README.md` yet - a shared-conventions doc (Huma's
  `GET /openapi.json`, common env vars, common flags) is written when
  Ingestion Service lands and there's a second real example to write
  it against, not against one hypothetical service today. Each
  service's own README stays self-contained until then, with a
  one-line link back to `services/README.md` added once it exists.

### OAS generation - explicitly deferred scope

Not bundling every service's OAS into one master spec to generate a
frontend client: architecturally, only the Gateway Service's REST
contract ever reaches the frontend (hard constraint, docs/ARCHITECTURE.md)
- Ingestion and Match Service OAS specs have no frontend consumer to
bundle for. Each Huma-using service just serves its own live spec.

## Out of scope (this spec)

- Real Feed Simulator -> Ingestion delivery (a later, separate landing)
- Any Ingestion/Match/Gateway business logic
- OpenTelemetry instrumentation (Phase 6)
- Containerization (Phase 7)

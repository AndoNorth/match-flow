---
status: draft
date: 2026-08-31
updated: 2026-08-31
issues: [3]
scope: [match-service]
---

# Match Service Design

## Problem

Ingestion Service (#2) publishes canonical events to Redis (`matchflow:events`),
but nothing subscribes to that channel. There is no system of record for "what is
currently true about a match" - no current score, no status, no event timeline -
and no API another service (Gateway, later) can query for that state. Events
land on Redis and are gone the instant nobody's listening.

## Goals

- A runnable Go service at `services/match-service/`, same `cmd/<service>/main.go`
  + `internal/` split as Ingestion Service.
- Subscribes to `matchflow:events`, decodes each message into the same
  `normalize.CanonicalEvent` shape Ingestion Service publishes. A shared
  contract test (fixture JSON, decoded/encoded on both sides) guards against
  the two hand-copied struct definitions silently drifting apart - see
  Validation.
- Maintains, per match: status (`scheduled`/`live`/`half_time`/`full_time`),
  home/away score, clock minute, and a full ordered event timeline - persisted
  in PostgreSQL via Goose migrations.
- A match row is created implicitly on first-seen `MatchID` - no separate
  match-registration flow. `sport` is set once at creation time from
  `MATCH_SERVICE_DEFAULT_SPORT`, never overwritten afterward.
- Applies events to Postgres via a hash-to-worker pool: `MatchID` hashed to one
  of N goroutine workers (`MATCH_SERVICE_WORKERS`, default 4), so events for the
  same match always apply in order on the same worker while different matches
  process in parallel. Send-to-worker blocks (backpressure) rather than
  drops when a worker's channel is full.
- Event-to-state-transition logic is sport-agnostic: a per-sport registry maps
  each `EventType` to a `Rule{Category, Status}` (`Category` ∈
  `MatchStart`/`PeriodBoundary`/`MatchEnd`/`ScoreEvent`/`Unknown`); the apply
  step only ever branches on `Category` and treats `Status` as an opaque value
  supplied by the registry, never comparing the original event-type string
  itself. Only a `football` registry exists this pass.
- `odds_update` events are decoded and discarded - not routed to a worker, not
  stored. Reserved for a future Odds Service subscribing to the same channel.
- `card` events map to `Unknown` - stored in the event timeline, no state
  transition of any kind (no status change, no score change, no clock update).
  Their betting relevance is future Odds Service scope.
- A per-match monotonic `last_sequence` guards against reprocessing: an
  incoming event whose `Sequence <= matches.last_sequence` for that match is
  skipped (logged, not applied); otherwise it's applied and `last_sequence` is
  advanced to it.
- Read-only REST API (Huma): `GET /matches`, `GET /matches/{id}`,
  `GET /matches/{id}/events`. Both `{id}` routes return `404` for an unknown
  `MatchID`.
- Graceful shutdown: `SIGTERM`/`SIGINT` cancels a context, the subscriber stops
  pulling new Redis messages and closes each worker's channel, each worker
  drains whatever's already buffered (applying with its own background
  context, not the cancelled one, so an in-flight transaction always
  completes) and exits when its channel closes, `main` blocks on a
  `sync.WaitGroup` until every worker has exited.
- Goose migrations run automatically at service startup, before the HTTP
  server and subscriber start - matching `DEVELOPMENT.md`'s "not a manual
  step" - and `goose` (the CLI) is added to `flake.nix`'s dev shell packages
  as part of this work, the first service needing it, so local rollback
  (`goose down`) is available the same way every other dev-shell tool is.
- `make build/run/test/test-integration/lint SVC=match-service` all work,
  reusing the Makefile/port-registry/README conventions from #1/#2.
  `PORT_match-service := 8082` (next free port after Ingestion's `8081`).

## Non-Goals

- gRPC API. ARCHITECTURE.md states Gateway -> Match Service should eventually
  be gRPC, but Gateway doesn't exist yet (#4) and there's no consumer to build
  a typed contract against today. REST-only for this pass; the gRPC pattern
  gets worked out when Gateway lands, against a real second caller instead of
  a guess.
- Gateway Service, SSE, or any client-facing realtime path - #4, Phase 5.
  Match Service does not push anything to anyone; Gateway independently
  subscribes to `matchflow:events` for the live stream and calls this
  service's REST API only for reconnect backfill (see ARCHITECTURE.md's
  Gateway Service section).
- Odds/betting logic of any kind - a future Odds Service, same
  independent-Redis-subscriber pattern as Match Service and Gateway, not
  Match Service's concern. `odds_update` and `card` events are explicitly
  parked for it, not partially handled here.
- Sport carried through the canonical event, and any code change to
  feed-simulator or ingestion-service that would require. Neither
  `DomainEvent` (feed-simulator) nor `CanonicalEvent` (ingestion) has a
  `sport` field today - only football exists, so nobody's needed one yet.
  This isn't solely Match Service's call to make (it touches two other
  services' wire shapes), so it's a Non-Goal here rather than an Out of
  Scope item this spec could revise on its own - see the note under Out of
  Scope for the distinction this document draws between the two sections.
- A second sport's registry (basketball, rugby, etc). The `Rule`/`Category`
  model is designed to support one without a schema change to the extent
  described under Out of Scope below, but no second sport exists in this
  codebase to build or test one against.
- A generic entity-profile/EAV abstraction for sport state. Considered and
  rejected - that pattern solves a problem MatchFlow doesn't have (an
  open-ended, runtime-configurable set of entity types). A fixed
  `matches`/`match_events` schema plus a per-sport `Rule` registry covers
  every team-ball sport in scope with less code and less indirection.
- Extracting `CanonicalEvent` into a shared Go module consumed by both
  Ingestion and Match Service. Real fix for the duplication risk noted above,
  but a cross-service module boundary is a bigger structural change than this
  spec's remit - the contract test is the cheap mitigation for now; revisit
  the shared-module question if a real drift incident happens or a third
  consumer shows up.
- Configurable per-worker channel buffer depth. One tuning knob
  (`MATCH_SERVICE_WORKERS`) is enough; buffer size is a fixed constant until a
  real need to tune it separately shows up.
- Retry/dead-letter handling for a failed Postgres write mid-event (as
  opposed to a skipped stale/duplicate event, which `last_sequence` handles
  deliberately, not as a failure). A write failure is logged and dropped,
  consistent with the project's existing "accept Redis pub/sub gaps" stance
  (ARCHITECTURE.md) rather than introducing a durability mechanism this
  project has deliberately excluded.
- OpenTelemetry instrumentation - Phase 6. This follows the same phasing
  Ingestion Service (#2) already used, and the same tension: `DEVELOPMENT.md`
  states services are "expected to emit OpenTelemetry traces, metrics, and
  logs from early in their development, not bolted on after the fact," which
  this spec knowingly doesn't do, for the same reason #2 gave - a proper OTel
  bootstrap (exporters, resource/propagator setup, trace-correlated logging,
  graceful shutdown) is real plumbing worth building once, uniformly, in
  Phase 6, not reinvented partially per service now.
- Containerization - Phase 7.
- Authentication/authorization - MatchFlow has no user accounts (GOALS.md).

## Architecture / Design

**Package layout**, mirroring #2's split:

- `cmd/match-service/main.go` - wiring only: load config, run Goose migrations,
  connect Postgres and Redis, build the worker pool, construct the Huma API,
  start the HTTP server, handle graceful shutdown.
- `internal/eventstream` - subscribes to `matchflow:events`, decodes each
  message into `normalize.CanonicalEvent` (same struct shape Ingestion
  publishes - duplicated here rather than imported, since the two services
  don't share a module boundary today; see the contract test in Validation
  and the Non-Goal above), drops `odds_update` and malformed messages, looks
  up the event's `Rule` in the active sport's registry, hashes `MatchID`
  (FNV-32) to pick a worker, sends the `(event, Rule)` pair onto that
  worker's channel.
- `internal/football` - the `football` sport package: `EventType -> Rule`
  registry, mirroring feed-simulator's `internal/simulator/football`
  vocabulary (`kickoff`, `goal`, `card`, `half_time`, `full_time`;
  `odds_update` never reaches this package since eventstream drops it first).
- `internal/matchstate` - `Category`/`Rule` types, a pure `Reduce` function
  (current state + `Rule` + event -> next state, no I/O), the worker pool,
  and the persistence wrapper that runs `Reduce`'s result inside one Postgres
  transaction (upsert `matches`, insert `match_events`). `Reduce` never
  compares the original event-type string, only `Rule.Category`/
  `Rule.Status`.
- `internal/api` - Huma route registration for the three read-only routes,
  reading from Postgres.
- `internal/healthz` - same tiny package pattern as the other two services.

**Data model** (`internal/matchstate/migrations`, Goose SQL):

```sql
CREATE TABLE matches (
    match_id      TEXT PRIMARY KEY,
    sport         TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'scheduled',
    home_score    INT NOT NULL DEFAULT 0,
    away_score    INT NOT NULL DEFAULT 0,
    clock_mins    INT NOT NULL DEFAULT 0,
    last_sequence INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE match_events (
    id          BIGSERIAL PRIMARY KEY,
    match_id    TEXT NOT NULL REFERENCES matches(match_id),
    sequence    INT NOT NULL,
    type        TEXT NOT NULL,
    payload     JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL,
    UNIQUE (match_id, sequence)
);
```

`matches` holds current derived state, one row per match. `sport` has no SQL
default deliberately - it's written exactly once, by the `INSERT` that
creates the row on first-seen `MatchID`, from `MATCH_SERVICE_DEFAULT_SPORT`.
That's the only place `sport` is ever set; nothing updates it afterward,
so there's exactly one source of truth for it, not two. `match_events` holds
the full ordered timeline, one row per event, for every event type including
`card` (no state transition, still recorded). `UNIQUE (match_id, sequence)`
is a second, DB-level guard against double-processing, on top of the
application-level `last_sequence` check described below. No sport-specific
columns and no per-sport table: score/status/clock/timeline generalize
across football, basketball, and rugby without a schema change for
pass/fail-style scoring - see the Out of Scope note on point-weighted
scoring (rugby tries, basketball threes) for where this generalization claim
stops.

**`last_sequence`**: the ordering/idempotency guard. Before applying an
event, the worker checks `event.Sequence <= matches.last_sequence` for that
`MatchID`; if true, the event is a duplicate or a late/out-of-order replay
and is logged and skipped, not applied. Otherwise it's applied and
`last_sequence` is set to `event.Sequence` in the same transaction. This is
deliberate, not a failure path - `Non-Goals` covers retry/dead-letter
handling for actual write *failures*, which is a different case.

**Category/Rule model** (`internal/matchstate`):

```go
type Category int

const (
    Unknown Category = iota
    MatchStart     // status -> Rule.Status
    PeriodBoundary // status -> Rule.Status
    MatchEnd       // status -> Rule.Status
    ScoreEvent     // increment home/away score from payload.team
)

// Rule is what a sport's registry maps each EventType to. Status is an
// opaque value the sport package chooses - matchstate never compares it to
// the original event type, only assigns it when Category calls for a
// status change. Only MatchStart/PeriodBoundary/MatchEnd read Status;
// ScoreEvent and Unknown ignore it.
type Rule struct {
    Category Category
    Status   string
}
```

```go
// internal/football/registry.go
var Registry = map[string]matchstate.Rule{
    "kickoff":   {Category: matchstate.MatchStart, Status: "live"},
    "half_time": {Category: matchstate.PeriodBoundary, Status: "half_time"},
    "full_time": {Category: matchstate.MatchEnd, Status: "full_time"},
    "goal":      {Category: matchstate.ScoreEvent},
    "card":      {Category: matchstate.Unknown},
}
```

This is what keeps `matchstate.Reduce` sport-agnostic while still letting
`half_time` and `full_time` (both `Status`-bearing, different literal
values) share the machinery: the *string* comes from a registry lookup
`eventstream` already did before the event reaches a worker, not from a type
comparison inside `matchstate` itself. A truly unrecognized `type` (absent
from the registry) resolves to the zero-value `Rule{}` (`Category: Unknown`),
same handling as `card` - stored, no transition.

`clock_mins` updates only for a `ScoreEvent` whose payload carries a
`minute` field (`goal` today). `card` carries no state transition of any
kind, including `clock_mins` - `Unknown` means *nothing* about `matches`
changes, only `match_events` gets a row. `kickoff`/`half_time`/`full_time`
also don't update `clock_mins` (no `minute` in their payload today) - a
known, deliberate gap, not solved this pass.

**Worker pool and graceful shutdown** (`internal/matchstate`):

```go
type workItem struct {
    event normalize.CanonicalEvent
    rule  matchstate.Rule
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
go func() { <-sigCh; cancel() }()

workerChans := make([]chan workItem, numWorkers)
var wg sync.WaitGroup
for i := range workerChans {
    workerChans[i] = make(chan workItem, bufferSize)
    wg.Add(1)
    go func(ch <-chan workItem) {
        defer wg.Done()
        for item := range ch {
            // A background context, not ctx: shutdown must never abort an
            // in-flight transaction - see below.
            apply(context.Background(), item.event, item.rule)
        }
    }(workerChans[i])
}

go func() { // subscriber
    for {
        select {
        case <-ctx.Done():
            for _, ch := range workerChans {
                close(ch)
            }
            return
        case msg := <-redisMessages:
            event, ok := decode(msg)
            if !ok || event.Type == "odds_update" {
                continue
            }
            rule := football.Registry[event.Type] // zero value if absent
            idx := fnv32(event.MatchID) % uint32(numWorkers)
            workerChans[idx] <- workItem{event: event, rule: rule} // blocks if full
        }
    }
}()

wg.Wait()
```

On `SIGTERM`, the subscriber stops pulling new Redis messages and closes
every worker channel instead of returning immediately. Each worker's `for
item := range ch` loop keeps draining whatever was already buffered -
applying each with its own `context.Background()` so a transaction already
in flight, or one about to start from a buffered item, always runs to
completion - and exits only once its channel is closed *and* drained. `main`
blocks on `wg.Wait()` until every worker has actually finished. No
half-applied event, no dropped buffered event, no abrupt kill.

Hashing `MatchID` (not round-robin) is what guarantees per-match ordering:
every event for a given match always lands on the same worker, whose apply
loop is strictly sequential. Different matches parallelize freely across
workers. This is standard in-process sharded fan-out - evaluated against
(and deliberately not adopting) the partition-per-process model used in
`~/workspace/rmq-kafka-flink-benchmark`'s Kafka/Redpanda consumer, which
achieves parallelism via broker-level partition assignment across separate
processes rather than in-process goroutines; that model doesn't fit a
single Redis pub/sub subscriber with no partition concept. The
`context.WithCancel` + `signal.Notify` half of the shutdown skeleton above
is adapted from that same benchmark repo's `consumer/main.go`; the
`sync.WaitGroup`-based drain is this design's own addition on top, since
that repo has no in-process worker pool of its own to drain.

**REST API** (Huma, read-only):

```
GET /matches              # list, optional ?status= filter
GET /matches/{id}         # sport, status, home_score, away_score, clock_mins
GET /matches/{id}/events  # ordered match_events timeline
```

Nothing creates or mutates a match through this API - only the Redis event
stream does that. Both `GET /matches/{id}` and `GET /matches/{id}/events`
return `404` for an unknown `MatchID` (the events route checks `matches`
for existence before querying `match_events`). A known match's `/events`
response is always a non-empty array in practice, since a match row is only
ever created by the same worker transaction that inserts its first
`match_events` row.

**Error handling**:

- Malformed Redis message (bad JSON) - log, drop, continue. Never crashes the
  subscriber.
- `event.Sequence <= matches.last_sequence` for that match - log at debug,
  skip, continue. Expected/deliberate, not an error.
- `Rule` resolves to `Category: Unknown` for a genuinely unrecognized `type`
  - same handling as `card`: stored, no transition, no error.
- Postgres write failure inside a worker's transaction - log with
  `match_id`/`sequence`, drop the event, continue to the next. No retry queue
  (see Non-Goals).

**Config** (env vars, matching Ingestion Service's plain-env-var pattern):

```
PORT                          # 8082, Makefile-registered as PORT_match-service
MATCH_SERVICE_WORKERS         # default 4
MATCH_SERVICE_DEFAULT_SPORT   # default "football"; written once per match at creation
REDIS_URL                     # default redis://localhost:6379
POSTGRES_DSN
```

## Validation

- **Unit** (Ginkgo/Gomega, no infra): `internal/football`'s registry is a
  plain map, asserted directly. `internal/matchstate.Reduce` - the pure
  state-transition function - gets table-driven specs per `Category` (state
  before/after for `MatchStart`, `PeriodBoundary`, `MatchEnd`, `ScoreEvent`,
  `Unknown`, and the `last_sequence` skip case), fed plain Go values with no
  database involved. `internal/eventstream`'s hashing, `odds_update`-drop,
  and registry-lookup logic are pure and unit-tested without Redis.
- **Contract** (Ginkgo/Gomega, no infra): a small fixture-JSON-based test
  shared in spirit with Ingestion Service's own `normalize` tests - encode a
  `normalize.CanonicalEvent` value with Ingestion's struct, decode the same
  JSON with Match Service's struct (and vice versa), assert the two round-trip
  identically. This is the cheap mitigation for the two services' hand-copied
  struct definitions drifting apart silently (see the Non-Goal on extracting
  a shared module) - it turns a silent decode-time failure into a loud,
  immediate test failure instead.
- **Integration** (`//go:build integration`, testcontainers-go spinning fresh
  Redis + Postgres per run, mirroring #2's pattern, migrations applied via
  the same startup path `cmd/match-service/main.go` uses): publish a sequence
  of canonical events (`kickoff`, `goal` x2, `card`, `half_time`, `goal`,
  `full_time`, `odds_update`) for one `MatchID` onto `matchflow:events`,
  assert via the REST API that `GET /matches/{id}` reflects the right final
  score/status/clock and `GET /matches/{id}/events` lists every non-odds
  event in order with the right count - proving decode, rule lookup, worker
  routing, and persistence together (`internal/matchstate`'s DB-writing
  wrapper around `Reduce`), not mocked. A second test publishes interleaved
  events for two different `MatchID`s and asserts both matches' final states
  are correct despite concurrent worker processing - the ordering guarantee
  under real parallelism, not just single-match happy path. A third
  publishes a duplicate/out-of-order `Sequence` for an already-applied event
  and asserts it's skipped (`last_sequence` unchanged, no new `match_events`
  row). A fourth requests `GET /matches/{unknown-id}` and
  `GET /matches/{unknown-id}/events` and asserts both `404`.
- A shutdown test: start the service, publish several events for one match in
  quick succession (enough to still be buffered in a worker's channel), send
  the process `SIGTERM`, and assert every buffered event was still applied
  (final state reflects all of them) before the process exited - proving the
  drain-on-close behavior, not just that shutdown doesn't crash.

## Open Questions

- Exact `MATCH_SERVICE_WORKERS` default (4 used as a placeholder) - tune
  during implementation, not load-bearing enough to block this spec.

## Out of Scope

Distinct from Non-Goals above: a Non-Goal is out of this document's remit
because something else already owns it - another service, another
documented phase in ROADMAP.md, or the architecture as documented. An Out of
Scope item is a design choice within Match Service's own remit that this
spec deliberately declines to make now, with no other document or service
already scheduled to make it - a candidate for revisiting within a future
revision of this same spec.

- Deriving `clock_mins` for `kickoff`/`half_time`/`full_time` events, which
  carry no `minute` in their payload today. Revisit if a real need for an
  always-accurate clock shows up (e.g. once Gateway needs to render a live
  ticking clock rather than the value from the last score event).
- Point-weighted scoring. `ScoreEvent` today means a flat +1 to the scoring
  team, which is exactly right for football's `goal` and is why the
  Non-Goals claim about generalizing "without a schema change" holds for
  football. A sport with variable-value scoring (rugby's try/conversion/
  penalty, basketball's 2s and 3s) would need `Rule` to carry a point value,
  which isn't designed here - deferred alongside the second-sport Non-Goal
  above, since there's nothing to build or test it against yet.

## References

- [docs/ARCHITECTURE.md#match-service](../../ARCHITECTURE.md#match-service)
- [docs/ARCHITECTURE.md#gateway-service](../../ARCHITECTURE.md#gateway-service) -
  SSE-default, Gateway-owns-the-Redis-subscription, backfill-from-Match-Service
  decisions this spec relies on and doesn't revisit
- [docs/ARCHITECTURE.md#cross-cutting-concerns](../../ARCHITECTURE.md#cross-cutting-concerns) -
  publisher/consumer schema coupling, the rationale behind this spec's
  contract test
- [docs/TECH_STACK.md](../../TECH_STACK.md) - Huma, Redis, Postgres, Goose,
  Ginkgo+Gomega
- [docs/DEVELOPMENT.md](../../DEVELOPMENT.md) - Goose migrations run as part
  of the standard workflow, not a manual step; OTel-from-early-development
  guidance this spec's OTel Non-Goal knowingly defers
- [docs/GOALS.md](../../GOALS.md) - non-goals (no user accounts)
- Issue #3 - Feat: Scaffold Match Service
- `services/ingestion-service/internal/normalize/normalize.go` - the
  canonical event shape this service decodes
- `services/ingestion-service/internal/eventbus/eventbus.go` - the
  `matchflow:events` channel this service subscribes to
- `services/feed-simulator/internal/simulator/football/football.go` - the
  event vocabulary (`kickoff`/`goal`/`card`/`odds_update`/`half_time`/
  `full_time`) and payload shapes this service's registry and apply logic
  are built against
- `~/workspace/rmq-kafka-flink-benchmark/consumer/main.go` - source of the
  `context.WithCancel`/`signal.Notify` half of the shutdown skeleton above
  (that repo has no `sync.WaitGroup`-based worker drain of its own - the
  drain logic is this design's own addition, not adapted from there)
- `docs/superpowers/specs/2026-08-30-ingestion-service-design.md` - prior
  spec, establishes the package-layout, Makefile conventions, and the
  Non-Goals/Out-of-Scope distinction this design reuses

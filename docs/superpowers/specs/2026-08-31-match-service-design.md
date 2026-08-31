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
  `normalize.CanonicalEvent` shape Ingestion Service publishes.
- Maintains, per match: status (`scheduled`/`live`/`half_time`/`full_time`),
  home/away score, clock minute, and a full ordered event timeline - persisted
  in PostgreSQL via Goose migrations.
- A match row is created implicitly on first-seen `MatchID` - no separate
  match-registration flow.
- Applies events to Postgres via a hash-to-worker pool: `MatchID` hashed to one
  of N goroutine workers (`MATCH_SERVICE_WORKERS`, default 4), so events for the
  same match always apply in order on the same worker while different matches
  process in parallel. Send-to-worker blocks (backpressure) rather than
  drops when a worker's channel is full.
- Event-to-state-transition logic is sport-agnostic: a per-sport registry maps
  each `EventType` to a `Category` (`MatchStart`/`PeriodBoundary`/`MatchEnd`/
  `ScoreEvent`/`Unknown`); the apply loop only ever branches on `Category`,
  never on a literal type string. Only a `football` registry exists this pass.
- `odds_update` events are decoded and discarded - not routed to a worker, not
  stored. Reserved for a future Odds Service subscribing to the same channel.
- `card` events map to `Unknown` - stored in the event timeline, no state
  transition. Their betting relevance is future Odds Service scope.
- Read-only REST API (Huma): `GET /matches`, `GET /matches/{id}`,
  `GET /matches/{id}/events`.
- Graceful shutdown: `SIGTERM`/`SIGINT` cancels a context, the subscriber stops
  pulling new messages, each worker finishes its in-flight transaction, `main`
  blocks on a `sync.WaitGroup` until every worker has exited.
- `make build/run/test/test-integration/lint SVC=match-service` all work,
  reusing the Makefile/port-registry/README conventions from #1/#2.

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
- Sport carried through the canonical event. Neither `DomainEvent`
  (feed-simulator) nor `CanonicalEvent` (ingestion) has a `sport` field today -
  only football exists, so nobody's needed one yet. Match Service's `sport`
  column defaults from `MATCH_SERVICE_DEFAULT_SPORT` (`football`) rather than
  being read per-event. Adding real per-event sport identification requires
  touching feed-simulator and ingestion-service too - out of this spec's remit.
- A second sport's registry (basketball, rugby, etc). The `Category` model is
  designed to support one without a schema change, but no second sport exists
  in this codebase to build or test one against.
- A generic entity-profile/EAV abstraction for sport state. Considered and
  rejected - that pattern solves a problem MatchFlow doesn't have (an
  open-ended, runtime-configurable set of entity types). A fixed
  `matches`/`match_events` schema plus a per-sport `Category` registry covers
  every team-ball sport in scope with less code and less indirection.
- Configurable per-worker channel buffer depth. One tuning knob
  (`MATCH_SERVICE_WORKERS`) is enough; buffer size is a fixed constant until a
  real need to tune it separately shows up.
- Retry/dead-letter handling for a failed Postgres write mid-event. Logged and
  dropped, consistent with the project's existing "accept Redis pub/sub gaps"
  stance (ARCHITECTURE.md) rather than introducing a durability mechanism this
  project has deliberately excluded.
- OpenTelemetry instrumentation - Phase 6, same phasing Ingestion Service (#2)
  already followed.
- Containerization - Phase 7.
- Authentication/authorization - MatchFlow has no user accounts (GOALS.md).

## Architecture / Design

**Package layout**, mirroring #2's split:

- `cmd/match-service/main.go` - wiring only: load config, connect Postgres and
  Redis, build the worker pool, construct the Huma API, start the HTTP server,
  handle graceful shutdown.
- `internal/eventstream` - subscribes to `matchflow:events`, decodes each
  message into `normalize.CanonicalEvent` (same struct shape Ingestion
  publishes - duplicated here rather than imported, since the two services
  don't share a module boundary today), drops `odds_update` and malformed
  messages, hashes `MatchID` (FNV-32) to pick a worker, sends onto that
  worker's channel.
- `internal/football` - the `football` sport package: `EventType ->
  Category` registry, mirroring feed-simulator's `internal/simulator/football`
  vocabulary (`kickoff`, `goal`, `card`, `half_time`, `full_time`;
  `odds_update` never reaches this package since eventstream drops it first).
- `internal/matchstate` - `Category` type, the worker pool, and the
  apply-one-event logic: given a `Category` and a `CanonicalEvent`, upsert
  `matches` and insert into `match_events` in one Postgres transaction. Pure
  branching on `Category`, no sport-specific string comparisons.
- `internal/api` - Huma route registration for the three read-only routes,
  reading from Postgres.
- `internal/healthz` - same tiny package pattern as the other two services.

**Data model** (`internal/matchstate/migrations`, Goose SQL):

```sql
CREATE TABLE matches (
    match_id      TEXT PRIMARY KEY,
    sport         TEXT NOT NULL DEFAULT 'football',
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

`matches` holds current derived state, one row per match. `match_events` holds
the full ordered timeline, one row per event, for every event type including
`card` (no state transition, still recorded). `UNIQUE (match_id, sequence)`
guards against double-processing. No sport-specific columns and no per-sport
table: score/status/clock/timeline generalize across football, basketball, and
rugby without a schema change - only the `Category` registry a new sport
supplies differs.

**Category model** (`internal/matchstate`):

```go
type Category int

const (
    Unknown Category = iota
    MatchStart     // status -> live
    PeriodBoundary // status -> event-specific (half_time)
    MatchEnd       // status -> full_time
    ScoreEvent     // increment home/away score from payload.team
)
```

```go
// internal/football/registry.go
var Registry = map[string]matchstate.Category{
    "kickoff":   matchstate.MatchStart,
    "half_time": matchstate.PeriodBoundary,
    "full_time": matchstate.MatchEnd,
    "goal":      matchstate.ScoreEvent,
    "card":      matchstate.Unknown,
}
```

`card` and any truly unrecognized `type` both map to (or default to)
`Unknown` - same category, same behavior: record the row, no state
transition. `clock_mins` updates only when a payload carries a `minute`
field (`goal`/`card` today); `kickoff`/`half_time`/`full_time` don't update it
- a known, deliberate gap, not solved this pass.

**Worker pool** (`internal/matchstate`):

```
eventstream (1 goroutine)
  redis.Subscribe(matchflow:events)
  for each message (or ctx.Done()):
    decode -> CanonicalEvent
    if type == "odds_update": continue
    category := football.Registry[event.Type]  // Unknown if absent
    workerIdx := fnv32(event.MatchID) % N
    workers[workerIdx].ch <- event  // blocks if full

worker (N goroutines, N = MATCH_SERVICE_WORKERS, default 4)
  for {
    select {
    case <-ctx.Done():
      return
    case ev := <-ch:
      apply(ctx, ev)  // one Postgres tx: upsert matches, insert match_events
    }
  }
```

Hashing `MatchID` (not round-robin) is what guarantees per-match ordering:
every event for a given match always lands on the same worker, whose apply
loop is strictly sequential. Different matches parallelize freely across
workers. This is standard in-process sharded fan-out - evaluated against (and
deliberately not adopting) the partition-per-process model used in
`~/workspace/rmq-kafka-flink-benchmark`'s Kafka/Redpanda consumer, which
achieves parallelism via broker-level partition assignment across separate
processes rather than in-process goroutines; that model doesn't fit a single
Redis pub/sub subscriber with no partition concept.

**Graceful shutdown**, adapted from that same benchmark repo's `consumer/main.go`
skeleton (`context.WithCancel` + `signal.Notify` + `sync.WaitGroup`), the one
piece of it that generalizes here:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
go func() { <-sigCh; cancel() }()

var wg sync.WaitGroup
for i := 0; i < numWorkers; i++ {
    wg.Add(1)
    go func(ch <-chan normalize.CanonicalEvent) {
        defer wg.Done()
        for {
            select {
            case <-ctx.Done():
                return
            case ev := <-ch:
                apply(ctx, ev)
            }
        }
    }(workerChans[i])
}
wg.Wait()
```

On `SIGTERM`, the subscriber stops pulling new messages and each worker
finishes whatever transaction it's mid-flight on before exiting - no
half-applied event, no abrupt kill.

**REST API** (Huma, read-only):

```
GET /matches              # list, optional ?status= filter
GET /matches/{id}         # sport, status, home_score, away_score, clock_mins
GET /matches/{id}/events  # ordered match_events timeline
```

Nothing creates or mutates a match through this API - only the Redis event
stream does that. `GET /matches/{id}` for an unknown ID returns `404`.

**Error handling**:

- Malformed Redis message (bad JSON) - log, drop, continue. Never crashes the
  subscriber.
- `Category` resolves to `Unknown` for a genuinely unrecognized `type` - same
  handling as `card`: stored, no transition, no error.
- Postgres write failure inside a worker's transaction - log with
  `match_id`/`sequence`, drop the event, continue to the next. No retry queue
  (see Non-Goals).

**Config** (env vars, matching Ingestion Service's plain-env-var pattern):

```
PORT                          # match-service's own port, Makefile-registered
MATCH_SERVICE_WORKERS         # default 4
MATCH_SERVICE_DEFAULT_SPORT   # default "football"
REDIS_URL                     # default redis://localhost:6379
POSTGRES_DSN
```

## Validation

- **Unit** (Ginkgo/Gomega, no infra): `internal/football`'s registry is a
  plain map, asserted directly. `internal/matchstate`'s apply logic gets
  table-driven specs per `Category` (state before/after for `MatchStart`,
  `PeriodBoundary`, `MatchEnd`, `ScoreEvent`, `Unknown`) against a real
  Postgres test database (state transitions are inherently a DB-write
  concern, not meaningfully unit-testable without one) - or, if a lighter
  seam is found during implementation, against an interface the transaction
  logic is written against. `internal/eventstream`'s hashing and
  `odds_update`-drop logic are pure and unit-tested without Redis.
- **Integration** (`//go:build integration`, testcontainers-go spinning fresh
  Redis + Postgres per run, mirroring #2's pattern): publish a sequence of
  canonical events (`kickoff`, `goal` x2, `card`, `half_time`, `goal`,
  `full_time`, `odds_update`) for one `MatchID` onto `matchflow:events`,
  assert via the REST API that `GET /matches/{id}` reflects the right final
  score/status/clock and `GET /matches/{id}/events` lists every non-odds
  event in order with the right count - proving decode, categorize, worker
  routing, and persistence together, not mocked. A second test publishes
  interleaved events for two different `MatchID`s and asserts both matches'
  final states are correct despite concurrent worker processing - the
  ordering guarantee under real parallelism, not just single-match happy path.
- A shutdown test: start the service, publish an event, send the process
  `SIGTERM` mid-flight (or simulate via context cancellation in-process), and
  assert the in-flight transaction still committed before exit.

## Open Questions

- Exact `MATCH_SERVICE_WORKERS` default (4 used as a placeholder) - tune
  during implementation, not load-bearing enough to block this spec.

## Out of Scope

- Deriving `clock_mins` for `kickoff`/`half_time`/`full_time` events, which
  carry no `minute` in their payload today. Revisit if a real need for an
  always-accurate clock shows up (e.g. once Gateway needs to render a live
  ticking clock rather than the value from the last score/card event).
- Carrying `sport` through the canonical event end-to-end (see Non-Goals) -
  candidate for a small follow-up touching feed-simulator, ingestion-service,
  and this spec together once a second sport is actually being added.

## References

- [docs/ARCHITECTURE.md#match-service](../../ARCHITECTURE.md#match-service)
- [docs/ARCHITECTURE.md#gateway-service](../../ARCHITECTURE.md#gateway-service) -
  SSE-default, Gateway-owns-the-Redis-subscription, backfill-from-Match-Service
  decisions this spec relies on and doesn't revisit
- [docs/TECH_STACK.md](../../TECH_STACK.md) - Huma, Redis, Postgres, Goose,
  Ginkgo+Gomega
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
  graceful-shutdown skeleton adapted above
- `docs/superpowers/specs/2026-08-30-ingestion-service-design.md` - prior
  spec, establishes the package-layout and Makefile conventions this design
  reuses

---
status: approved
date: 2026-08-30
issues: [2]
---

# Ingestion Service Design

## Problem

Feed Simulator (#1) generates realistic match events and encodes each one
into two structurally different provider wire shapes (`ProviderA`,
`ProviderB`), but today it can only log them.
There is no service that receives those payloads, no normalization step
that reconciles the two shapes into one canonical event, and no
publication into Redis - so nothing downstream (Match Service, later)
has anything to consume.
The gap this closes: turn "Feed Simulator logs events to a terminal" into
"Feed Simulator submits real events over HTTP to a service that validates,
normalizes, and distributes them."

## Goals

- A runnable Go service at `services/ingestion-service/` following the
  `cmd/<service>/main.go` + `internal/` split locked in by #1.
- A Huma REST endpoint receives events from Feed Simulator over real
  HTTP; `GET /openapi.json` serves a live spec.
- Both `ProviderA` and `ProviderB` wire shapes normalize to one identical
  canonical event (same `MatchID`/`Sequence`/`Type`/`Timestamp`/`Payload`),
  regardless of which shape arrived.
- Malformed core fields are rejected, no silent coercion - using Huma's
  own default status codes rather than fighting them: unparseable JSON
  or an unparseable timestamp is `400 Bad Request`; a well-formed JSON
  body that fails schema validation (missing `MatchID`, wrong-typed
  `Sequence`, empty `Type`) is `422 Unprocessable Entity`. `Payload`
  stays an opaque, unvalidated map (sport-specific, not this service's
  concern).
- Every normalized event is published to a single Redis pub/sub channel.
- Feed Simulator's `Runner` is updated to POST each event to Ingestion
  (alongside its existing logging, not instead of it), proving the real
  Feed Simulator -> Ingestion hop end to end.
- `make build/run/test/lint SVC=ingestion-service` all work, reusing the
  Makefile `PORT_<svc>` / port-guard / `services/<name>/README.md`
  conventions from #1.
- `make test-integration SVC=ingestion-service` spins a fresh Redis via
  testcontainers-go and passes - first entry of `testcontainers-go` into
  `go.mod`.

## Non-Goals

- Match Service consuming from Redis - #3.
- OpenTelemetry instrumentation - Phase 6.
  A proper OTel bootstrap is real plumbing, not a one-line addition:
  OTLP trace/metric exporters, resource and propagator setup, a
  trace-correlated log handler, and graceful shutdown all need to exist
  and be gotten right once. Retrofitting it uniformly across every
  MatchFlow service in Phase 6 beats each service inventing its own
  partial version now.
  This knowingly follows ROADMAP.md's Phase-6 phasing and Feed
  Simulator's (#1) own precedent, over DEVELOPMENT.md's "expected to
  emit... from early in development, not bolted on after the fact"
  guidance - a pre-existing tension in the project's own phasing, not
  one this spec introduces or resolves.
- Containerization - Phase 7.
- Durable persistence of events. Ingestion is a stateless hop: validate,
  normalize, publish, forget. No database, no write-ahead log, no
  dead-letter queue. Postgres and durable state belong solely to Match
  Service (Phase 4), per ARCHITECTURE.md's ownership rule.
- Redis Streams, consumer groups, or replay guarantees. ARCHITECTURE.md
  already commits to plain pub/sub semantics ("delivers to whoever is
  subscribed at publish time... keeps nothing") - Streams would
  contradict the documented model, not extend it.
- Validating or interpreting `Payload` contents - it is sport-specific
  and opaque to Ingestion by design.
- Authentication/authorization on the endpoint - MatchFlow has no user
  accounts anywhere in the system (GOALS.md non-goals).

## Architecture / Design

**Package layout**, mirroring #1's split:

- `cmd/ingestion-service/main.go` - wiring only: load config, construct
  Redis publisher, construct Huma API, construct HTTP server, start,
  handle graceful shutdown (same `signal.NotifyContext` +
  `server.Shutdown` pattern as Feed Simulator's `main.go`).
- `internal/api` - Huma route registration and typed request/response
  structs, one route per provider shape.
- `internal/normalize` - takes a common `Input` struct (`MatchID`,
  `Sequence`, `Type`, `Timestamp time.Time`, `Payload`) that both route
  handlers build identically after their own Huma-tag validation and
  timestamp conversion, and attaches provenance (`Provider`,
  `IngestedAt`) to produce the canonical event. Pure function, no I/O, no
  rejection logic of its own - by the time a value reaches this package,
  both providers' route-specific differences (wire shape, timestamp
  format) are already resolved into the same `Input` shape. Fully
  unit-testable in isolation.
- `internal/eventbus` - thin Redis publisher: one method,
  `Publish(ctx, CanonicalEvent) error`, JSON-encodes and `PUBLISH`s to the
  fixed channel constant. Publisher failure surfaces as `5xx` from the
  handler - Redis is Ingestion's only output, not an optional cache, so
  the service treats it as a hard dependency and fails startup if the
  initial connection can't be established.
- `internal/healthz` - same tiny package pattern as Feed Simulator's.

**Two routes, not one content-sniffing route.** Feed Simulator's `Runner`
already knows which encoder produced a given payload (it alternates
deterministically), so it can POST to the matching route directly:

- `POST /events/provider-a` - decodes `ProviderA`'s nested/abbreviated
  shape (`data.mid`, `data.seq`, `data.typ`, `data.ts`, `data.pl`).
- `POST /events/provider-b` - decodes `ProviderB`'s flat shape
  (`match_id`, `sequence`, `event_type`, `occurred_at`, `details`).

**Huma owns validation, exclusively - using its own default status
codes, not a fixed `400` everywhere.** Two typed Huma request structs
(one per route) carry `required`/type validation tags for `MatchID`,
`Sequence`, `Type`, and the timestamp field. Huma rejects a body it
can't even parse as JSON with `400 Bad Request`; a body that parses but
fails schema validation (missing/wrong-typed field) gets Huma's own
`422 Unprocessable Entity`. `internal/normalize` never rejects anything;
it only runs on input Huma has already accepted. This is also where the
two providers' differing timestamp wire formats are pinned down
explicitly:

- `ProviderA`'s request struct: `TS int64` (Unix seconds - matches
  `providers.go`'s `e.Timestamp.Unix()` encoding).
- `ProviderB`'s request struct: a plain `OccurredAt string` field,
  parsed manually in the handler with `time.Parse(time.RFC3339, ...)`
  rather than a Huma format-validation tag - a schema-level tag would
  make Huma reject a bad timestamp with `422` instead of the `400` this
  design requires (matches `providers.go`'s
  `e.Timestamp.Format(time.RFC3339)` encoding).

Each route's handler converts its own struct's timestamp field
(`time.Unix(ts, 0)` or `time.Parse(time.RFC3339, ...)`) into `Input`'s
`time.Time` field before calling `normalize` - the format difference is
resolved at the route boundary, producing the same `Input` shape
regardless of which provider it came from, not inside `normalize`.

**Canonical event** (what gets published to Redis, JSON-encoded):

```
MatchID    string
Sequence   int
Type       string
Timestamp  time.Time   // converted from the route's own wire format (see above) before reaching this struct
Payload    map[string]any  // opaque passthrough, untouched
Provider   string      // "provider-a" | "provider-b" - which route it arrived on
IngestedAt time.Time   // server-set on receipt, not from the wire payload
```

`Provider` and `IngestedAt` are the provenance fields added in this
design specifically to make "which shape did this come in as, and when
did Ingestion see it" answerable from Redis alone, without correlating
back to Feed Simulator's own logs.

**Redis client**: `github.com/redis/go-redis/v9` - the de facto standard
Go Redis client, not yet in `go.mod`. Connects once at startup using a
`REDIS_URL` env var (default `redis://localhost:6379`, matching
`docker-compose.dev.yml`'s single-instance Redis - same credentials/ports
convention DEVELOPMENT.md already establishes for both dev loops).

**Config**: plain env vars, no config framework - matches Feed
Simulator's existing `os.Getenv("PORT")` pattern exactly. Adds
`REDIS_URL` on Ingestion's side; the channel name stays a Go constant,
not an env var (one channel is all this needs, e.g. `matchflow:events`).
Ingestion's own port follows the same `PORT` env var pattern as Feed
Simulator, registered in the Makefile as `PORT_ingestion-service := 8081`
(next free port after Feed Simulator's `8080`).

**Feed Simulator changes**: `Runner.Run` gains a submit step after
encoding, POSTing the payload to whichever route matches the encoder
just used, via a small HTTP client the `Runner` is constructed with
(injected, so unit tests still pass a `nil`/no-op submitter or a
`httptest.Server`). The client is configured with an `INGESTION_URL` env
var, read in Feed Simulator's `main.go` (default
`http://localhost:8081`, matching Ingestion's port above). Existing
`logger.Info("event", ...)` line stays -
submission failures log an error and continue the loop rather than
stopping the simulation (Feed Simulator's job is to keep generating
events; Ingestion being briefly down shouldn't kill the generator).

## Validation

- **Unit** (Ginkgo/Gomega, no infra): `internal/normalize` gets
  table-driven specs feeding an `Input` value directly (bypassing HTTP
  entirely, since `Input` is already provider-agnostic by construction)
  and asserting the output canonical event carries the right `Provider`
  and a set `IngestedAt`. `normalize` has no rejection paths to test - it
  only ever receives Huma-validated, already-converted input. Separately,
  each route handler in `internal/api` gets its own table-driven specs
  (using real `ProviderA`/`ProviderB` JSON fixtures reusing the same
  field shapes `services/feed-simulator` already defines) asserting it
  builds the correct `Input` value from its provider's wire shape - this
  is where the two providers' actual decode/timestamp-conversion logic is
  exercised.
- **Integration** (`//go:build integration`, testcontainers-go spinning a
  fresh Redis per run - independent of `make dev-infra` being up): POST
  a valid payload, a schema-invalid payload (missing `MatchID`, bad
  `Sequence`, empty `Type`), and an unparseable-timestamp payload, to
  both routes against a real HTTP server. Valid payloads assert the
  canonical event arrives on the subscribed Redis channel; schema-invalid
  payloads assert Huma's `422 Unprocessable Entity`; unparseable-JSON or
  unparseable-timestamp payloads assert `400 Bad Request`. All rejection
  cases also assert nothing was published. This is the contract proof -
  request validation, normalization, and the Redis hop together, not
  mocked.
- Feed Simulator's `Runner` gets a unit spec asserting it POSTs to the
  correct route per encoder in the alternation, using an injected
  `httptest.Server` double - no real Ingestion process needed for this
  test.

## E2E / Manual QA

One-time manual verification, not an automated suite (see main
conversation: unit + integration already cover the contract; the only
thing left to prove is "two real processes actually talk over HTTP,"
which isn't worth automating at this scale):

1. `make dev-infra` (Redis up).
2. `make run SVC=ingestion-service`.
3. `make run SVC=feed-simulator`.
4. `redis-cli SUBSCRIBE matchflow:events` in a separate terminal.
5. Confirm normalized events appear on the channel as the simulated match
   plays out, alternating `Provider: "provider-a"` / `"provider-b"` every
   other event.

`redis-cli SUBSCRIBE` is sufficient here - no Grafana/otel-lgtm needed.
That stack is Phase 6's concern once services actually emit traces and
metrics into it; it also isn't a Redis pub/sub inspector even once wired,
so it would be the wrong tool for this check at any phase.

### Observed results (2026-08-30)

Ran the procedure above with real processes: `make dev-infra`, then
`make run SVC=ingestion-service` and `make run SVC=feed-simulator` each
in the background, then `redis-cli SUBSCRIBE matchflow:events` (wrapped
in `timeout 12` to capture a bounded sample instead of hanging).

Messages appeared on `matchflow:events` within seconds of both services
being up. Representative sample:

```
{"payload":{"minute":14,"team":"home"},"timestamp":"2026-08-30T22:57:50+01:00","ingested_at":"2026-08-30T22:57:50.349590587+01:00","match_id":"match-1","type":"goal","provider":"provider-b","sequence":14}
{"payload":{"market":"match_winner","price":2.7685299153023526,"selection":"home"},"timestamp":"2026-08-30T22:57:55+01:00","ingested_at":"2026-08-30T22:57:55.348965654+01:00","match_id":"match-1","type":"odds_update","provider":"provider-a","sequence":19}
{"payload":{"minute":22,"team":"home"},"timestamp":"2026-08-30T22:57:58+01:00","ingested_at":"2026-08-30T22:57:58.350781569+01:00","match_id":"match-1","type":"goal","provider":"provider-b","sequence":22}
```

All expected canonical-event fields were present on every message:
`match_id`, `sequence`, `type`, `timestamp`, `payload`, `provider`, and
`ingested_at`. The `provider` field alternated `provider-a`/`provider-b`
across consecutive events, matching Feed Simulator's alternation by
sequence parity. Feed Simulator's own stdout logged an `event` line for
every generated event and contained zero `submit event failed` lines
across the run, confirming every POST to Ingestion Service was accepted.

One thing the automated layers couldn't have caught, and that isn't a
defect: `redis-cli SUBSCRIBE` only sees messages published *after* it
connects, so the very first event (sequence 1, `kickoff`, generated a
few seconds before the subscriber attached) never appeared in the
sample window even though Ingestion Service logged no error handling
it. Anyone repeating this check should read a short gap between
starting the services and starting the subscriber as expected, not as a
dropped event.

Teardown (`kill` the two background processes, `make dev-infra-down`)
completed cleanly with no lingering `air`/service processes and no
errors from Docker Compose.

## Open Questions

- Exact channel name (`matchflow:events` used as a placeholder above) -
  pick during implementation, not load-bearing enough to block this spec.

## Out of Scope

Distinct from Non-Goals above: a Non-Goal is out of this document's
remit because something else already owns it - another service, another
documented phase in ROADMAP.md, or the architecture as documented (that
"something else" may itself revisit it later; this spec still won't). An
Out of Scope item is a design choice within Ingestion Service's own
remit that this spec deliberately declines to make now, with no other
document already scheduled to make it - a candidate for revisiting
within a future revision of this same spec, not a different one.

- **Config-driven provider decoding.** Discussed and deferred: a data
  file pairing an emulated provider shape to a decoding path, so adding a
  third external-provider wire format wouldn't require a code change.
  Real future win, but no third provider exists or is planned, and it's
  a separate axis from supporting a new sport - sport/domain vocabulary
  is already pluggable via the `domain.Sport` interface (a new package
  per sport), independent of how a provider's payload is physically
  encoded. Two hardcoded provider shapes (typed Huma request structs
  mirroring `ProviderA`/`ProviderB`) are enough for now; revisit
  config-driven decoding only if/when a real third shape shows up.

## References

- [docs/ARCHITECTURE.md#ingestion-service](../../ARCHITECTURE.md#ingestion-service)
- [docs/TECH_STACK.md](../../TECH_STACK.md) - Huma, Redis, Ginkgo+Gomega
- [docs/GOALS.md](../../GOALS.md) - non-goals (no Kafka/NATS-style broker, no user accounts)
- Issue #2 - Feat: Scaffold Ingestion Service
- `services/feed-simulator/internal/simulator/providers/providers.go` -
  the two provider wire shapes this service normalizes
- `services/feed-simulator/internal/simulator/runner.go` - where the
  POST-submission step gets added
- `docs/superpowers/specs/2026-08-30-feed-simulator-and-service-dev-loop-design.md` -
  prior spec, establishes the Makefile/port-registry/README conventions
  this design reuses

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
- Malformed core fields (missing/invalid `MatchID`, `Sequence`, `Type`,
  unparseable timestamp) are rejected with `400` - no silent coercion.
  `Payload` stays an opaque, unvalidated map (sport-specific, not this
  service's concern).
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
  Reference implementation reviewed for scale (`packages/common/otel` in
  a sibling codebase): OTLP exporters, resource/propagator setup, a
  trace-correlated slog handler, panic-recovering exporter wrappers,
  Go runtime metrics - real plumbing, not a one-line addition.
  Retrofitting it once, uniformly, across every MatchFlow service in
  Phase 6 beats each service inventing its own partial version now.
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
- `internal/normalize` - decode + validate each provider's wire shape,
  attach provenance, produce the canonical event. Pure functions, no I/O,
  fully unit-testable in isolation.
- `internal/eventbus` - thin Redis publisher: one method,
  `Publish(ctx, CanonicalEvent) error`, JSON-encodes and `PUBLISH`s to the
  configured channel. Publisher failure surfaces as `5xx` from the
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

Two typed Huma request structs (one per route) with validation tags give
Huma's own schema validation the strict-reject behavior for free -
no hand-rolled sniffing logic, no ambiguity about which shape a payload
is in.

**Canonical event** (what gets published to Redis, JSON-encoded):

```
MatchID    string
Sequence   int
Type       string
Timestamp  time.Time   // parsed from either provider's own timestamp format
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
`REDIS_URL` and the channel name (constant, not configurable - one
channel is all this needs, e.g. `matchflow:events`).

**Feed Simulator changes**: `Runner.Run` gains a submit step after
encoding, POSTing the payload to whichever route matches the encoder
just used, via a small HTTP client the `Runner` is constructed with
(injected, so unit tests still pass a `nil`/no-op submitter or a
`httptest.Server`). Existing `logger.Info("event", ...)` line stays -
submission failures log an error and continue the loop rather than
stopping the simulation (Feed Simulator's job is to keep generating
events; Ingestion being briefly down shouldn't kill the generator).

## Validation

- **Unit** (Ginkgo/Gomega, no infra): `internal/normalize` gets
  table-driven specs feeding real `ProviderA`/`ProviderB` JSON fixtures
  (reusing the same encode/decode shapes `services/feed-simulator`
  already defines) and asserting both produce an identical canonical
  event apart from `Provider`. Separate specs cover each required-field
  rejection case (missing `MatchID`, bad `Sequence`, empty `Type`,
  unparseable timestamp) for both routes.
- **Integration** (`//go:build integration`, testcontainers-go spinning a
  fresh Redis per run - independent of `make dev-infra` being up):
  POST a valid payload to each route against a real HTTP server, then
  assert the canonical event actually arrives on the subscribed Redis
  channel. This is the contract proof - normalization plus the Redis
  hop together, not mocked.
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

## Open Questions

- Exact channel name (`matchflow:events` used as a placeholder above) -
  pick during implementation, not load-bearing enough to block this spec.

## References

- [docs/ARCHITECTURE.md#ingestion-service](../../ARCHITECTURE.md#ingestion-service)
- [docs/TECH_STACK.md](../../TECH_STACK.md) - Huma, Redis, Ginkgo+Gomega, testcontainers-go
- [docs/GOALS.md](../../GOALS.md) - non-goals (no Kafka/NATS-style broker, no user accounts)
- Issue #2 - Feat: Scaffold Ingestion Service
- `services/feed-simulator/internal/simulator/providers/providers.go` -
  the two provider wire shapes this service normalizes
- `services/feed-simulator/internal/simulator/runner.go` - where the
  POST-submission step gets added
- `docs/superpowers/specs/2026-08-30-feed-simulator-and-service-dev-loop-design.md` -
  prior spec, establishes the Makefile/port-registry/README conventions
  this design reuses

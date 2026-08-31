---
status: draft
date: 2026-08-31
updated: 2026-08-31
issues: [4, 9]
scope: [gateway-service, match-service, proto]
---

# Gateway Service + Match Service gRPC API

## Problem

MatchFlow has no client-facing entry point yet.
Match Service (#3) owns match state and exposes it over REST, but nothing else in the system can
reach it except through that REST API, and nothing exists to translate internal service shapes
into a stable client-facing contract or to push realtime updates to a browser.
The Frontend Application (#5) has nothing to build against until this exists.

Separately, Match Service's REST API was built before the Gateway existed to decide what
contract it needed, so it has no typed, service-to-service contract - only a REST/JSON surface
meant for eventual client consumption.
A second backend domain service (an Odds Service) is planned, and the realtime fan-out pattern
needs to be proven now, with one domain service, so it demonstrably extends to a second one
without a redesign.

## Goals

- Match Service exposes `ListMatches`, `GetMatch`, and `ListMatchEvents` over gRPC
  (connect-go), returning data equivalent to its existing REST routes, without changing or
  removing those REST routes.
- Gateway Service runs as its own Go service (`services/gateway-service`) with
  `GET /healthz`, and REST routes `GET /matches`, `GET /matches/{id}`, `GET /matches/{id}/events`
  that mirror Match Service's three reads, served via Huma.
- A resolver layer is the only place that knows both the Gateway's REST/JSON shapes and Match
  Service's protobuf messages; route handlers never see protobuf types directly.
- A gRPC-to-HTTP error mapping in one place turns Match Service's `NOT_FOUND` into the Gateway's
  `404`, `INVALID_ARGUMENT` into `400`, and `UNAVAILABLE` into `503`, so no route hand-rolls its
  own mapping.
- Gateway exposes an SSE endpoint that subscribes directly to Ingestion Service's
  `matchflow:events` Redis channel and re-emits each event only to clients that requested that
  match (or to every client that requested no specific match), backfilling current match state
  via the gRPC client on (re)connect.
- `buf` + `connect-go` codegen is established under `proto/matchflow/<service>/v1/` and
  `gen/go/matchflow/<service>/v1/`, in a shape a future Odds Service can reuse without inventing a second
  convention.
- The SSE endpoint can be verified manually with `curl -N` alone, since no frontend client
  exists yet.

## Non-Goals

- Building the Odds Service itself. This spec only needs to prove the fan-out pattern
  (see [Extending to a second domain service](#extending-to-a-second-domain-service)) extends to
  a second publisher - it does not require shipping one.
- Frontend consumption of any Gateway endpoint - tracked separately in #5.
- OpenTelemetry instrumentation (Phase 6) or containerization (Phase 7) for either service.
- Any mutation endpoint on Match Service or the Gateway - both stay read-only from the outside;
  match state changes only via the Redis event stream Match Service already consumes.
- WebSocket support - SSE is the realtime protocol per
  [ARCHITECTURE.md](../../ARCHITECTURE.md#gateway-service); revisit only if a bidirectional
  need appears.

## Architecture / Design

### Proto and codegen

`proto/matchflow/match_service/v1/match_service.proto` (`syntax = "proto3"`) defines a
`MatchService` with three RPCs mirroring the existing REST routes:

- `ListMatches(ListMatchesRequest{status: string}) -> ListMatchesResponse{matches: []Match}`
- `GetMatch(GetMatchRequest{match_id: string}) -> Match`
- `ListMatchEvents(ListMatchEventsRequest{match_id: string}) -> ListMatchEventsResponse{events: []MatchEvent}`

`Match` mirrors `matchBody` in `internal/api/api.go`: `match_id, sport, status, home_score,
away_score, clock_mins`.
`MatchEvent` mirrors `eventBody`: `type, sequence`, and `payload` typed as
`google.protobuf.Struct` - the native proto representation for the arbitrary
`map[string]any` JSON the event payload already is, so the schema stays honest about that field
instead of hiding it behind an opaque string.
Not-found reads return the gRPC `NOT_FOUND` status, matching the REST routes' `404`.

`buf.gen.yaml` (Go + connect-go plugins) lives at the repo root. `buf.yaml` defines a single buf
module rooted at `proto/` (a buf v2 `modules: [{path: proto}]` entry in the root config, or
`buf.yaml` placed directly inside `proto/` - either way the module's root is `proto/`, not the
repo root). Import paths inside `.proto` files are therefore written relative to `proto/`, e.g.
`import "matchflow/match_service/v1/match_service.proto";`, never prefixed with `proto/` -
the one convention a future Odds Service's `.proto` files must follow to avoid a second,
inconsistent import shape. Generated Go lands in `gen/go/matchflow/match_service/v1/` as plain packages in
the existing single Go module (`github.com/AndoNorth/match-flow`) - no per-package `go.mod`
needed, unlike the multi-module reference this convention was adapted from, because that module
split exists there to let many independent external consumers pin different versions across repo
boundaries, a problem two services in one repo don't have.
Generated code is checked in; a `make gen-proto` target runs `buf generate`, and a checkout
builds without invoking the proto toolchain.

### Match Service: adding gRPC alongside REST

A new connect-go handler in `services/match-service/internal/api` implements the generated
`MatchServiceHandler` interface against the same `reader` interface (`matchstate.Store`) the
REST routes already use - no new data-access code, just a second protocol wrapping the same
reads.
connect-go's generated handler is a plain `http.Handler` mounted at its own path prefix
(`/matchflow.match_service.v1.MatchService/`), so it's registered on the same `http.ServeMux`
Match Service's `main.go` already builds, on the same `PORT` - no second listener, no second
port to configure or firewall.

### Gateway Service

`services/gateway-service/cmd/gateway-service/main.go` + `internal/` follows the same shape as
Feed Simulator, Ingestion Service, and Match Service: env-var-only config, `http.ServeMux`,
Huma for REST, graceful shutdown on `SIGTERM`/`SIGINT`.

- `internal/matchclient` - thin connect-go client wrapping the three RPCs, address from
  `MATCH_SERVICE_ADDR`, defaulting to `localhost:8082` for local dev - matching the same
  local-first default convention `REDIS_URL`/`POSTGRES_DSN` already use in Ingestion Service and
  Match Service. The Kubernetes-DNS-shaped hostname
  [ARCHITECTURE.md](../../ARCHITECTURE.md#gateway-service) describes for the in-cluster case is
  supplied as an override once a deployment manifest sets it (Phase 8, out of scope here), not a
  compiled-in default.
- `internal/resolver` - converts between REST/JSON request/response shapes (including the SSE
  snapshot payload - see `GET /events` below) and `matchclient`'s protobuf types, and maps gRPC
  status codes to HTTP status codes (`NOT_FOUND` -> 404, `INVALID_ARGUMENT` -> 400,
  `UNAVAILABLE`/`DEADLINE_EXCEEDED` -> 503, anything else -> 500). Both `internal/api` and the SSE
  handler call the resolver and never import the generated protobuf types directly.
- `internal/api` - the three Huma routes (`GET /matches`, `GET /matches/{id}`,
  `GET /matches/{id}/events`), structurally identical to Match Service's own `api.go` but backed
  by `matchclient` + `resolver` instead of a direct store - including the same optional
  `?status=` query parameter `GET /matches` forwards to `ListMatchesRequest.status` via the
  resolver.
- `internal/realtime` - owns one Redis subscription to `matchflow:events` and a registry of
  connected SSE clients, keyed by client ID, each entry holding a buffered `chan []byte` and the
  `match_id` that client requested (guarded by a `sync.RWMutex`). The subscriber goroutine reads
  one message at a time, decodes its match ID, and fans it out only to clients whose requested
  `match_id` matches (or to every client if a client requested none, i.e. "all matches"); a
  client whose channel is full (a slow consumer) has its message dropped rather than blocking
  delivery to every other client - one Redis subscription total, not one per client, and no
  client sees another match's events unless it asked for all of them.
- `GET /events` (SSE) - registers a client with `internal/realtime`, writes an initial snapshot
  as an `event: snapshot` frame (fetched via `matchclient` and passed through the resolver: the
  current match's state for the requested `match_id`, or all matches if none given, in the same
  JSON shape `GET /matches`/`GET /matches/{id}` return), then streams every subsequent message
  from the client's channel as an `event: update` frame (a single canonical event, resolver-shaped
  the same way `GET /matches/{id}/events` returns them) until the client disconnects. The two
  named SSE event types let a client (e.g. `EventSource.addEventListener`) distinguish a full
  snapshot from an incremental update without inspecting payload shape. Mirrors the
  snapshot-then-deltas shape real market-data streams use (e.g. Betfair's Stream API - see
  References) independent of the transport choice.
- `internal/healthz` - identical to the other services' `GET /healthz`.

### Extending to a second domain service

When an Odds Service exists, it publishes to its own Redis channel the same way Match Service's
event producers do, and `internal/realtime` adds a second subscription the same way it holds the
first - no new fan-out mechanism, no new RPC shape, per
[ARCHITECTURE.md](../../ARCHITECTURE.md#multi-service-realtime-fan-out).
Whether that's a second named channel or a pattern-subscribe is an open question below.

### Makefile / environment

- `PORT_gateway-service := 8083` added to the root `Makefile` alongside the existing
  `PORT_match-service`/`PORT_ingestion-service` entries.
- Gateway env vars: `PORT` (default `8083`), `REDIS_URL` (default
  `redis://localhost:6379`), `MATCH_SERVICE_ADDR` (default `localhost:8082`).

## Validation

- **Match Service**: Ginkgo/Gomega unit tests for the new connect-go handler, mirroring the
  existing REST route tests (`api_test.go`) against the same `reader` fakes - same inputs, same
  fakes, asserting on the protobuf response instead of the JSON body. An integration test
  (`-tags=integration`) exercises a real connect-go client against the running server, alongside
  the existing REST integration test.
- **Gateway**: unit tests for `internal/resolver`'s REST<->protobuf conversion and its gRPC-to-HTTP
  status mapping, using a fake `matchclient`. Unit tests for `internal/realtime`'s fan-out
  registry (a published message reaches only clients registered for that `match_id`, plus any
  client registered for "all matches"; a full client channel doesn't block delivery to others). An
  integration test builds and runs the real `match-service` binary as an OS subprocess (via `go
  build` against its full package path, since its internals live under its own `internal/` tree
  and are unreachable from the Gateway's) alongside testcontainers Postgres and Redis, then
  exercises the three REST routes end-to-end against it through the Gateway's real REST server
  over actual HTTP/gRPC.
- SSE streaming itself is covered by a unit test using `httptest.NewRecorder` (a `http.Flusher`
  fake) asserting the handler writes one `event: update` frame per published Redis message, and
  by the manual `curl -N` check below - there is no automated way to assert on real browser
  `EventSource` reconnect behavior without a frontend client, which is exactly why the manual
  check exists.

## References

- [ARCHITECTURE.md#gateway-service](../../ARCHITECTURE.md#gateway-service)
- [ARCHITECTURE.md#multi-service-realtime-fan-out](../../ARCHITECTURE.md#multi-service-realtime-fan-out)
- [ARCHITECTURE.md#protobuf--grpc-structure](../../ARCHITECTURE.md#protobuf--grpc-structure)
- Issue #4 (Gateway Service), #9 (Match Service gRPC API)
- `services/match-service/internal/api/api.go` - the REST routes and DTOs this spec mirrors
- [Betfair: Market & Order Stream API - How it works](https://support.developer.betfair.com/hc/en-us/articles/360000402291-Market-Order-Stream-API-How-does-it-work) -
  snapshot-then-deltas shape referenced for the SSE payload
- [Confluent: Building a Real-Time Betting Platform with Confluent Cloud and
  Ably](https://www.confluent.io/blog/real-time-betting-platform-with-confluent-cloud-and-ably/) -
  broker-to-single-edge-tier fan-out pattern this design mirrors
- [Shopify Engineering: Using Server-Sent Events to Simplify Real-time Streaming at
  Scale](https://shopify.engineering/server-sent-events-data-streaming) - SSE at production scale
  for one-way feeds

## Open Questions

- Named channel per domain service (`matchflow:events`, `matchflow:odds` later) vs. one
  pattern-subscribe (`matchflow:*`) in `internal/realtime`. Leaning named channels - explicit,
  and lets Gateway skip subscribing to a channel it has no consumer wiring for yet - but not
  settled until Odds Service's actual channel-naming shows up.

## E2E / Manual QA

No frontend client exists yet, so the SSE endpoint is verified manually with `curl -N` (no
buffering, prints each chunk as it streams) rather than a browser.

| Case | Steps | Expected |
|------|-------|----------|
| Snapshot on connect | `curl -N http://localhost:8083/events?match_id=<id>` against a match with existing state | First frame is `event: snapshot` carrying the current match state |
| Live delta delivery | With the above connection open, publish a new event to `matchflow:events` for that match (via Feed Simulator or `redis-cli PUBLISH`) | A new `event: update` frame arrives immediately (push-based, no polling), reflecting the update |
| Multiple clients | Open two `curl -N` connections to the same match, publish one event | Both connections receive the same event |
| Slow/disconnected client | Open a connection, stop reading (`curl` paused, e.g. `Ctrl+Z`), publish several events, resume | Registry doesn't block other clients; resumed client either catches up within its buffer or drops without crashing the server |
| REST backfill | `curl http://localhost:8083/matches/<id>` | Returns the same JSON shape and data as `curl http://localhost:8082/matches/<id>` (Match Service directly) |
| Not-found mapping | `curl -i http://localhost:8083/matches/does-not-exist` | HTTP `404`, not `500` |

### Results (2026-08-31, Task 12)

Run against `make dev-infra` (Postgres, Redis, otel-lgtm) plus `make run SVC=match-service` and
`make run SVC=gateway-service`, both listening on their default ports (8082, 8083).

Only the two SSE-specific rows below were exercised in this pass - the automated integration
test added in this task (`api_integration_test.go`) already covers the REST backfill and
not-found mapping rows end-to-end against a real match-service process, and multi-client /
slow-client behavior is exercised by `internal/realtime`'s existing unit suite, not manual QA.

- **Snapshot on connect**: `curl -N "http://localhost:8083/events?match_id=match-1"` against an
  already-seeded `match-1`. First frame received immediately:
  ```
  event: snapshot
  data: {"match_id":"match-1","sport":"football","status":"full_time","home_score":1,"away_score":1,"clock_mins":91}
  ```
  Matches expected: `event: snapshot` arrives first, carrying current match state.

- **Live delta delivery**: with that connection open, published to `matchflow:events` via
  `redis-cli PUBLISH matchflow:events '{"match_id":"match-1","type":"goal","sequence":91,"payload":{"team":"home","minute":91},"timestamp":"2026-08-31T00:00:00Z","ingested_at":"2026-08-31T00:00:00Z","provider":"manual-test"}'`.
  A second frame arrived on the same connection within about two seconds, with no reconnect or
  polling:
  ```
  event: update
  data: {"payload":{"minute":91,"team":"home"},"sequence":91,"type":"goal"}
  ```
  Matches expected: a new `event: update` frame arrives immediately, reflecting the published
  event.

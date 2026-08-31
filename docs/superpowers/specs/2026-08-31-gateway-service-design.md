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
  `matchflow:events` Redis channel and re-emits events to every connected client, backfilling
  current match state via the gRPC client on (re)connect.
- `buf` + `connect-go` codegen is established under `proto/matchflow/<service>/v1/` and
  `gen/go/<service>_v1/`, in a shape a future Odds Service can reuse without inventing a second
  convention.
- The SSE endpoint can be verified manually with `curl -N` alone, since no frontend client
  exists yet.

## Non-Goals

- Building the Odds Service itself - only the fan-out pattern needs to demonstrably support a
  second publisher, not a second publisher.
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

`buf.yaml` (module root) and `buf.gen.yaml` (Go + connect-go plugins) live at the repo root,
generating into `gen/go/match_service_v1/` as plain packages in the existing single Go module
(`github.com/AndoNorth/match-flow`) - no per-package `go.mod` needed, unlike the multi-module
reference this convention was adapted from, because that module split exists there to let many
independent external consumers pin different versions across repo boundaries, a problem two
services in one repo don't have.
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
  `MATCH_SERVICE_ADDR` (default `match-service.matchflow.svc.cluster.local:8082` in-cluster,
  overridable to `localhost:8082` for local dev, per
  [ARCHITECTURE.md](../../ARCHITECTURE.md#gateway-service)).
- `internal/resolver` - converts between REST/JSON request and response shapes and
  `matchclient`'s protobuf types, and maps gRPC status codes to HTTP status codes
  (`NOT_FOUND` -> 404, `INVALID_ARGUMENT` -> 400, `UNAVAILABLE`/`DEADLINE_EXCEEDED` -> 503,
  anything else -> 500). Route handlers in `internal/api` call the resolver and never import
  the generated protobuf types directly.
- `internal/api` - the three Huma routes (`GET /matches`, `GET /matches/{id}`,
  `GET /matches/{id}/events`), structurally identical to Match Service's own `api.go` but backed
  by `matchclient` + `resolver` instead of a direct store.
- `internal/realtime` - owns one Redis subscription to `matchflow:events` and a registry of
  connected SSE clients (a map of client ID to a buffered `chan []byte`, guarded by a
  `sync.RWMutex`). The subscriber goroutine reads one message at a time and fans it out to every
  registered client's channel; a client whose channel is full (a slow consumer) has its message
  dropped rather than blocking the subscriber for every other client - one Redis subscription
  total, not one per client.
- `GET /events` (SSE) - registers a client with `internal/realtime`, writes an initial snapshot
  fetched via `matchclient` (current match state for the requested match, or all matches),
  then streams every subsequent message from the client's channel as an SSE `data:` frame until
  the client disconnects. Mirrors the snapshot-then-deltas shape real market-data streams use
  (e.g. Betfair's Stream API - see References) independent of the transport choice.
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
  `redis://localhost:6379`), `MATCH_SERVICE_ADDR` (default `localhost:8082` for local dev).

## Validation

- **Match Service**: Ginkgo/Gomega unit tests for the new connect-go handler, mirroring the
  existing REST route tests (`api_test.go`) against the same `reader` fakes - same inputs, same
  fakes, asserting on the protobuf response instead of the JSON body. An integration test
  (`-tags=integration`) exercises a real connect-go client against the running server, alongside
  the existing REST integration test.
- **Gateway**: unit tests for `internal/resolver`'s REST<->protobuf conversion and its gRPC-to-HTTP
  status mapping, using a fake `matchclient`. Unit tests for `internal/realtime`'s fan-out
  registry (multiple registered clients, one published message, every client's channel receives
  it; a full client channel doesn't block delivery to others). An integration test brings up a
  real Match Service and Redis (testcontainers, matching the pattern already used in
  `match-service`'s integration tests) and exercises the three REST routes end-to-end through the
  Gateway's gRPC client.
- SSE streaming itself is covered by a unit test using `httptest.NewRecorder` (a `http.Flusher`
  fake) asserting the handler writes one `data:` frame per published Redis message, and by the
  manual `curl -N` check below - there is no automated way to assert on real browser
  `EventSource` reconnect behavior without a frontend client, which is exactly why the manual
  check exists.

## Open Questions

- Named channel per domain service (`matchflow:events`, `matchflow:odds` later) vs. one
  pattern-subscribe (`matchflow:*`) in `internal/realtime`. Leaning named channels - explicit,
  and lets Gateway skip subscribing to a channel it has no consumer wiring for yet - but not
  settled until Odds Service's actual channel-naming shows up.

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

## E2E / Manual QA

No frontend client exists yet, so the SSE endpoint is verified manually with `curl -N` (no
buffering, prints each chunk as it streams) rather than a browser.

| Case | Steps | Expected |
|------|-------|----------|
| Snapshot on connect | `curl -N http://localhost:8083/events?match_id=<id>` against a match with existing state | First `data:` frame is the current match snapshot |
| Live delta delivery | With the above connection open, publish a new event to `matchflow:events` for that match (via Feed Simulator or `redis-cli PUBLISH`) | A new `data:` frame arrives within the poll interval, reflecting the update |
| Multiple clients | Open two `curl -N` connections to the same match, publish one event | Both connections receive the same event |
| Slow/disconnected client | Open a connection, stop reading (`curl` paused, e.g. `Ctrl+Z`), publish several events, resume | Registry doesn't block other clients; resumed client either catches up within its buffer or drops without crashing the server |
| REST backfill | `curl http://localhost:8083/matches/<id>` | Returns the same JSON shape and data as `curl http://localhost:8082/matches/<id>` (Match Service directly) |
| Not-found mapping | `curl -i http://localhost:8083/matches/does-not-exist` | HTTP `404`, not `500` |

Results to be appended here once run against a live `make dev-infra` + both services.

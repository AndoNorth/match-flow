# Architecture

## Overview

MatchFlow is composed of a small number of independent services that
together simulate, ingest, distribute, and display live sports match
events. The architecture favors clarity over completeness: each service
has one responsibility, and communication patterns are chosen to give
engineers practice with both request/response and event distribution
styles.

```
Feed Simulator -> Ingestion Service -> Redis -> Match Service
                                                     |
                                              Gateway Service
                                                     |
                                          Frontend Application
```

## Services

### Feed Simulator

Simulates third-party sports data providers. Produces live match events
and generates realistic event streams for the rest of the system to
consume. Exists so the system has a continuous, believable source of
events without depending on a real external provider.

### Ingestion Service

Receives incoming events from the Feed Simulator (or any future event
source). Validates incoming data, normalizes event payloads into a
consistent internal shape, and publishes events for downstream
consumers. Acts as the boundary between "whatever shape external data
arrives in" and "the shape the rest of the system relies on."

### Match Service

Maintains current match state, built from the event stream. Exposes
match-related APIs and provides match information to other services.
The system of record for "what is currently true about a match."

### Gateway Service

The entry point for client applications. Exposes REST APIs, provides
real-time communication endpoints, and distributes updates to connected
clients. Owns the client-facing contract, so internal service changes
don't necessarily ripple to clients.

The Gateway is the bridge between internal event distribution and the
browser: Redis pub/sub is a server-to-server protocol only, a browser
never subscribes to it directly. The Gateway holds the Redis
subscription and re-emits each message down its own connections to
clients.

**Server-Sent Events (SSE) is the default realtime protocol**, not
WebSocket. Match/odds updates only need to flow one way (server to
client) - the frontend never needs to push data back over the same
channel - so SSE's plain-HTTP, auto-reconnecting model fits without the
extra complexity of a WebSocket's full-duplex upgrade handshake.
WebSocket is a reasonable future upgrade only if a bidirectional need
shows up (e.g. a client-initiated action that must ride the same
connection); until then it's unnecessary surface area.

Redis pub/sub delivers to whoever is subscribed at publish time and
keeps nothing - a client that reconnects mid-match has missed whatever
was published while it was disconnected. The Gateway is responsible for
backfilling current state from the Match Service's API on (re)connect,
rather than assuming Redis has anything to replay.

**Talking to Match Service over gRPC.** The Gateway is a REST/SSE
service on its client-facing side and a gRPC client on its backend
side - the same "gateway fronting typed internal services" shape shows
up in most systems that split a public HTTP API from internal RPC.
Three pieces make that translation clean rather than ad hoc:

- **A resolver layer** that converts between the Gateway's REST/JSON
  request and response shapes and Match Service's protobuf messages.
  This is the only place either vocabulary is known to the other -
  route handlers work in REST shapes, gRPC client code works in
  protobuf shapes, and the resolver is the seam between them.
- **gRPC-to-HTTP error translation** - a gRPC status code
  (`NOT_FOUND`, `INVALID_ARGUMENT`, ...) maps to an HTTP status
  (`404`, `400`, ...) in one place, so every route doesn't hand-roll
  its own mapping.
- **Service addresses via environment variable**, defaulting to a
  Kubernetes-DNS-shaped hostname (e.g.
  `match-service.matchflow.svc.cluster.local:<port>`) for the
  in-cluster case, overridable for local dev (plain `localhost:<port>`
  when running outside Kind/Tilt). One config surface, not a
  hardcoded address baked into either environment.

### Frontend Application

Displays matches, live events, and current match state. Consumes the
Gateway Service's REST and realtime APIs. The only service with a
human-facing UI.

Built with Next.js (App Router) and React/TypeScript. Expected shape:

- **Server-rendered match views** - match lists and match detail pages are
  fetched server-side from the Gateway's REST API, so an initial page load
  doesn't depend on the realtime channel being connected yet.
- **A realtime client layer** - once loaded, the page opens a connection to
  the Gateway's realtime endpoint and applies incoming event updates to
  the already-rendered match state (e.g. score changes, new events),
  rather than re-fetching on every update.
- **Route structure by access pattern** - grouping routes by whether they
  need a live connection (a match-in-progress view) versus a static
  snapshot (a completed match's summary) is a reasonable default; the
  concrete grouping is left to the engineer building it.
- **A thin API/client boundary** - all calls to the Gateway (REST and
  realtime) go through one client module, so the transport detail (which
  realtime protocol, which REST paths) isn't scattered across components.

As with the backend services, implementation detail (state management
choice, exact folder layout, component library) is left open. The
constraint that matters architecturally is the one above: the frontend
talks to the system only through the Gateway Service, never directly to
Match Service, Ingestion Service, or Redis.

## Communication Patterns

- **Feed Simulator -> Ingestion Service**: event submission (REST or
  gRPC, per implementation).
- **Ingestion Service -> Redis**: event publication for distribution.
- **Match Service**: consumes distributed events from Redis to update
  state; exposes that state via REST/gRPC to other services.
- **Gateway Service -> Match Service**: service-to-service queries,
  favoring gRPC where a typed contract is valuable.
- **Gateway Service -> Frontend Application**: REST for initial/on-demand
  data (server-rendered pages, one-off queries), Server-Sent Events (SSE)
  as the default realtime channel for push updates applied client-side
  after load (see [Gateway Service](#gateway-service)).

## Protobuf & gRPC Structure

Doesn't exist yet (no service calls gRPC until Phase 4, see
[ROADMAP.md](ROADMAP.md)), but the shape is decided so it's built once,
not guessed at per-service later:

- **`proto/matchflow/<service>/*.proto`** - source protobuf definitions,
  one directory per service that exposes a gRPC API.
- **`gen/go/<service>/`** - generated Go stubs, checked in rather than
  generated on every build (so a checkout builds without also invoking
  the proto toolchain). Regenerated via a single command when a
  `.proto` file changes.
- **One repo, not a separate protobuf repo.** A dedicated repo for
  proto definitions makes sense at a scale where many independent teams
  consume the same contracts across repo boundaries - two devs and four
  services in one repo don't have that problem, and a separate repo
  would just be a second place to keep in sync with every API change.
- **Codegen tool**: not yet chosen between plain `protoc` +
  `protoc-gen-go` + `protoc-gen-go-grpc` (maximum compatibility, more
  manual wiring) and `buf` (lint + breaking-change detection + simpler
  config, the more common modern default) - a soft lean toward `buf`
  since nothing here has a compatibility reason to avoid it.

## Cross-Cutting Concerns

- **Observability**: every service emits traces, metrics, and logs via
  OpenTelemetry, regardless of its role in the request path.
- **Concurrency**: event ingestion and distribution are expected to
  handle concurrent event streams; this is a deliberate practice area,
  not an edge case.
- **Testability**: service boundaries are drawn so each service can be
  tested in isolation, with integration tests validating the
  boundaries between them.
- **Publisher/consumer schema coupling**: the event payload published to
  Redis and the shape the Gateway decodes for its realtime stream must
  agree. A drift between them (a renamed field, a changed type) tends to
  fail silently rather than loudly - a single decode failure on one
  malformed field can drop the entire message, and a stream that still
  looks "connected" gives no signal that events are being lost. Treat
  the published event shape as a contract between Ingestion/Match
  Service and the Gateway, not an implementation detail either side can
  change unilaterally.

## Guiding Principles

- Scalability: services scale independently; state is held by the
  service that owns it (Match Service), not smeared across the system.
- Maintainability: one responsibility per service, explicit contracts
  between them.
- Observability: instrumentation is a first-class concern, not an
  afterthought bolted on later.
- Testability: boundaries are drawn to be testable, both in isolation
  and end-to-end.
- Developer experience: local setup, running, and iterating on any
  single service should be fast and low-friction.

## Future Enhancements

These are explicitly out of scope for the initial build, but are
plausible next steps once the core system is working:

- **Kafka / RabbitMQ / NATS** - a dedicated message broker, if Redis-based
  distribution outgrows its use case (e.g. durable/replayable event logs).
- **Service mesh** - if inter-service traffic management (retries,
  circuit breaking, mTLS) becomes complex enough to warrant it.
- **Terraform** - for managing cloud infrastructure as code, once
  deployment targets extend beyond a single Kubernetes cluster.
- **ArgoCD** - for GitOps-style continuous deployment, once there's a
  deployment pipeline mature enough to benefit from it.

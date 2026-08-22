# Technology Stack

This document records technology direction and the reasoning behind it.
It intentionally avoids implementation detail. Choices below are the
default direction for this project; a service may deviate if there's a
good reason, documented alongside the deviation.

## Backend

- **Go (latest stable)** - the primary language for all backend services.
  Practicing Go across multiple cooperating services is a core goal of
  this project.
- **REST via Huma** - the default API style for client-facing and simple
  service-to-service interactions. [Huma](https://huma.rocks/) generates
  the OpenAPI spec directly from Go handler/input types, so every REST
  endpoint gets a browsable/debuggable API spec for free instead of a
  hand-maintained one.
- **gRPC + Protocol Buffers** - used where strongly-typed, efficient
  service-to-service communication is the better fit than REST. See
  [ARCHITECTURE.md](ARCHITECTURE.md#protobuf--grpc-structure) for where
  `.proto` files and generated code live.
- **Redis** - backs event distribution between services and any
  transient/real-time state. Chosen over a dedicated message broker
  (Kafka, RabbitMQ, NATS) to keep the operational surface small while
  still practicing pub/sub-style distribution. See
  [ARCHITECTURE.md](ARCHITECTURE.md#future-enhancements) for when a
  dedicated broker would be introduced.
- **PostgreSQL** - system of record for durable state (e.g. match
  history).
- **OpenTelemetry** - instrumentation standard for traces, metrics, and
  logs across all services.

## Frontend

- **Next.js / React / TypeScript** - the frontend application consuming
  REST and realtime APIs.

## Testing

- **Backend: Ginkgo + Gomega** - behavior-driven testing for Go services.
- **Frontend: Vitest** - unit/component testing.
- **Frontend: Playwright** - end-to-end testing.
- **Frontend: Fallow** - dependency/dead-code audit (flags unused or
  undeclared dependencies), run as part of the pre-commit lint pass
  alongside Biome.

## Infrastructure

- **Docker / Docker Compose** - local multi-service development and
  packaging.
- **Kubernetes** - target deployment platform.
- **Kind + Tilt** - local Kubernetes dev loop, standing infra
  (Redis/Postgres via Bitnami Helm charts) and observability up without
  needing a real cluster or MatchFlow's own images to exist yet.
- **Helm** - packaging and configuration of Kubernetes deployments, both
  for third-party infra charts locally and for MatchFlow's own services
  once they exist.

## Developer Experience

- **Nix Flakes** - reproducible development environments.
- **Air** - live reload for Go services during development (Go has no
  built-in dev server). The frontend needs no equivalent - `next dev`
  already includes hot reload; no Vite involved, Next.js isn't built on
  it.
- **Makefile-based workflows** - a consistent, discoverable set of
  commands across services (build, test, run, lint, etc.).

## Observability

- **OpenTelemetry** - traces, metrics, and logs emitted by every service.
- **Grafana** - dashboards and visualization.
- **Loki** - log aggregation.
- **Mimir** - metrics storage.
- **Alloy** - telemetry collection and routing.
- **Pyroscope** - continuous profiling (CPU/memory). Not Rust-only or an
  odd fit for Go - it's built directly on Go's own `runtime/pprof`
  format, arguably its most mature language integration. A service
  either pushes profiles via the `pyroscope-go` SDK, or exposes a pprof
  endpoint for pull-mode scraping.

For local dev (both the Docker Compose and K8s/Kind loops, see
[DEVELOPMENT.md](DEVELOPMENT.md)), `grafana/otel-lgtm` - a public image,
[hub.docker.com/r/grafana/otel-lgtm](https://hub.docker.com/r/grafana/otel-lgtm),
free to pull, no account needed - stands in for the full stack above:
one container bundling an OTel collector, Prometheus (metrics), Loki
(logs), Tempo (traces), Pyroscope (profiles), and Grafana. Services only
need to speak OTLP (or push/expose pprof for profiling) either way, so
swapping the local collector for Alloy and Prometheus for Mimir later
(if the gap from the target production stack starts to matter) doesn't
change any service code - see
[ROADMAP.md](ROADMAP.md#phase-6---service-instrumentation).

## Database Migrations

- **SQL-first migrations** managed with **Goose**. Migrations are plain
  SQL, kept under version control alongside the service that owns the
  schema.

## Deliberately Excluded (For Now)

The following are common in production distributed systems but are left
out of the core build to keep the learning surface focused. They are
documented as future enhancements in
[ARCHITECTURE.md](ARCHITECTURE.md#future-enhancements):

- Kafka, RabbitMQ, NATS
- Service mesh
- Terraform
- ArgoCD

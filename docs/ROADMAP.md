# Roadmap

Phases are sequential in intent but not strictly gated; later phases may
begin once the foundational pieces they depend on are in place.

Infra and observability (Phase 2) are pulled ahead of service/frontend
development on purpose: both the Docker and K8s local dev loops - and a
way to see what's happening inside them - should exist and be simple to
use *before* there's application code generating events to watch.

## Phase 1 - Repository Setup & Developer Experience

- Repository structure and documentation
- Nix Flakes dev shell, git hooks (lint + security checks only)
- Makefile workflow conventions
- CI: not wired yet, but `nix flake check` already runs the same lint +
  security hooks (golangci-lint, govulncheck, betteralign, gitleaks,
  Biome) with zero
  service code required - a `.github/workflows/ci.yml` calling it is a
  same-day addition whenever it's prioritized, not blocked on anything
  later in this roadmap

## Phase 2 - Local Dev Infrastructure & Observability

- Docker Compose loop: Redis, Postgres, `grafana/otel-lgtm` (a public
  image bundling an OTel collector, Prometheus, Loki, Tempo, Pyroscope,
  and Grafana)
- K8s loop: Kind cluster + Tilt, Redis/Postgres via Bitnami Helm charts,
  the same `otel-lgtm` bundle as a plain Deployment
- Both loops stay interchangeable for local dev - same credentials, same
  ports, pick whichever fits the day's task

## Phase 3 - Core Service Scaffolding

- `cmd/<service>/main.go` + `internal/` split for each service (locked
  in, see [README.md](../README.md#repository-layout)) - service names
  and directory names under `services/` are not locked in yet
- Per-service `build`/`run`/`test`/`lint` Makefile targets (`make run
  SVC=<name>` with Air hot-reload)

## Phase 4 - API Development, Persistence & Service Communication

- REST (via Huma) and gRPC contracts between services, following the
  `proto/` + `gen/` structure in
  [ARCHITECTURE.md](ARCHITECTURE.md#protobuf--grpc-structure)
- PostgreSQL schema and Goose migrations for durable state
- Service-to-service communication implemented per
  [ARCHITECTURE.md](ARCHITECTURE.md#communication-patterns)

## Phase 5 - Realtime Communication

- Realtime endpoint(s) on the Gateway Service (SSE by default, see
  [ARCHITECTURE.md](ARCHITECTURE.md#gateway-service))
- Redis-backed event distribution from Ingestion Service through to
  connected clients
- Frontend consumption of realtime updates

## Phase 6 - Service Instrumentation

- OpenTelemetry instrumentation added to every service, exporting into
  the observability loop already running since Phase 2
- Swap the local `otel-lgtm` collector for Alloy if/when the gap between
  local dev and the target production stack in
  [TECH_STACK.md](TECH_STACK.md#observability) starts to matter

## Phase 7 - Containerization

- Dockerfiles per service, built against the same Kind/Tilt loop set up
  in Phase 2

## Phase 8 - Kubernetes Deployment

- Helm charts for MatchFlow's own services, under `k8s/charts/<service>/`
  in this repo (infra/observability charts already exist from Phase 2) -
  one repo for app code and charts, see
  [DEVELOPMENT.md](DEVELOPMENT.md#charts--local-config)
- Deployment to a Kubernetes cluster (local or cloud)

## Phase 9 - Load Testing & Performance Validation

- Load testing the event pipeline end-to-end
- Validating real-time distribution under load
- Identifying and documenting scaling bottlenecks

# MatchFlow - Agent Operating Guide

## System Purpose (Core Technologies)

MatchFlow is a real-time sports event distribution platform, used as a
learning/reference project for distributed systems patterns. Core stack:

- Go services (Feed Simulator, Ingestion Service, Match Service, Gateway
  Service)
- Redis for event distribution
- Next.js/React/TypeScript frontend
- PostgreSQL for durable state
- OpenTelemetry/Grafana/Loki/Mimir/Alloy/Pyroscope for observability

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and
[docs/TECH_STACK.md](docs/TECH_STACK.md) for the full picture.

## Architecture Rules (HARD CONSTRAINTS)

- The frontend talks to the system only through the Gateway Service -
  never directly to Match Service, Ingestion Service, or Redis.
- Event distribution between backend services goes through Redis, not
  ad-hoc direct calls between services that don't own the data.
- Match Service is the only owner of match state. Other services read it
  through Match Service's API, they don't maintain their own copy.
- Schema changes go through Goose SQL migrations, not manual DDL.

## Running Commands

A Nix flake provides the dev shell (Go, Node/pnpm, Air, golangci-lint,
govulncheck, betteralign, Biome, gitleaks, Redis CLI, Kind, Tilt,
kubectl, Helm) - see
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md). Docker itself is NOT in the
shell - it's a host-level prerequisite (a native `dockerd`, not Docker
Desktop - see [README.md](README.md#getting-started)) a devShell can't
provide or start; `make check-docker` verifies it before
`dev-infra`/`dev-k8s` use it. A non-interactive/agent shell
does not have direnv's hook wired in, so plain `go`/`node`/etc. commands
fail with "command not found" even though `.envrc` exists. Prefix
commands with `nix develop --command` (or `direnv exec .` if direnv is
available) to load the environment for that one command, e.g.:

```bash
nix develop --command golangci-lint run
nix develop --command govulncheck ./...
```

Environment-lifecycle targets exist now: `make dev-setup`,
`make dev-infra` (Redis + Postgres + `otel-lgtm` via Docker Compose),
`make dev-infra-down`/`make dev-infra-clean`, `make dev-all`,
`make dev-clean`, `make check-docker`/`make check-infra-ports`
(preflight checks), and `make dev-k8s`/`make dev-k8s-down` (the same
infra as a Kind + Tilt K8s loop - see [Tiltfile](Tiltfile)). Per-service
targets now exist: `make build/run/test/test-integration/lint SVC=<name>`
(see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)). Services
scaffolded so far: feed-simulator, ingestion-service. See
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full list.

## Services

Scaffolded so far (see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for
the full 4-service design) - each entry links to that service's own
README for env vars, endpoints, and how to run it:

- [feed-simulator](services/feed-simulator/README.md) - simulates a
  third-party sports data provider and submits events to Ingestion
  Service over HTTP
- [ingestion-service](services/ingestion-service/README.md) - receives
  events, validates and normalizes them into one canonical shape, and
  publishes them to Redis

## Documentation Map

- [docs/GOALS.md](docs/GOALS.md) - what this project is for and isn't
- [docs/TECH_STACK.md](docs/TECH_STACK.md) - technology choices and why
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - services and data flow
- [docs/ROADMAP.md](docs/ROADMAP.md) - phased build-out plan
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) - local dev workflow

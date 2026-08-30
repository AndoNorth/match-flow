# MatchFlow

MatchFlow is a real-time sports event distribution platform built to
demonstrate distributed systems concepts in Go: service-to-service
communication, Redis-backed event distribution, real-time client updates,
observability, and cloud-native deployment.

This repository is a learning and reference project.
It is intentionally architecture-focused rather than feature-focused.

## Why This Exists

Experienced backend engineers often know a language well but lack hands-on
reps with the patterns that show up in modern distributed systems: service
decomposition, event distribution without a heavyweight broker, real-time
fan-out to clients, and the observability stack needed to operate it all.
MatchFlow gives that practice around one small, understandable domain
(live sports match events) so the distributed systems concepts stay the
focus, not the domain logic.

## Getting Started

**Prerequisites** (assumes Linux, macOS, or WSL2 - not native Windows):

- [Nix](https://nixos.org/download) with flakes enabled - the
  [Determinate Nix installer](https://determinate.systems/nix-installer/)
  enables flakes by default; otherwise add
  `experimental-features = nix-command flakes` to your Nix config.
- A native `dockerd` - not Docker Desktop. Not provided by the Nix shell
  below - see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md#environment) for
  why. Docker Desktop runs containers inside its own Linux VM, which
  obscures real CPU/RAM numbers from any container-level metrics
  collection (e.g. cAdvisor) - a native engine reports the host's real
  usage. On WSL2, Docker's own [engine install
  guide](https://docs.docker.com/engine/install/) covers installing
  `dockerd` directly inside the distro, no Desktop required.
- [direnv](https://direnv.net/) (optional) - auto-loads the dev shell on
  `cd`, otherwise run `nix develop` manually each session.

**Quick start:**

```bash
git clone <this-repo> && cd match-flow
nix develop              # or: direnv allow
make dev-setup           # brings up Redis, Postgres, and an observability bundle
```

That's the Docker Compose loop; an equivalent Kind/Tilt (Kubernetes) loop
also exists. Full walkthrough, verification steps, a Kubernetes
alternative, and resource estimates:
[docs/DEV_SETUP.md](docs/DEV_SETUP.md).

## Documentation

- [docs/GOALS.md](docs/GOALS.md) - what this project is for and what it is not
- [docs/DEV_SETUP.md](docs/DEV_SETUP.md) - step-by-step local bootstrap (start here to run it)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - services, communication patterns, data flow
- [docs/TECH_STACK.md](docs/TECH_STACK.md) - technology choices and rationale
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) - workflow reference (linting, testing, Makefile targets - dip in as needed, not required reading before you start)
- [docs/ROADMAP.md](docs/ROADMAP.md) - phased build-out plan

## Repository Layout

```
.
├── README.md
├── docs/                    # engineering documentation
├── k8s/local/               # local K8s dev loop (Kind config, otel-lgtm manifest, Helm values)
├── scripts/                 # dev-loop preflight checks (Docker, port squatters)
├── Tiltfile                 # local K8s dev loop orchestration
├── docker-compose.dev.yml   # local Docker Compose dev loop
├── flake.nix                # Nix dev shell + git hooks
├── services/                # Go services
└── frontend/                # Next.js client application
```

Services scaffolded so far (see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full 4-service
design):

- [feed-simulator](services/feed-simulator/README.md) - simulates a
  third-party sports data provider and submits events to Ingestion
  Service over HTTP
- [ingestion-service](services/ingestion-service/README.md) - receives
  events, validates and normalizes them into one canonical shape, and
  publishes them to Redis

`frontend/` doesn't have anything under it yet, and service
names/boundaries beyond these two aren't locked in -
[ARCHITECTURE.md](docs/ARCHITECTURE.md) describes responsibilities, not
directory names.

The internal shape of each Go service **is** locked in, though - standard
Go project layout, every service the same way:

```
services/<service>/
├── cmd/<service>/main.go   # entrypoint - wiring only, no logic
└── internal/               # everything else, private to this service
```

`cmd/<service>/main.go` builds and starts the service; `internal/` holds
every package that does the actual work. Nothing under `internal/` is
importable by another service or by `cmd/` of a different service - Go
enforces that boundary at compile time, which is the point of using it.
This is decided regardless of what the services end up being named.

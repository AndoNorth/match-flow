# Local Dev Environment - Bootstrap Walkthrough

> Two interchangeable local loops exist: Docker Compose and Kind/Tilt
> (K8s). Same Redis/Postgres credentials, same ports, same
> `otel-lgtm` observability bundle either way - pick whichever fits the
> task, or run only one. Neither runs application code yet (no service
> is scaffolded - Phase 3, see [ROADMAP.md](ROADMAP.md)).

This chains together what already exists in the repo (`Makefile`,
`flake.nix`, `docker-compose.dev.yml`, `Tiltfile`) into one first-time
walkthrough. It's intentionally short right now - there's no service
code yet, so there isn't much to chain. Expect this doc to grow a step
at a time as each phase in [ROADMAP.md](ROADMAP.md) lands, rather than
all at once.

## Before you start: resource estimates

Rough ballparks, not measured on this repo - run `docker stats` (Compose)
or `kubectl top pods` (Kind, needs metrics-server) once things are up for
real numbers on your machine. Enough to sanity-check "will this run
comfortably" before troubleshooting a slow laptop instead of a bug.

| Piece | Approx. idle RAM | Notes |
|---|---|---|
| Redis | 10-30 MB | negligible CPU |
| Postgres | 50-100 MB | negligible CPU idle |
| `otel-lgtm` | 1-2 GB | the heaviest single piece by far - it's an OTel collector, Prometheus, Loki, Tempo, Pyroscope, and Grafana bundled into one container |
| **Docker Compose loop total** | **~1.5-2.5 GB** | Redis + Postgres + otel-lgtm |
| Kind control plane | +1-1.5 GB | API server, etcd, scheduler, controller-manager, kubelet, CNI - all run as one node container, on top of the same pods below |
| Redis/Postgres/otel-lgtm as pods | ~same as Compose row | same images, running in-cluster instead of as plain containers |
| **Kind/Tilt loop total** | **~3-4 GB** | control-plane tax makes this loop meaningfully heavier than Compose for the same infra - pick Compose for day-to-day work, Kind when you specifically need the K8s loop |
| Go service + Air (once scaffolded) | 50-150 MB each | negligible CPU idle |
| Next.js dev server (once scaffolded) | 300-500 MB | noticeable CPU during compiles/HMR, not idle |

The two loops bind the same ports, so they're an either/or resource ask,
not additive - budget for whichever one you're running. 8 GB free RAM is
comfortable for the Compose loop alongside an editor and a browser; the
Kind loop wants closer to 16 GB total on the machine to stay comfortable
once an IDE and browser are also open.

## 1. Enter the dev shell

```bash
nix develop
# or, with direnv installed:
direnv allow
```

Gives you the Go toolchain, Node/pnpm, Air, golangci-lint, govulncheck,
betteralign, Biome, gitleaks, Redis CLI, Kind, Tilt, kubectl, and Helm - see
[DEVELOPMENT.md](DEVELOPMENT.md#environment). Entering the shell also
renders the pre-commit git hooks (lint + secret-scanning only).

**Docker itself is not part of this shell** - install a native `dockerd`
yourself first (not Docker Desktop, see [README.md](../README.md#getting-started));
both loops below need it. `make check-docker` (run automatically by
`dev-infra`/`dev-k8s`) checks it's installed and its daemon is reachable
before anything else runs.

## 2. Bring up infra - Docker Compose

```bash
make dev-setup
```

Currently just `dev-infra` (Redis, Postgres, `otel-lgtm` via Docker
Compose) plus a reminder to enter the Nix shell if you haven't.
`check-infra-ports` runs first and warns if port 6379 or 5432 is already
taken by something other than Docker - set `SKIP_INFRA_PORT_CHECK=1` to
skip it, `PORT_CHECK_MODE=fail` to hard-fail instead of warn.

Check it's actually up:

```bash
docker compose -f docker-compose.dev.yml ps
redis-cli ping                              # PONG
psql postgresql://matchflow:matchflow@localhost:5432/matchflow -c '\dt'
open http://localhost:3000                  # Grafana (otel-lgtm)
```

## 2b. Bring up infra - Kind/Tilt (K8s), instead of or alongside Compose

```bash
make dev-k8s
```

Creates a `matchflow` Kind cluster if one doesn't already exist, then
runs `tilt up` - installs Redis and Postgres from the Bitnami Helm
charts (`k8s/local/helm-values/`) and applies the `otel-lgtm` manifest
(`k8s/local/otel-lgtm.yaml`), port-forwarding all three to the same
`localhost` ports Compose uses. Tilt's UI (printed on startup, usually
`http://localhost:10350`) shows build/health status for each resource.

Note: both loops bind the same host ports (6379, 5432, 3000, 4317,
4318) - don't run Compose and Kind's port-forwards at once against the
same ports, one will fail to bind.

Verification is identical to the Compose path above once `tilt up`
reports everything healthy.

## 3. Run a service

Nothing to run yet - no service is scaffolded (Phase 3, see
[ROADMAP.md](ROADMAP.md)). Once `services/<name>/cmd` exists, this
section documents `make run SVC=<name>` (Compose loop, Air hot-reload)
and the Tiltfile addition needed to build/deploy it into the Kind loop.

## Cleanup

```bash
# Docker Compose loop
make dev-infra-down    # containers down, volumes kept (safe, reversible)
make dev-infra-clean   # containers + volumes wiped - loses local data
make dev-clean         # same as dev-infra-clean, repo-wide alias

# Kind/Tilt loop
make dev-k8s-down      # tilt down + delete the Kind cluster
```

## Related

- [DEVELOPMENT.md](DEVELOPMENT.md) - environment, linting, testing, and
  Makefile target reference
- [ARCHITECTURE.md](ARCHITECTURE.md) - what each service is for
- [ROADMAP.md](ROADMAP.md) - what's built vs. planned

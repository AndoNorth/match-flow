# Development Workflow

This document describes how engineers are expected to work in this
repository. It covers workflow conventions, not implementation details -
those are left to each service as it is scaffolded in Phase 3.

This is reference material, not a getting-started guide - to actually
get the repo running, see [DEV_SETUP.md](DEV_SETUP.md) first and come
back here for detail on any piece as you need it.

## Environment

- **Nix Flakes** provide one reproducible development shell for the whole
  repo (Go toolchain, Node/pnpm, Air, golangci-lint, govulncheck,
  betteralign, Biome,
  gitleaks, Redis CLI, Kind, Tilt, kubectl, Helm) - `nix develop`, or
  `direnv allow` once (`.envrc` is `use flake`) so the shell loads
  automatically on `cd`.
- **Docker itself is NOT in the Nix shell.** The `docker` CLI and a
  native `dockerd` (not Docker Desktop - see
  [README.md](../README.md#getting-started)) are a host-level
  prerequisite you install yourself - a devShell can put binaries on
  PATH but can't start or manage a running system service. `make
  check-docker` verifies it's installed and reachable before
  `dev-infra`/`dev-k8s` try to use it, with a clear message instead of a
  confusing compose/Kind connection error if it's missing.
- **Air** gives live reload for Go services during local development -
  Go has no built-in dev server, so a file watcher + rebuild-and-restart
  tool is needed. The frontend doesn't need an equivalent: Next.js's own
  `next dev` already includes hot reload (Fast Refresh) out of the box.
  Vite is the tool you'd reach for in a plain React app without a
  framework - Next.js isn't built on Vite, so adding it here would just
  be a second, redundant dev server.
- **Docker Compose** brings up dependent infrastructure (Redis,
  Postgres, `otel-lgtm` observability bundle) and, once containerized,
  the services themselves.
- **Kind + Tilt** provide the same infra (Redis/Postgres via Bitnami
  Helm charts, same `otel-lgtm` bundle) as a local Kubernetes loop - use
  whichever loop fits the task, both stay interchangeable (same
  credentials, same ports). Kind also needs the host's Docker to run
  cluster nodes as containers - same prerequisite as the Compose loop.
  See [DEV_SETUP.md](DEV_SETUP.md).

## Linting and Security Checks

Entering the dev shell (`nix develop` / direnv) renders a set of
[git-hooks.nix](https://github.com/cachix/git-hooks.nix)-managed
pre-commit hooks - lint and secret-scanning only, no architecture or
boundary rules:

- `gofmt` + `goimports` - Go formatting
- `golangci-lint` - Go linting (`.golangci.yml`): correctness (errcheck,
  govet + the `composites` analyzer for unkeyed struct literals,
  staticcheck, ineffassign, unused, bodyclose), security (gosec),
  error-handling discipline (wrapcheck), Ginkgo-specific checks
  (ginkgolinter - harmless on non-Ginkgo code), and style (lll, mnd)
- `govulncheck` - Go known-vulnerability scanning
- `betteralign` - flags Go structs whose field order wastes memory to
  padding. Check-only (no `-apply`) - auto-reordering fields can
  silently break positional (unkeyed) struct literals elsewhere in the
  codebase (the `composites` analyzer above catches those directly), so
  a misalignment is surfaced for a human to fix, not rewritten unattended
- `go test ./...` - runs on every commit touching a `.go` file. A no-op
  today (no test files exist), a real safety net once they do
- `biome check` - TypeScript/JS formatting and linting for the frontend
  (`frontend/biome.json`, Biome's recommended rule set)
- `gitleaks` - secret scanning on staged changes
- `fallow` (dependency/dead-code audit) is planned alongside Biome once
  the frontend has a `package.json` to audit - not wired into
  `flake.nix` yet since there's nothing for it to check.

**Semgrep - considered, not added.** It's only as good as its rule set,
and a useful one is bespoke work (custom rules tuned to this codebase's
own patterns) that doesn't exist yet and isn't worth authoring before
there's Go code to write rules against. `gosec` (in golangci-lint above)
already covers standard Go security patterns without that setup cost.
Revisit once there's a real, recurring class of bug worth writing a
custom rule for.

These run locally via git hooks and are expected to be re-run in CI
against the same tool versions (the point of driving them from the Nix
shell rather than a separately-versioned CI toolchain) - `nix flake
check` runs the exact same set, so wiring a CI workflow later is just
calling that one command, not re-deriving the check list. Not wired
yet (see [ROADMAP.md](ROADMAP.md), Phase 1) since nothing's forcing the
question while everything still runs locally.

## Common Workflow

Each service is expected to expose a consistent set of Makefile targets,
so switching between services doesn't require relearning a workflow.
At minimum, expect:

- `make build SVC=<name>` - build the service
- `make run SVC=<name>` - run the service locally with Air hot-reload
- `make test SVC=<name>` - run the service's unit tests
- `make test-integration SVC=<name>` - run its integration tests (adds
  `-tags=integration`, requires Docker)
- `make lint SVC=<name>` - run linting/static analysis

Repo-wide Makefile targets are expected to orchestrate these across all
services (e.g. `make test` at the root running every service's tests).

A small set of environment-lifecycle targets is already in place
(`Makefile`, `docker-compose.dev.yml`):

- `make dev-setup` - one-time bootstrap: brings up infra, points you at
  `nix develop`/direnv for the toolchain
- `make dev-infra` - bring up infrastructure only (Redis, Postgres) via
  Docker Compose, without starting any services
- `make dev-infra-down` / `make dev-infra-clean` - stop infra, or stop
  and wipe its volumes
- `make dev-all` - bring up infra; now that feed-simulator exists under
  `services/<name>/cmd`, this also starts it (and the frontend dev
  server) together - for now it just brings up infra and says so
- `make dev-clean` - full teardown (infra + volumes)
- `make check-infra-ports` - warn if Redis/Postgres host ports are
  already taken before `dev-infra` brings the stack up (a prereq of
  `dev-infra`, also runnable standalone)
- `make check-docker` - verify Docker is installed and its daemon is
  reachable (a prereq of `dev-infra` and `dev-k8s`, also runnable
  standalone)
- `make help` - list every target with its one-line description
- `make dev-k8s` - create the local Kind cluster (if it doesn't already
  exist) and run `tilt up`: Redis/Postgres via Bitnami Helm charts, plus
  `otel-lgtm`, all in-cluster
- `make dev-k8s-down` - tear down Tilt resources and delete the Kind
  cluster

The Makefile is deliberately kept to environment-lifecycle plumbing, not
a home for every workflow - see
[DEV_SETUP.md](DEV_SETUP.md) for the narrative, step-by-step version of
"getting from a fresh checkout to a running local environment," which
will grow to chain these targets together (and note the gotchas) the
way the Makefile targets alone don't.

## Testing Conventions

- **Backend services** are tested with Ginkgo/Gomega. Tests should cover
  a service's behavior at its boundaries (its API, its event handling),
  not just individual functions.
- **Frontend unit/component tests** use Vitest, colocated with the code
  they test (e.g. `foo.test.ts` next to `foo.ts`). Mock at the module
  boundary the component actually imports (the Gateway client), not
  deeper - the realtime/REST calls, not the browser APIs underneath them.
- **Frontend end-to-end tests** use Playwright, living in the frontend
  app's own `e2e/` directory, against a running (or composed)
  environment. Cover flows that only make sense end-to-end - loading a
  match page and seeing a live update arrive - not things a unit test
  already covers.
- New behavior is written test-first where practical: write the test
  against the interface you want, confirm it fails for the right reason,
  then implement.
- Automated tests are expected before a service is considered part of
  the working system, in line with the goals in
  [GOALS.md](GOALS.md).

## Database Migrations

Schema changes are made via SQL-first Goose migrations, checked in
alongside the service that owns the schema. Migrations run as part of
the standard local and deployment workflow, not as a manual step.

## Observability While Developing

Services are expected to emit OpenTelemetry traces, metrics, and logs
from early in their development, not bolted on after the fact. Both
local dev loops already run `grafana/otel-lgtm` (an OTel collector,
Prometheus, Loki, Tempo, Pyroscope, and Grafana, bundled) since Phase 2 -
point a service's OTLP exporter at `localhost:4317` (or `otel-lgtm:4317`
in-cluster) and its traces/metrics/logs show up in Grafana at
`localhost:3000`, no separate setup needed. See
[TECH_STACK.md](TECH_STACK.md#observability) for how this relates to the
target production stack (Grafana/Loki/Mimir/Alloy).

## Charts & Local Config

**Single repo, on purpose.** Some reference setups split Helm charts and
deploy config into a separate repo from the application code. MatchFlow
doesn't - two devs don't need two repos to keep in sync, and a chart
that's out of step with the code it deploys is a worse failure mode than
a slightly bigger repo. Everything - app code, local dev infra, and
(later) MatchFlow's own service charts - lives here.

**Where charts live:**

- `k8s/local/` - local dev infra only: `kind-config.yaml` (the Kind
  cluster), `otel-lgtm.yaml` (a plain manifest, not a chart - one
  container doesn't need one), and `helm-values/` (values files for the
  third-party Bitnami Redis/Postgres charts, referenced by chart name in
  `Tiltfile` - the chart source itself isn't vendored, Tilt/Helm pull it
  from the Bitnami repo declared there). None of this deploys anywhere
  but a dev machine.
- `k8s/charts/<service>/` (doesn't exist yet - Phase 8, see
  [ROADMAP.md](ROADMAP.md)) - MatchFlow's own Helm chart per service,
  once there's an image to package. Local/staging/production differ by
  a `values-<env>.yaml` file per chart, not by a separate repo or a
  forked chart.

**Extending local infra**: adding a new piece of infra (another
datastore, another bundled tool) means updating both loops together,
since staying interchangeable is the whole point - add the container to
`docker-compose.dev.yml`, and add the matching chart + values file under
`k8s/local/helm-values/` plus a `helm_resource`/`k8s_yaml` line in
`Tiltfile`. Skipping one half leaves the loops silently diverged.

**Local env config**: no service reads an env var yet, so there's no
`.env` file to manage - see [ROADMAP.md](ROADMAP.md), Phase 1 note. When
one does, the convention is one root `.env` (gitignored) + a checked-in
`.env.example` documenting every key, the same file feeding both loops:
Docker Compose reads `.env` automatically, and the `Tiltfile` reads the
same file (`read_file`/`os.getenv`) rather than maintaining a second,
K8s-specific copy of the same config. Infra credentials
(Redis/Postgres) are the exception - they're hardcoded dev-only values
in `docker-compose.dev.yml` and `k8s/local/helm-values/*.yaml` since
nothing needs them to vary, and keeping them identical there is what
makes "same credentials, same ports" true across both loops.

## Optional Local Tooling

None of this is part of the stack - nothing here is installed, run, or
assumed by any Makefile target. It's an informed menu so picking a tool
is a choice, not a guess.

**Inspecting Postgres/Redis:**

- **CLI** (already in the Nix shell): `psql
  postgresql://matchflow:matchflow@localhost:5432/matchflow`,
  `redis-cli -h localhost`. Zero setup, works against either dev loop.
- **Desktop GUI clients**: DBeaver or TablePlus (Postgres, general SQL),
  pgAdmin (Postgres-specific), RedisInsight (Redis) - all connect to
  `localhost` like any other tool, no container needed since the ports
  are already exposed on the host.
- **Container/pod option**: pgAdmin, `redis-commander`, or RedisInsight
  can run *as* a service instead of a desktop app - add it to
  `docker-compose.dev.yml` for the Compose loop, or as a `helm_resource`/
  `k8s_yaml` resource in the `Tiltfile` for the K8s loop (see
  [Charts & Local Config](#charts--local-config) above for the pattern
  both already follow). Not done here by default - one more always-on
  container per loop for something a desktop app already covers for
  free.

**Pod/container management:**

- **CLI** (already in the Nix shell/host): `kubectl`, `docker`.
- **Terminal UI**: `k9s` (Kubernetes), `lazydocker` (Docker) - browse
  pods/containers, logs, and resource usage without leaving the
  terminal, no extra container to run.
- **GUI**: Portainer covers both Docker and Kubernetes, but runs as its
  own persistent container/deployment - one more thing to keep patched
  and secured even for local-only use, for what `k9s`/`lazydocker`
  already give you for free.

**Container CPU/RAM metrics (cAdvisor)**: not wired in - not because it
isn't useful, but because it's currently a production-cluster
observability concern (container-level resource metrics feeding cluster
alerting), not a local-dev-loop one, and MatchFlow has no production
cluster yet (Phase 8, see [ROADMAP.md](ROADMAP.md)). Revisit if/when
that phase lands. Separately: a native `dockerd` (see
[README.md](../README.md#getting-started)) reports real host CPU/RAM to
tools like this - Docker Desktop's own Linux VM would obscure the
numbers.

**Observability beyond Grafana**: `otel-lgtm` (see
[Observability While Developing](#observability-while-developing) above)
already bundles Prometheus, Loki, Tempo, and Pyroscope behind one
Grafana UI - traces, logs, metrics, and profiles in one pane of glass.
A standalone Jaeger UI or Prometheus UI would just be a second view of
data Grafana already shows; not worth the extra container unless a
specific limitation with the bundled view actually shows up.

## Contribution Flow

- Work happens on feature branches; open a pull request against the
  default branch.
- Keep changes scoped to a single service or concern where possible, to
  match the service-oriented structure of the repository.
- Documentation (this directory) should be updated alongside any change
  that affects architecture, tech stack, or workflow.

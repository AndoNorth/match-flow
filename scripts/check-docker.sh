#!/usr/bin/env bash
# Preflight for `make dev-infra` / `make dev-k8s`: Docker is a host-level
# prerequisite (a native dockerd - see README.md's Getting Started for
# why not Docker Desktop) - the Nix shell can't provide or start it,
# only check for it. Fails with a clear message instead of a confusing
# docker-compose/Kind connection error.
set -euo pipefail

if ! command -v docker >/dev/null 2>&1; then
  echo "check-docker: 'docker' not found on PATH." >&2
  echo "  Install a native Docker Engine - the Nix shell does not provide it. See README.md's Getting Started section." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "check-docker: 'docker' is installed but the daemon isn't reachable." >&2
  echo "  Start dockerd, then retry." >&2
  exit 1
fi

#!/usr/bin/env bash
# Pre-flight check for `make dev-infra`: warns if a host port
# docker-compose.dev.yml binds is already held by something else, so a
# stuck port shows up as a clear message here instead of a confusing
# compose failure (or worse, a silent bind to the wrong process).
#
# SKIP_INFRA_PORT_CHECK=1 skips this entirely.
# PORT_CHECK_MODE=warn (default) prints and continues; =fail exits 1.
set -euo pipefail

if [ -n "${SKIP_INFRA_PORT_CHECK:-}" ]; then
  exit 0
fi

PORTS=(6379 5432) # redis, postgres - keep in sync with docker-compose.dev.yml
MODE="${PORT_CHECK_MODE:-warn}"

port_tool=""
if command -v lsof >/dev/null 2>&1; then
  port_tool="lsof"
elif command -v ss >/dev/null 2>&1; then
  port_tool="ss"
else
  echo "check-infra-ports: neither lsof nor ss found, skipping port check" >&2
  exit 0
fi

occupied=()
for port in "${PORTS[@]}"; do
  if [ "$port_tool" = "lsof" ]; then
    lsof -iTCP:"$port" -sTCP:LISTEN -P -n -t >/dev/null 2>&1 && occupied+=("$port")
  else
    ss -ltn "( sport = :$port )" 2>/dev/null | grep -q ":$port" && occupied+=("$port")
  fi
done

if [ "${#occupied[@]}" -eq 0 ]; then
  exit 0
fi

echo "check-infra-ports: port(s) already in use: ${occupied[*]}" >&2
echo "  'docker compose up' will fail (or bind the wrong process) if these aren't Docker's own containers." >&2
echo "  Skip this check with SKIP_INFRA_PORT_CHECK=1." >&2

if [ "$MODE" = "fail" ]; then
  exit 1
fi

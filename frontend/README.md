# frontend

Next.js (App Router) client - the only service the human-facing UI talks to,
and it only talks to the Gateway Service.

## Running

```bash
make run SVC=frontend
```

Requires the Gateway Service reachable (`make run SVC=gateway-service`), which
in turn requires Redis, Postgres, and Match Service running - see the repo
root [DEVELOPMENT.md](../docs/DEVELOPMENT.md).

That's enough to load the app, but the match list stays empty until
something actually feeds events into the pipeline. To see live data:

```bash
make dev-infra                        # Redis, Postgres, otel-lgtm
make run SVC=match-service            # separate terminal
make run SVC=ingestion-service        # separate terminal
make run SVC=gateway-service          # separate terminal
make run SVC=frontend                 # separate terminal
make run SVC=feed-simulator           # separate terminal - starts emitting events
```

Feed Simulator posts events to Ingestion Service, which publishes them to
Redis; Match Service consumes them and persists state; the Gateway serves
that state to this frontend over REST and SSE. Each `make run` is its own
long-running process - five terminals (plus this one), or use a terminal
multiplexer. See [feed-simulator's README](../services/feed-simulator/README.md)
for its own env vars.

Serves on port `3001`, not Next's own default `3000` - `otel-lgtm`'s Grafana
UI already binds `3000` in local dev (see [DEVELOPMENT.md](../docs/DEVELOPMENT.md#observability-while-developing)),
and the two colliding meant Next silently fell back to a random port,
breaking the Gateway's CORS check. Revisit once containerization
(Phase 7) gives every service its own port mapping.

## Environment variables

| Variable                    | Default                 | Description        |
|------------------------------|--------------------------|---------------------|
| `NEXT_PUBLIC_GATEWAY_URL`    | `http://localhost:8083` | Gateway Service base URL, used by both server-rendered fetches and the browser's realtime connection |

## Testing

- `make test SVC=frontend` - Vitest unit/component tests.
- `make test-integration SVC=frontend` - Playwright e2e tests. Requires Redis,
  Match Service, and the Gateway Service running locally (`make dev-infra`,
  then `make run SVC=match-service` and `make run SVC=gateway-service` in
  separate terminals) - the suite seeds fixture events by publishing directly
  to Redis, bypassing Feed Simulator/Ingestion.

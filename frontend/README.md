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

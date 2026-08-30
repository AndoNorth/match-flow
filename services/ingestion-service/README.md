# ingestion-service

Receives events from Feed Simulator (or any future event source), validates and
normalizes them into one canonical shape, and publishes them to Redis - the
boundary between "whatever shape external data arrives in" and the shape the
rest of the system relies on.

## Building

```bash
make build SVC=ingestion-service
```

Builds `bin/ingestion-service`.

## Running

```bash
make run SVC=ingestion-service
```

Runs with Air hot-reload. Requires Redis reachable at `REDIS_URL` (`make
dev-infra` starts one locally) - the service pings Redis at startup and exits
if it's unreachable.

## Environment variables

No CLI flags - configuration is env-var only, matching every other service in
this repo.

| Variable    | Default                    | Description                       |
|-------------|-----------------------------|-----------------------------------|
| `PORT`      | `8081`                      | HTTP port                         |
| `REDIS_URL` | `redis://localhost:6379`    | Redis connection string           |

## Endpoints

- `GET /healthz` - returns 200 while the service is running. Not part of the
  API below (registered directly, not through Huma).

Everything else is served by Huma - browse `GET /docs` (interactive UI) or
fetch `GET /openapi.json` for the full, always-current schema of both
`POST /events/*` routes, their request shapes, and their response codes,
rather than a second, hand-maintained copy of the same thing here.

Both event routes validate the request, normalize it into one canonical
event, and publish it to the `matchflow:events` Redis pub/sub channel.

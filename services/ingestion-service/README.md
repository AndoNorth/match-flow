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

- `GET /healthz` - returns 200 while the service is running.
- `GET /openapi.json` - live OpenAPI spec (served automatically by Huma).
- `POST /events/provider-a` - submit a ProviderA-shaped match event
  (nested/abbreviated JSON: `data.mid`, `data.seq`, `data.typ`, `data.ts`,
  `data.pl`).
- `POST /events/provider-b` - submit a ProviderB-shaped match event
  (flat JSON: `match_id`, `sequence`, `event_type`, `occurred_at`, `details`).

Both event endpoints validate the request, normalize it into one canonical
event, and publish it to the `matchflow:events` Redis pub/sub channel. A
request that can't even be parsed as JSON gets `400`; a well-formed body that
fails schema validation (missing/wrong-typed required field) gets `422` -
Huma's own default status codes, not overridden.

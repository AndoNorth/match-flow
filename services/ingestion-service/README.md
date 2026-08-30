# ingestion-service

Receives events from Feed Simulator (or any future event source), validates and
normalizes them into one canonical shape, and publishes them to Redis.
See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md#ingestion-service) for
its role in the system, and
[docs/superpowers/specs/2026-08-30-ingestion-service-design.md](../../docs/superpowers/specs/2026-08-30-ingestion-service-design.md)
for the full design.

## Running

```bash
make run SVC=ingestion-service
```

Requires Redis reachable at `REDIS_URL` (`make dev-infra` starts one locally).

## Environment variables

| Variable    | Default                    | Description                       |
|-------------|-----------------------------|-----------------------------------|
| `PORT`      | `8081`                      | HTTP port                         |
| `REDIS_URL` | `redis://localhost:6379`    | Redis connection string           |

## Endpoints

- `GET /healthz` - returns 200 while the service is running.
- `GET /openapi.json` - live OpenAPI spec (served automatically by Huma).
- `POST /events/provider-a` - submit a ProviderA-shaped match event.
- `POST /events/provider-b` - submit a ProviderB-shaped match event.

Both event endpoints validate the request (Huma's own `400`/`422` split - see
the design spec), normalize it into one canonical event, and publish it to the
`matchflow:events` Redis pub/sub channel.

# gateway-service

The entry point for client applications. Proxies REST reads to Match
Service over gRPC and re-emits `matchflow:events` to connected clients
over SSE - the only service the frontend talks to.

## Building

```bash
make build SVC=gateway-service
```

Builds `bin/gateway-service`.

## Running

```bash
make run SVC=gateway-service
```

Runs with Air hot-reload. Requires Redis and Match Service reachable
(`make dev-infra` starts Redis locally; run Match Service separately
with `make run SVC=match-service`) - the service pings Redis at
startup and exits if it's unreachable.

## Environment variables

| Variable             | Default                  | Description                     |
|-----------------------|---------------------------|----------------------------------|
| `PORT`                | `8083`                    | HTTP port                       |
| `REDIS_URL`           | `redis://localhost:6379`  | Redis connection string         |
| `MATCH_SERVICE_ADDR`  | `localhost:8082`          | Match Service's gRPC address    |

## Talking to Match Service

This service's own outbound calls to Match Service (`internal/matchclient`) are gRPC (via
connect-go), not REST - Match Service's REST API exists for client consumption, but the
service-to-service call here is typed against
[`proto/matchflow/match_service/v1/match_service.proto`](../../proto/matchflow/match_service/v1/match_service.proto).
That `.proto` file is the source of truth for that contract; there's no `/openapi.json`
equivalent for it and no gRPC reflection registered on Match Service to introspect it at
runtime - see [ARCHITECTURE.md](../../docs/ARCHITECTURE.md#protobuf--grpc-structure).

## Endpoints

- `GET /healthz` - returns 200 while the service is running. Not part
  of the API below (registered directly, not through Huma).

Everything else is served by Huma - browse `GET /docs` or fetch
`GET /openapi.json` for the full schema.

- `GET /matches` - list matches, optional `?status=` filter.
- `GET /matches/{id}` - a single match's current state. `404` for an
  unknown match ID.
- `GET /matches/{id}/events` - a match's full event timeline.
- `GET /events?match_id=<id>` (SSE, outside Huma) - streams an
  `event: snapshot` frame with the current match state (or every
  match's state if `match_id` is omitted), then an `event: update`
  frame per subsequent event for that match. Verify manually with
  `curl -N http://localhost:8083/events?match_id=<id>` - no
  buffering, prints each frame as it streams.

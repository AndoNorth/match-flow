# match-service

Subscribes to Ingestion Service's `matchflow:events` Redis channel, maintains
current match state (status/score/clock) and a full event timeline in
PostgreSQL, and exposes both read-only over REST - the system of record for
"what is currently true about a match."

## Building

```bash
make build SVC=match-service
```

Builds `bin/match-service`.

## Running

```bash
make run SVC=match-service
```

Runs with Air hot-reload. Requires Redis and PostgreSQL reachable (`make
dev-infra` starts both locally) - the service pings Redis and runs its Goose
migrations against Postgres at startup, exiting if either is unreachable.

## Environment variables

| Variable                      | Default                                                        | Description                              |
|--------------------------------|------------------------------------------------------------------|-------------------------------------------|
| `PORT`                        | `8082`                                                          | HTTP port                                |
| `REDIS_URL`                   | `redis://localhost:6379`                                        | Redis connection string                  |
| `POSTGRES_DSN`                | `postgres://matchflow:matchflow@localhost:5432/matchflow?sslmode=disable` | Postgres connection string      |
| `MATCH_SERVICE_WORKERS`       | `4`                                                              | Worker pool size                         |
| `MATCH_SERVICE_DEFAULT_SPORT` | `football`                                                       | Sport a newly-seen match is created with |

## Endpoints

- `GET /healthz` - returns 200 while the service is running, independent of
  Redis/Postgres reachability.

Everything else is served by Huma - browse `GET /docs` (interactive UI) or
fetch `GET /openapi.json` for the full, always-current schema, rather than a
second, hand-maintained copy of the same thing here.

- `GET /matches` - list matches, optional `?status=` filter.
- `GET /matches/{id}` - a single match's current state. `404` for an unknown
  match ID.
- `GET /matches/{id}/events` - a match's full event timeline, ordered by
  sequence. `404` for an unknown match ID.

A gRPC (connect-go) API is also mounted on the same port, at path prefix
`/matchflow.match_service.v1.MatchService/`, exposing the same three reads as
the REST routes above (`ListMatches`, `GetMatch`, `ListMatchEvents`) - this is
what Gateway Service talks to.

Nothing here creates or mutates a match - state comes entirely from the
`matchflow:events` Redis channel Ingestion Service publishes to.

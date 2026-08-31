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

- `GET /healthz` - returns 200 while the service is running.

Everything else is served by Huma - browse `GET /docs` or fetch
`GET /openapi.json` for the current schema.

# feed-simulator

Simulates a third-party sports data provider - generates a continuous,
believable stream of football match events, alternating between two
structurally different provider wire shapes, and submits each one to
Ingestion Service.

## Building

```bash
make build SVC=feed-simulator
```

Builds `bin/feed-simulator`.

## Running

```bash
make run SVC=feed-simulator
```

Runs with Air hot-reload. Ingestion Service doesn't need to be running first -
a submission failure is logged and the simulation continues, so a briefly
unreachable Ingestion Service never stops event generation.

## Environment variables

No CLI flags - configuration is env-var only, matching every other service in
this repo.

| Variable         | Default                   | Description                                                                |
|------------------|----------------------------|-----------------------------------------------------------------------------|
| `PORT`           | `8080`                     | HTTP port for `/healthz`                                                    |
| `INGESTION_URL`  | `http://localhost:8081`   | Base URL of Ingestion Service - each event is POSTed here                  |

## Endpoints

- `GET /healthz` - returns 200 while the service is running.

Every generated event is logged to stdout **and** POSTed to Ingestion Service,
alternating `POST /events/provider-a` (nested/abbreviated JSON shape) and
`POST /events/provider-b` (flat JSON shape) to match which encoder produced
it - two structurally different payloads for the same logical event, on
purpose, simulating two competing real-world data providers.

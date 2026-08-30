# feed-simulator

Simulates a third-party sports data provider - generates a continuous,
believable stream of football match events for the rest of the system
to consume.
See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md#feed-simulator)
for its role in the system, and
[docs/superpowers/specs/2026-08-30-feed-simulator-and-service-dev-loop-design.md](../../docs/superpowers/specs/2026-08-30-feed-simulator-and-service-dev-loop-design.md)
for the original design, plus
[docs/superpowers/specs/2026-08-30-ingestion-service-design.md](../../docs/superpowers/specs/2026-08-30-ingestion-service-design.md)
for how it wires into Ingestion Service.

## Running

```bash
make run SVC=feed-simulator
```

## Environment variables

| Variable         | Default                   | Description                                                                |
|------------------|----------------------------|-----------------------------------------------------------------------------|
| `PORT`           | `8080`                     | HTTP port for `/healthz`                                                    |
| `INGESTION_URL`  | `http://localhost:8081`   | Base URL of Ingestion Service - each event is POSTed here                  |

## Endpoints

- `GET /healthz` - returns 200 while the service is running.

Every generated event is logged to stdout **and** POSTed to Ingestion Service
(alternating `/events/provider-a` / `/events/provider-b` to match which
encoder produced it). A submission failure is logged and the simulation
continues - Ingestion being briefly unreachable never stops event generation.

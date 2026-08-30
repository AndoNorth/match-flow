# feed-simulator

Simulates a third-party sports data provider - generates a continuous,
believable stream of football match events for the rest of the system
to consume.
See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md#feed-simulator)
for its role in the system, and
[docs/superpowers/specs/2026-08-30-feed-simulator-and-service-dev-loop-design.md](../../docs/superpowers/specs/2026-08-30-feed-simulator-and-service-dev-loop-design.md)
for the full design.

## Running

```bash
make run SVC=feed-simulator
```

## Environment variables

| Variable | Default | Description |
|----------|---------|--------------|
| `PORT`   | `8080`  | HTTP port for `/healthz` (picked arbitrarily - no doc precedent, open to change) |

## Endpoints

- `GET /healthz` - returns 200 while the service is running.

Events are logged to stdout only for now (no Redis/Ingestion wiring
yet - see the design spec above).

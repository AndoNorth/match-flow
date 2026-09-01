# feed-simulator

Simulates a third-party sports data provider - continuously feeds live
matches, alternating between two structurally different provider wire
shapes, and submits each event to Ingestion Service. Runs one match after
another forever by default; `POST /control/start`/`stop` control that loop,
and `POST /matches/trigger` starts one extra match on demand, all
independent of each other - multiple matches can be in progress at once.

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
| `PORT`           | `8080`                     | HTTP port                                                                   |
| `INGESTION_URL`  | `http://localhost:8081`   | Base URL of Ingestion Service - each event is POSTed here                  |
| `TEMPLATES_DIR`  | `../../templates` (this service's `templates/` dir, relative to Air's cwd) | Directory `template`/`kind` JSON files are loaded from |

## Endpoints

- `GET /healthz` - returns 200 while the service is running.
- `POST /control/start` - turn on the auto-respawn loop: a match runs, and as
  soon as it finishes another starts, forever. Body `{"template": "<name>"}`
  picks a template for every respawned match (see below); omit it for the
  default unbounded-random mode this service always ran in before these
  endpoints existed. If no match is currently running, one starts
  immediately. Returns `{"match_id": "..."}`.
- `POST /control/stop` - turn the auto-loop off and immediately cancel every
  currently-running match (a hard stop, not "let it finish naturally").
  Returns `{"running": 0}`.
- `POST /matches/trigger` - start exactly one additional match right now,
  running alongside anything already in progress, independent of the
  auto-loop's on/off state. Same body/response shape as `/control/start`.
- `GET /templates` - list available template names (every `*.json` file's
  own `name` field in `TEMPLATES_DIR`, not necessarily its filename).

Every generated event is logged to stdout **and** POSTed to Ingestion Service,
alternating `POST /events/provider-a` (nested/abbreviated JSON shape) and
`POST /events/provider-b` (flat JSON shape) to match which encoder produced
it - two structurally different payloads for the same logical event, on
purpose, simulating two competing real-world data providers.

## Templates

A template is one named match profile - a JSON file in `templates/` (see
`internal/simulator/template` for the loader, `internal/simulator/football`
for how a template turns into an actual sequence of events). Two kinds:

- **`"kind": "bounded"`** - exact or ranged goal/card counts
  (`home_goals`/`away_goals`/`yellow_cards`/`red_cards`, each
  `{"min": N, "max": N}`), randomly timed. `kickoff`/`half_time`/`full_time`
  always happen regardless - a template only controls goals and cards.
  `{"min": 0, "max": 0}` (or omitting a field) means "never happens".
- **`"kind": "literal"`** - a fully scripted `events` list
  (`{"type": "goal", "team": "home", "minute": 10}`, etc.), played in minute
  order regardless of the order they're declared in the file. `kickoff` is
  always automatic and implicit - don't include it. If the list has no
  `full_time` entry, the match just runs out of schedule and ends quietly
  once every entry has fired.

See `templates/goalless_draw.json`, `templates/goalless_yellow.json`,
`templates/high_scoring_chaos.json`, and `templates/scripted_demo.json` for
one example of each shape.

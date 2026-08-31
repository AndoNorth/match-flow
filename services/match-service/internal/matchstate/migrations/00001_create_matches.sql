-- +goose Up
CREATE TABLE matches (
    match_id      TEXT PRIMARY KEY,
    sport         TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'scheduled',
    home_score    INT NOT NULL DEFAULT 0,
    away_score    INT NOT NULL DEFAULT 0,
    clock_mins    INT NOT NULL DEFAULT 0,
    last_sequence INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE matches;

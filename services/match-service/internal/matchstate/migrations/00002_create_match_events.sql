-- +goose Up
CREATE TABLE match_events (
    id          BIGSERIAL PRIMARY KEY,
    match_id    TEXT NOT NULL REFERENCES matches(match_id),
    sequence    INT NOT NULL,
    type        TEXT NOT NULL,
    payload     JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL,
    UNIQUE (match_id, sequence)
);

-- +goose Down
DROP TABLE match_events;

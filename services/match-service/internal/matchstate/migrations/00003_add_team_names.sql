-- +goose Up
ALTER TABLE matches
    ADD COLUMN home_team TEXT NOT NULL DEFAULT '',
    ADD COLUMN away_team TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE matches
    DROP COLUMN home_team,
    DROP COLUMN away_team;

-- +goose Up
-- The acting user's last model choice per stage (PRD F-4; plan 04). This table exists
-- only to refill the dropdowns: the Start* RPCs receive their ModelRefs explicitly and
-- record them on the job row, so a row here is never read on the generation path.
CREATE TABLE model_selections (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL, -- observe | write | analyze
    provider_id TEXT NOT NULL,
    model_id    TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, stage)
);

-- +goose Down
DROP TABLE model_selections;

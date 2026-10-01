-- +goose Up
CREATE TABLE model_selections_v3 (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL CHECK (stage IN ('observe', 'write', 'analyze')),
    slot        TEXT NOT NULL CHECK (slot IN ('active', 'candidate_a', 'candidate_b', 'candidate_c', 'candidate_d', 'candidate_e')),
    provider_id TEXT NOT NULL,
    model_id    TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, stage, slot)
);
INSERT INTO model_selections_v3 (user_id, stage, slot, provider_id, model_id, updated_at)
SELECT user_id, stage, slot, provider_id, model_id, updated_at FROM model_selections;
DROP TABLE model_selections;
ALTER TABLE model_selections_v3 RENAME TO model_selections;

-- +goose Down
CREATE TABLE model_selections_v2 (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL CHECK (stage IN ('observe', 'write', 'analyze')),
    slot        TEXT NOT NULL CHECK (slot IN ('active', 'candidate_a', 'candidate_b')),
    provider_id TEXT NOT NULL,
    model_id    TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, stage, slot)
);
INSERT INTO model_selections_v2 (user_id, stage, slot, provider_id, model_id, updated_at)
SELECT user_id, stage, slot, provider_id, model_id, updated_at FROM model_selections
WHERE slot IN ('active', 'candidate_a', 'candidate_b');
DROP TABLE model_selections;
ALTER TABLE model_selections_v2 RENAME TO model_selections;

-- +goose Up
-- Recommendation sets are operator-curated rows (MODEL-69). They used to be a list in
-- providers.yaml, which made changing the advice a deploy; the file now declares the
-- provider connection alone.
--
-- A set is advice, not a choice: applying one copies its slots into model_selections, and
-- nothing here is referenced from there (MODEL-71). The slots name catalog models WITHOUT a
-- foreign key on purpose: a model the operator retires must not silently rewrite a saved set,
-- it is flagged on the admin list instead (MODEL-70), and a removed ref stays legible
-- (MODEL-40).
CREATE TABLE recommendation_sets (
    id         TEXT PRIMARY KEY,
    label      TEXT NOT NULL,
    -- The operator's order. Not unique: a move swaps two rows inside one transaction, and a
    -- tie (never written by the app) still reads in a stable order by created_at, id.
    position   INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- The stage/slot vocabulary is model_selections' own, so a set maps one-to-one onto an
-- apply. Analyze keeps its active slot alone (MODEL-23).
CREATE TABLE recommendation_set_slots (
    set_id      TEXT NOT NULL REFERENCES recommendation_sets(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL CHECK (stage IN ('observe', 'analyze', 'write')),
    slot        TEXT NOT NULL CHECK (slot IN ('active', 'candidate_a', 'candidate_b')),
    provider_id TEXT NOT NULL,
    model_id    TEXT NOT NULL,
    PRIMARY KEY (set_id, stage, slot),
    CHECK (stage <> 'analyze' OR slot = 'active')
);

-- The one set a fresh installation starts with (MODEL-26): the set providers.yaml shipped,
-- slot for slot, so an existing installation keeps offering exactly what it offered.
INSERT INTO recommendation_sets (id, label, position, created_at, updated_at) VALUES
    ('balanced-2026-08', 'Balanced · August 2026', 1,
     '2026-09-30T00:00:00.000000000Z', '2026-09-30T00:00:00.000000000Z');

INSERT INTO recommendation_set_slots (set_id, stage, slot, provider_id, model_id) VALUES
    ('balanced-2026-08', 'observe', 'active',      'openrouter', 'google/gemini-3.7-flash'),
    ('balanced-2026-08', 'observe', 'candidate_a', 'openrouter', 'google/gemini-3.7-flash'),
    ('balanced-2026-08', 'observe', 'candidate_b', 'openrouter', 'qwen/qwen3.8-flash'),
    ('balanced-2026-08', 'analyze', 'active',      'openrouter', 'openai/gpt-5.6-luna'),
    ('balanced-2026-08', 'write',   'active',      'openrouter', 'anthropic/claude-sonnet-5'),
    ('balanced-2026-08', 'write',   'candidate_a', 'openrouter', 'anthropic/claude-sonnet-5'),
    ('balanced-2026-08', 'write',   'candidate_b', 'openrouter', 'x-ai/grok-4.6');

-- +goose Down
DROP TABLE recommendation_set_slots;
DROP TABLE recommendation_sets;

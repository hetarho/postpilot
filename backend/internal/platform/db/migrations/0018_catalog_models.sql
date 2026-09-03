-- +goose Up
-- The usable-model catalog (plan 18). Until now the list of models a user could pick lived
-- in providers.yaml, so adding or retiring one was a repo edit plus an image rebuild. The
-- candidates now come live from OpenRouter's public catalog and an operator curates them
-- here, which makes the decision the yaml encoded — WHICH models are worth exposing, and at
-- which plan tier — a product surface instead of a deploy.
--
-- providers.yaml keeps what it is still the authority for: how to REACH the provider (base
-- url, key env, reasoning dialect) and the versioned recommendation sets. Connection is a
-- file; models are rows.
--
-- A row exists once an operator has touched the model, and survives disabling: min_plan and
-- the reasoning override are curation decisions that must come back intact when the model is
-- re-enabled, so `enabled = 0` is a hidden row rather than a deleted one.
CREATE TABLE catalog_models (
    -- The OpenRouter id, e.g. 'openai/gpt-5.6-sol'. It is also ModelRef.model_id, so the
    -- selections and job rows written before this migration keep pointing at the same models.
    model_id           TEXT PRIMARY KEY,
    -- The vendor segment before the first '/'. Derived at write and stored so the admin
    -- screen can group and filter without re-splitting every id; it is NOT the registry's
    -- provider id (that stays 'openrouter' for every row).
    provider_slug      TEXT NOT NULL,
    label              TEXT NOT NULL,
    vision             INTEGER NOT NULL CHECK (vision IN (0,1)),
    structured_output  INTEGER NOT NULL CHECK (structured_output IN (0,1)),
    -- Dated display metadata, refreshed from the source on an operator refresh. Nullable
    -- because a candidate may publish no pricing; reported provider cost stays authoritative.
    context_tokens         INTEGER,
    input_usd_per_million  TEXT,
    output_usd_per_million TEXT,
    pricing_checked_at     TEXT,
    -- The lowest plan allowed to run this model (plan 17). Required, as it was in the yaml:
    -- a model with no floor would silently be free for everyone.
    min_plan           TEXT NOT NULL CHECK (min_plan IN ('free','basic','max')),
    -- Optional strict per-model override of the stage reasoning policy. NULL defers to the
    -- stage; 'unset' deliberately omits the wire key and keeps provider defaults.
    reasoning_effort   TEXT CHECK (reasoning_effort IN ('unset','none','minimal','low','medium','high','xhigh','max')),
    enabled            INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
    -- Whether the source still offered this model at the last SUCCESSFUL refresh. It is a
    -- flag, never an action: a delisted model is served disabled-with-reason and waits for an
    -- operator, so an OpenRouter outage or a one-off bad response cannot retire a model.
    listed             INTEGER NOT NULL DEFAULT 1 CHECK (listed IN (0,1)),
    last_seen_at       TEXT,
    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL
);

-- The user-facing catalog reads the enabled rows on every ListModels; the admin screen reads
-- all of them. Both want the grouping order the screen shows.
CREATE INDEX idx_catalog_models_enabled ON catalog_models(enabled, provider_slug, model_id);

-- The twelve models providers.yaml shipped, carried over verbatim (labels, capability flags,
-- dated metadata, floors, and claude-sonnet-5's 'unset' reasoning override). This is what
-- makes the cutover invisible: every saved selection still resolves, so nothing is cleared by
-- the vanished-selection rule and generation behaves identically.
--
-- created_at/updated_at are a fixed instant rather than a clock read: goose applies this file
-- once per database, and the value is only ever shown as "when curation last touched the row".
INSERT INTO catalog_models (
    model_id, provider_slug, label, vision, structured_output,
    context_tokens, input_usd_per_million, output_usd_per_million, pricing_checked_at,
    min_plan, reasoning_effort, enabled, listed, last_seen_at, created_at, updated_at
) VALUES
    ('openrouter/free', 'openrouter', 'OpenRouter free (auto)', 1, 0,
     NULL, NULL, NULL, NULL, 'free', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('z-ai/glm-5.3-flash', 'z-ai', 'GLM 5.3 Flash', 1, 1,
     1310720, '0.075', '0.25', '2026-08-30', 'free', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('qwen/qwen3.8-flash', 'qwen', 'Qwen3.8 Flash', 1, 1,
     1000000, '0.15', '0.47', '2026-08-30', 'free', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('google/gemini-3.7-flash', 'google', 'Gemini 3.7 Flash', 1, 1,
     1048576, '0.75', '3.75', '2026-08-30', 'basic', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('deepseek/deepseek-v4-flash-0731', 'deepseek', 'DeepSeek V4 Flash 0731', 0, 1,
     1310720, '0.045', '0.09', '2026-08-30', 'free', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('deepseek/deepseek-v4-pro-0813', 'deepseek', 'DeepSeek V4 Pro 0813', 0, 1,
     1048576, '0.66', '1.98', '2026-08-30', 'basic', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('z-ai/glm-5.3', 'z-ai', 'GLM 5.3', 0, 1,
     1310720, '1.40', '4.40', '2026-08-30', 'basic', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('openai/gpt-5.6-luna', 'openai', 'GPT-5.6 Luna', 1, 1,
     1050000, '0.20', '1.20', '2026-08-30', 'basic', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('openai/gpt-5.6-sol', 'openai', 'GPT-5.6 Sol', 1, 1,
     1050000, '2.00', '10.00', '2026-08-30', 'max', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('x-ai/grok-4.6', 'x-ai', 'Grok 4.6', 1, 1,
     500000, '2.00', '6.00', '2026-08-30', 'max', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('anthropic/claude-sonnet-5', 'anthropic', 'Claude Sonnet 5', 1, 1,
     1000000, '2.00', '10.00', '2026-08-30', 'max', 'unset', 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z'),
    ('anthropic/claude-opus-5', 'anthropic', 'Claude Opus 5', 1, 1,
     1000000, '5.00', '25.00', '2026-08-30', 'max', NULL, 1, 1, NULL,
     '2026-09-03T00:00:00.000000000Z', '2026-09-03T00:00:00.000000000Z');

-- +goose Down
DROP INDEX idx_catalog_models_enabled;
DROP TABLE catalog_models;

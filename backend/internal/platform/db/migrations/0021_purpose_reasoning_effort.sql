-- +goose Up
-- The reasoning override moves onto the REGISTRATION (change 24).
--
-- Change 20 made registration per purpose; the override stayed one column on
-- catalog_models, so a model registered to both photo-analysis and writing had exactly one
-- effort for both while the code-owned stage policy was per stage. On 2026-09-03 an
-- operator lowered the effort on the 글 작성 tab and silently changed photo observation —
-- a different task with a different policy — and the value chosen (`minimal`, an
-- OpenAI-dialect value the observing model does not honor) behaved like sending nothing:
-- 647-733 completion tokens became 8,192, the cap, and the observation failed.
--
-- The PK is already (model_id, purpose), so the column just joins it. The value CHECK is
-- the one the dropped column carried.
ALTER TABLE catalog_model_purposes ADD COLUMN reasoning_effort TEXT CHECK (reasoning_effort IN
    ('unset','none','minimal','low','medium','high','xhigh','max'));

-- Dropped WITHOUT copying anything forward, which clears every existing override and
-- returns every curated model to the code-owned stage policy (observation `low`, writing
-- and revision `low`, analysis nothing). That is the configuration measured working before
-- 09-03; carrying today's blanket `minimal` into five purposes each would propagate a
-- setting already known to break observation.
ALTER TABLE catalog_models DROP COLUMN reasoning_effort;

-- Where a completion budget actually went. The ledger recorded prompt and completion
-- tokens and cost, so "wrote 8,192 tokens of post" and "spent 8,192 tokens thinking and
-- wrote nothing" were indistinguishable and both 09-03 investigations had to infer the
-- split from an empty body. Zero keeps meaning "not reported", like the other usage
-- fields, which is also what every pre-existing row correctly says.
ALTER TABLE usage_events ADD COLUMN reasoning_tokens INTEGER NOT NULL DEFAULT 0;

-- The spend aggregate is scanned as (stage, time range), and the only existing index on this
-- table is (user_id, created_at) for the quota windows. Without this every load of an
-- operator's purpose tab full-scans the ledger, which grows one row per provider call.
CREATE INDEX idx_usage_events_stage_created ON usage_events(stage, created_at);

-- +goose Down
DROP INDEX idx_usage_events_stage_created;
ALTER TABLE usage_events DROP COLUMN reasoning_tokens;
ALTER TABLE catalog_models ADD COLUMN reasoning_effort TEXT CHECK (reasoning_effort IN
    ('unset','none','minimal','low','medium','high','xhigh','max'));
ALTER TABLE catalog_model_purposes DROP COLUMN reasoning_effort;

-- +goose Up
-- The operator's user-facing LEVEL, per registration (MODEL-57).
--
-- A user picking a model sees ids and prices and has no way to read "which of these is the
-- good one". The level is the operator's answer in four words the product owns
-- (가성비·밸런스·고급·최고), and it rides the registration rather than the model because the
-- same model is a different bargain for a different task: photo analysis pays for input
-- tokens per photo, writing pays for output tokens, and one price band cannot say both.
--
-- The column joins the PK (model_id, purpose) exactly as reasoning_effort did in 0021.
--
-- NULLABLE AND NOT BACKFILLED. Unset is the absence of a value, not a fifth level: deriving
-- one from today's prices would bake a snapshot of a catalog that reprices every few weeks
-- into a column the operator then has to correct. Every existing registration reads as
-- unset, which is what it is, and unset sorts last and shows no badge (MODEL-58).
ALTER TABLE catalog_model_purposes ADD COLUMN level TEXT CHECK (level IN
    ('value','balanced','premium','top'));

-- +goose Down
ALTER TABLE catalog_model_purposes DROP COLUMN level;

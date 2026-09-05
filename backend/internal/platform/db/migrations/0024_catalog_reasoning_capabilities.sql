-- +goose Up
-- What OpenRouter publishes about each model's reasoning (change 27).
--
-- The catalog consumed exactly one entry of `supported_parameters` (`structured_outputs`)
-- and ignored the response's top-level `reasoning` object entirely, so the admin offered the
-- same eight effort values for every model. `deepseek/deepseek-v4-pro-0813` accepts
-- max·high·low and not `medium`, and is disable-able yet lists no `none`: an operator could
-- pick a value the model does not take, and `unset` read as "will not reason" when it means
-- the model's own default.
--
-- ABSENCE IS NOT "SUPPORTS NOTHING", the same rule the pricing columns already follow: the
-- defaults below make every existing row read as UNKNOWN, which is exactly what it is until
-- the next refresh. Nothing is backfilled and no fetch happens at boot — a refresh is an
-- explicit admin action, and only a successful one writes.
ALTER TABLE catalog_models ADD COLUMN reasons INTEGER NOT NULL DEFAULT 0;
ALTER TABLE catalog_models ADD COLUMN reasoning_mandatory INTEGER NOT NULL DEFAULT 0;
-- Whether the provider receives the effort STRING itself rather than a token budget
-- OpenRouter derived from it. Nothing consumes it in this migration; change 29 needs it to
-- size a completion budget safely.
ALTER TABLE catalog_models ADD COLUMN reasoning_native_effort INTEGER NOT NULL DEFAULT 0;
ALTER TABLE catalog_models ADD COLUMN reasoning_max_tokens INTEGER NOT NULL DEFAULT 0;
-- The source returns supported_efforts in DESCENDING effort order and that order is
-- meaningful (it is the order a selector should offer), so the list is stored exactly as the
-- source ordered it rather than normalized or sorted.
ALTER TABLE catalog_models ADD COLUMN reasoning_efforts TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_models ADD COLUMN reasoning_default_effort TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE catalog_models DROP COLUMN reasoning_default_effort;
ALTER TABLE catalog_models DROP COLUMN reasoning_efforts;
ALTER TABLE catalog_models DROP COLUMN reasoning_max_tokens;
ALTER TABLE catalog_models DROP COLUMN reasoning_native_effort;
ALTER TABLE catalog_models DROP COLUMN reasoning_mandatory;
ALTER TABLE catalog_models DROP COLUMN reasons;

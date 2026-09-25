-- +goose Up
-- An empty intro or outro preset has meant "the shared defaults" (CLIP-139).
-- The defaults become intro A and outro B for new projects (CLIP-111), so every
-- project that never chose first takes the presets it renders in today — intro
-- B, outro E — and keeps its look. Nothing is re-rendered or marked stale.
UPDATE clip_projects SET intro_preset = 'b' WHERE intro_preset = '';
UPDATE clip_projects SET outro_preset = 'e' WHERE outro_preset = '';

-- +goose Down
-- A backfilled id reads exactly like one the owner chose, and an older binary
-- renders both the same, so there is nothing to put back.
SELECT 1;

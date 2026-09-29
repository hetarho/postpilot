-- +goose Up
-- MODEL r21 (T474): 말투 반영 비교 is a write comparison sourced from one voice instead of a post
-- (MODEL-67). The stage stays 'write' so its verdicts count on the write board; `source` tells
-- the variant apart, and the prompt and the answer it withheld are named, the answer's text
-- living only in the purgeable snapshot (MODEL-42).
ALTER TABLE model_experiments ADD COLUMN source TEXT NOT NULL DEFAULT 'post'
    CHECK (source IN ('post', 'voice'));
ALTER TABLE model_experiments ADD COLUMN voice_prompt_key TEXT;
ALTER TABLE model_experiments ADD COLUMN voice_material_id TEXT;

-- +goose Down
ALTER TABLE model_experiments DROP COLUMN voice_material_id;
ALTER TABLE model_experiments DROP COLUMN voice_prompt_key;
ALTER TABLE model_experiments DROP COLUMN source;

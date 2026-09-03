-- +goose Up
-- Purpose-scoped model enablement (change 20). Curation stops being one global `enabled`
-- flag: a model is REGISTERED TO PURPOSES — photo-analysis, style-analysis, writing,
-- image-generation, video-generation — and only the first three feed a user-facing stage.
-- The generation purposes are settings for features that do not exist yet; nothing reads
-- them outside the operator screen.
--
-- Deliberately NO seed rows (interview decision): every purpose starts empty, all stage
-- dropdowns come up empty after the cutover, and previously saved selections are cleared by
-- the existing vanished-selection machinery when they are next read. The operator
-- re-curates each purpose from /admin.
CREATE TABLE catalog_model_purposes (
    model_id   TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    purpose    TEXT NOT NULL CHECK (purpose IN
                 ('photo-analysis','style-analysis','writing','image-generation','video-generation')),
    created_at TEXT NOT NULL,
    PRIMARY KEY (model_id, purpose)
);

-- What a model can OUTPUT, from the source's architecture.output_modalities. The image/video
-- generation purposes gate on these the way photo-analysis gates on `vision`. Existing rows
-- start at 0 and pick up their real value on the next successful operator refresh — until
-- then they simply cannot be registered to a generation purpose.
ALTER TABLE catalog_models ADD COLUMN image_output INTEGER NOT NULL DEFAULT 0 CHECK (image_output IN (0,1));
ALTER TABLE catalog_models ADD COLUMN video_output INTEGER NOT NULL DEFAULT 0 CHECK (video_output IN (0,1));

-- `enabled` has nothing left to answer: user visibility is now "registered to a purpose",
-- and a row with no registrations is exactly what `enabled = 0` used to be — kept, so the
-- reasoning override survives, but served to nobody. The index referencing the column must
-- go first; the browse/list order it served keeps an index without the dropped column.
DROP INDEX idx_catalog_models_enabled;
ALTER TABLE catalog_models DROP COLUMN enabled;
CREATE INDEX idx_catalog_models_order ON catalog_models(provider_slug, model_id);

-- +goose Down
DROP INDEX idx_catalog_models_order;
ALTER TABLE catalog_models ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1));
CREATE INDEX idx_catalog_models_enabled ON catalog_models(enabled, provider_slug, model_id);
ALTER TABLE catalog_models DROP COLUMN video_output;
ALTER TABLE catalog_models DROP COLUMN image_output;
DROP TABLE catalog_model_purposes;

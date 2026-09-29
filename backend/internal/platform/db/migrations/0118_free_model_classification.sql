-- +goose Up
CREATE TABLE catalog_model_purposes_new (
    model_id TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN
        ('photo-analysis','style-analysis','writing','image-generation','video-generation')),
    created_at TEXT NOT NULL,
    reasoning_effort TEXT CHECK (reasoning_effort IN
        ('unset','none','minimal','low','medium','high','xhigh','max')),
    level TEXT CHECK (level IN ('free','value','balanced','premium','top')),
    PRIMARY KEY (model_id, purpose)
);
INSERT INTO catalog_model_purposes_new(model_id,purpose,created_at,reasoning_effort,level)
SELECT model_id,purpose,created_at,reasoning_effort,level FROM catalog_model_purposes;
DROP TABLE catalog_model_purposes;
ALTER TABLE catalog_model_purposes_new RENAME TO catalog_model_purposes;
ALTER TABLE usage_admissions ADD COLUMN admitted_plan TEXT;
ALTER TABLE usage_admissions ADD COLUMN admitted_models_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE usage_admissions DROP COLUMN admitted_models_json;
ALTER TABLE usage_admissions DROP COLUMN admitted_plan;
CREATE TABLE catalog_model_purposes_old (
    model_id TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN
        ('photo-analysis','style-analysis','writing','image-generation','video-generation')),
    created_at TEXT NOT NULL,
    reasoning_effort TEXT CHECK (reasoning_effort IN
        ('unset','none','minimal','low','medium','high','xhigh','max')),
    level TEXT CHECK (level IN ('value','balanced','premium','top')),
    PRIMARY KEY (model_id, purpose)
);
INSERT INTO catalog_model_purposes_old(model_id,purpose,created_at,reasoning_effort,level)
SELECT model_id,purpose,created_at,reasoning_effort,
       CASE WHEN level='free' THEN NULL ELSE level END FROM catalog_model_purposes;
DROP TABLE catalog_model_purposes;
ALTER TABLE catalog_model_purposes_old RENAME TO catalog_model_purposes;

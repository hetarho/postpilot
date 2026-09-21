-- +goose Up
-- Estimator combos now use the exact same vocabulary as registration levels (QUOTA-39).
-- Rebuild because SQLite cannot replace the existing CHECK constraint in place.
CREATE TABLE estimator_combos_next (
    combo            TEXT PRIMARY KEY
                     CHECK (combo IN ('value','balanced','premium','top')),
    observe_model_id TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    write_model_id   TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    updated_at       TEXT NOT NULL
);

-- The old and new vocabularies both had four ordered steps. Preserve an assignment only
-- when both purpose registrations already carry the destination level; anything else must
-- return to the explicit unassigned state rather than appearing under the wrong card.
INSERT INTO estimator_combos_next (combo, observe_model_id, write_model_id, updated_at)
SELECT
    CASE old.combo
        WHEN 'cheapest' THEN 'value'
        WHEN 'value' THEN 'balanced'
        WHEN 'balanced' THEN 'premium'
        WHEN 'quality' THEN 'top'
    END,
    old.observe_model_id,
    old.write_model_id,
    old.updated_at
FROM estimator_combos AS old
JOIN catalog_model_purposes AS observe_registration
  ON observe_registration.model_id = old.observe_model_id
 AND observe_registration.purpose = 'photo-analysis'
JOIN catalog_model_purposes AS write_registration
  ON write_registration.model_id = old.write_model_id
 AND write_registration.purpose = 'writing'
WHERE observe_registration.level = CASE old.combo
        WHEN 'cheapest' THEN 'value'
        WHEN 'value' THEN 'balanced'
        WHEN 'balanced' THEN 'premium'
        WHEN 'quality' THEN 'top'
    END
  AND write_registration.level = CASE old.combo
        WHEN 'cheapest' THEN 'value'
        WHEN 'value' THEN 'balanced'
        WHEN 'balanced' THEN 'premium'
        WHEN 'quality' THEN 'top'
    END;

DROP TABLE estimator_combos;
ALTER TABLE estimator_combos_next RENAME TO estimator_combos;

-- +goose Down
CREATE TABLE estimator_combos_previous (
    combo            TEXT PRIMARY KEY
                     CHECK (combo IN ('quality','balanced','value','cheapest')),
    observe_model_id TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    write_model_id   TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    updated_at       TEXT NOT NULL
);

INSERT INTO estimator_combos_previous (combo, observe_model_id, write_model_id, updated_at)
SELECT
    CASE combo
        WHEN 'value' THEN 'cheapest'
        WHEN 'balanced' THEN 'value'
        WHEN 'premium' THEN 'balanced'
        WHEN 'top' THEN 'quality'
    END,
    observe_model_id,
    write_model_id,
    updated_at
FROM estimator_combos;

DROP TABLE estimator_combos;
ALTER TABLE estimator_combos_previous RENAME TO estimator_combos;

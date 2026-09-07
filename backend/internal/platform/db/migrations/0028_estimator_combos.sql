-- +goose Up
-- Which two models each estimator combo is priced with (QUOTA-39).
--
-- Four rows at most, keyed by the combo name so an assignment replaces rather than
-- accumulates. Both columns reference the curated catalog: a combo priced with a model the
-- operator has removed would quote a price nothing can charge, and the cascade means the
-- comparison drops that combo the moment its model does.
CREATE TABLE estimator_combos (
    combo            TEXT PRIMARY KEY
                     CHECK (combo IN ('quality','balanced','value','cheapest')),
    -- Curated model ids. The provider is not stored: every ref in this product is
    -- `openrouter` and catalog_models is keyed by model_id alone.
    observe_model_id TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    write_model_id   TEXT NOT NULL REFERENCES catalog_models(model_id) ON DELETE CASCADE,
    updated_at       TEXT NOT NULL
);

-- No rows are seeded: a combo priced with a model nobody chose would be an invented price,
-- and an unassigned combo is simply absent from the comparison.

-- +goose Down
DROP TABLE estimator_combos;

-- +goose Up
-- The guideline preset's state. A post's guidelines are the owner's texts alone (GUIDE-14), no
-- prompt carries the preset's line and no procedure reads or writes it, so both tables go: the
-- fields first, because they hang off the preset row. No index, view or other table names
-- either one.
DROP TABLE guideline_preset_fields;
DROP TABLE guideline_presets;

-- +goose Down
-- A rolled-back binary reads and writes both tables, so they come back empty with 0078's DDL:
-- there is no state to restore, and a missing row reads as the preset off with no field.
CREATE TABLE guideline_presets (
    user_id    TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled    INTEGER NOT NULL CHECK (enabled IN (0,1)),
    updated_at TEXT NOT NULL
);

CREATE TABLE guideline_preset_fields (
    user_id TEXT NOT NULL REFERENCES guideline_presets(user_id) ON DELETE CASCADE,
    field   TEXT NOT NULL,
    PRIMARY KEY (user_id, field)
);

-- +goose Up
-- Additive metadata preserves the revision keys referenced by owned voices.
-- NULL retains legacy classification; an empty catalog_grade means unclassified.
ALTER TABLE speech_profile_revisions ADD COLUMN catalog_grade TEXT
    CHECK(catalog_grade IS NULL OR catalog_grade IN ('','free','value','balanced','premium','top'));
ALTER TABLE speech_profile_revisions ADD COLUMN tariff_revision INTEGER NOT NULL DEFAULT 0;

CREATE TABLE speech_catalog_registrations (
    provider_id TEXT NOT NULL,
    design_model_id TEXT NOT NULL,
    speech_model_id TEXT NOT NULL,
    profile_id TEXT NOT NULL UNIQUE REFERENCES speech_profiles(id),
    PRIMARY KEY(provider_id, design_model_id, speech_model_id)
);
-- Existing duplicate histories survive; the oldest registration owns the pair.
INSERT OR IGNORE INTO speech_catalog_registrations
SELECT r.provider_id, r.design_model_id, r.speech_model_id, p.id
FROM speech_profiles p JOIN speech_profile_revisions r
ON r.profile_id = p.id AND r.revision = p.current_revision
ORDER BY p.created_at, p.id;

CREATE TABLE speech_account_tariffs (
    revision INTEGER PRIMARY KEY CHECK(revision > 0),
    connection_scope TEXT NOT NULL,
    design_usd_per_unit TEXT NOT NULL,
    speech_usd_per_unit TEXT NOT NULL,
    confirmation_usd TEXT NOT NULL,
    source TEXT NOT NULL,
    complete INTEGER NOT NULL CHECK(complete IN (0,1)),
    checked_at TEXT NOT NULL
);
-- A singleton CAS pointer serializes common-tariff changes.
CREATE TABLE speech_account_tariff_current (
    id INTEGER PRIMARY KEY CHECK(id = 1),
    revision INTEGER NOT NULL REFERENCES speech_account_tariffs(revision)
);

-- +goose Down
DROP TABLE speech_account_tariff_current;
DROP TABLE speech_account_tariffs;
DROP TABLE speech_catalog_registrations;
ALTER TABLE speech_profile_revisions DROP COLUMN tariff_revision;
ALTER TABLE speech_profile_revisions DROP COLUMN catalog_grade;

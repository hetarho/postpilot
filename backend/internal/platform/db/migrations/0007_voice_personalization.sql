-- +goose Up
-- Post-owned mutable content and immutable latest machine baseline. Manual saves only
-- advance content_revision; machine writes advance both revisions atomically.
ALTER TABLE posts ADD COLUMN content_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE posts ADD COLUMN machine_baseline TEXT;
ALTER TABLE posts ADD COLUMN machine_baseline_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE posts ADD COLUMN target_length INTEGER NOT NULL DEFAULT 1200;

-- The legacy styleguide/rules columns remain untouched and continue to be prompt
-- guidance. current_version=0 is the explicit empty/unknown state.
ALTER TABLE voice_profiles ADD COLUMN current_version INTEGER NOT NULL DEFAULT 0;

CREATE TABLE voice_profile_versions (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL CHECK (version > 0),
    snapshot    TEXT NOT NULL,
    origin      TEXT NOT NULL CHECK (origin IN ('analysis','manual','restore','rule','confirmation')),
    restored_from_version INTEGER,
    created_at  TEXT NOT NULL,
    UNIQUE (user_id, version)
);

CREATE TABLE voice_manual_overrides (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    layer       TEXT NOT NULL CHECK (layer IN ('lexical','endings','syntax','structure','axes')),
    field       TEXT NOT NULL,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, layer, field)
);

CREATE TABLE voice_learning_events (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug   TEXT NOT NULL,
    baseline_revision INTEGER NOT NULL CHECK (baseline_revision > 0),
    input_hash  TEXT NOT NULL,
    baseline_content TEXT NOT NULL,
    final_content TEXT NOT NULL,
    model_ref   TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('queued','running','done','retryable','failed')),
    job_id      TEXT,
    error       TEXT,
    created_at  TEXT NOT NULL,
    processed_at TEXT,
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE,
    UNIQUE (user_id, post_slug, baseline_revision, input_hash)
);

CREATE TABLE voice_authored_sources (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug   TEXT,
    learning_event_id TEXT REFERENCES voice_learning_events(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    tags        TEXT NOT NULL,
    body        TEXT NOT NULL,
    excerpt     TEXT NOT NULL,
    embedding_ref TEXT,
    created_at  TEXT NOT NULL,
    UNIQUE (user_id, learning_event_id)
);

CREATE TABLE voice_contrast_rules (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    statement   TEXT NOT NULL,
    canonical_key TEXT NOT NULL,
    layer       TEXT NOT NULL CHECK (layer IN ('lexical','endings','syntax','structure')),
    evidence_count INTEGER NOT NULL CHECK (evidence_count >= 0),
    status      TEXT NOT NULL CHECK (status IN ('candidate','active','retired','rejected')),
    origin      TEXT NOT NULL CHECK (origin IN ('diff','manual','ab_test')),
    created_at  TEXT NOT NULL,
    last_evidence_at TEXT NOT NULL,
    UNIQUE (user_id, canonical_key)
);

CREATE TABLE voice_rule_evidence (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule_id     TEXT NOT NULL REFERENCES voice_contrast_rules(id) ON DELETE CASCADE,
    event_id    TEXT REFERENCES voice_learning_events(id) ON DELETE CASCADE,
    origin      TEXT NOT NULL CHECK (origin IN ('diff','manual','ab_test')),
    payload_ref TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE (rule_id, event_id, origin)
);

CREATE TABLE voice_rule_confirmations (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule_id     TEXT NOT NULL REFERENCES voice_contrast_rules(id) ON DELETE CASCADE,
    proposed_statement TEXT NOT NULL,
    event_id    TEXT REFERENCES voice_learning_events(id) ON DELETE CASCADE,
    status      TEXT NOT NULL CHECK (status IN ('pending','keep','replace')),
    created_at  TEXT NOT NULL,
    resolved_at TEXT
);

CREATE TABLE voice_sentence_feedback (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug   TEXT NOT NULL,
    sentence_ref TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('diff','thumbs','ab_test','satisfaction')),
    reason      TEXT CHECK (reason IS NULL OR reason IN ('vocabulary','ending','length','structure')),
    payload_ref TEXT NOT NULL,
    processing_state TEXT NOT NULL CHECK (processing_state IN ('pending','processed','ignored')),
    created_at  TEXT NOT NULL,
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE,
    UNIQUE (user_id, post_slug, sentence_ref, kind, payload_ref)
);

CREATE TABLE voice_rule_comparisons (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule_id     TEXT NOT NULL REFERENCES voice_contrast_rules(id) ON DELETE CASCADE,
    source_id   TEXT NOT NULL REFERENCES voice_authored_sources(id) ON DELETE CASCADE,
    profile_version INTEGER NOT NULL,
    model_ref   TEXT NOT NULL,
    target_length INTEGER NOT NULL,
    input_snapshot TEXT NOT NULL,
    rule_on_side TEXT NOT NULL CHECK (rule_on_side IN ('left','right')),
    status      TEXT NOT NULL CHECK (status IN ('queued','running','review','partial','decided','failed')),
    job_id      TEXT,
    chosen_side TEXT CHECK (chosen_side IS NULL OR chosen_side IN ('left','right')),
    created_at  TEXT NOT NULL,
    decided_at  TEXT
);

CREATE TABLE voice_rule_comparison_candidates (
    id          TEXT PRIMARY KEY,
    comparison_id TEXT NOT NULL REFERENCES voice_rule_comparisons(id) ON DELETE CASCADE,
    display_side TEXT NOT NULL CHECK (display_side IN ('left','right')),
    output      TEXT,
    status      TEXT NOT NULL CHECK (status IN ('pending','running','succeeded','failed')),
    error       TEXT,
    UNIQUE (comparison_id, display_side)
);

CREATE TABLE voice_profile_validations (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_version INTEGER NOT NULL,
    analyze_model_ref TEXT NOT NULL,
    write_model_ref TEXT NOT NULL,
    judge_enabled INTEGER NOT NULL CHECK (judge_enabled IN (0,1)),
    status      TEXT NOT NULL CHECK (status IN ('queued','running','review','done','partial','failed')),
    job_id      TEXT,
    y_count     INTEGER,
    total_count INTEGER,
    created_at  TEXT NOT NULL,
    finished_at TEXT
);

CREATE TABLE voice_profile_validation_items (
    id          TEXT PRIMARY KEY,
    validation_id TEXT NOT NULL REFERENCES voice_profile_validations(id) ON DELETE CASCADE,
    source_id   TEXT NOT NULL REFERENCES voice_authored_sources(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL CHECK (position BETWEEN 0 AND 2),
    neutral_summary TEXT,
    regenerated_content TEXT,
    scores      TEXT,
    status      TEXT NOT NULL CHECK (status IN ('pending','summarized','generated','scored','failed')),
    error       TEXT,
    UNIQUE (validation_id, position),
    UNIQUE (validation_id, source_id)
);

CREATE INDEX voice_versions_user_version ON voice_profile_versions(user_id, version DESC);
CREATE INDEX voice_sources_user_created ON voice_authored_sources(user_id, created_at DESC, id DESC);
CREATE INDEX voice_learning_user_created ON voice_learning_events(user_id, created_at DESC, id DESC);
CREATE INDEX voice_rules_user_status_evidence ON voice_contrast_rules(user_id, status, evidence_count DESC, last_evidence_at DESC);
CREATE UNIQUE INDEX voice_pending_confirmation_per_rule ON voice_rule_confirmations(rule_id) WHERE status='pending';
CREATE UNIQUE INDEX voice_active_comparison_per_rule ON voice_rule_comparisons(user_id, rule_id) WHERE status IN ('queued','running','review','partial');
CREATE UNIQUE INDEX voice_active_validation_per_user ON voice_profile_validations(user_id) WHERE status IN ('queued','running','review','partial');

-- +goose Down
DROP INDEX voice_active_validation_per_user;
DROP INDEX voice_active_comparison_per_rule;
DROP INDEX voice_pending_confirmation_per_rule;
DROP INDEX voice_rules_user_status_evidence;
DROP INDEX voice_learning_user_created;
DROP INDEX voice_sources_user_created;
DROP INDEX voice_versions_user_version;
DROP TABLE voice_profile_validation_items;
DROP TABLE voice_profile_validations;
DROP TABLE voice_rule_comparison_candidates;
DROP TABLE voice_rule_comparisons;
DROP TABLE voice_sentence_feedback;
DROP TABLE voice_rule_confirmations;
DROP TABLE voice_rule_evidence;
DROP TABLE voice_contrast_rules;
DROP TABLE voice_authored_sources;
DROP TABLE voice_learning_events;
DROP TABLE voice_manual_overrides;
DROP TABLE voice_profile_versions;
ALTER TABLE voice_profiles DROP COLUMN current_version;
ALTER TABLE posts DROP COLUMN target_length;
ALTER TABLE posts DROP COLUMN machine_baseline_revision;
ALTER TABLE posts DROP COLUMN machine_baseline;
ALTER TABLE posts DROP COLUMN content_revision;

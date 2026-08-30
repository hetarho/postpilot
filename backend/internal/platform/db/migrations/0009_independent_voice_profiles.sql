-- +goose NO TRANSACTION
-- +goose Up
-- Independent voices (plan 10). Every account owns one or more mutually isolated voices;
-- a voice owns exactly one profile and every row that can change it, and a post names the
-- one voice it is written in.
--
-- NO TRANSACTION plus an explicit PRAGMA is deliberate. Several tables here are FK parents
-- with ON DELETE CASCADE children (posts, voice_learning_events, voice_contrast_rules,
-- voice_authored_sources), and SQLite's supported way to change such a table's constraints
-- is the rebuild below — which would cascade-delete those children if foreign keys stayed
-- on while the old table is dropped. `PRAGMA foreign_keys` is a no-op inside a transaction,
-- so goose must not open one for us; the work still runs in one explicit transaction, and
-- `foreign_key_check` proves the graph before it commits.

PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE voices (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    is_default  INTEGER NOT NULL CHECK (is_default IN (0, 1)),
    deleted_at  TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

-- The composite target every owned row points at: it carries the account through the FK, so
-- a row can never name a voice belonging to someone else even if a query forgets to say so.
CREATE UNIQUE INDEX voices_id_user ON voices(id, user_id);
-- At most one default and no duplicate display name among an account's ACTIVE voices. A
-- tombstone keeps its name so history reads correctly, and may collide with an active one
-- until it is renamed — which is what makes restore refusable rather than destructive.
CREATE UNIQUE INDEX voices_one_default ON voices(user_id) WHERE is_default = 1 AND deleted_at IS NULL;
CREATE UNIQUE INDEX voices_active_name ON voices(user_id, name) WHERE deleted_at IS NULL;

-- Exactly one voice per existing account, holding everything that account has already
-- written. `기본 말투` is the same name `cmd/adduser` gives a new account's first voice.
INSERT INTO voices (id, user_id, name, is_default, deleted_at, created_at, updated_at)
SELECT lower(hex(randomblob(16))),
       users.id,
       '기본 말투',
       1,
       NULL,
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM users;

----------------------------------------------------------------------------------
-- posts: the assignment plus the voice a machine baseline was written under.
----------------------------------------------------------------------------------
CREATE TABLE posts_new (
    slug         TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id     TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    memo         TEXT NOT NULL DEFAULT '',
    observations TEXT,
    content      TEXT,
    status       TEXT NOT NULL DEFAULT 'draft',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    content_revision INTEGER NOT NULL DEFAULT 0,
    machine_baseline TEXT,
    machine_baseline_revision INTEGER NOT NULL DEFAULT 0,
    -- NULL for a post that has never had a machine result. Finalization learning compares it
    -- with voice_id, so a reassigned post cannot publish its old voice's baseline anywhere.
    machine_baseline_voice_id TEXT,
    target_length INTEGER CHECK (target_length IS NULL OR target_length > 0),
    finalized_revision INTEGER,
    finalized_at TEXT,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);

INSERT INTO posts_new (slug, user_id, voice_id, title, memo, observations, content, status,
                       created_at, updated_at, content_revision, machine_baseline,
                       machine_baseline_revision, machine_baseline_voice_id, target_length,
                       finalized_revision, finalized_at)
SELECT p.slug, p.user_id, v.id, p.title, p.memo, p.observations, p.content, p.status,
       p.created_at, p.updated_at, p.content_revision, p.machine_baseline,
       p.machine_baseline_revision,
       CASE WHEN p.machine_baseline IS NOT NULL THEN v.id ELSE NULL END,
       p.target_length, p.finalized_revision, p.finalized_at
FROM posts p JOIN voices v ON v.user_id = p.user_id;

DROP TABLE posts;
ALTER TABLE posts_new RENAME TO posts;
CREATE INDEX idx_posts_user_updated ON posts(user_id, updated_at);
CREATE UNIQUE INDEX posts_slug_user_idx ON posts(slug, user_id);
CREATE INDEX posts_voice_idx ON posts(voice_id);

----------------------------------------------------------------------------------
-- voice_profiles: one profile per voice, not per account. Every voice gets a row —
-- including a voice whose account never had a profile — so a read never creates one.
----------------------------------------------------------------------------------
CREATE TABLE voice_profiles_new (
    voice_id    TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    styleguide  TEXT NOT NULL DEFAULT '',
    rules       TEXT NOT NULL DEFAULT '',
    corpus_version INTEGER NOT NULL DEFAULT 0,
    current_version INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);

INSERT INTO voice_profiles_new (voice_id, user_id, styleguide, rules, corpus_version, current_version, updated_at)
SELECT v.id, v.user_id,
       COALESCE(p.styleguide, ''), COALESCE(p.rules, ''),
       COALESCE(p.corpus_version, 0), COALESCE(p.current_version, 0),
       COALESCE(p.updated_at, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
FROM voices v LEFT JOIN voice_profiles p ON p.user_id = v.user_id;

DROP TABLE voice_profiles;
ALTER TABLE voice_profiles_new RENAME TO voice_profiles;

----------------------------------------------------------------------------------
-- voice_samples: a corpus belongs to one voice.
----------------------------------------------------------------------------------
CREATE TABLE voice_samples_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    label       TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_samples_new (id, user_id, voice_id, label, body, created_at)
SELECT o.id, o.user_id, v.id, o.label, o.body, o.created_at
FROM voice_samples o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_samples;
ALTER TABLE voice_samples_new RENAME TO voice_samples;
CREATE INDEX voice_samples_voice_created_idx ON voice_samples(voice_id, created_at DESC, id DESC);

----------------------------------------------------------------------------------
-- voice_profile_versions: version numbers count per voice, so the account-wide UNIQUE
-- has to go — two voices both publishing v1 is the normal case now.
----------------------------------------------------------------------------------
CREATE TABLE voice_profile_versions_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    version     INTEGER NOT NULL CHECK (version > 0),
    snapshot    TEXT NOT NULL,
    origin      TEXT NOT NULL CHECK (origin IN ('analysis','manual','restore','rule','confirmation')),
    restored_from_version INTEGER,
    created_at  TEXT NOT NULL,
    UNIQUE (voice_id, version),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_profile_versions_new (id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at)
SELECT o.id, o.user_id, v.id, o.version, o.snapshot, o.origin, o.restored_from_version, o.created_at
FROM voice_profile_versions o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_profile_versions;
ALTER TABLE voice_profile_versions_new RENAME TO voice_profile_versions;
CREATE INDEX voice_versions_voice_version ON voice_profile_versions(voice_id, version DESC);

----------------------------------------------------------------------------------
-- voice_manual_overrides: keyed by voice, since two voices hold different manual answers.
----------------------------------------------------------------------------------
CREATE TABLE voice_manual_overrides_new (
    voice_id    TEXT NOT NULL,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    layer       TEXT NOT NULL CHECK (layer IN ('lexical','endings','syntax','structure','axes')),
    field       TEXT NOT NULL,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (voice_id, layer, field),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_manual_overrides_new (voice_id, user_id, layer, field, value, updated_at)
SELECT v.id, o.user_id, o.layer, o.field, o.value, o.updated_at
FROM voice_manual_overrides o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_manual_overrides;
ALTER TABLE voice_manual_overrides_new RENAME TO voice_manual_overrides;

----------------------------------------------------------------------------------
-- voice_learning_events: one finalization freezes one voice for the life of the event,
-- so a later post reassignment cannot retarget work already done.
----------------------------------------------------------------------------------
CREATE TABLE voice_learning_events_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
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
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    UNIQUE (voice_id, post_slug, baseline_revision, input_hash)
);
INSERT INTO voice_learning_events_new (id, user_id, voice_id, post_slug, baseline_revision, input_hash,
                                       baseline_content, final_content, model_ref, status, job_id, error,
                                       created_at, processed_at)
SELECT o.id, o.user_id, v.id, o.post_slug, o.baseline_revision, o.input_hash, o.baseline_content,
       o.final_content, o.model_ref, o.status, o.job_id, o.error, o.created_at, o.processed_at
FROM voice_learning_events o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_learning_events;
ALTER TABLE voice_learning_events_new RENAME TO voice_learning_events;
CREATE INDEX voice_learning_voice_created ON voice_learning_events(voice_id, created_at DESC, id DESC);
CREATE UNIQUE INDEX voice_learning_event_owner ON voice_learning_events(id, voice_id, user_id);

----------------------------------------------------------------------------------
-- voice_authored_sources: the few-shot bank is per voice.
----------------------------------------------------------------------------------
CREATE TABLE voice_authored_sources_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    post_slug   TEXT,
    learning_event_id TEXT,
    title       TEXT NOT NULL,
    tags        TEXT NOT NULL,
    body        TEXT NOT NULL,
    excerpt     TEXT NOT NULL,
    embedding_ref TEXT,
    created_at  TEXT NOT NULL,
    UNIQUE (voice_id, learning_event_id),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    FOREIGN KEY (learning_event_id, voice_id, user_id)
        REFERENCES voice_learning_events(id, voice_id, user_id) ON DELETE CASCADE
);
INSERT INTO voice_authored_sources_new (id, user_id, voice_id, post_slug, learning_event_id, title, tags,
                                        body, excerpt, embedding_ref, created_at)
SELECT o.id, o.user_id, v.id, o.post_slug, o.learning_event_id, o.title, o.tags, o.body, o.excerpt,
       o.embedding_ref, o.created_at
FROM voice_authored_sources o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_authored_sources;
ALTER TABLE voice_authored_sources_new RENAME TO voice_authored_sources;
CREATE INDEX voice_sources_voice_created ON voice_authored_sources(voice_id, created_at DESC, id DESC);
CREATE UNIQUE INDEX voice_source_owner ON voice_authored_sources(id, voice_id, user_id);

----------------------------------------------------------------------------------
-- voice_contrast_rules: a canonical key is unique within one voice, so two voices can
-- hold rules that contradict each other — the point of the whole plan.
----------------------------------------------------------------------------------
CREATE TABLE voice_contrast_rules_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    statement   TEXT NOT NULL,
    canonical_key TEXT NOT NULL,
    layer       TEXT NOT NULL CHECK (layer IN ('lexical','endings','syntax','structure')),
    evidence_count INTEGER NOT NULL CHECK (evidence_count >= 0),
    status      TEXT NOT NULL CHECK (status IN ('candidate','active','retired','rejected')),
    origin      TEXT NOT NULL CHECK (origin IN ('diff','manual','ab_test')),
    created_at  TEXT NOT NULL,
    last_evidence_at TEXT NOT NULL,
    UNIQUE (voice_id, canonical_key),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_contrast_rules_new (id, user_id, voice_id, statement, canonical_key, layer,
                                      evidence_count, status, origin, created_at, last_evidence_at)
SELECT o.id, o.user_id, v.id, o.statement, o.canonical_key, o.layer, o.evidence_count, o.status,
       o.origin, o.created_at, o.last_evidence_at
FROM voice_contrast_rules o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_contrast_rules;
ALTER TABLE voice_contrast_rules_new RENAME TO voice_contrast_rules;
CREATE INDEX voice_rules_voice_status_evidence ON voice_contrast_rules(voice_id, status, evidence_count DESC, last_evidence_at DESC);
CREATE UNIQUE INDEX voice_rule_owner ON voice_contrast_rules(id, voice_id, user_id);

----------------------------------------------------------------------------------
-- voice_sentence_feedback: derived from the post, but carried explicitly so a query
-- cannot read another voice's feedback by joining only on the account.
----------------------------------------------------------------------------------
CREATE TABLE voice_sentence_feedback_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    post_slug   TEXT NOT NULL,
    sentence_ref TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('diff','thumbs','ab_test','satisfaction')),
    reason      TEXT CHECK (reason IS NULL OR reason IN ('vocabulary','ending','length','structure')),
    payload_ref TEXT NOT NULL,
    processing_state TEXT NOT NULL CHECK (processing_state IN ('pending','processed','ignored')),
    created_at  TEXT NOT NULL,
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    UNIQUE (voice_id, post_slug, sentence_ref, kind, payload_ref)
);
INSERT INTO voice_sentence_feedback_new (id, user_id, voice_id, post_slug, sentence_ref, kind, reason,
                                         payload_ref, processing_state, created_at)
SELECT o.id, o.user_id, v.id, o.post_slug, o.sentence_ref, o.kind, o.reason, o.payload_ref,
       o.processing_state, o.created_at
FROM voice_sentence_feedback o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_sentence_feedback;
ALTER TABLE voice_sentence_feedback_new RENAME TO voice_sentence_feedback;

----------------------------------------------------------------------------------
-- Rows whose voice is already implied by an owned parent still carry it: evidence and
-- confirmations hang off a rule, comparisons off a rule and a source, validations off
-- the voice they were started for. Their guards move from per-account to per-voice.
----------------------------------------------------------------------------------
CREATE TABLE voice_rule_evidence_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    rule_id     TEXT NOT NULL,
    event_id    TEXT,
    origin      TEXT NOT NULL CHECK (origin IN ('diff','manual','ab_test')),
    payload_ref TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE (rule_id, event_id, origin),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    FOREIGN KEY (rule_id, voice_id, user_id) REFERENCES voice_contrast_rules(id, voice_id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (event_id, voice_id, user_id) REFERENCES voice_learning_events(id, voice_id, user_id) ON DELETE CASCADE
);
INSERT INTO voice_rule_evidence_new (id, user_id, voice_id, rule_id, event_id, origin, payload_ref, created_at)
SELECT o.id, o.user_id, v.id, o.rule_id, o.event_id, o.origin, o.payload_ref, o.created_at
FROM voice_rule_evidence o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_rule_evidence;
ALTER TABLE voice_rule_evidence_new RENAME TO voice_rule_evidence;

CREATE TABLE voice_rule_confirmations_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    rule_id     TEXT NOT NULL,
    proposed_statement TEXT NOT NULL,
    event_id    TEXT,
    status      TEXT NOT NULL CHECK (status IN ('pending','keep','replace')),
    created_at  TEXT NOT NULL,
    resolved_at TEXT,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    FOREIGN KEY (rule_id, voice_id, user_id) REFERENCES voice_contrast_rules(id, voice_id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (event_id, voice_id, user_id) REFERENCES voice_learning_events(id, voice_id, user_id) ON DELETE CASCADE
);
INSERT INTO voice_rule_confirmations_new (id, user_id, voice_id, rule_id, proposed_statement, event_id, status, created_at, resolved_at)
SELECT o.id, o.user_id, v.id, o.rule_id, o.proposed_statement, o.event_id, o.status, o.created_at, o.resolved_at
FROM voice_rule_confirmations o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_rule_confirmations;
ALTER TABLE voice_rule_confirmations_new RENAME TO voice_rule_confirmations;
CREATE UNIQUE INDEX voice_pending_confirmation_per_rule ON voice_rule_confirmations(rule_id) WHERE status='pending';

CREATE TABLE voice_rule_comparisons_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    rule_id     TEXT NOT NULL,
    source_id   TEXT NOT NULL,
    profile_version INTEGER NOT NULL,
    model_ref   TEXT NOT NULL,
    target_length INTEGER NOT NULL,
    input_snapshot TEXT NOT NULL,
    rule_on_side TEXT NOT NULL CHECK (rule_on_side IN ('left','right')),
    status      TEXT NOT NULL CHECK (status IN ('queued','running','review','partial','decided','failed')),
    job_id      TEXT,
    chosen_side TEXT CHECK (chosen_side IS NULL OR chosen_side IN ('left','right')),
    created_at  TEXT NOT NULL,
    decided_at  TEXT,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    FOREIGN KEY (rule_id, voice_id, user_id) REFERENCES voice_contrast_rules(id, voice_id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (source_id, voice_id, user_id) REFERENCES voice_authored_sources(id, voice_id, user_id) ON DELETE CASCADE
);
INSERT INTO voice_rule_comparisons_new (id, user_id, voice_id, rule_id, source_id, profile_version, model_ref,
                                        target_length, input_snapshot, rule_on_side, status, job_id, chosen_side,
                                        created_at, decided_at)
SELECT o.id, o.user_id, v.id, o.rule_id, o.source_id, o.profile_version, o.model_ref, o.target_length,
       o.input_snapshot, o.rule_on_side, o.status, o.job_id, o.chosen_side, o.created_at, o.decided_at
FROM voice_rule_comparisons o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_rule_comparisons;
ALTER TABLE voice_rule_comparisons_new RENAME TO voice_rule_comparisons;
CREATE UNIQUE INDEX voice_comparison_owner ON voice_rule_comparisons(id, voice_id, user_id);
CREATE UNIQUE INDEX voice_active_comparison_per_rule ON voice_rule_comparisons(voice_id, rule_id)
    WHERE status IN ('queued','running','review','partial');

CREATE TABLE voice_profile_validations_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    profile_version INTEGER NOT NULL,
    analyze_model_ref TEXT NOT NULL,
    write_model_ref TEXT NOT NULL,
    judge_enabled INTEGER NOT NULL CHECK (judge_enabled IN (0,1)),
    status      TEXT NOT NULL CHECK (status IN ('queued','running','review','done','partial','failed')),
    job_id      TEXT,
    y_count     INTEGER,
    total_count INTEGER,
    created_at  TEXT NOT NULL,
    finished_at TEXT,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_profile_validations_new (id, user_id, voice_id, profile_version, analyze_model_ref, write_model_ref,
                                           judge_enabled, status, job_id, y_count, total_count, created_at, finished_at)
SELECT o.id, o.user_id, v.id, o.profile_version, o.analyze_model_ref, o.write_model_ref, o.judge_enabled,
       o.status, o.job_id, o.y_count, o.total_count, o.created_at, o.finished_at
FROM voice_profile_validations o JOIN voices v ON v.user_id = o.user_id;
DROP TABLE voice_profile_validations;
ALTER TABLE voice_profile_validations_new RENAME TO voice_profile_validations;
CREATE UNIQUE INDEX voice_validation_owner ON voice_profile_validations(id, voice_id, user_id);
CREATE UNIQUE INDEX voice_active_validation_per_voice ON voice_profile_validations(voice_id)
    WHERE status IN ('queued','running','review','partial');

-- Validation items also name an authored source. Carrying the partition here makes it
-- impossible to attach voice A's source to voice B's validation with crafted ids.
CREATE TABLE voice_profile_validation_items_new (
    id          TEXT PRIMARY KEY,
    validation_id TEXT NOT NULL,
    source_id   TEXT NOT NULL,
    voice_id    TEXT NOT NULL,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL CHECK (position BETWEEN 0 AND 2),
    neutral_summary TEXT,
    regenerated_content TEXT,
    scores      TEXT,
    status      TEXT NOT NULL CHECK (status IN ('pending','summarized','generated','scored','failed')),
    error       TEXT,
    UNIQUE (validation_id, position),
    UNIQUE (validation_id, source_id),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    FOREIGN KEY (validation_id, voice_id, user_id) REFERENCES voice_profile_validations(id, voice_id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (source_id, voice_id, user_id) REFERENCES voice_authored_sources(id, voice_id, user_id) ON DELETE CASCADE
);
INSERT INTO voice_profile_validation_items_new (id, validation_id, source_id, voice_id, user_id, position,
                                                neutral_summary, regenerated_content, scores, status, error)
SELECT i.id, i.validation_id, i.source_id, v.voice_id, v.user_id, i.position, i.neutral_summary,
       i.regenerated_content, i.scores, i.status, i.error
FROM voice_profile_validation_items i JOIN voice_profile_validations v ON v.id = i.validation_id;
DROP TABLE voice_profile_validation_items;
ALTER TABLE voice_profile_validation_items_new RENAME TO voice_profile_validation_items;

----------------------------------------------------------------------------------
-- Durable execution freezes the voice it was started for. The job context does not read
-- voice tables; it only carries the id it was handed and gives it back on projection.
----------------------------------------------------------------------------------
CREATE TABLE generation_jobs_new (
    id             TEXT PRIMARY KEY,
    post_slug      TEXT,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id       TEXT,
    kind           TEXT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed')),
    stage          TEXT,
    progress_done  INTEGER NOT NULL DEFAULT 0,
    progress_total INTEGER NOT NULL DEFAULT 0,
    error          TEXT,
    observe_model  TEXT,
    write_model    TEXT,
    payload        TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    started_at     TEXT,
    finished_at    TEXT,
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO generation_jobs_new (id, post_slug, user_id, voice_id, kind, status, stage, progress_done,
                                 progress_total, error, observe_model, write_model, payload, created_at,
                                 updated_at, started_at, finished_at)
SELECT j.id, j.post_slug, j.user_id,
       CASE WHEN j.post_slug IS NOT NULL THEN p.voice_id
            WHEN j.kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile') THEN v.id
            ELSE NULL END,
       j.kind, j.status, j.stage, j.progress_done, j.progress_total, j.error, j.observe_model,
       j.write_model, j.payload, j.created_at, j.updated_at, j.started_at, j.finished_at
FROM generation_jobs j
LEFT JOIN posts p ON p.slug = j.post_slug AND p.user_id = j.user_id
JOIN voices v ON v.user_id = j.user_id;
DROP TABLE generation_jobs;
ALTER TABLE generation_jobs_new RENAME TO generation_jobs;
-- Voice-owned work is now guarded per voice rather than per account, so two voices can
-- run concurrently while one voice still cannot run two of the same personalization kind.
-- Learning and comparison jobs retain their triggering post, so this guard deliberately
-- overlaps the post guard for those rows.
CREATE UNIQUE INDEX generation_jobs_active_post_idx
    ON generation_jobs(post_slug)
    WHERE post_slug IS NOT NULL AND status IN ('queued', 'running');
-- Pre-0009 databases may legitimately contain several active learning jobs for different posts
-- that now map to one voice. A unique index would either make the lossless migration fail or
-- force it to rewrite durable statuses. Keep those rows byte-for-byte and serialize every NEW
-- row with a trigger; SQLite's one writer makes the existence check race-safe.
CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile')
      AND status IN ('queued', 'running');
-- +goose StatementBegin
CREATE TRIGGER generation_jobs_refuse_duplicate_voice_work
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL
 AND NEW.kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile')
 AND NEW.status IN ('queued', 'running')
 AND EXISTS (
     SELECT 1 FROM generation_jobs active
     WHERE active.voice_id = NEW.voice_id
       AND active.kind = NEW.kind
       AND active.status IN ('queued', 'running')
 )
BEGIN
    SELECT RAISE(ABORT, 'active voice job already exists');
END;
-- +goose StatementEnd
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx
    ON generation_jobs(user_id, kind)
    WHERE post_slug IS NULL AND voice_id IS NULL AND status IN ('queued', 'running');
CREATE INDEX generation_jobs_queue_idx ON generation_jobs(status, created_at);

CREATE TABLE model_experiments_new (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug           TEXT REFERENCES posts(slug) ON DELETE SET NULL,
    voice_id            TEXT,
    stage               TEXT NOT NULL CHECK (stage IN ('observe', 'write', 'analyze')),
    status              TEXT NOT NULL CHECK (status IN ('queued', 'running', 'review', 'partial', 'decided', 'dismissed', 'failed')),
    job_id              TEXT,
    input_snapshot      TEXT,
    input_hash          TEXT NOT NULL,
    prompt_version      TEXT NOT NULL,
    winner_candidate_id TEXT,
    outcome             TEXT CHECK (outcome IS NULL OR outcome IN ('winner', 'skipped', 'unpaired')),
    apply_error         TEXT,
    applied_at          TEXT,
    created_at          TEXT NOT NULL,
    finished_at         TEXT,
    decided_at          TEXT,
    content_expires_at  TEXT,
    adoption_error      TEXT,
    adopted_at          TEXT,
    adoption_requested  INTEGER NOT NULL DEFAULT 0 CHECK (adoption_requested IN (0, 1)),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO model_experiments_new (id, user_id, post_slug, voice_id, stage, status, job_id, input_snapshot,
                                   input_hash, prompt_version, winner_candidate_id, outcome, apply_error,
                                   applied_at, created_at, finished_at, decided_at, content_expires_at,
                                   adoption_error, adopted_at, adoption_requested)
SELECT e.id, e.user_id, e.post_slug,
       CASE WHEN e.post_slug IS NOT NULL THEN p.voice_id WHEN e.stage = 'analyze' THEN v.id ELSE NULL END,
       e.stage, e.status, e.job_id, e.input_snapshot, e.input_hash, e.prompt_version,
       e.winner_candidate_id, e.outcome, e.apply_error, e.applied_at, e.created_at, e.finished_at,
       e.decided_at, e.content_expires_at, e.adoption_error, e.adopted_at, e.adoption_requested
FROM model_experiments e
LEFT JOIN posts p ON p.slug = e.post_slug AND p.user_id = e.user_id
JOIN voices v ON v.user_id = e.user_id;
DROP TABLE model_experiments;
ALTER TABLE model_experiments_new RENAME TO model_experiments;
CREATE UNIQUE INDEX one_unresolved_write_experiment_per_post
ON model_experiments(user_id, post_slug)
WHERE stage = 'write' AND post_slug IS NOT NULL
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
    OR (status = 'decided' AND (applied_at IS NULL OR (adoption_requested = 1 AND adopted_at IS NULL)))
  );
CREATE INDEX model_experiments_user_stage_created
ON model_experiments(user_id, stage, created_at DESC, id DESC);
CREATE INDEX model_experiments_terminal_expiry
ON model_experiments(content_expires_at)
WHERE input_snapshot IS NOT NULL AND status IN ('decided', 'dismissed');

----------------------------------------------------------------------------------
-- Cross-context lifecycle race guards.
--
-- The services perform the same checks to return useful domain errors, but a check in
-- one context and a write in another cannot share a Go mutex or a store transaction.
-- SQLite serializes these trigger checks with the write itself, so either the tombstone
-- wins or the new post/work row wins; both can never commit. Reads remain mutation-free.
----------------------------------------------------------------------------------
-- +goose StatementBegin
CREATE TRIGGER posts_require_active_voice_on_insert
BEFORE INSERT ON posts
WHEN NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'post voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posts_require_active_voice_on_reassign
BEFORE UPDATE OF voice_id ON posts
WHEN NEW.voice_id <> OLD.voice_id AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'post voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER generation_jobs_require_active_voice
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'job voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER model_experiments_require_active_voice
BEFORE INSERT ON model_experiments
WHEN NEW.voice_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'experiment voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER voice_rule_comparisons_require_active_voice
BEFORE INSERT ON voice_rule_comparisons
WHEN NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'comparison voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER voice_profile_validations_require_active_voice
BEFORE INSERT ON voice_profile_validations
WHEN NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'validation voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER voices_refuse_publishable_work_on_delete
BEFORE UPDATE OF deleted_at ON voices
WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL
BEGIN
    SELECT CASE WHEN OLD.is_default = 1
        THEN RAISE(ABORT, 'default voice cannot be deleted') END;
    SELECT CASE WHEN (
        SELECT count(*) FROM voices
        WHERE user_id = OLD.user_id AND deleted_at IS NULL
    ) <= 1 THEN RAISE(ABORT, 'last active voice cannot be deleted') END;
    SELECT CASE WHEN
        EXISTS (
            SELECT 1 FROM generation_jobs
            WHERE voice_id = OLD.id AND status IN ('queued', 'running')
        ) OR EXISTS (
            SELECT 1 FROM voice_rule_comparisons
            WHERE voice_id = OLD.id AND status IN ('queued', 'running', 'review', 'partial')
        ) OR EXISTS (
            SELECT 1 FROM voice_profile_validations
            WHERE voice_id = OLD.id AND status IN ('queued', 'running', 'review', 'partial')
        ) OR EXISTS (
            SELECT 1 FROM model_experiments
            WHERE voice_id = OLD.id
              AND (status IN ('queued', 'running', 'review', 'partial')
                   OR (status = 'decided' AND applied_at IS NULL))
        )
        THEN RAISE(ABORT, 'voice has publishable work') END;
END;
-- +goose StatementEnd

-- Nothing may be left unassigned: a NULL here would be a post or a profile row with no
-- owner, which every guard below this line assumes cannot exist ([I7]).
-- The CHECK is the abort: SQLite's RAISE() only runs inside a trigger, so the assertion is
-- an insert that can only succeed when the SELECT finds nothing to complain about.
DROP TABLE IF EXISTS migration_0009_guard;
CREATE TABLE migration_0009_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0009_guard (problem)
SELECT 'a row was left without a voice'
WHERE EXISTS (
    SELECT 1 FROM pragma_foreign_key_check
    UNION ALL SELECT 1 FROM posts WHERE voice_id IS NULL OR voice_id NOT IN (SELECT id FROM voices)
    UNION ALL SELECT 1 FROM voice_profiles WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_samples WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_profile_versions WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_manual_overrides WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_learning_events WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_authored_sources WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_contrast_rules WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_sentence_feedback WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_rule_evidence WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_rule_confirmations WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_rule_comparisons WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_profile_validations WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM voice_profile_validation_items WHERE voice_id IS NULL
    UNION ALL SELECT 1 FROM generation_jobs
        WHERE voice_id IS NULL AND (post_slug IS NOT NULL OR kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile'))
    UNION ALL SELECT 1 FROM model_experiments
        WHERE voice_id IS NULL AND (post_slug IS NOT NULL OR stage = 'analyze')
);
INSERT INTO migration_0009_guard (problem)
SELECT 'an account has no active default voice'
WHERE EXISTS (
    SELECT 1 FROM users u WHERE NOT EXISTS (
        SELECT 1 FROM voices v WHERE v.user_id = u.id AND v.is_default = 1 AND v.deleted_at IS NULL
    )
);
DROP TABLE migration_0009_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
-- Only a database that never used more than the one migrated voice per account can go
-- back: collapsing two voices would have to choose which profile, which rules and which
-- history survive, and that is data loss no rollback should decide on its own.
PRAGMA foreign_keys=OFF;

-- Checked before the transaction opens: a refusal must leave nothing half-done and no
-- transaction hanging on the writer for the next attempt to trip over.
DROP TABLE IF EXISTS migration_0009_down_guard;
CREATE TABLE migration_0009_down_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0009_down_guard (problem)
SELECT 'this database has more than one voice per account'
WHERE EXISTS (SELECT 1 FROM voices GROUP BY user_id HAVING COUNT(*) > 1);
DROP TABLE migration_0009_down_guard;

BEGIN;

DROP TRIGGER voices_refuse_publishable_work_on_delete;
DROP TRIGGER voice_profile_validations_require_active_voice;
DROP TRIGGER voice_rule_comparisons_require_active_voice;
DROP TRIGGER model_experiments_require_active_voice;
DROP TRIGGER generation_jobs_require_active_voice;
DROP TRIGGER generation_jobs_refuse_duplicate_voice_work;
DROP TRIGGER posts_require_active_voice_on_reassign;
DROP TRIGGER posts_require_active_voice_on_insert;

CREATE TABLE posts_old (
    slug         TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        TEXT NOT NULL DEFAULT '',
    memo         TEXT NOT NULL DEFAULT '',
    observations TEXT,
    content      TEXT,
    status       TEXT NOT NULL DEFAULT 'draft',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    content_revision INTEGER NOT NULL DEFAULT 0,
    machine_baseline TEXT,
    machine_baseline_revision INTEGER NOT NULL DEFAULT 0,
    target_length INTEGER CHECK (target_length IS NULL OR target_length > 0),
    finalized_revision INTEGER,
    finalized_at TEXT
);
INSERT INTO posts_old SELECT slug, user_id, title, memo, observations, content, status, created_at,
       updated_at, content_revision, machine_baseline, machine_baseline_revision, target_length,
       finalized_revision, finalized_at FROM posts;
DROP TABLE posts;
ALTER TABLE posts_old RENAME TO posts;
CREATE INDEX idx_posts_user_updated ON posts(user_id, updated_at);
CREATE UNIQUE INDEX posts_slug_user_idx ON posts(slug, user_id);

CREATE TABLE voice_profiles_old (
    user_id     TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    styleguide  TEXT NOT NULL DEFAULT '',
    rules       TEXT NOT NULL DEFAULT '',
    corpus_version INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL,
    current_version INTEGER NOT NULL DEFAULT 0
);
INSERT INTO voice_profiles_old SELECT user_id, styleguide, rules, corpus_version, updated_at, current_version FROM voice_profiles;
DROP TABLE voice_profiles;
ALTER TABLE voice_profiles_old RENAME TO voice_profiles;

CREATE TABLE voice_samples_old (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL
);
INSERT INTO voice_samples_old (id, user_id, label, body, created_at)
SELECT id, user_id, label, body, created_at FROM voice_samples;
DROP TABLE voice_samples;
ALTER TABLE voice_samples_old RENAME TO voice_samples;
CREATE INDEX voice_samples_user_created_idx ON voice_samples(user_id, created_at DESC, id DESC);

CREATE TABLE voice_profile_versions_old (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL CHECK (version > 0),
    snapshot    TEXT NOT NULL,
    origin      TEXT NOT NULL CHECK (origin IN ('analysis','manual','restore','rule','confirmation')),
    restored_from_version INTEGER,
    created_at  TEXT NOT NULL,
    UNIQUE (user_id, version)
);
INSERT INTO voice_profile_versions_old SELECT id, user_id, version, snapshot, origin, restored_from_version, created_at FROM voice_profile_versions;
DROP TABLE voice_profile_versions;
ALTER TABLE voice_profile_versions_old RENAME TO voice_profile_versions;
CREATE INDEX voice_versions_user_version ON voice_profile_versions(user_id, version DESC);

CREATE TABLE voice_manual_overrides_old (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    layer       TEXT NOT NULL CHECK (layer IN ('lexical','endings','syntax','structure','axes')),
    field       TEXT NOT NULL,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, layer, field)
);
INSERT INTO voice_manual_overrides_old SELECT user_id, layer, field, value, updated_at FROM voice_manual_overrides;
DROP TABLE voice_manual_overrides;
ALTER TABLE voice_manual_overrides_old RENAME TO voice_manual_overrides;

CREATE TABLE voice_learning_events_old (
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
INSERT INTO voice_learning_events_old SELECT id, user_id, post_slug, baseline_revision, input_hash,
       baseline_content, final_content, model_ref, status, job_id, error, created_at, processed_at
FROM voice_learning_events;
DROP TABLE voice_learning_events;
ALTER TABLE voice_learning_events_old RENAME TO voice_learning_events;
CREATE INDEX voice_learning_user_created ON voice_learning_events(user_id, created_at DESC, id DESC);

CREATE TABLE voice_authored_sources_old (
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
INSERT INTO voice_authored_sources_old SELECT id, user_id, post_slug, learning_event_id, title, tags,
       body, excerpt, embedding_ref, created_at FROM voice_authored_sources;
DROP TABLE voice_authored_sources;
ALTER TABLE voice_authored_sources_old RENAME TO voice_authored_sources;
CREATE INDEX voice_sources_user_created ON voice_authored_sources(user_id, created_at DESC, id DESC);

CREATE TABLE voice_contrast_rules_old (
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
INSERT INTO voice_contrast_rules_old SELECT id, user_id, statement, canonical_key, layer, evidence_count,
       status, origin, created_at, last_evidence_at FROM voice_contrast_rules;
DROP TABLE voice_contrast_rules;
ALTER TABLE voice_contrast_rules_old RENAME TO voice_contrast_rules;
CREATE INDEX voice_rules_user_status_evidence ON voice_contrast_rules(user_id, status, evidence_count DESC, last_evidence_at DESC);

CREATE TABLE voice_sentence_feedback_old (
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
INSERT INTO voice_sentence_feedback_old SELECT id, user_id, post_slug, sentence_ref, kind, reason,
       payload_ref, processing_state, created_at FROM voice_sentence_feedback;
DROP TABLE voice_sentence_feedback;
ALTER TABLE voice_sentence_feedback_old RENAME TO voice_sentence_feedback;

CREATE TABLE voice_rule_evidence_old (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule_id     TEXT NOT NULL REFERENCES voice_contrast_rules(id) ON DELETE CASCADE,
    event_id    TEXT REFERENCES voice_learning_events(id) ON DELETE CASCADE,
    origin      TEXT NOT NULL CHECK (origin IN ('diff','manual','ab_test')),
    payload_ref TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE (rule_id, event_id, origin)
);
INSERT INTO voice_rule_evidence_old (id, user_id, rule_id, event_id, origin, payload_ref, created_at)
SELECT id, user_id, rule_id, event_id, origin, payload_ref, created_at FROM voice_rule_evidence;
DROP TABLE voice_rule_evidence;
ALTER TABLE voice_rule_evidence_old RENAME TO voice_rule_evidence;

CREATE TABLE voice_rule_confirmations_old (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule_id     TEXT NOT NULL REFERENCES voice_contrast_rules(id) ON DELETE CASCADE,
    proposed_statement TEXT NOT NULL,
    event_id    TEXT REFERENCES voice_learning_events(id) ON DELETE CASCADE,
    status      TEXT NOT NULL CHECK (status IN ('pending','keep','replace')),
    created_at  TEXT NOT NULL,
    resolved_at TEXT
);
INSERT INTO voice_rule_confirmations_old (id, user_id, rule_id, proposed_statement, event_id, status, created_at, resolved_at)
SELECT id, user_id, rule_id, proposed_statement, event_id, status, created_at, resolved_at FROM voice_rule_confirmations;
DROP TABLE voice_rule_confirmations;
ALTER TABLE voice_rule_confirmations_old RENAME TO voice_rule_confirmations;
CREATE UNIQUE INDEX voice_pending_confirmation_per_rule ON voice_rule_confirmations(rule_id) WHERE status='pending';

CREATE TABLE voice_rule_comparisons_old (
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
INSERT INTO voice_rule_comparisons_old (id, user_id, rule_id, source_id, profile_version, model_ref,
                                        target_length, input_snapshot, rule_on_side, status, job_id,
                                        chosen_side, created_at, decided_at)
SELECT id, user_id, rule_id, source_id, profile_version, model_ref, target_length, input_snapshot,
       rule_on_side, status, job_id, chosen_side, created_at, decided_at FROM voice_rule_comparisons;
DROP TABLE voice_rule_comparisons;
ALTER TABLE voice_rule_comparisons_old RENAME TO voice_rule_comparisons;
CREATE UNIQUE INDEX voice_active_comparison_per_rule ON voice_rule_comparisons(user_id, rule_id)
    WHERE status IN ('queued','running','review','partial');

CREATE TABLE voice_profile_validations_old (
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
INSERT INTO voice_profile_validations_old (id, user_id, profile_version, analyze_model_ref, write_model_ref,
                                           judge_enabled, status, job_id, y_count, total_count, created_at, finished_at)
SELECT id, user_id, profile_version, analyze_model_ref, write_model_ref, judge_enabled, status, job_id,
       y_count, total_count, created_at, finished_at FROM voice_profile_validations;
DROP TABLE voice_profile_validations;
ALTER TABLE voice_profile_validations_old RENAME TO voice_profile_validations;
CREATE UNIQUE INDEX voice_active_validation_per_user ON voice_profile_validations(user_id)
    WHERE status IN ('queued','running','review','partial');

CREATE TABLE voice_profile_validation_items_old (
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
INSERT INTO voice_profile_validation_items_old (id, validation_id, source_id, position, neutral_summary,
                                                regenerated_content, scores, status, error)
SELECT id, validation_id, source_id, position, neutral_summary, regenerated_content, scores, status, error
FROM voice_profile_validation_items;
DROP TABLE voice_profile_validation_items;
ALTER TABLE voice_profile_validation_items_old RENAME TO voice_profile_validation_items;

CREATE TABLE generation_jobs_old (
    id             TEXT PRIMARY KEY,
    post_slug      TEXT,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind           TEXT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed')),
    stage          TEXT,
    progress_done  INTEGER NOT NULL DEFAULT 0,
    progress_total INTEGER NOT NULL DEFAULT 0,
    error          TEXT,
    observe_model  TEXT,
    write_model    TEXT,
    payload        TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    started_at     TEXT,
    finished_at    TEXT,
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE
);
INSERT INTO generation_jobs_old (id, post_slug, user_id, kind, status, stage, progress_done, progress_total,
                                 error, observe_model, write_model, payload, created_at, updated_at,
                                 started_at, finished_at)
SELECT id, post_slug, user_id, kind, status, stage, progress_done, progress_total, error, observe_model,
       write_model, payload, created_at, updated_at, started_at, finished_at FROM generation_jobs;
DROP TABLE generation_jobs;
ALTER TABLE generation_jobs_old RENAME TO generation_jobs;
CREATE UNIQUE INDEX generation_jobs_active_post_idx
    ON generation_jobs(post_slug)
    WHERE post_slug IS NOT NULL AND status IN ('queued', 'running');
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx
    ON generation_jobs(user_id, kind)
    WHERE post_slug IS NULL AND status IN ('queued', 'running');
CREATE INDEX generation_jobs_queue_idx ON generation_jobs(status, created_at);

CREATE TABLE model_experiments_old (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug           TEXT REFERENCES posts(slug) ON DELETE SET NULL,
    stage               TEXT NOT NULL CHECK (stage IN ('observe', 'write', 'analyze')),
    status              TEXT NOT NULL CHECK (status IN ('queued', 'running', 'review', 'partial', 'decided', 'dismissed', 'failed')),
    job_id              TEXT,
    input_snapshot      TEXT,
    input_hash          TEXT NOT NULL,
    prompt_version      TEXT NOT NULL,
    winner_candidate_id TEXT,
    outcome             TEXT CHECK (outcome IS NULL OR outcome IN ('winner', 'skipped', 'unpaired')),
    apply_error         TEXT,
    applied_at          TEXT,
    created_at          TEXT NOT NULL,
    finished_at         TEXT,
    decided_at          TEXT,
    content_expires_at  TEXT,
    adoption_error      TEXT,
    adopted_at          TEXT,
    adoption_requested  INTEGER NOT NULL DEFAULT 0 CHECK (adoption_requested IN (0, 1))
);
INSERT INTO model_experiments_old (id, user_id, post_slug, stage, status, job_id, input_snapshot,
                                   input_hash, prompt_version, winner_candidate_id, outcome, apply_error,
                                   applied_at, created_at, finished_at, decided_at, content_expires_at,
                                   adoption_error, adopted_at, adoption_requested)
SELECT id, user_id, post_slug, stage, status, job_id, input_snapshot, input_hash, prompt_version,
       winner_candidate_id, outcome, apply_error, applied_at, created_at, finished_at, decided_at,
       content_expires_at, adoption_error, adopted_at, adoption_requested FROM model_experiments;
DROP TABLE model_experiments;
ALTER TABLE model_experiments_old RENAME TO model_experiments;
CREATE UNIQUE INDEX one_unresolved_write_experiment_per_post
ON model_experiments(user_id, post_slug)
WHERE stage = 'write' AND post_slug IS NOT NULL
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
    OR (status = 'decided' AND (applied_at IS NULL OR (adoption_requested = 1 AND adopted_at IS NULL)))
  );
CREATE INDEX model_experiments_user_stage_created
ON model_experiments(user_id, stage, created_at DESC, id DESC);
CREATE INDEX model_experiments_terminal_expiry
ON model_experiments(content_expires_at)
WHERE input_snapshot IS NOT NULL AND status IN ('decided', 'dismissed');

DROP TABLE voices;

DROP TABLE IF EXISTS migration_0009_down_integrity_guard;
CREATE TABLE migration_0009_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0009_down_integrity_guard (problem)
SELECT 'rollback left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0009_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Up
-- Closed language provenance and structured durable failures (plan 13). Required
-- language columns use Korean as their migration default so SQLite can add them to a
-- populated table; every application create boundary still requires an explicit value.
ALTER TABLE voices ADD COLUMN source_language TEXT NOT NULL DEFAULT 'ko'
  CHECK (source_language IN ('ko','en'));

ALTER TABLE posts ADD COLUMN target_language TEXT NOT NULL DEFAULT 'ko'
  CHECK (target_language IN ('ko','en'));
ALTER TABLE posts ADD COLUMN content_language TEXT
  CHECK (content_language IS NULL OR content_language IN ('ko','en'));

-- Existing canonical content and machine baselines are Korean. Contentless drafts keep
-- no provenance; no text is inspected or translated.
UPDATE posts
SET content_language = 'ko'
WHERE content IS NOT NULL OR machine_baseline IS NOT NULL;

ALTER TABLE model_experiments ADD COLUMN target_language TEXT
  CHECK (target_language IS NULL OR target_language IN ('ko','en'));
UPDATE model_experiments SET target_language = 'ko' WHERE stage = 'write';

ALTER TABLE generation_jobs ADD COLUMN target_language TEXT
  CHECK (target_language IS NULL OR target_language IN ('ko','en'));
UPDATE generation_jobs SET target_language = 'ko' WHERE kind IN ('generate','revise');

-- Frozen JSON written by older binaries is amended only when it is a JSON object. Empty,
-- malformed, or otherwise opaque legacy payloads are deliberately left byte-for-byte for
-- the compatibility decoders to interpret as Korean.
UPDATE generation_jobs
SET payload = json_set(payload, '$.target_language', 'ko')
WHERE kind = 'generate'
  AND json_valid(payload) AND json_type(payload) = 'object'
  AND json_extract(payload, '$.target_language') IS NULL;
UPDATE generation_jobs
SET payload = json_set(payload, '$.content_language', 'ko')
WHERE kind = 'revise'
  AND json_valid(payload) AND json_type(payload) = 'object'
  AND json_extract(payload, '$.content_language') IS NULL;
UPDATE model_experiments
SET input_snapshot = json_set(input_snapshot, '$.target_language', 'ko')
WHERE stage = 'write' AND input_snapshot IS NOT NULL
  AND json_valid(input_snapshot) AND json_type(input_snapshot) = 'object'
  AND json_extract(input_snapshot, '$.target_language') IS NULL;

-- Every params column is either absent or a valid JSON object. Legacy prose stays in its
-- old column and is copied to technical_detail beside a stable generic reason.
ALTER TABLE generation_jobs ADD COLUMN error_reason TEXT;
ALTER TABLE generation_jobs ADD COLUMN error_params TEXT
  CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object'));
ALTER TABLE generation_jobs ADD COLUMN technical_detail TEXT;
UPDATE generation_jobs
SET error_reason = 'UNKNOWN_FAILURE', error_params = '{}', technical_detail = error
WHERE error IS NOT NULL AND error <> '';

ALTER TABLE model_experiment_candidates ADD COLUMN error_reason TEXT;
ALTER TABLE model_experiment_candidates ADD COLUMN error_params TEXT
  CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object'));
ALTER TABLE model_experiment_candidates ADD COLUMN technical_detail TEXT;
UPDATE model_experiment_candidates
SET error_reason = 'UNKNOWN_FAILURE', error_params = '{}', technical_detail = error
WHERE error IS NOT NULL AND error <> '';

ALTER TABLE model_experiments ADD COLUMN apply_error_reason TEXT;
ALTER TABLE model_experiments ADD COLUMN apply_error_params TEXT
  CHECK (apply_error_params IS NULL OR (json_valid(apply_error_params) AND json_type(apply_error_params) = 'object'));
ALTER TABLE model_experiments ADD COLUMN apply_technical_detail TEXT;
UPDATE model_experiments
SET apply_error_reason = 'UNKNOWN_FAILURE', apply_error_params = '{}', apply_technical_detail = apply_error
WHERE apply_error IS NOT NULL AND apply_error <> '';

ALTER TABLE model_experiments ADD COLUMN adoption_error_reason TEXT;
ALTER TABLE model_experiments ADD COLUMN adoption_error_params TEXT
  CHECK (adoption_error_params IS NULL OR (json_valid(adoption_error_params) AND json_type(adoption_error_params) = 'object'));
ALTER TABLE model_experiments ADD COLUMN adoption_technical_detail TEXT;
UPDATE model_experiments
SET adoption_error_reason = 'UNKNOWN_FAILURE', adoption_error_params = '{}', adoption_technical_detail = adoption_error
WHERE adoption_error IS NOT NULL AND adoption_error <> '';

ALTER TABLE voice_learning_events ADD COLUMN content_language TEXT
  CHECK (content_language IS NULL OR content_language IN ('ko','en'));
ALTER TABLE voice_learning_events ADD COLUMN source_language TEXT
  CHECK (source_language IS NULL OR source_language IN ('ko','en'));
ALTER TABLE voice_learning_events ADD COLUMN error_reason TEXT;
ALTER TABLE voice_learning_events ADD COLUMN error_params TEXT
  CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object'));
ALTER TABLE voice_learning_events ADD COLUMN technical_detail TEXT;
UPDATE voice_learning_events
SET content_language = 'ko', source_language = 'ko';
UPDATE voice_learning_events
SET error_reason = 'UNKNOWN_FAILURE', error_params = '{}', technical_detail = error
WHERE error IS NOT NULL AND error <> '';

ALTER TABLE voice_rule_comparisons ADD COLUMN source_language TEXT
  CHECK (source_language IS NULL OR source_language IN ('ko','en'));
UPDATE voice_rule_comparisons SET source_language = 'ko';
ALTER TABLE voice_rule_comparison_candidates ADD COLUMN error_reason TEXT;
ALTER TABLE voice_rule_comparison_candidates ADD COLUMN error_params TEXT
  CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object'));
ALTER TABLE voice_rule_comparison_candidates ADD COLUMN technical_detail TEXT;
UPDATE voice_rule_comparison_candidates
SET error_reason = 'UNKNOWN_FAILURE', error_params = '{}', technical_detail = error
WHERE error IS NOT NULL AND error <> '';

ALTER TABLE voice_profile_validations ADD COLUMN source_language TEXT
  CHECK (source_language IS NULL OR source_language IN ('ko','en'));
UPDATE voice_profile_validations SET source_language = 'ko';
ALTER TABLE voice_profile_validation_items ADD COLUMN error_reason TEXT;
ALTER TABLE voice_profile_validation_items ADD COLUMN error_params TEXT
  CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object'));
ALTER TABLE voice_profile_validation_items ADD COLUMN technical_detail TEXT;
UPDATE voice_profile_validation_items
SET error_reason = 'UNKNOWN_FAILURE', error_params = '{}', technical_detail = error
WHERE error IS NOT NULL AND error <> '';

ALTER TABLE publish_jobs ADD COLUMN target_language TEXT
  CHECK (target_language IS NULL OR target_language IN ('ko','en'));
ALTER TABLE publish_jobs ADD COLUMN content_language TEXT
  CHECK (content_language IS NULL OR content_language IN ('ko','en'));
ALTER TABLE publish_jobs ADD COLUMN voice_source_language TEXT
  CHECK (voice_source_language IS NULL OR voice_source_language IN ('ko','en'));
ALTER TABLE publish_jobs ADD COLUMN error_reason TEXT;
ALTER TABLE publish_jobs ADD COLUMN error_params TEXT
  CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object'));
ALTER TABLE publish_jobs ADD COLUMN technical_detail TEXT;
UPDATE publish_jobs
SET target_language = 'ko', content_language = 'ko', voice_source_language = 'ko';
UPDATE publish_jobs
SET error_reason = 'UNKNOWN_FAILURE', error_params = '{}',
    technical_detail = COALESCE(NULLIF(error_message, ''), NULLIF(error_code, ''))
WHERE COALESCE(error_message, error_code, '') <> '';

-- +goose Down
ALTER TABLE publish_jobs DROP COLUMN technical_detail;
ALTER TABLE publish_jobs DROP COLUMN error_params;
ALTER TABLE publish_jobs DROP COLUMN error_reason;
ALTER TABLE publish_jobs DROP COLUMN voice_source_language;
ALTER TABLE publish_jobs DROP COLUMN content_language;
ALTER TABLE publish_jobs DROP COLUMN target_language;

ALTER TABLE voice_profile_validation_items DROP COLUMN technical_detail;
ALTER TABLE voice_profile_validation_items DROP COLUMN error_params;
ALTER TABLE voice_profile_validation_items DROP COLUMN error_reason;
ALTER TABLE voice_profile_validations DROP COLUMN source_language;

ALTER TABLE voice_rule_comparison_candidates DROP COLUMN technical_detail;
ALTER TABLE voice_rule_comparison_candidates DROP COLUMN error_params;
ALTER TABLE voice_rule_comparison_candidates DROP COLUMN error_reason;
ALTER TABLE voice_rule_comparisons DROP COLUMN source_language;

ALTER TABLE voice_learning_events DROP COLUMN technical_detail;
ALTER TABLE voice_learning_events DROP COLUMN error_params;
ALTER TABLE voice_learning_events DROP COLUMN error_reason;
ALTER TABLE voice_learning_events DROP COLUMN source_language;
ALTER TABLE voice_learning_events DROP COLUMN content_language;

ALTER TABLE model_experiments DROP COLUMN adoption_technical_detail;
ALTER TABLE model_experiments DROP COLUMN adoption_error_params;
ALTER TABLE model_experiments DROP COLUMN adoption_error_reason;
ALTER TABLE model_experiments DROP COLUMN apply_technical_detail;
ALTER TABLE model_experiments DROP COLUMN apply_error_params;
ALTER TABLE model_experiments DROP COLUMN apply_error_reason;

ALTER TABLE model_experiment_candidates DROP COLUMN technical_detail;
ALTER TABLE model_experiment_candidates DROP COLUMN error_params;
ALTER TABLE model_experiment_candidates DROP COLUMN error_reason;

ALTER TABLE generation_jobs DROP COLUMN technical_detail;
ALTER TABLE generation_jobs DROP COLUMN error_params;
ALTER TABLE generation_jobs DROP COLUMN error_reason;
ALTER TABLE generation_jobs DROP COLUMN target_language;

ALTER TABLE model_experiments DROP COLUMN target_language;
ALTER TABLE posts DROP COLUMN content_language;
ALTER TABLE posts DROP COLUMN target_language;
ALTER TABLE voices DROP COLUMN source_language;

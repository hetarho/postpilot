-- Voices. The account owns the directory; every profile/evidence row below belongs to
-- exactly one voice, and every query names both so a same-account id from another voice
-- cannot reach this one's aggregate.

-- name: InsertVoice :exec
INSERT INTO voices (id, user_id, name, source_language, is_default, deleted_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, NULL, ?, ?);

-- name: ListVoices :many
SELECT * FROM voices WHERE user_id = ?
ORDER BY deleted_at IS NOT NULL, is_default DESC, name, id;

-- name: GetVoice :one
SELECT * FROM voices WHERE id = ? AND user_id = ?;

-- name: GetDefaultVoice :one
SELECT * FROM voices WHERE user_id = ? AND is_default = 1 AND deleted_at IS NULL;

-- name: CountActiveVoices :one
SELECT count(*) FROM voices WHERE user_id = ? AND deleted_at IS NULL;

-- name: RenameVoice :execrows
UPDATE voices SET name = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: ClearDefaultVoice :exec
UPDATE voices SET is_default = 0, updated_at = ? WHERE user_id = ? AND is_default = 1;

-- name: SetDefaultVoice :execrows
UPDATE voices SET is_default = 1, updated_at = ?
WHERE id = ? AND user_id = ? AND deleted_at IS NULL;

-- name: SoftDeleteVoice :execrows
UPDATE voices SET deleted_at = ?, updated_at = ?
WHERE id = ? AND user_id = ? AND deleted_at IS NULL AND is_default = 0;

-- name: RestoreVoice :execrows
UPDATE voices SET deleted_at = NULL, updated_at = ?
WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL;

-- name: CountUndecidedVoiceWork :one
-- Work this context owns that would have nowhere to land if the voice left selection. Jobs
-- and analyze experiments are asked for through their own contexts' ports, not read here:
-- a voice query may not touch another aggregate's tables (ARCHITECTURE section 2).
SELECT (SELECT count(*) FROM voice_rule_comparisons c
         WHERE c.voice_id = ? AND c.status IN ('queued','running','review','partial'))
     + (SELECT count(*) FROM voice_profile_validations v
         WHERE v.voice_id = ? AND v.status IN ('queued','running','review','partial'));

-- name: UpsertProfile :exec
INSERT INTO voice_profiles (voice_id, user_id, styleguide, rules, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(voice_id) DO UPDATE SET
    styleguide = excluded.styleguide,
    rules = excluded.rules,
    updated_at = excluded.updated_at;

-- name: GetProfile :one
SELECT voice_id, user_id, styleguide, rules, current_version, corpus_version, updated_at
FROM voice_profiles
WHERE voice_id = ? AND user_id = ?;

-- name: SetStyleguideIfCorpusVersion :execrows
UPDATE voice_profiles
SET styleguide = ?, updated_at = ?
WHERE voice_id = ? AND user_id = ? AND corpus_version = ?;

-- name: SetStyleguide :exec
INSERT INTO voice_profiles (voice_id, user_id, styleguide, rules, updated_at)
VALUES (?, ?, ?, '', ?)
ON CONFLICT(voice_id) DO UPDATE SET
    styleguide = excluded.styleguide,
    updated_at = excluded.updated_at;

-- name: SetRules :exec
INSERT INTO voice_profiles (voice_id, user_id, styleguide, rules, updated_at)
VALUES (?, ?, '', ?, ?)
ON CONFLICT(voice_id) DO UPDATE SET
    rules = excluded.rules,
    updated_at = excluded.updated_at;

-- name: InsertSample :exec
INSERT INTO voice_samples (id, voice_id, user_id, label, body, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: BumpCorpusVersion :exec
INSERT INTO voice_profiles (voice_id, user_id, styleguide, rules, corpus_version, updated_at)
VALUES (?, ?, '', '', 1, ?)
ON CONFLICT(voice_id) DO UPDATE SET
    corpus_version = voice_profiles.corpus_version + 1,
    updated_at = excluded.updated_at;

-- name: GetCorpusVersion :one
SELECT corpus_version FROM voice_profiles WHERE voice_id = ? AND user_id = ?;

-- name: ListSamples :many
SELECT id, label, length(body) AS chars, created_at
FROM voice_samples
WHERE voice_id = ? AND user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: ListSampleBodies :many
SELECT id, label, body, created_at
FROM voice_samples
WHERE voice_id = ? AND user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: GetSampleBody :one
SELECT id, label, body, created_at
FROM voice_samples
WHERE id = ? AND voice_id = ? AND user_id = ?;

-- name: DeleteSample :execrows
DELETE FROM voice_samples WHERE id = ? AND voice_id = ? AND user_id = ?;

-- name: CountSamples :one
SELECT count(*) FROM voice_samples WHERE voice_id = ? AND user_id = ?;

-- name: InsertProfileVersion :exec
INSERT INTO voice_profile_versions
    (id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: SetProfileHead :exec
INSERT INTO voice_profiles(voice_id, user_id, styleguide, rules, corpus_version, current_version, updated_at)
VALUES (?, ?, '', '', 0, ?, ?)
ON CONFLICT(voice_id) DO UPDATE SET current_version=excluded.current_version, updated_at=excluded.updated_at;

-- name: GetProfileVersion :one
SELECT id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at FROM voice_profile_versions WHERE voice_id=? AND user_id=? AND version=?;

-- name: ListProfileVersions :many
SELECT id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at FROM voice_profile_versions WHERE voice_id=? AND user_id=? ORDER BY version DESC;

-- name: ListManualOverrides :many
SELECT voice_id, user_id, layer, field, value, updated_at FROM voice_manual_overrides WHERE voice_id=? AND user_id=? ORDER BY layer, field;

-- name: UpsertManualOverride :exec
INSERT INTO voice_manual_overrides(voice_id, user_id, layer, field, value, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(voice_id,layer,field) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at;

-- name: DeleteManualOverride :execrows
DELETE FROM voice_manual_overrides WHERE voice_id=? AND user_id=? AND layer=? AND field=?;

-- name: InsertLearningEvent :exec
INSERT INTO voice_learning_events
    (id,user_id,voice_id,post_slug,baseline_revision,input_hash,baseline_content,final_content,model_ref,status,job_id,error,created_at,processed_at,content_language,source_language,error_reason,error_params,technical_detail)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);

-- name: GetLearningEventByInput :one
SELECT * FROM voice_learning_events
WHERE voice_id=? AND user_id=? AND post_slug=? AND baseline_revision=? AND input_hash=?;

-- name: GetLearningEvent :one
SELECT * FROM voice_learning_events WHERE id=? AND user_id=?;

-- name: SetLearningEventJob :exec
UPDATE voice_learning_events SET job_id=?, status='queued', error=NULL,
    error_reason=NULL,error_params=NULL,technical_detail=NULL WHERE id=? AND user_id=?;

-- name: SetLearningEventStatus :exec
UPDATE voice_learning_events SET status=?, error=?, error_reason=?,error_params=?,technical_detail=?,processed_at=? WHERE id=? AND user_id=?;

-- name: InsertAuthoredSource :exec
INSERT INTO voice_authored_sources
    (id,user_id,voice_id,post_slug,learning_event_id,title,tags,body,excerpt,embedding_ref,created_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?);

-- name: ListAuthoredSources :many
SELECT s.id, s.user_id, s.voice_id, s.post_slug, s.learning_event_id, s.title, s.tags,
       s.body, s.excerpt, s.embedding_ref, s.created_at,
       COALESCE(e.source_language, v.source_language) AS source_language
FROM voice_authored_sources s
JOIN voices v ON v.id = s.voice_id AND v.user_id = s.user_id
LEFT JOIN voice_learning_events e ON e.id = s.learning_event_id
WHERE s.voice_id=? AND s.user_id=?
ORDER BY s.created_at DESC,s.id DESC;

-- name: GetAuthoredSource :one
SELECT s.id, s.user_id, s.voice_id, s.post_slug, s.learning_event_id, s.title, s.tags,
       s.body, s.excerpt, s.embedding_ref, s.created_at,
       COALESCE(e.source_language, v.source_language) AS source_language
FROM voice_authored_sources s
JOIN voices v ON v.id = s.voice_id AND v.user_id = s.user_id
LEFT JOIN voice_learning_events e ON e.id = s.learning_event_id
WHERE s.id=? AND s.voice_id=? AND s.user_id=?;

-- name: CountAuthoredSources :one
SELECT count(*) FROM voice_authored_sources WHERE voice_id=? AND user_id=?;

-- name: InsertContrastRule :exec
INSERT INTO voice_contrast_rules
    (id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?);

-- name: GetContrastRuleByKey :one
SELECT id, user_id, voice_id, statement, canonical_key, layer, evidence_count, status, origin, created_at, last_evidence_at FROM voice_contrast_rules WHERE voice_id=? AND user_id=? AND canonical_key=?;

-- name: GetContrastRule :one
SELECT id, user_id, voice_id, statement, canonical_key, layer, evidence_count, status, origin, created_at, last_evidence_at FROM voice_contrast_rules WHERE id=? AND voice_id=? AND user_id=?;

-- name: GetContrastRuleForUser :one
-- Rule-derived operations (status changes, comparisons) name only the rule, so the voice is
-- read off the row: a same-account caller cannot point a rule at another voice.
SELECT id, user_id, voice_id, statement, canonical_key, layer, evidence_count, status, origin, created_at, last_evidence_at FROM voice_contrast_rules WHERE id=? AND user_id=?;

-- name: ListContrastRules :many
SELECT id, user_id, voice_id, statement, canonical_key, layer, evidence_count, status, origin, created_at, last_evidence_at FROM voice_contrast_rules WHERE voice_id=? AND user_id=?
ORDER BY CASE status WHEN 'active' THEN 0 WHEN 'candidate' THEN 1 WHEN 'retired' THEN 2 ELSE 3 END,
         evidence_count DESC,last_evidence_at DESC,id;

-- name: UpdateContrastRuleEvidence :exec
UPDATE voice_contrast_rules SET evidence_count=?,status=?,last_evidence_at=? WHERE id=? AND voice_id=? AND user_id=?;

-- name: UpdateContrastRuleStatus :exec
UPDATE voice_contrast_rules SET status=?,last_evidence_at=? WHERE id=? AND voice_id=? AND user_id=?;

-- name: ReplaceContrastRule :exec
UPDATE voice_contrast_rules SET statement=?,canonical_key=?,evidence_count=1,status='candidate',last_evidence_at=?
WHERE id=? AND voice_id=? AND user_id=?;

-- name: RetireStaleRules :execrows
UPDATE voice_contrast_rules SET status='retired'
WHERE voice_id=? AND user_id=? AND status='active' AND last_evidence_at < ?;

-- name: InsertRuleEvidence :exec
INSERT INTO voice_rule_evidence(id,user_id,voice_id,rule_id,event_id,origin,payload_ref,created_at)
VALUES (?,?,?,?,?,?,?,?);

-- name: InsertRuleConfirmation :exec
INSERT INTO voice_rule_confirmations(id,user_id,voice_id,rule_id,proposed_statement,event_id,status,created_at,resolved_at)
VALUES (?,?,?,?,?,?,'pending',?,NULL);

-- name: ListRuleConfirmations :many
SELECT id, user_id, voice_id, rule_id, proposed_statement, event_id, status, created_at, resolved_at FROM voice_rule_confirmations WHERE voice_id=? AND user_id=? ORDER BY created_at DESC,id DESC;

-- name: GetRuleConfirmation :one
SELECT id, user_id, voice_id, rule_id, proposed_statement, event_id, status, created_at, resolved_at FROM voice_rule_confirmations WHERE id=? AND user_id=?;

-- name: ResolveRuleConfirmation :execrows
UPDATE voice_rule_confirmations SET status=?,resolved_at=? WHERE id=? AND user_id=? AND status='pending';

-- name: InsertSentenceFeedback :exec
INSERT INTO voice_sentence_feedback
    (id,user_id,voice_id,post_slug,sentence_ref,kind,reason,payload_ref,processing_state,created_at)
VALUES (?,?,?,?,?,?,?,?,?,?);

-- name: ListSentenceFeedback :many
SELECT id, user_id, voice_id, post_slug, sentence_ref, kind, reason, payload_ref, processing_state, created_at FROM voice_sentence_feedback WHERE voice_id=? AND user_id=? ORDER BY created_at DESC,id DESC;

-- name: InsertRuleComparison :exec
INSERT INTO voice_rule_comparisons
    (id,user_id,voice_id,rule_id,source_id,profile_version,model_ref,target_length,input_snapshot,rule_on_side,status,job_id,chosen_side,created_at,decided_at,source_language)
VALUES (?,?,?,?,?,?,?,?,?,?,'queued',?,NULL,?,NULL,?);

-- name: InsertRuleComparisonCandidate :exec
INSERT INTO voice_rule_comparison_candidates(id,comparison_id,display_side,output,status,error)
VALUES (?,?,?,NULL,'pending',NULL);

-- name: GetRuleComparison :one
SELECT * FROM voice_rule_comparisons WHERE id=? AND user_id=?;

-- name: ListRuleComparisonCandidates :many
SELECT * FROM voice_rule_comparison_candidates WHERE comparison_id=? ORDER BY display_side;

-- name: UpdateRuleComparisonCandidate :exec
UPDATE voice_rule_comparison_candidates
SET output=?,status=?,error=?,error_reason=?,error_params=?,technical_detail=?
WHERE id=? AND comparison_id=?;

-- name: UpdateRuleComparisonStatus :exec
UPDATE voice_rule_comparisons SET status=? WHERE id=? AND user_id=?;

-- name: SetRuleComparisonJob :exec
UPDATE voice_rule_comparisons SET job_id=? WHERE id=? AND user_id=?;

-- name: DecideRuleComparison :execrows
UPDATE voice_rule_comparisons SET status='decided',chosen_side=?,decided_at=?
WHERE id=? AND user_id=? AND status IN ('review','partial');

-- name: InsertProfileValidation :exec
INSERT INTO voice_profile_validations
    (id,user_id,voice_id,profile_version,analyze_model_ref,write_model_ref,judge_enabled,status,job_id,y_count,total_count,created_at,finished_at,source_language)
VALUES (?,?,?,?,?,?,?,'queued',?,NULL,NULL,?,NULL,?);

-- name: InsertProfileValidationItem :exec
INSERT INTO voice_profile_validation_items
    (id,validation_id,source_id,voice_id,user_id,position,neutral_summary,regenerated_content,scores,status,error)
VALUES (?,?,?,?,?,?,NULL,NULL,NULL,'pending',NULL);

-- name: GetProfileValidation :one
SELECT * FROM voice_profile_validations WHERE id=? AND user_id=?;

-- name: ListProfileValidations :many
SELECT * FROM voice_profile_validations WHERE voice_id=? AND user_id=? ORDER BY created_at DESC,id DESC;

-- name: ListProfileValidationItems :many
SELECT * FROM voice_profile_validation_items WHERE validation_id=? ORDER BY position;

-- name: UpdateProfileValidationItem :exec
UPDATE voice_profile_validation_items
SET neutral_summary=?,regenerated_content=?,scores=?,status=?,error=?,
    error_reason=?,error_params=?,technical_detail=?
WHERE id=? AND validation_id=?;

-- name: FinishProfileValidation :exec
UPDATE voice_profile_validations SET status=?,y_count=?,total_count=?,finished_at=? WHERE id=? AND user_id=?;

-- name: SetProfileValidationJob :exec
UPDATE voice_profile_validations SET job_id=? WHERE id=? AND user_id=?;

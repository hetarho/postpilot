-- name: UpsertProfile :exec
INSERT INTO voice_profiles (user_id, styleguide, rules, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
    styleguide = excluded.styleguide,
    rules = excluded.rules,
    updated_at = excluded.updated_at;

-- name: GetProfile :one
SELECT user_id, styleguide, rules, current_version, corpus_version, updated_at
FROM voice_profiles
WHERE user_id = ?;

-- name: SetStyleguideIfCorpusVersion :execrows
UPDATE voice_profiles
SET styleguide = ?, updated_at = ?
WHERE user_id = ? AND corpus_version = ?;

-- name: SetStyleguide :exec
INSERT INTO voice_profiles (user_id, styleguide, rules, updated_at)
VALUES (?, ?, '', ?)
ON CONFLICT(user_id) DO UPDATE SET
    styleguide = excluded.styleguide,
    updated_at = excluded.updated_at;

-- name: SetRules :exec
INSERT INTO voice_profiles (user_id, styleguide, rules, updated_at)
VALUES (?, '', ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
    rules = excluded.rules,
    updated_at = excluded.updated_at;

-- name: InsertSample :exec
INSERT INTO voice_samples (id, user_id, label, body, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: BumpCorpusVersion :exec
INSERT INTO voice_profiles (user_id, styleguide, rules, corpus_version, updated_at)
VALUES (?, '', '', 1, ?)
ON CONFLICT(user_id) DO UPDATE SET
    corpus_version = voice_profiles.corpus_version + 1,
    updated_at = excluded.updated_at;

-- name: GetCorpusVersion :one
SELECT corpus_version FROM voice_profiles WHERE user_id = ?;

-- name: ListSamples :many
SELECT id, label, length(body) AS chars, created_at
FROM voice_samples
WHERE user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: ListSampleBodies :many
SELECT id, label, body, created_at
FROM voice_samples
WHERE user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: GetSampleBody :one
SELECT id, label, body, created_at
FROM voice_samples
WHERE id = ? AND user_id = ?;

-- name: DeleteSample :execrows
DELETE FROM voice_samples WHERE id = ? AND user_id = ?;

-- name: CountSamples :one
SELECT count(*) FROM voice_samples WHERE user_id = ?;

-- name: InsertProfileVersion :exec
INSERT INTO voice_profile_versions
    (id, user_id, version, snapshot, origin, restored_from_version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: SetProfileHead :exec
INSERT INTO voice_profiles(user_id, styleguide, rules, corpus_version, current_version, updated_at)
VALUES (?, '', '', 0, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET current_version=excluded.current_version, updated_at=excluded.updated_at;

-- name: GetProfileVersion :one
SELECT * FROM voice_profile_versions WHERE user_id=? AND version=?;

-- name: ListProfileVersions :many
SELECT * FROM voice_profile_versions WHERE user_id=? ORDER BY version DESC;

-- name: ListManualOverrides :many
SELECT * FROM voice_manual_overrides WHERE user_id=? ORDER BY layer, field;

-- name: UpsertManualOverride :exec
INSERT INTO voice_manual_overrides(user_id, layer, field, value, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(user_id,layer,field) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at;

-- name: DeleteManualOverride :execrows
DELETE FROM voice_manual_overrides WHERE user_id=? AND layer=? AND field=?;

-- name: InsertLearningEvent :exec
INSERT INTO voice_learning_events
    (id,user_id,post_slug,baseline_revision,input_hash,baseline_content,final_content,model_ref,status,job_id,error,created_at,processed_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?);

-- name: GetLearningEventByInput :one
SELECT * FROM voice_learning_events
WHERE user_id=? AND post_slug=? AND baseline_revision=? AND input_hash=?;

-- name: GetLearningEvent :one
SELECT * FROM voice_learning_events WHERE id=? AND user_id=?;

-- name: SetLearningEventJob :exec
UPDATE voice_learning_events SET job_id=?, status='queued', error=NULL WHERE id=? AND user_id=?;

-- name: SetLearningEventStatus :exec
UPDATE voice_learning_events SET status=?, error=?, processed_at=? WHERE id=? AND user_id=?;

-- name: InsertAuthoredSource :exec
INSERT INTO voice_authored_sources
    (id,user_id,post_slug,learning_event_id,title,tags,body,excerpt,embedding_ref,created_at)
VALUES (?,?,?,?,?,?,?,?,?,?);

-- name: ListAuthoredSources :many
SELECT * FROM voice_authored_sources WHERE user_id=? ORDER BY created_at DESC,id DESC;

-- name: GetAuthoredSource :one
SELECT * FROM voice_authored_sources WHERE id=? AND user_id=?;

-- name: CountAuthoredSources :one
SELECT count(*) FROM voice_authored_sources WHERE user_id=?;

-- name: InsertContrastRule :exec
INSERT INTO voice_contrast_rules
    (id,user_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at)
VALUES (?,?,?,?,?,?,?,?,?,?);

-- name: GetContrastRuleByKey :one
SELECT * FROM voice_contrast_rules WHERE user_id=? AND canonical_key=?;

-- name: GetContrastRule :one
SELECT * FROM voice_contrast_rules WHERE id=? AND user_id=?;

-- name: ListContrastRules :many
SELECT * FROM voice_contrast_rules WHERE user_id=?
ORDER BY CASE status WHEN 'active' THEN 0 WHEN 'candidate' THEN 1 WHEN 'retired' THEN 2 ELSE 3 END,
         evidence_count DESC,last_evidence_at DESC,id;

-- name: UpdateContrastRuleEvidence :exec
UPDATE voice_contrast_rules SET evidence_count=?,status=?,last_evidence_at=? WHERE id=? AND user_id=?;

-- name: UpdateContrastRuleStatus :exec
UPDATE voice_contrast_rules SET status=?,last_evidence_at=? WHERE id=? AND user_id=?;

-- name: ReplaceContrastRule :exec
UPDATE voice_contrast_rules SET statement=?,canonical_key=?,evidence_count=1,status='candidate',last_evidence_at=?
WHERE id=? AND user_id=?;

-- name: RetireStaleRules :execrows
UPDATE voice_contrast_rules SET status='retired'
WHERE user_id=? AND status='active' AND last_evidence_at < ?;

-- name: InsertRuleEvidence :exec
INSERT INTO voice_rule_evidence(id,user_id,rule_id,event_id,origin,payload_ref,created_at)
VALUES (?,?,?,?,?,?,?);

-- name: InsertRuleConfirmation :exec
INSERT INTO voice_rule_confirmations(id,user_id,rule_id,proposed_statement,event_id,status,created_at,resolved_at)
VALUES (?,?,?,?,?,'pending',?,NULL);

-- name: ListRuleConfirmations :many
SELECT * FROM voice_rule_confirmations WHERE user_id=? ORDER BY created_at DESC,id DESC;

-- name: GetRuleConfirmation :one
SELECT * FROM voice_rule_confirmations WHERE id=? AND user_id=?;

-- name: ResolveRuleConfirmation :execrows
UPDATE voice_rule_confirmations SET status=?,resolved_at=? WHERE id=? AND user_id=? AND status='pending';

-- name: InsertSentenceFeedback :exec
INSERT INTO voice_sentence_feedback
    (id,user_id,post_slug,sentence_ref,kind,reason,payload_ref,processing_state,created_at)
VALUES (?,?,?,?,?,?,?,?,?);

-- name: ListSentenceFeedback :many
SELECT * FROM voice_sentence_feedback WHERE user_id=? ORDER BY created_at DESC,id DESC;

-- name: InsertRuleComparison :exec
INSERT INTO voice_rule_comparisons
    (id,user_id,rule_id,source_id,profile_version,model_ref,target_length,input_snapshot,rule_on_side,status,job_id,chosen_side,created_at,decided_at)
VALUES (?,?,?,?,?,?,?,?,?,'queued',?,NULL,?,NULL);

-- name: InsertRuleComparisonCandidate :exec
INSERT INTO voice_rule_comparison_candidates(id,comparison_id,display_side,output,status,error)
VALUES (?,?,?,NULL,'pending',NULL);

-- name: GetRuleComparison :one
SELECT * FROM voice_rule_comparisons WHERE id=? AND user_id=?;

-- name: ListRuleComparisonCandidates :many
SELECT * FROM voice_rule_comparison_candidates WHERE comparison_id=? ORDER BY display_side;

-- name: UpdateRuleComparisonCandidate :exec
UPDATE voice_rule_comparison_candidates SET output=?,status=?,error=? WHERE id=? AND comparison_id=?;

-- name: UpdateRuleComparisonStatus :exec
UPDATE voice_rule_comparisons SET status=? WHERE id=? AND user_id=?;

-- name: SetRuleComparisonJob :exec
UPDATE voice_rule_comparisons SET job_id=? WHERE id=? AND user_id=?;

-- name: DecideRuleComparison :execrows
UPDATE voice_rule_comparisons SET status='decided',chosen_side=?,decided_at=?
WHERE id=? AND user_id=? AND status IN ('review','partial');

-- name: InsertProfileValidation :exec
INSERT INTO voice_profile_validations
    (id,user_id,profile_version,analyze_model_ref,write_model_ref,judge_enabled,status,job_id,y_count,total_count,created_at,finished_at)
VALUES (?,?,?,?,?,?,'queued',?,NULL,NULL,?,NULL);

-- name: InsertProfileValidationItem :exec
INSERT INTO voice_profile_validation_items
    (id,validation_id,source_id,position,neutral_summary,regenerated_content,scores,status,error)
VALUES (?,?,?,?,NULL,NULL,NULL,'pending',NULL);

-- name: GetProfileValidation :one
SELECT * FROM voice_profile_validations WHERE id=? AND user_id=?;

-- name: ListProfileValidations :many
SELECT * FROM voice_profile_validations WHERE user_id=? ORDER BY created_at DESC,id DESC;

-- name: ListProfileValidationItems :many
SELECT * FROM voice_profile_validation_items WHERE validation_id=? ORDER BY position;

-- name: UpdateProfileValidationItem :exec
UPDATE voice_profile_validation_items
SET neutral_summary=?,regenerated_content=?,scores=?,status=?,error=?
WHERE id=? AND validation_id=?;

-- name: FinishProfileValidation :exec
UPDATE voice_profile_validations SET status=?,y_count=?,total_count=?,finished_at=? WHERE id=? AND user_id=?;

-- name: SetProfileValidationJob :exec
UPDATE voice_profile_validations SET job_id=? WHERE id=? AND user_id=?;

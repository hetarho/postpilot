package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
func nullableTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return nullableString(formatTime(*value))
}

func encodeProfile(value voice.StructuredProfile) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}
func decodeProfile(value string) (voice.StructuredProfile, error) {
	var out voice.StructuredProfile
	err := json.Unmarshal([]byte(value), &out)
	return out, err
}

func (s *Store) ListProfileVersions(ctx context.Context, userID, voiceID string) ([]voice.ProfileVersion, error) {
	rows, err := s.read.ListProfileVersions(ctx, sqlc.ListProfileVersionsParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]voice.ProfileVersion, 0, len(rows))
	for _, row := range rows {
		item, err := profileVersion(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}
func (s *Store) GetProfileVersion(ctx context.Context, userID, voiceID string, version int64) (voice.ProfileVersion, error) {
	row, err := s.read.GetProfileVersion(ctx, sqlc.GetProfileVersionParams{VoiceID: voiceID, UserID: userID, Version: version})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.ProfileVersion{}, voice.ErrLearningNotFound
	}
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	return profileVersion(row)
}
func profileVersion(row sqlc.VoiceProfileVersion) (voice.ProfileVersion, error) {
	p, err := decodeProfile(row.Snapshot)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	return voice.ProfileVersion{ID: row.ID, UserID: row.UserID, VoiceID: row.VoiceID, Version: row.Version, Profile: p, Origin: row.Origin, RestoredFromVersion: row.RestoredFromVersion.Int64, CreatedAt: created}, nil
}

func (s *Store) PublishProfileVersion(ctx context.Context, userID, voiceID string, profile voice.StructuredProfile, origin string, restoredFrom int64, now time.Time) (voice.ProfileVersion, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	version, err := publishProfileWithQueries(ctx, q, userID, voiceID, profile, origin, restoredFrom, now)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	if err = tx.Commit(); err != nil {
		return voice.ProfileVersion{}, err
	}
	return version, nil
}

func (s *Store) PublishProfileVersionIfHead(ctx context.Context, userID, voiceID string, profile voice.StructuredProfile, origin string, expectedHead int64, now time.Time) (voice.ProfileVersion, bool, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return voice.ProfileVersion{}, false, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	current := int64(0)
	row, err := q.GetProfile(ctx, sqlc.GetProfileParams{VoiceID: voiceID, UserID: userID})
	if err == nil {
		current = row.CurrentVersion
	} else if !errors.Is(err, sql.ErrNoRows) {
		return voice.ProfileVersion{}, false, err
	}
	if current != expectedHead {
		if err = tx.Commit(); err != nil {
			return voice.ProfileVersion{}, false, err
		}
		return voice.ProfileVersion{}, false, nil
	}
	version, err := publishProfileWithQueries(ctx, q, userID, voiceID, profile, origin, 0, now)
	if err != nil {
		return voice.ProfileVersion{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return voice.ProfileVersion{}, false, err
	}
	return version, true, nil
}

// publishProfileWithQueries appends the next immutable version for ONE voice and moves that
// voice's head; version numbers count per voice, so two voices both at v1 is the normal case.
func publishProfileWithQueries(ctx context.Context, q *sqlc.Queries, userID, voiceID string, profile voice.StructuredProfile, origin string, restoredFrom int64, now time.Time) (voice.ProfileVersion, error) {
	current := int64(0)
	row, err := q.GetProfile(ctx, sqlc.GetProfileParams{VoiceID: voiceID, UserID: userID})
	if err == nil {
		current = row.CurrentVersion
	} else if !errors.Is(err, sql.ErrNoRows) {
		return voice.ProfileVersion{}, err
	}
	profile.Version, profile.UpdatedAt = current+1, now
	encoded, err := encodeProfile(profile)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	id := storeID()
	if err = q.InsertProfileVersion(ctx, sqlc.InsertProfileVersionParams{ID: id, UserID: userID, VoiceID: voiceID, Version: profile.Version, Snapshot: encoded, Origin: origin, RestoredFromVersion: sql.NullInt64{Int64: restoredFrom, Valid: restoredFrom > 0}, CreatedAt: formatTime(now)}); err != nil {
		return voice.ProfileVersion{}, err
	}
	if err = q.SetProfileHead(ctx, sqlc.SetProfileHeadParams{VoiceID: voiceID, UserID: userID, CurrentVersion: profile.Version, UpdatedAt: formatTime(now)}); err != nil {
		return voice.ProfileVersion{}, err
	}
	return voice.ProfileVersion{ID: id, UserID: userID, VoiceID: voiceID, Version: profile.Version, Profile: profile, Origin: origin, RestoredFromVersion: restoredFrom, CreatedAt: now}, nil
}

func (s *Store) ListManualOverrides(ctx context.Context, userID, voiceID string) ([]voice.ManualOverride, error) {
	rows, err := s.read.ListManualOverrides(ctx, sqlc.ListManualOverridesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]voice.ManualOverride, 0, len(rows))
	for _, r := range rows {
		at, e := parseTime(r.UpdatedAt)
		if e != nil {
			return nil, e
		}
		out = append(out, voice.ManualOverride{UserID: r.UserID, VoiceID: r.VoiceID, Layer: voice.RuleLayer(r.Layer), Field: r.Field, Value: r.Value, UpdatedAt: at})
	}
	return out, nil
}
func (s *Store) SetManualOverride(ctx context.Context, v voice.ManualOverride) error {
	return s.write.UpsertManualOverride(ctx, sqlc.UpsertManualOverrideParams{VoiceID: v.VoiceID, UserID: v.UserID, Layer: string(v.Layer), Field: v.Field, Value: v.Value, UpdatedAt: formatTime(v.UpdatedAt)})
}
func (s *Store) DeleteManualOverride(ctx context.Context, userID, voiceID string, layer voice.RuleLayer, field string) (bool, error) {
	n, err := s.write.DeleteManualOverride(ctx, sqlc.DeleteManualOverrideParams{VoiceID: voiceID, UserID: userID, Layer: string(layer), Field: field})
	return n > 0, err
}
func (s *Store) ApplyOverrideAndPublish(ctx context.Context, override voice.ManualOverride, value *string, profile voice.StructuredProfile, now time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if value == nil {
		if _, err = q.DeleteManualOverride(ctx, sqlc.DeleteManualOverrideParams{VoiceID: override.VoiceID, UserID: override.UserID, Layer: string(override.Layer), Field: override.Field}); err != nil {
			return err
		}
	} else {
		if err = q.UpsertManualOverride(ctx, sqlc.UpsertManualOverrideParams{VoiceID: override.VoiceID, UserID: override.UserID, Layer: string(override.Layer), Field: override.Field, Value: *value, UpdatedAt: formatTime(now)}); err != nil {
			return err
		}
	}
	if _, err = publishProfileWithQueries(ctx, q, override.UserID, override.VoiceID, profile, "manual", 0, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) InsertLearningEvent(ctx context.Context, e voice.LearningEvent) error {
	if !e.ContentLanguage.Valid() || !e.SourceLanguage.Valid() {
		return voice.ErrLanguageRequired
	}
	reason, params, detail, err := encodeFailure(e.Failure)
	if err != nil {
		return err
	}
	return s.write.InsertLearningEvent(ctx, sqlc.InsertLearningEventParams{ID: e.ID, UserID: e.UserID, VoiceID: e.VoiceID, PostSlug: e.PostSlug, BaselineRevision: e.BaselineRevision, InputHash: e.InputHash, BaselineContent: e.BaselineJSON, FinalContent: e.FinalJSON, ModelRef: e.ModelRef, Status: e.Status, JobID: nullableString(e.JobID), Error: sql.NullString{}, CreatedAt: formatTime(e.CreatedAt), ProcessedAt: nullableTime(e.ProcessedAt), ContentLanguage: nullableString(string(e.ContentLanguage)), SourceLanguage: nullableString(string(e.SourceLanguage)), ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail})
}
func (s *Store) FindLearningEvent(ctx context.Context, userID, voiceID, postSlug string, baselineRevision int64, inputHash string) (*voice.LearningEvent, error) {
	row, err := s.read.GetLearningEventByInput(ctx, sqlc.GetLearningEventByInputParams{VoiceID: voiceID, UserID: userID, PostSlug: postSlug, BaselineRevision: baselineRevision, InputHash: inputHash})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v, e := learningEvent(row)
	return &v, e
}
func (s *Store) GetLearningEvent(ctx context.Context, userID, eventID string) (*voice.LearningEvent, error) {
	row, err := s.read.GetLearningEvent(ctx, sqlc.GetLearningEventParams{ID: eventID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, voice.ErrLearningNotFound
	}
	if err != nil {
		return nil, err
	}
	v, e := learningEvent(row)
	return &v, e
}
func learningEvent(r sqlc.VoiceLearningEvent) (voice.LearningEvent, error) {
	created, err := parseTime(r.CreatedAt)
	if err != nil {
		return voice.LearningEvent{}, err
	}
	var processed *time.Time
	if r.ProcessedAt.Valid {
		v, e := parseTime(r.ProcessedAt.String)
		if e != nil {
			return voice.LearningEvent{}, e
		}
		processed = &v
	}
	contentLanguage, err := voice.ParseLanguage(r.ContentLanguage.String)
	if err != nil {
		return voice.LearningEvent{}, err
	}
	sourceLanguage, err := voice.ParseLanguage(r.SourceLanguage.String)
	if err != nil {
		return voice.LearningEvent{}, err
	}
	failure, err := decodeFailure(r.ErrorReason, r.ErrorParams, r.TechnicalDetail, r.Error)
	if err != nil {
		return voice.LearningEvent{}, err
	}
	return voice.LearningEvent{ID: r.ID, UserID: r.UserID, VoiceID: r.VoiceID, PostSlug: r.PostSlug, BaselineRevision: r.BaselineRevision, InputHash: r.InputHash, BaselineJSON: r.BaselineContent, FinalJSON: r.FinalContent, ModelRef: r.ModelRef, Status: r.Status, JobID: r.JobID.String, Error: r.Error.String, CreatedAt: created, ProcessedAt: processed, ContentLanguage: contentLanguage, SourceLanguage: sourceLanguage, Failure: failure}, nil
}
func (s *Store) SetLearningEventJob(ctx context.Context, userID, eventID, jobID string) error {
	return s.write.SetLearningEventJob(ctx, sqlc.SetLearningEventJobParams{JobID: nullableString(jobID), ID: eventID, UserID: userID})
}
func (s *Store) SetLearningEventStatus(ctx context.Context, userID, eventID, status string, failure *voice.Failure, processedAt *time.Time) error {
	reason, params, detail, err := encodeFailure(failure)
	if err != nil {
		return err
	}
	return s.write.SetLearningEventStatus(ctx, sqlc.SetLearningEventStatusParams{Status: status, ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail, ProcessedAt: nullableTime(processedAt), ID: eventID, UserID: userID})
}

func (s *Store) ListAuthoredSources(ctx context.Context, userID, voiceID string) ([]voice.AuthoredSource, error) {
	rows, err := s.read.ListAuthoredSources(ctx, sqlc.ListAuthoredSourcesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]voice.AuthoredSource, 0, len(rows))
	for _, r := range rows {
		v, e := authoredSourceFromList(r)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) GetAuthoredSource(ctx context.Context, userID, voiceID, sourceID string) (voice.AuthoredSource, error) {
	r, err := s.read.GetAuthoredSource(ctx, sqlc.GetAuthoredSourceParams{ID: sourceID, VoiceID: voiceID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.AuthoredSource{}, voice.ErrLearningNotFound
	}
	if err != nil {
		return voice.AuthoredSource{}, err
	}
	return authoredSourceFromGet(r)
}
func authoredSourceFromList(r sqlc.ListAuthoredSourcesRow) (voice.AuthoredSource, error) {
	return authoredSource(r.ID, r.UserID, r.VoiceID, r.PostSlug, r.LearningEventID, r.Title, r.Tags, r.Body, r.Excerpt, r.EmbeddingRef, r.CreatedAt, r.SourceLanguage)
}
func authoredSourceFromGet(r sqlc.GetAuthoredSourceRow) (voice.AuthoredSource, error) {
	return authoredSource(r.ID, r.UserID, r.VoiceID, r.PostSlug, r.LearningEventID, r.Title, r.Tags, r.Body, r.Excerpt, r.EmbeddingRef, r.CreatedAt, r.SourceLanguage)
}
func authoredSource(id, userID, voiceID string, postSlug, learningEventID sql.NullString, title, rawTags, body, excerpt string, embeddingRef sql.NullString, rawCreated, rawLanguage string) (voice.AuthoredSource, error) {
	created, err := parseTime(rawCreated)
	if err != nil {
		return voice.AuthoredSource{}, err
	}
	var tags []string
	if err = json.Unmarshal([]byte(rawTags), &tags); err != nil {
		return voice.AuthoredSource{}, err
	}
	language, err := voice.ParseLanguage(rawLanguage)
	if err != nil {
		return voice.AuthoredSource{}, err
	}
	return voice.AuthoredSource{ID: id, UserID: userID, VoiceID: voiceID, PostSlug: postSlug.String, LearningEventID: learningEventID.String, Title: title, Tags: tags, Body: body, Excerpt: excerpt, EmbeddingRef: embeddingRef.String, CreatedAt: created, SourceLanguage: language}, nil
}

func (s *Store) ListRules(ctx context.Context, userID, voiceID string) ([]voice.ContrastRule, error) {
	rows, err := s.read.ListContrastRules(ctx, sqlc.ListContrastRulesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]voice.ContrastRule, 0, len(rows))
	for _, r := range rows {
		v, e := contrastRule(r)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

// GetRule is account-scoped and carries the rule's voice out: rule-derived operations take
// the voice from the row rather than from the caller.
func (s *Store) GetRule(ctx context.Context, userID, ruleID string) (voice.ContrastRule, error) {
	r, err := s.read.GetContrastRuleForUser(ctx, sqlc.GetContrastRuleForUserParams{ID: ruleID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.ContrastRule{}, voice.ErrRuleNotFound
	}
	if err != nil {
		return voice.ContrastRule{}, err
	}
	return contrastRule(r)
}
func contrastRule(r sqlc.VoiceContrastRule) (voice.ContrastRule, error) {
	created, err := parseTime(r.CreatedAt)
	if err != nil {
		return voice.ContrastRule{}, err
	}
	last, err := parseTime(r.LastEvidenceAt)
	if err != nil {
		return voice.ContrastRule{}, err
	}
	return voice.ContrastRule{ID: r.ID, UserID: r.UserID, VoiceID: r.VoiceID, Statement: r.Statement, CanonicalKey: r.CanonicalKey, Layer: voice.RuleLayer(r.Layer), EvidenceCount: int(r.EvidenceCount), Status: voice.RuleStatus(r.Status), Origin: r.Origin, CreatedAt: created, LastEvidenceAt: last}, nil
}
func (s *Store) SetRuleStatus(ctx context.Context, userID, voiceID, ruleID string, status voice.RuleStatus, now time.Time) error {
	return s.write.UpdateContrastRuleStatus(ctx, sqlc.UpdateContrastRuleStatusParams{Status: string(status), LastEvidenceAt: formatTime(now), ID: ruleID, VoiceID: voiceID, UserID: userID})
}
func (s *Store) RetireStaleRules(ctx context.Context, userID, voiceID string, before time.Time) (int, error) {
	n, err := s.write.RetireStaleRules(ctx, sqlc.RetireStaleRulesParams{VoiceID: voiceID, UserID: userID, LastEvidenceAt: formatTime(before)})
	return int(n), err
}
func (s *Store) ApplyRuleStatusAndPublish(ctx context.Context, userID, voiceID, ruleID string, status voice.RuleStatus, profile voice.StructuredProfile, now time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err = q.UpdateContrastRuleStatus(ctx, sqlc.UpdateContrastRuleStatusParams{Status: string(status), LastEvidenceAt: formatTime(now), ID: ruleID, VoiceID: voiceID, UserID: userID}); err != nil {
		return err
	}
	if _, err = publishProfileWithQueries(ctx, q, userID, voiceID, profile, "rule", 0, now); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) RetireStaleRulesAndPublish(ctx context.Context, userID, voiceID string, before, now time.Time) (int, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	n, err := q.RetireStaleRules(ctx, sqlc.RetireStaleRulesParams{VoiceID: voiceID, UserID: userID, LastEvidenceAt: formatTime(before)})
	if err != nil || n == 0 {
		if err == nil {
			err = tx.Commit()
		}
		return int(n), err
	}
	row, err := q.GetProfile(ctx, sqlc.GetProfileParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return 0, err
	}
	version, err := q.GetProfileVersion(ctx, sqlc.GetProfileVersionParams{VoiceID: voiceID, UserID: userID, Version: row.CurrentVersion})
	if err != nil {
		return 0, err
	}
	profile, err := decodeProfile(version.Snapshot)
	if err != nil {
		return 0, err
	}
	rules, err := q.ListContrastRules(ctx, sqlc.ListContrastRulesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return 0, err
	}
	profile.Rules = profile.Rules[:0]
	for _, item := range rules {
		mapped, mapErr := contrastRule(item)
		if mapErr != nil {
			return 0, mapErr
		}
		profile.Rules = append(profile.Rules, mapped)
	}
	if _, err = publishProfileWithQueries(ctx, q, userID, voiceID, profile, "rule", 0, now); err != nil {
		return 0, err
	}
	return int(n), tx.Commit()
}

func (s *Store) InsertFeedback(ctx context.Context, f voice.Feedback) error {
	return s.write.InsertSentenceFeedback(ctx, sqlc.InsertSentenceFeedbackParams{ID: f.ID, UserID: f.UserID, VoiceID: f.VoiceID, PostSlug: f.PostSlug, SentenceRef: f.SentenceRef, Kind: f.Kind, Reason: nullableString(f.Reason), PayloadRef: f.PayloadRef, ProcessingState: f.ProcessingState, CreatedAt: formatTime(f.CreatedAt)})
}

func (s *Store) ListConfirmations(ctx context.Context, userID, voiceID string) ([]voice.RuleConfirmation, error) {
	rows, err := s.read.ListRuleConfirmations(ctx, sqlc.ListRuleConfirmationsParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]voice.RuleConfirmation, 0, len(rows))
	for _, r := range rows {
		item, e := s.ruleConfirmation(ctx, r)
		if e != nil {
			return nil, e
		}
		out = append(out, item)
	}
	return out, nil
}
func (s *Store) GetConfirmation(ctx context.Context, userID, confirmationID string) (voice.RuleConfirmation, error) {
	r, err := s.read.GetRuleConfirmation(ctx, sqlc.GetRuleConfirmationParams{ID: confirmationID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.RuleConfirmation{}, voice.ErrConfirmationNotFound
	}
	if err != nil {
		return voice.RuleConfirmation{}, err
	}
	return s.ruleConfirmation(ctx, r)
}
func (s *Store) ruleConfirmation(ctx context.Context, r sqlc.VoiceRuleConfirmation) (voice.RuleConfirmation, error) {
	created, e := parseTime(r.CreatedAt)
	if e != nil {
		return voice.RuleConfirmation{}, e
	}
	var resolved *time.Time
	if r.ResolvedAt.Valid {
		v, e := parseTime(r.ResolvedAt.String)
		if e != nil {
			return voice.RuleConfirmation{}, e
		}
		resolved = &v
	}
	existing := ""
	if rule, e := s.read.GetContrastRule(ctx, sqlc.GetContrastRuleParams{ID: r.RuleID, VoiceID: r.VoiceID, UserID: r.UserID}); e == nil {
		existing = rule.Statement
	}
	return voice.RuleConfirmation{ID: r.ID, UserID: r.UserID, VoiceID: r.VoiceID, RuleID: r.RuleID, ExistingStatement: existing, ProposedStatement: r.ProposedStatement, EventID: r.EventID.String, Status: r.Status, CreatedAt: created, ResolvedAt: resolved}, nil
}

func (s *Store) ResolveConfirmation(ctx context.Context, userID, confirmationID string, replace bool, now time.Time) error {
	return s.resolveConfirmation(ctx, userID, confirmationID, replace, false, now)
}
func (s *Store) ResolveConfirmationAndPublish(ctx context.Context, userID, confirmationID string, replace bool, now time.Time) error {
	return s.resolveConfirmation(ctx, userID, confirmationID, replace, true, now)
}
func (s *Store) resolveConfirmation(ctx context.Context, userID, confirmationID string, replace, publish bool, now time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	found, err := q.GetRuleConfirmation(ctx, sqlc.GetRuleConfirmationParams{ID: confirmationID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.ErrConfirmationNotFound
	}
	if err != nil {
		return err
	}
	voiceID := found.VoiceID
	status := "keep"
	if replace {
		status = "replace"
		if err = q.ReplaceContrastRule(ctx, sqlc.ReplaceContrastRuleParams{Statement: found.ProposedStatement, CanonicalKey: canonicalRule(found.ProposedStatement), LastEvidenceAt: formatTime(now), ID: found.RuleID, VoiceID: voiceID, UserID: userID}); err != nil {
			return err
		}
	}
	n, err := q.ResolveRuleConfirmation(ctx, sqlc.ResolveRuleConfirmationParams{Status: status, ResolvedAt: nullableString(formatTime(now)), ID: confirmationID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return voice.ErrInvalidLifecycle
	}
	if publish {
		profile, profileErr := currentProfileWithRules(ctx, q, userID, voiceID)
		if profileErr != nil {
			return profileErr
		}
		if _, err = publishProfileWithQueries(ctx, q, userID, voiceID, profile, "confirmation", 0, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func currentProfileWithRules(ctx context.Context, q *sqlc.Queries, userID, voiceID string) (voice.StructuredProfile, error) {
	var profile voice.StructuredProfile
	head, err := q.GetProfile(ctx, sqlc.GetProfileParams{VoiceID: voiceID, UserID: userID})
	if err == nil && head.CurrentVersion > 0 {
		version, versionErr := q.GetProfileVersion(ctx, sqlc.GetProfileVersionParams{VoiceID: voiceID, UserID: userID, Version: head.CurrentVersion})
		if versionErr != nil {
			return voice.StructuredProfile{}, versionErr
		}
		profile, err = decodeProfile(version.Snapshot)
		if err != nil {
			return voice.StructuredProfile{}, err
		}
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return voice.StructuredProfile{}, err
	}
	rules, err := q.ListContrastRules(ctx, sqlc.ListContrastRulesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return voice.StructuredProfile{}, err
	}
	profile.Rules = make([]voice.ContrastRule, 0, len(rules))
	for _, row := range rules {
		rule, mapErr := contrastRule(row)
		if mapErr != nil {
			return voice.StructuredProfile{}, mapErr
		}
		profile.Rules = append(profile.Rules, rule)
	}
	return profile, nil
}
func (s *Store) ListFeedback(ctx context.Context, userID, voiceID string) ([]voice.Feedback, error) {
	rows, err := s.read.ListSentenceFeedback(ctx, sqlc.ListSentenceFeedbackParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]voice.Feedback, 0, len(rows))
	for _, r := range rows {
		at, e := parseTime(r.CreatedAt)
		if e != nil {
			return nil, e
		}
		out = append(out, voice.Feedback{ID: r.ID, UserID: r.UserID, VoiceID: r.VoiceID, PostSlug: r.PostSlug, SentenceRef: r.SentenceRef, Kind: r.Kind, Reason: r.Reason.String, PayloadRef: r.PayloadRef, ProcessingState: r.ProcessingState, CreatedAt: at})
	}
	return out, nil
}

func canonicalRule(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, value)
}
func negative(value string) bool {
	return strings.Contains(value, "않") || strings.Contains(value, "말") || strings.Contains(value, "금지")
}
func ruleContradiction(a, b string) bool {
	strip := func(v string) string {
		v = canonicalRule(v)
		for _, n := range []string{"않", "말", "금지"} {
			v = strings.ReplaceAll(v, n, "")
		}
		return v
	}
	return negative(a) != negative(b) && strip(a) == strip(b)
}

// ApplyLearningResult publishes source, evidence/rules, immutable profile version and
// event completion in one short transaction after all provider calls have returned. Every
// row lands in the event's frozen voice.
func (s *Store) ApplyLearningResult(ctx context.Context, event voice.LearningEvent, result voice.LearningResult, cfg voice.PersonalizationConfig, now time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	row, err := q.GetLearningEvent(ctx, sqlc.GetLearningEventParams{ID: event.ID, UserID: event.UserID})
	if err != nil {
		return err
	}
	if row.Status == "done" {
		return tx.Commit()
	}
	voiceID := row.VoiceID
	tags, _ := json.Marshal(result.Source.Tags)
	err = q.InsertAuthoredSource(ctx, sqlc.InsertAuthoredSourceParams{ID: result.Source.ID, UserID: event.UserID, VoiceID: voiceID, PostSlug: nullableString(event.PostSlug), LearningEventID: nullableString(event.ID), Title: result.Source.Title, Tags: string(tags), Body: result.Source.Body, Excerpt: result.Source.Excerpt, EmbeddingRef: nullableString(result.Source.EmbeddingRef), CreatedAt: formatTime(now)})
	if err != nil && !isUniqueViolation(err) {
		return err
	}
	if err == nil {
		if err = q.BumpCorpusVersion(ctx, sqlc.BumpCorpusVersionParams{VoiceID: voiceID, UserID: event.UserID, UpdatedAt: formatTime(now)}); err != nil {
			return err
		}
	}
	existing, err := q.ListContrastRules(ctx, sqlc.ListContrastRulesParams{VoiceID: voiceID, UserID: event.UserID})
	if err != nil {
		return err
	}
	// A contradiction is a confirmation boundary for the whole learning event. The
	// authored source is retained, but no evidence, rule, or profile head changes until
	// the owner explicitly resolves the pending item.
	var contradictions []struct {
		rule      sqlc.VoiceContrastRule
		candidate voice.ExtractedRule
	}
	for _, candidate := range result.Rules {
		for _, rule := range existing {
			annotated := candidate.ContradictsRuleID != "" && rule.ID == candidate.ContradictsRuleID
			direct := rule.Layer == string(candidate.Layer) && ruleContradiction(rule.Statement, candidate.Statement)
			if rule.Status == "active" && (annotated || direct) {
				contradictions = append(contradictions, struct {
					rule      sqlc.VoiceContrastRule
					candidate voice.ExtractedRule
				}{rule: rule, candidate: candidate})
				break
			}
		}
	}
	if len(contradictions) > 0 {
		for _, item := range contradictions {
			if err = q.InsertRuleConfirmation(ctx, sqlc.InsertRuleConfirmationParams{ID: storeID(), UserID: event.UserID, VoiceID: voiceID, RuleID: item.rule.ID, ProposedStatement: item.candidate.Statement, EventID: nullableString(event.ID), CreatedAt: formatTime(now)}); err != nil && !isUniqueViolation(err) {
				return err
			}
		}
		if err = q.SetLearningEventStatus(ctx, sqlc.SetLearningEventStatusParams{Status: "done", ProcessedAt: nullableString(formatTime(now)), ID: event.ID, UserID: event.UserID}); err != nil {
			return err
		}
		return tx.Commit()
	}
	seenCandidates := make(map[string]struct{}, len(result.Rules))
	seenEvidenceRules := make(map[string]struct{}, len(result.Rules))
	for _, candidate := range result.Rules {
		key := canonicalRule(candidate.Statement)
		if _, duplicate := seenCandidates[key]; duplicate {
			continue
		}
		seenCandidates[key] = struct{}{}
		var matched *sqlc.VoiceContrastRule
		var contradiction *sqlc.VoiceContrastRule
		for i := range existing {
			if candidate.MatchRuleID != "" && existing[i].ID == candidate.MatchRuleID && existing[i].Layer == string(candidate.Layer) {
				matched = &existing[i]
				break
			}
			if candidate.ContradictsRuleID != "" && existing[i].ID == candidate.ContradictsRuleID && existing[i].Layer == string(candidate.Layer) && existing[i].Status == "active" {
				contradiction = &existing[i]
				break
			}
			if existing[i].CanonicalKey == key {
				matched = &existing[i]
				break
			}
			if existing[i].Status == "active" && existing[i].Layer == string(candidate.Layer) && ruleContradiction(existing[i].Statement, candidate.Statement) {
				contradiction = &existing[i]
			}
		}
		if contradiction != nil {
			if err = q.InsertRuleConfirmation(ctx, sqlc.InsertRuleConfirmationParams{ID: storeID(), UserID: event.UserID, VoiceID: voiceID, RuleID: contradiction.ID, ProposedStatement: candidate.Statement, EventID: nullableString(event.ID), CreatedAt: formatTime(now)}); err != nil && !isUniqueViolation(err) {
				return err
			}
			continue
		}
		if matched == nil {
			id := storeID()
			if err = q.InsertContrastRule(ctx, sqlc.InsertContrastRuleParams{ID: id, UserID: event.UserID, VoiceID: voiceID, Statement: candidate.Statement, CanonicalKey: key, Layer: string(candidate.Layer), EvidenceCount: 0, Status: string(voice.RuleCandidate), Origin: "diff", CreatedAt: formatTime(now), LastEvidenceAt: formatTime(now)}); err != nil {
				return err
			}
			existing = append(existing, sqlc.VoiceContrastRule{ID: id, UserID: event.UserID, VoiceID: voiceID, Statement: candidate.Statement, CanonicalKey: key, Layer: string(candidate.Layer), EvidenceCount: 0, Status: string(voice.RuleCandidate), Origin: "diff", CreatedAt: formatTime(now), LastEvidenceAt: formatTime(now)})
			matched = &existing[len(existing)-1]
		}
		if _, duplicate := seenEvidenceRules[matched.ID]; duplicate {
			continue
		}
		seenEvidenceRules[matched.ID] = struct{}{}
		if err = q.InsertRuleEvidence(ctx, sqlc.InsertRuleEvidenceParams{ID: storeID(), UserID: event.UserID, VoiceID: voiceID, RuleID: matched.ID, EventID: nullableString(event.ID), Origin: "diff", PayloadRef: event.ID, CreatedAt: formatTime(now)}); isUniqueViolation(err) {
			continue
		} else if err != nil {
			return err
		}
		count := matched.EvidenceCount + 1
		status := matched.Status
		if int(count) >= cfg.RuleActivationEvidence {
			status = string(voice.RuleActive)
		}
		if err = q.UpdateContrastRuleEvidence(ctx, sqlc.UpdateContrastRuleEvidenceParams{EvidenceCount: count, Status: status, LastEvidenceAt: formatTime(now), ID: matched.ID, VoiceID: voiceID, UserID: event.UserID}); err != nil {
			return err
		}
		matched.EvidenceCount = count
		matched.Status = status
	}
	// The immutable snapshot is the whole effective aggregate, including the
	// rule state produced by this same learning event.
	latestRules, err := q.ListContrastRules(ctx, sqlc.ListContrastRulesParams{VoiceID: voiceID, UserID: event.UserID})
	if err != nil {
		return err
	}
	result.Profile.Rules = make([]voice.ContrastRule, 0, len(latestRules))
	for _, row := range latestRules {
		rule, mapErr := contrastRule(row)
		if mapErr != nil {
			return mapErr
		}
		result.Profile.Rules = append(result.Profile.Rules, rule)
	}
	current := int64(0)
	profileRow, err := q.GetProfile(ctx, sqlc.GetProfileParams{VoiceID: voiceID, UserID: event.UserID})
	if err == nil {
		current = profileRow.CurrentVersion
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	result.Profile.Version = current + 1
	result.Profile.UpdatedAt = now
	result.Profile.Empty = false
	encoded, err := encodeProfile(result.Profile)
	if err != nil {
		return err
	}
	if err = q.InsertProfileVersion(ctx, sqlc.InsertProfileVersionParams{ID: storeID(), UserID: event.UserID, VoiceID: voiceID, Version: result.Profile.Version, Snapshot: encoded, Origin: "analysis", CreatedAt: formatTime(now)}); err != nil {
		return err
	}
	if err = q.SetProfileHead(ctx, sqlc.SetProfileHeadParams{VoiceID: voiceID, UserID: event.UserID, CurrentVersion: result.Profile.Version, UpdatedAt: formatTime(now)}); err != nil {
		return err
	}
	if err = q.SetLearningEventStatus(ctx, sqlc.SetLearningEventStatusParams{Status: "done", ProcessedAt: nullableString(formatTime(now)), ID: event.ID, UserID: event.UserID}); err != nil {
		return err
	}
	return tx.Commit()
}

func storeID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}

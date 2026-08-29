package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

func (s *Store) InsertRuleComparison(ctx context.Context, c voice.RuleComparison) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err = q.InsertRuleComparison(ctx, sqlc.InsertRuleComparisonParams{ID: c.ID, UserID: c.UserID, RuleID: c.RuleID, SourceID: c.SourceID, ProfileVersion: c.ProfileVersion, ModelRef: c.ModelRef, TargetLength: int64(c.TargetLength), InputSnapshot: c.InputSnapshot, RuleOnSide: c.RuleOnSide, JobID: nullableString(c.JobID), CreatedAt: formatTime(c.CreatedAt)}); err != nil {
		return err
	}
	for _, candidate := range c.Candidates {
		if err = q.InsertRuleComparisonCandidate(ctx, sqlc.InsertRuleComparisonCandidateParams{ID: candidate.ID, ComparisonID: c.ID, DisplaySide: candidate.DisplaySide}); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) SetRuleComparisonJob(ctx context.Context, userID, comparisonID, jobID string) error {
	return s.write.SetRuleComparisonJob(ctx, sqlc.SetRuleComparisonJobParams{JobID: nullableString(jobID), ID: comparisonID, UserID: userID})
}
func (s *Store) GetRuleComparison(ctx context.Context, userID, comparisonID string) (voice.RuleComparison, error) {
	row, err := s.read.GetRuleComparison(ctx, sqlc.GetRuleComparisonParams{ID: comparisonID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.RuleComparison{}, voice.ErrComparisonNotFound
	}
	if err != nil {
		return voice.RuleComparison{}, err
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return voice.RuleComparison{}, err
	}
	var decided *time.Time
	if row.DecidedAt.Valid {
		v, e := parseTime(row.DecidedAt.String)
		if e != nil {
			return voice.RuleComparison{}, e
		}
		decided = &v
	}
	c := voice.RuleComparison{ID: row.ID, UserID: row.UserID, RuleID: row.RuleID, SourceID: row.SourceID, ProfileVersion: row.ProfileVersion, ModelRef: row.ModelRef, TargetLength: int(row.TargetLength), InputSnapshot: row.InputSnapshot, RuleOnSide: row.RuleOnSide, Status: row.Status, JobID: row.JobID.String, ChosenSide: row.ChosenSide.String, CreatedAt: created, DecidedAt: decided}
	rows, err := s.read.ListRuleComparisonCandidates(ctx, comparisonID)
	if err != nil {
		return voice.RuleComparison{}, err
	}
	for _, v := range rows {
		c.Candidates = append(c.Candidates, voice.ComparisonCandidate{ID: v.ID, ComparisonID: v.ComparisonID, DisplaySide: v.DisplaySide, Output: v.Output.String, Status: v.Status, Error: v.Error.String})
	}
	return c, nil
}
func (s *Store) UpdateRuleComparison(ctx context.Context, c voice.RuleComparison) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	for _, v := range c.Candidates {
		if err = q.UpdateRuleComparisonCandidate(ctx, sqlc.UpdateRuleComparisonCandidateParams{Output: nullableString(v.Output), Status: v.Status, Error: nullableString(v.Error), ID: v.ID, ComparisonID: c.ID}); err != nil {
			return err
		}
	}
	if c.ChosenSide != "" {
		if c.DecidedAt == nil {
			return voice.ErrInvalidLifecycle
		}
		n, err := q.DecideRuleComparison(ctx, sqlc.DecideRuleComparisonParams{ChosenSide: nullableString(c.ChosenSide), DecidedAt: nullableString(formatTime(*c.DecidedAt)), ID: c.ID, UserID: c.UserID})
		if err != nil {
			return err
		}
		if n == 0 {
			return voice.ErrInvalidLifecycle
		}
		rule, err := q.GetContrastRule(ctx, sqlc.GetContrastRuleParams{ID: c.RuleID, UserID: c.UserID})
		if err != nil {
			return err
		}
		if c.ChosenSide == c.RuleOnSide {
			count := rule.EvidenceCount + 1
			status := rule.Status
			threshold := int64(c.ActivationEvidence)
			if threshold <= 0 {
				return voice.ErrInvalidLifecycle
			}
			if count >= threshold {
				status = "active"
			}
			if err = q.UpdateContrastRuleEvidence(ctx, sqlc.UpdateContrastRuleEvidenceParams{EvidenceCount: count, Status: status, LastEvidenceAt: formatTime(*c.DecidedAt), ID: rule.ID, UserID: c.UserID}); err != nil {
				return err
			}
			if err = q.InsertRuleEvidence(ctx, sqlc.InsertRuleEvidenceParams{ID: storeID(), UserID: c.UserID, RuleID: rule.ID, Origin: "ab_test", PayloadRef: c.ID, CreatedAt: formatTime(*c.DecidedAt)}); err != nil {
				return err
			}
		} else {
			if err = q.UpdateContrastRuleStatus(ctx, sqlc.UpdateContrastRuleStatusParams{Status: "rejected", LastEvidenceAt: formatTime(*c.DecidedAt), ID: rule.ID, UserID: c.UserID}); err != nil {
				return err
			}
		}
		if c.ProfileAfterDecision == nil {
			return voice.ErrInvalidLifecycle
		}
		if _, err = publishProfileWithQueries(ctx, q, c.UserID, *c.ProfileAfterDecision, "rule", 0, *c.DecidedAt); err != nil {
			return err
		}
	} else {
		if err = q.UpdateRuleComparisonStatus(ctx, sqlc.UpdateRuleComparisonStatusParams{Status: c.Status, ID: c.ID, UserID: c.UserID}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) InsertProfileValidation(ctx context.Context, v voice.ProfileValidation) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	judge := int64(0)
	if v.JudgeEnabled {
		judge = 1
	}
	if err = q.InsertProfileValidation(ctx, sqlc.InsertProfileValidationParams{ID: v.ID, UserID: v.UserID, ProfileVersion: v.ProfileVersion, AnalyzeModelRef: v.AnalyzeModelRef, WriteModelRef: v.WriteModelRef, JudgeEnabled: judge, JobID: nullableString(v.JobID), CreatedAt: formatTime(v.CreatedAt)}); err != nil {
		return err
	}
	for _, item := range v.Items {
		if err = q.InsertProfileValidationItem(ctx, sqlc.InsertProfileValidationItemParams{ID: item.ID, ValidationID: v.ID, SourceID: item.SourceID, Position: int64(item.Position)}); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) SetProfileValidationJob(ctx context.Context, userID, validationID, jobID string) error {
	return s.write.SetProfileValidationJob(ctx, sqlc.SetProfileValidationJobParams{JobID: nullableString(jobID), ID: validationID, UserID: userID})
}
func (s *Store) GetProfileValidation(ctx context.Context, userID, validationID string) (voice.ProfileValidation, error) {
	row, err := s.read.GetProfileValidation(ctx, sqlc.GetProfileValidationParams{ID: validationID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.ProfileValidation{}, voice.ErrValidationNotFound
	}
	if err != nil {
		return voice.ProfileValidation{}, err
	}
	return s.profileValidation(ctx, row)
}
func (s *Store) ListProfileValidations(ctx context.Context, userID string) ([]voice.ProfileValidation, error) {
	rows, err := s.read.ListProfileValidations(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]voice.ProfileValidation, 0, len(rows))
	for _, row := range rows {
		v, e := s.profileValidation(ctx, row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) profileValidation(ctx context.Context, row sqlc.VoiceProfileValidation) (voice.ProfileValidation, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return voice.ProfileValidation{}, err
	}
	var finished *time.Time
	if row.FinishedAt.Valid {
		v, e := parseTime(row.FinishedAt.String)
		if e != nil {
			return voice.ProfileValidation{}, e
		}
		finished = &v
	}
	out := voice.ProfileValidation{ID: row.ID, UserID: row.UserID, ProfileVersion: row.ProfileVersion, AnalyzeModelRef: row.AnalyzeModelRef, WriteModelRef: row.WriteModelRef, JudgeEnabled: row.JudgeEnabled == 1, Status: row.Status, JobID: row.JobID.String, YCount: int(row.YCount.Int64), TotalCount: int(row.TotalCount.Int64), CreatedAt: created, FinishedAt: finished}
	rows, err := s.read.ListProfileValidationItems(ctx, row.ID)
	if err != nil {
		return voice.ProfileValidation{}, err
	}
	for _, item := range rows {
		source, sourceErr := s.read.GetAuthoredSource(ctx, sqlc.GetAuthoredSourceParams{ID: item.SourceID, UserID: row.UserID})
		original := ""
		if sourceErr == nil {
			original = source.Body
		}
		out.Items = append(out.Items, voice.ValidationItem{ID: item.ID, ValidationID: item.ValidationID, SourceID: item.SourceID, Position: int(item.Position), Original: original, NeutralSummary: item.NeutralSummary.String, Regenerated: item.RegeneratedContent.String, ScoresJSON: item.Scores.String, Status: item.Status, Error: item.Error.String})
	}
	return out, nil
}
func (s *Store) UpdateProfileValidation(ctx context.Context, v voice.ProfileValidation) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	for _, item := range v.Items {
		if err = q.UpdateProfileValidationItem(ctx, sqlc.UpdateProfileValidationItemParams{NeutralSummary: nullableString(item.NeutralSummary), RegeneratedContent: nullableString(item.Regenerated), Scores: nullableString(item.ScoresJSON), Status: item.Status, Error: nullableString(item.Error), ID: item.ID, ValidationID: v.ID}); err != nil {
			return err
		}
	}
	if err = q.FinishProfileValidation(ctx, sqlc.FinishProfileValidationParams{Status: v.Status, YCount: sql.NullInt64{Int64: int64(v.YCount), Valid: v.TotalCount > 0}, TotalCount: sql.NullInt64{Int64: int64(v.TotalCount), Valid: v.TotalCount > 0}, FinishedAt: nullableTime(v.FinishedAt), ID: v.ID, UserID: v.UserID}); err != nil {
		return err
	}
	return tx.Commit()
}

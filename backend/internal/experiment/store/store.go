// Package store maps SQLite rows to the pure experiment aggregate.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/experiment/store/sqlc"
)

const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

func (s *Store) Create(ctx context.Context, found experiment.Experiment) error {
	targetLanguage, err := targetLanguageForStage(found.Stage, found.TargetLanguage)
	if err != nil {
		return err
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin experiment create: %w", err)
	}
	defer tx.Rollback()
	queries := sqlc.New(tx)
	err = queries.InsertExperiment(ctx, sqlc.InsertExperimentParams{
		ID: found.ID, UserID: found.UserID, PostSlug: nullString(found.PostSlug), VoiceID: nullString(found.VoiceID),
		PurposeName: found.PurposeName, TargetLanguage: targetLanguage,
		Stage: string(found.Stage), Status: string(found.Status), JobID: nullString(found.JobID),
		InputSnapshot: nullBytes(found.InputSnapshot), InputHash: found.InputHash,
		PromptVersion: found.PromptVersion, CreatedAt: formatTime(found.CreatedAt),
	})
	if err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "unique constraint failed") {
			return experiment.ErrInvalidState
		}
		if strings.Contains(message, "experiment voice must be active") {
			return experiment.ErrVoiceUnavailable
		}
		return fmt.Errorf("insert experiment: %w", err)
	}
	for _, candidate := range found.Candidates {
		if err := queries.InsertCandidate(ctx, sqlc.InsertCandidateParams{
			ID: candidate.ID, ExperimentID: found.ID, ModelProviderID: candidate.Model.ProviderID,
			ModelID: candidate.Model.ModelID, ModelLabel: candidate.ModelLabel,
			DisplaySide: string(candidate.DisplaySide), Status: string(candidate.Status),
		}); err != nil {
			return fmt.Errorf("insert candidate: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit experiment create: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	return s.write.DeleteExperiment(ctx, id)
}

func (s *Store) Get(ctx context.Context, id string) (experiment.Experiment, error) {
	row, err := s.read.GetExperiment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return experiment.Experiment{}, experiment.ErrNotFound
	}
	if err != nil {
		return experiment.Experiment{}, fmt.Errorf("get experiment: %w", err)
	}
	return s.withCandidates(ctx, row)
}

func (s *Store) List(ctx context.Context, userID string, stage experiment.Stage) ([]experiment.Experiment, error) {
	rows, err := s.read.ListExperimentsForUser(ctx, sqlc.ListExperimentsForUserParams{UserID: userID, Column2: string(stage), Stage: string(stage)})
	if err != nil {
		return nil, fmt.Errorf("list experiments: %w", err)
	}
	out := make([]experiment.Experiment, 0, len(rows))
	for _, row := range rows {
		found, err := s.withCandidates(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, found)
	}
	return out, nil
}

func (s *Store) PendingForPost(ctx context.Context, userID, postSlug string) (*experiment.Experiment, error) {
	row, err := s.read.PendingWriteForPost(ctx, sqlc.PendingWriteForPostParams{UserID: userID, PostSlug: nullString(postSlug)})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pending experiment: %w", err)
	}
	found, err := s.withCandidates(ctx, row)
	return &found, err
}

func (s *Store) SetJob(ctx context.Context, id, userID, jobID string) error {
	return s.write.SetExperimentJob(ctx, sqlc.SetExperimentJobParams{JobID: nullString(jobID), ID: id, UserID: userID})
}

func (s *Store) SetSnapshot(ctx context.Context, id string, snapshot experiment.Snapshot, hash string) error {
	return s.write.SetExperimentSnapshot(ctx, sqlc.SetExperimentSnapshotParams{
		InputSnapshot: nullBytes(snapshot.Content), InputHash: hash, PromptVersion: snapshot.PromptVersion, ID: id,
	})
}

func (s *Store) SetStatus(ctx context.Context, id string, status experiment.Status, finishedAt *time.Time) error {
	return s.write.SetExperimentStatus(ctx, sqlc.SetExperimentStatusParams{
		Status: string(status), FinishedAt: nullTime(finishedAt), ID: id,
	})
}

func (s *Store) StartCandidate(ctx context.Context, experimentID, candidateID string, now time.Time) error {
	count, err := s.write.StartCandidate(ctx, sqlc.StartCandidateParams{StartedAt: nullTime(&now), ID: candidateID, ExperimentID: experimentID})
	if err != nil {
		return err
	}
	if count != 1 {
		return experiment.ErrInvalidState
	}
	return nil
}

func (s *Store) CompleteCandidate(ctx context.Context, candidate experiment.Candidate) error {
	if candidate.Status == experiment.CandidateFailed && candidate.Failure == nil {
		return fmt.Errorf("complete candidate: failed candidate requires failure")
	}
	if candidate.Status != experiment.CandidateFailed && candidate.Failure != nil {
		return fmt.Errorf("complete candidate: non-failed candidate cannot carry failure")
	}
	reason, params, detail, err := failureColumns(candidate.Failure)
	if err != nil {
		return fmt.Errorf("complete candidate failure: %w", err)
	}
	usageKnown := candidate.Usage.CostSource != experiment.CostUnavailable && candidate.Usage.CostSource != ""
	count, err := s.write.CompleteCandidate(ctx, sqlc.CompleteCandidateParams{
		Status: string(candidate.Status), Output: nullBytes(candidate.Output), ErrorReason: reason,
		ErrorParams: params, TechnicalDetail: detail,
		PromptTokens:     sql.NullInt64{Int64: candidate.Usage.PromptTokens, Valid: candidate.Usage.PromptTokens > 0},
		CompletionTokens: sql.NullInt64{Int64: candidate.Usage.CompletionTokens, Valid: candidate.Usage.CompletionTokens > 0},
		CostMicrousd:     sql.NullInt64{Int64: candidate.Usage.CostMicrousd, Valid: usageKnown},
		CostSource:       nullString(string(candidate.Usage.CostSource)),
		LatencyMs:        sql.NullInt64{Int64: candidate.Usage.LatencyMS, Valid: candidate.Usage.LatencyMS >= 0},
		FinishedAt:       nullTime(candidate.FinishedAt), ID: candidate.ID, ExperimentID: candidate.ExperimentID,
	})
	if err != nil {
		return err
	}
	if count != 1 {
		return experiment.ErrInvalidState
	}
	return nil
}

func (s *Store) FailUnfinished(ctx context.Context, experimentID string, failure experiment.Failure, now time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin interrupted experiment recovery: %w", err)
	}
	defer tx.Rollback()
	if err := failUnfinished(ctx, sqlc.New(tx), experimentID, failure, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit interrupted experiment recovery: %w", err)
	}
	return nil
}

func (s *Store) RecoverInterrupted(ctx context.Context, failure experiment.Failure, now time.Time) (int64, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin interrupted experiment sweep: %w", err)
	}
	defer tx.Rollback()
	queries := sqlc.New(tx)
	ids, err := queries.ListInterruptedExperimentIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list interrupted experiments: %w", err)
	}
	for _, id := range ids {
		if err := failUnfinished(ctx, queries, id, failure, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit interrupted experiment sweep: %w", err)
	}
	return int64(len(ids)), nil
}

func (s *Store) ListQueued(ctx context.Context) ([]string, error) {
	ids, err := s.read.ListQueuedExperimentIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list queued experiments: %w", err)
	}
	return ids, nil
}

func failUnfinished(ctx context.Context, queries *sqlc.Queries, experimentID string, failure experiment.Failure, now time.Time) error {
	reason, params, detail, err := failureColumns(&failure)
	if err != nil {
		return fmt.Errorf("fail unfinished candidate failure: %w", err)
	}
	if _, err := queries.FailUnfinishedCandidates(ctx, sqlc.FailUnfinishedCandidatesParams{
		ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail,
		FinishedAt: nullTime(&now), ExperimentID: experimentID,
	}); err != nil {
		return fmt.Errorf("fail unfinished candidates: %w", err)
	}
	count, err := queries.FinishInterruptedExperiment(ctx, sqlc.FinishInterruptedExperimentParams{
		FinishedAt: nullTime(&now), ID: experimentID,
	})
	if err != nil {
		return fmt.Errorf("finish interrupted experiment: %w", err)
	}
	if count != 1 {
		return experiment.ErrInvalidState
	}
	return nil
}

func (s *Store) ResetFailedCandidates(ctx context.Context, experimentID string) (int64, error) {
	return s.write.ResetFailedCandidates(ctx, experimentID)
}

func (s *Store) RestoreFailedCandidates(ctx context.Context, experimentID string, candidates []experiment.Candidate) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin candidate retry rollback: %w", err)
	}
	defer tx.Rollback()
	queries := sqlc.New(tx)
	for _, candidate := range candidates {
		if candidate.Status != experiment.CandidateFailed {
			continue
		}
		if candidate.Failure == nil {
			return fmt.Errorf("restore failed candidate: failure is required")
		}
		reason, params, detail, err := failureColumns(candidate.Failure)
		if err != nil {
			return fmt.Errorf("restore failed candidate failure: %w", err)
		}
		count, err := queries.RestoreFailedCandidate(ctx, sqlc.RestoreFailedCandidateParams{
			ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail, StartedAt: nullTime(candidate.StartedAt),
			FinishedAt: nullTime(candidate.FinishedAt), ExperimentID: experimentID, ID: candidate.ID,
		})
		if err != nil {
			return fmt.Errorf("restore failed candidate: %w", err)
		}
		if count != 1 {
			return experiment.ErrInvalidState
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit candidate retry rollback: %w", err)
	}
	return nil
}

func (s *Store) Decide(ctx context.Context, id, userID, candidateID string, status experiment.Status, outcome experiment.Outcome, adoptionRequested bool, decidedAt, expiresAt time.Time) (bool, error) {
	count, err := s.write.DecideExperiment(ctx, sqlc.DecideExperimentParams{
		Status: string(status), WinnerCandidateID: nullString(candidateID), Outcome: nullString(string(outcome)),
		DecidedAt: nullTime(&decidedAt), ContentExpiresAt: nullTime(&expiresAt),
		AdoptionRequested: boolValue(adoptionRequested), ID: id, UserID: userID,
	})
	return count == 1, err
}

func (s *Store) SetApplyFailure(ctx context.Context, id, userID string, failure experiment.Failure) error {
	reason, params, detail, err := failureColumns(&failure)
	if err != nil {
		return fmt.Errorf("set apply failure: %w", err)
	}
	return s.write.SetApplyFailure(ctx, sqlc.SetApplyFailureParams{
		ApplyErrorReason: reason, ApplyErrorParams: params, ApplyTechnicalDetail: detail, ID: id, UserID: userID,
	})
}

func (s *Store) SetApplied(ctx context.Context, id, userID string, now time.Time) error {
	count, err := s.write.SetExperimentApplied(ctx, sqlc.SetExperimentAppliedParams{
		AppliedAt: nullTime(&now), ID: id, UserID: userID,
	})
	if err != nil {
		return err
	}
	if count != 1 {
		return experiment.ErrInvalidState
	}
	return nil
}

func (s *Store) SetAdoptionFailure(ctx context.Context, id, userID string, failure experiment.Failure) error {
	reason, params, detail, err := failureColumns(&failure)
	if err != nil {
		return fmt.Errorf("set adoption failure: %w", err)
	}
	return s.write.SetAdoptionFailure(ctx, sqlc.SetAdoptionFailureParams{
		AdoptionErrorReason: reason, AdoptionErrorParams: params, AdoptionTechnicalDetail: detail, ID: id, UserID: userID,
	})
}

func (s *Store) SetAdopted(ctx context.Context, id, userID string, now time.Time) error {
	count, err := s.write.SetExperimentAdopted(ctx, sqlc.SetExperimentAdoptedParams{
		AdoptedAt: nullTime(&now), ID: id, UserID: userID,
	})
	if err != nil {
		return err
	}
	if count != 1 {
		return experiment.ErrInvalidState
	}
	return nil
}

func (s *Store) LeaderboardData(ctx context.Context, userID string, stage experiment.Stage) ([]experiment.Experiment, []experiment.Candidate, error) {
	experimentRows, err := s.read.ListDecidedForLeaderboard(ctx, sqlc.ListDecidedForLeaderboardParams{UserID: userID, Stage: string(stage)})
	if err != nil {
		return nil, nil, fmt.Errorf("list decided experiments: %w", err)
	}
	found := make([]experiment.Experiment, 0, len(experimentRows))
	for _, row := range experimentRows {
		mapped, err := toExperiment(row)
		if err != nil {
			return nil, nil, err
		}
		found = append(found, mapped)
	}
	candidateRows, err := s.read.ListCandidatesForLeaderboard(ctx, sqlc.ListCandidatesForLeaderboardParams{UserID: userID, Stage: string(stage)})
	if err != nil {
		return nil, nil, fmt.Errorf("list leaderboard candidates: %w", err)
	}
	candidates := make([]experiment.Candidate, 0, len(candidateRows))
	for _, row := range candidateRows {
		mapped, err := toCandidate(row)
		if err != nil {
			return nil, nil, err
		}
		candidates = append(candidates, mapped)
	}
	return found, candidates, nil
}

func (s *Store) PurgeExpired(ctx context.Context, before time.Time) (int64, error) {
	value := nullTime(&before)
	if _, err := s.write.PurgeExpiredCandidateOutput(ctx, value); err != nil {
		return 0, fmt.Errorf("purge candidate output: %w", err)
	}
	count, err := s.write.PurgeExpiredContent(ctx, value)
	if err != nil {
		return 0, fmt.Errorf("purge experiment input: %w", err)
	}
	return count, nil
}

func (s *Store) CountPublishableForVoice(ctx context.Context, userID, voiceID string) (int, error) {
	n, err := s.read.CountPublishableForVoice(ctx, sqlc.CountPublishableForVoiceParams{VoiceID: nullString(voiceID), UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("count publishable experiments for voice: %w", err)
	}
	return int(n), nil
}

func (s *Store) PurgePost(ctx context.Context, userID, postSlug string) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin post experiment purge: %w", err)
	}
	defer tx.Rollback()
	queries := sqlc.New(tx)
	params := sqlc.PurgePostCandidateOutputParams{UserID: userID, PostSlug: nullString(postSlug)}
	if err := queries.PurgePostCandidateOutput(ctx, params); err != nil {
		return err
	}
	if err := queries.PurgePostContent(ctx, sqlc.PurgePostContentParams{UserID: userID, PostSlug: nullString(postSlug)}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) withCandidates(ctx context.Context, row sqlc.ModelExperiment) (experiment.Experiment, error) {
	found, err := toExperiment(row)
	if err != nil {
		return experiment.Experiment{}, err
	}
	rows, err := s.read.ListCandidates(ctx, row.ID)
	if err != nil {
		return experiment.Experiment{}, fmt.Errorf("list candidates: %w", err)
	}
	for _, row := range rows {
		candidate, err := toCandidate(row)
		if err != nil {
			return experiment.Experiment{}, err
		}
		found.Candidates = append(found.Candidates, candidate)
	}
	return found, nil
}

func toExperiment(row sqlc.ModelExperiment) (experiment.Experiment, error) {
	created, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return experiment.Experiment{}, fmt.Errorf("experiment %s created_at: %w", row.ID, err)
	}
	targetLanguage, err := languageFromRow(row.Stage, row.TargetLanguage)
	if err != nil {
		return experiment.Experiment{}, fmt.Errorf("experiment %s target_language: %w", row.ID, err)
	}
	applyFailure, err := failureFromRow(row.ApplyErrorReason, row.ApplyErrorParams, row.ApplyTechnicalDetail, row.ApplyError)
	if err != nil {
		return experiment.Experiment{}, fmt.Errorf("experiment %s apply failure: %w", row.ID, err)
	}
	adoptionFailure, err := failureFromRow(row.AdoptionErrorReason, row.AdoptionErrorParams, row.AdoptionTechnicalDetail, row.AdoptionError)
	if err != nil {
		return experiment.Experiment{}, fmt.Errorf("experiment %s adoption failure: %w", row.ID, err)
	}
	return experiment.Experiment{
		ID: row.ID, UserID: row.UserID, PostSlug: row.PostSlug.String, VoiceID: row.VoiceID.String,
		PurposeName: row.PurposeName, TargetLanguage: targetLanguage, Stage: experiment.Stage(row.Stage),
		Status: experiment.Status(row.Status), JobID: row.JobID.String, InputSnapshot: []byte(row.InputSnapshot.String),
		InputHash: row.InputHash, PromptVersion: row.PromptVersion, WinnerCandidateID: row.WinnerCandidateID.String,
		Outcome: experiment.Outcome(row.Outcome.String), ApplyFailure: applyFailure, CreatedAt: created,
		AppliedAt: parseOptional(row.AppliedAt), AdoptionRequested: row.AdoptionRequested == 1, AdoptionFailure: adoptionFailure,
		AdoptedAt:  parseOptional(row.AdoptedAt),
		FinishedAt: parseOptional(row.FinishedAt), DecidedAt: parseOptional(row.DecidedAt),
		ContentExpiresAt: parseOptional(row.ContentExpiresAt),
	}, nil
}

func targetLanguageForStage(stage experiment.Stage, value *experiment.Language) (sql.NullString, error) {
	if stage != experiment.StageWrite {
		if value != nil {
			return sql.NullString{}, experiment.ErrLanguageRequired
		}
		return sql.NullString{}, nil
	}
	if value == nil || !value.Valid() {
		return sql.NullString{}, experiment.ErrLanguageRequired
	}
	return sql.NullString{String: value.String(), Valid: true}, nil
}

func languageFromRow(stage string, value sql.NullString) (*experiment.Language, error) {
	if experiment.Stage(stage) != experiment.StageWrite {
		if value.Valid {
			return nil, experiment.ErrLanguageRequired
		}
		return nil, nil
	}
	if !value.Valid {
		return nil, experiment.ErrLanguageRequired
	}
	language, err := experiment.ParseLanguage(value.String)
	if err != nil {
		return nil, err
	}
	return &language, nil
}

func boolValue(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func toCandidate(row sqlc.ModelExperimentCandidate) (experiment.Candidate, error) {
	failure, err := failureFromRow(row.ErrorReason, row.ErrorParams, row.TechnicalDetail, row.Error)
	if err != nil {
		return experiment.Candidate{}, fmt.Errorf("candidate %s failure: %w", row.ID, err)
	}
	return experiment.Candidate{
		ID: row.ID, ExperimentID: row.ExperimentID,
		Model: experiment.ModelRef{ProviderID: row.ModelProviderID, ModelID: row.ModelID}, ModelLabel: row.ModelLabel,
		DisplaySide: experiment.DisplaySide(row.DisplaySide), Status: experiment.CandidateStatus(row.Status),
		Output: []byte(row.Output.String), Failure: failure,
		Usage: experiment.Usage{PromptTokens: row.PromptTokens.Int64, CompletionTokens: row.CompletionTokens.Int64,
			CostMicrousd: row.CostMicrousd.Int64, CostSource: experiment.CostSource(row.CostSource.String), LatencyMS: row.LatencyMs.Int64},
		StartedAt: parseOptional(row.StartedAt), FinishedAt: parseOptional(row.FinishedAt),
	}, nil
}

func failureColumns(failure *experiment.Failure) (sql.NullString, sql.NullString, sql.NullString, error) {
	if failure == nil {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, nil
	}
	if !validFailureReason(failure.Reason) {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, fmt.Errorf("invalid reason %q", failure.Reason)
	}
	params := failure.Params
	if params == nil {
		params = map[string]string{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, fmt.Errorf("encode params: %w", err)
	}
	return nullString(failure.Reason), sql.NullString{String: string(raw), Valid: true}, nullString(failure.TechnicalDetail), nil
}

func failureFromRow(reason, params, detail, legacy sql.NullString) (*experiment.Failure, error) {
	if !reason.Valid || strings.TrimSpace(reason.String) == "" {
		if params.Valid || detail.Valid {
			return nil, errors.New("failure params/detail present without reason")
		}
		technical := strings.TrimSpace(legacy.String)
		if technical == "" {
			return nil, nil
		}
		return &experiment.Failure{Reason: experiment.FailureReasonUnknown, TechnicalDetail: technical}, nil
	}
	if !validFailureReason(reason.String) {
		return nil, fmt.Errorf("invalid reason %q", reason.String)
	}
	if !params.Valid || !strings.HasPrefix(strings.TrimSpace(params.String), "{") {
		return nil, errors.New("failure params must be a JSON object")
	}
	decoded := map[string]string{}
	if err := json.Unmarshal([]byte(params.String), &decoded); err != nil {
		return nil, fmt.Errorf("decode failure params: %w", err)
	}
	if len(decoded) == 0 {
		decoded = nil
	}
	return &experiment.Failure{Reason: reason.String, Params: decoded, TechnicalDetail: detail.String}, nil
}

func validFailureReason(reason string) bool {
	if reason == "" || reason[0] < 'A' || reason[0] > 'Z' || reason[len(reason)-1] == '_' {
		return false
	}
	previousUnderscore := false
	for i, char := range reason {
		if char >= 'A' && char <= 'Z' {
			previousUnderscore = false
			continue
		}
		if char == '_' && !previousUnderscore {
			previousUnderscore = true
			continue
		}
		if i > 0 && char >= '0' && char <= '9' {
			previousUnderscore = false
			continue
		}
		return false
	}
	return true
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
func nullBytes(value []byte) sql.NullString { return nullString(string(value)) }
func nullTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*value), Valid: true}
}
func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }
func parseOptional(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil
	}
	return &parsed
}

var _ experiment.Store = (*Store)(nil)

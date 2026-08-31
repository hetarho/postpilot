// Package store persists the job context. sqlc types stop at this boundary.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/job/store/sqlc"
)

const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	write *sqlc.Queries
	read  *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{write: sqlc.New(writer), read: sqlc.New(reader)}
}

func (s *Store) Insert(ctx context.Context, found job.Job) error {
	err := s.write.InsertJob(ctx, sqlc.InsertJobParams{
		ID: found.ID, PostSlug: nullStringPtr(found.PostSlug), UserID: found.UserID, VoiceID: nullString(found.VoiceID),
		Kind: found.Kind, ObserveModel: nullString(found.ObserveModel),
		WriteModel: nullString(found.WriteModel), Payload: string(found.Payload),
		TargetLanguage: nullString(found.TargetLanguage),
		CreatedAt:      formatTime(found.CreatedAt), UpdatedAt: formatTime(found.UpdatedAt),
	})
	if err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "unique constraint failed") || strings.Contains(message, "active voice job already exists") {
			return job.ErrActiveConflict
		}
		if strings.Contains(message, "job voice must be active") {
			return job.ErrVoiceUnavailable
		}
		if strings.Contains(message, "foreign key constraint failed") {
			return job.ErrInvalidTarget
		}
		return fmt.Errorf("insert generation job: %w", err)
	}
	return nil
}

func (s *Store) PickNextQueued(ctx context.Context, now time.Time) (job.Job, error) {
	row, err := s.write.PickNextQueued(ctx, sqlc.PickNextQueuedParams{
		StartedAt: sql.NullString{String: formatTime(now), Valid: true}, UpdatedAt: formatTime(now),
	})
	if err != nil {
		return job.Job{}, mapNotFound(err, "pick queued job")
	}
	return toJob(row)
}

func (s *Store) UpdateProgress(ctx context.Context, id, stage string, done, total int, now time.Time) error {
	err := s.write.UpdateProgress(ctx, sqlc.UpdateProgressParams{
		Stage: nullString(stage), ProgressDone: int64(done), ProgressTotal: int64(total),
		UpdatedAt: formatTime(now), ID: id,
	})
	if err != nil {
		return fmt.Errorf("update job progress: %w", err)
	}
	return nil
}

func (s *Store) Finish(ctx context.Context, id, status string, failure *job.Failure, now time.Time) error {
	if status == job.StatusFailed && failure == nil {
		return errors.New("finish job: failed status requires failure")
	}
	if status == job.StatusDone && failure != nil {
		return errors.New("finish job: done status cannot carry failure")
	}
	reason, params, detail, err := failureColumns(failure)
	if err != nil {
		return fmt.Errorf("finish job failure: %w", err)
	}
	err = s.write.FinishJob(ctx, sqlc.FinishJobParams{
		Status: status, ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail,
		FinishedAt: sql.NullString{String: formatTime(now), Valid: true}, UpdatedAt: formatTime(now), ID: id,
	})
	if err != nil {
		return fmt.Errorf("finish job: %w", err)
	}
	return nil
}

func (s *Store) FailQueued(ctx context.Context, id, userID string, failure job.Failure, now time.Time) (bool, error) {
	reason, params, detail, err := failureColumns(&failure)
	if err != nil {
		return false, fmt.Errorf("fail queued job failure: %w", err)
	}
	n, err := s.write.FailQueuedJob(ctx, sqlc.FailQueuedJobParams{
		ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail,
		FinishedAt: sql.NullString{String: formatTime(now), Valid: true}, UpdatedAt: formatTime(now), ID: id, UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("fail queued job: %w", err)
	}
	return n == 1, nil
}

func (s *Store) SweepRunning(ctx context.Context, failure job.Failure, now time.Time) (int64, error) {
	reason, params, detail, err := failureColumns(&failure)
	if err != nil {
		return 0, fmt.Errorf("sweep running job failure: %w", err)
	}
	n, err := s.write.SweepRunning(ctx, sqlc.SweepRunningParams{
		ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail,
		FinishedAt: sql.NullString{String: formatTime(now), Valid: true}, UpdatedAt: formatTime(now),
	})
	if err != nil {
		return 0, fmt.Errorf("sweep running jobs: %w", err)
	}
	return n, nil
}

func (s *Store) SweepQueuedPersonalization(ctx context.Context, failure job.Failure, now time.Time) (int64, error) {
	reason, params, detail, err := failureColumns(&failure)
	if err != nil {
		return 0, fmt.Errorf("sweep queued personalization failure: %w", err)
	}
	n, err := s.write.SweepQueuedPersonalization(ctx, sqlc.SweepQueuedPersonalizationParams{
		ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail,
		FinishedAt: sql.NullString{String: formatTime(now), Valid: true}, UpdatedAt: formatTime(now),
	})
	if err != nil {
		return 0, fmt.Errorf("sweep queued personalization: %w", err)
	}
	return n, nil
}

func (s *Store) ActiveForPost(ctx context.Context, slug string) (*job.Job, error) {
	row, err := s.read.ActiveForPost(ctx, sql.NullString{String: slug, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select active post job: %w", err)
	}
	found, err := toJob(row)
	return &found, err
}

func (s *Store) ActiveForPostUser(ctx context.Context, slug, userID string) (*job.Job, error) {
	row, err := s.read.ActiveForPostUser(ctx, sqlc.ActiveForPostUserParams{
		PostSlug: sql.NullString{String: slug, Valid: true}, UserID: userID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select active post job for user: %w", err)
	}
	found, err := toJob(row)
	return &found, err
}

func (s *Store) ActiveForUserKind(ctx context.Context, userID, kind string) (*job.Job, error) {
	row, err := s.read.ActiveForUserKind(ctx, sqlc.ActiveForUserKindParams{UserID: userID, Kind: kind})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select active user job: %w", err)
	}
	found, err := toJob(row)
	return &found, err
}

func (s *Store) ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*job.Job, error) {
	row, err := s.read.ActiveForVoiceKind(ctx, sqlc.ActiveForVoiceKindParams{VoiceID: nullString(voiceID), Kind: kind})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select active voice job: %w", err)
	}
	found, err := toJob(row)
	return &found, err
}

func (s *Store) ActiveForVoice(ctx context.Context, voiceID string) (*job.Job, error) {
	row, err := s.read.ActiveForVoice(ctx, nullString(voiceID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select active jobs for voice: %w", err)
	}
	found, err := toJob(row)
	return &found, err
}

func (s *Store) ActiveModelExperiment(ctx context.Context, experimentID string) (*job.Job, error) {
	row, err := s.read.ActiveModelExperiment(ctx, experimentID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select active experiment job: %w", err)
	}
	found, err := toJob(row)
	return &found, err
}

func (s *Store) GetByID(ctx context.Context, id string) (job.Job, error) {
	row, err := s.read.GetJobByID(ctx, id)
	if err != nil {
		return job.Job{}, mapNotFound(err, "select job")
	}
	return toJob(row)
}

func toJob(row sqlc.GenerationJob) (job.Job, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return job.Job{}, fmt.Errorf("job %s created_at: %w", row.ID, err)
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return job.Job{}, fmt.Errorf("job %s updated_at: %w", row.ID, err)
	}
	started, err := parseOptionalTime(row.StartedAt)
	if err != nil {
		return job.Job{}, fmt.Errorf("job %s started_at: %w", row.ID, err)
	}
	finished, err := parseOptionalTime(row.FinishedAt)
	if err != nil {
		return job.Job{}, fmt.Errorf("job %s finished_at: %w", row.ID, err)
	}
	failure, err := failureFromRow(row.ErrorReason, row.ErrorParams, row.TechnicalDetail, row.Error)
	if err != nil {
		return job.Job{}, fmt.Errorf("job %s failure: %w", row.ID, err)
	}
	return job.Job{
		ID: row.ID, PostSlug: stringPtr(row.PostSlug), UserID: row.UserID, VoiceID: row.VoiceID.String, Kind: row.Kind,
		Status: row.Status, Stage: row.Stage.String, ProgressDone: int(row.ProgressDone),
		ProgressTotal: int(row.ProgressTotal), Failure: failure,
		ObserveModel: row.ObserveModel.String, WriteModel: row.WriteModel.String, TargetLanguage: row.TargetLanguage.String,
		Payload: []byte(row.Payload), CreatedAt: created, UpdatedAt: updated,
		StartedAt: started, FinishedAt: finished,
	}, nil
}

func failureColumns(failure *job.Failure) (sql.NullString, sql.NullString, sql.NullString, error) {
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

func failureFromRow(reason, params, detail, legacy sql.NullString) (*job.Failure, error) {
	if !reason.Valid || strings.TrimSpace(reason.String) == "" {
		if params.Valid || detail.Valid {
			return nil, errors.New("failure params/detail present without reason")
		}
		technical := strings.TrimSpace(legacy.String)
		if technical == "" {
			return nil, nil
		}
		return &job.Failure{Reason: job.FailureReasonUnknown, TechnicalDetail: technical}, nil
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
	return &job.Failure{Reason: reason.String, Params: decoded, TechnicalDetail: detail.String}, nil
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

func mapNotFound(err error, op string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return job.ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullStringPtr(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func stringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// Package store persists the job context. sqlc types stop at this boundary.
package store

import (
	"context"
	"database/sql"
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
		CreatedAt: formatTime(found.CreatedAt), UpdatedAt: formatTime(found.UpdatedAt),
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

func (s *Store) Finish(ctx context.Context, id, status, message string, now time.Time) error {
	err := s.write.FinishJob(ctx, sqlc.FinishJobParams{
		Status: status, Error: nullString(message), FinishedAt: sql.NullString{String: formatTime(now), Valid: true},
		UpdatedAt: formatTime(now), ID: id,
	})
	if err != nil {
		return fmt.Errorf("finish job: %w", err)
	}
	return nil
}

func (s *Store) FailQueued(ctx context.Context, id, userID, message string, now time.Time) (bool, error) {
	n, err := s.write.FailQueuedJob(ctx, sqlc.FailQueuedJobParams{
		Error: nullString(message), FinishedAt: sql.NullString{String: formatTime(now), Valid: true},
		UpdatedAt: formatTime(now), ID: id, UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("fail queued job: %w", err)
	}
	return n == 1, nil
}

func (s *Store) SweepRunning(ctx context.Context, message string, now time.Time) (int64, error) {
	n, err := s.write.SweepRunning(ctx, sqlc.SweepRunningParams{
		Error: nullString(message), FinishedAt: sql.NullString{String: formatTime(now), Valid: true},
		UpdatedAt: formatTime(now),
	})
	if err != nil {
		return 0, fmt.Errorf("sweep running jobs: %w", err)
	}
	return n, nil
}

func (s *Store) SweepQueuedPersonalization(ctx context.Context, message string, now time.Time) (int64, error) {
	n, err := s.write.SweepQueuedPersonalization(ctx, sqlc.SweepQueuedPersonalizationParams{
		Error: nullString(message), FinishedAt: sql.NullString{String: formatTime(now), Valid: true}, UpdatedAt: formatTime(now),
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
	return job.Job{
		ID: row.ID, PostSlug: stringPtr(row.PostSlug), UserID: row.UserID, VoiceID: row.VoiceID.String, Kind: row.Kind,
		Status: row.Status, Stage: row.Stage.String, ProgressDone: int(row.ProgressDone),
		ProgressTotal: int(row.ProgressTotal), Error: row.Error.String,
		ObserveModel: row.ObserveModel.String, WriteModel: row.WriteModel.String,
		Payload: []byte(row.Payload), CreatedAt: created, UpdatedAt: updated,
		StartedAt: started, FinishedAt: finished,
	}, nil
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

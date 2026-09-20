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

// The subject dimensions this schema can address, and the column each one lives in. The
// names are the owning contexts' words for their own subjects; this table is the only
// place they meet a column, and an unknown dimension is refused rather than ignored.
const (
	dimensionPost       = "post"
	dimensionVoice      = "voice"
	dimensionProject    = "clip_project"
	dimensionExperiment = "model_experiment"
)

// attachable dimensions are the ones a job is inserted with. The experiment dimension is
// read-only: an experiment's id is its payload, surfaced by a generated column.
func attachableDimension(dimension string) bool {
	switch dimension {
	case dimensionPost, dimensionVoice, dimensionProject:
		return true
	default:
		return false
	}
}

// Kinds is what the composition root tells the store about the work it will hold, so no
// statement here names a product: which kinds wait for an activation before they may be
// dispatched, which an owner may cancel, and which must authorize every model call
// against that cancellation. Empty lists are a real mode — a queue where nothing waits,
// nothing is cancelled and nothing is authorized — and must be stated, not defaulted into.
type Kinds struct {
	Deferred    []string
	Cancellable []string
	Authorized  []string
}

type Store struct {
	write *sqlc.Queries
	read  *sqlc.Queries
	kinds Kinds
}

func New(writer, reader *sql.DB, kinds Kinds) *Store {
	return &Store{write: sqlc.New(writer), read: sqlc.New(reader), kinds: kinds}
}

func NewTx(tx *sql.Tx, kinds Kinds) *Store {
	return &Store{write: sqlc.New(tx), read: sqlc.New(tx), kinds: kinds}
}

func (s *Store) deferred(kind string) bool {
	for _, deferredKind := range s.kinds.Deferred {
		if deferredKind == kind {
			return true
		}
	}
	return false
}

func kindsJSON(kinds []string) (string, error) {
	if kinds == nil {
		kinds = []string{}
	}
	raw, err := json.Marshal(kinds)
	if err != nil {
		return "", fmt.Errorf("encode job kinds: %w", err)
	}
	return string(raw), nil
}

func (s *Store) Insert(ctx context.Context, found job.Job) error {
	for _, subject := range found.Subjects {
		if !attachableDimension(subject.Dimension) {
			return fmt.Errorf("insert generation job: subject dimension %q cannot be attached", subject.Dimension)
		}
	}
	err := s.write.InsertJob(ctx, sqlc.InsertJobParams{
		CancellationPolicyVersion: int64(found.CancellationPolicyVersion),
		ID:                        found.ID, PostSlug: nullString(found.Subject(dimensionPost)), UserID: found.UserID, VoiceID: nullString(found.Subject(dimensionVoice)),
		ClipProjectID: nullString(found.Subject(dimensionProject)), DispatchReady: s.dispatchReady(found.Kind), Kind: found.Kind, ObserveModel: nullString(found.ObserveModel),
		WriteModel: nullString(found.WriteModel), Payload: string(found.Payload),
		TargetLanguage: nullString(found.TargetLanguage),
		CreatedAt:      formatTime(found.CreatedAt), UpdatedAt: formatTime(found.UpdatedAt),
	})
	if err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "unique constraint failed") || strings.Contains(message, "active voice job already exists") {
			return job.ErrActiveConflict
		}
		if strings.Contains(message, "clip job target unavailable") {
			return job.ErrInvalidTarget
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
	if (status == job.StatusDone || status == job.StatusCancelled) && failure != nil {
		return errors.New("finish job: done status cannot carry failure")
	}
	reason, params, detail, err := failureColumns(failure)
	if err != nil {
		return fmt.Errorf("finish job failure: %w", err)
	}
	changed, err := s.write.FinishJob(ctx, sqlc.FinishJobParams{
		Status: status, ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail,
		FinishedAt: sql.NullString{String: formatTime(now), Valid: true}, UpdatedAt: formatTime(now), ID: id,
	})
	if err != nil {
		return fmt.Errorf("finish job: %w", err)
	}
	if changed != 1 {
		j, readErr := s.GetByID(ctx, id)
		// A deferred kind is also the cancellable one: its terminal write can legitimately
		// run twice, once from the worker and once from the cancellation that raced it.
		if readErr == nil && s.deferred(j.Kind) && job.Terminal(j.Status) && j.Status == status {
			return nil
		}
		return errors.New("finish job: no running job changed")
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

// ActiveFor resolves the dimension the caller named to its column. A dimension this
// schema does not know is an error: matching nothing would read as "not busy".
func (s *Store) ActiveFor(ctx context.Context, subject job.Subject, filter job.Filter) (*job.Job, error) {
	var (
		row sqlc.GenerationJob
		err error
	)
	switch subject.Dimension {
	case dimensionPost:
		switch {
		case filter.Kind != "":
			return nil, fmt.Errorf("active for post: kind filter unsupported")
		case filter.UserID != "":
			row, err = s.read.ActiveForPostUser(ctx, sqlc.ActiveForPostUserParams{PostSlug: nullString(subject.ID), UserID: filter.UserID})
		default:
			row, err = s.read.ActiveForPost(ctx, nullString(subject.ID))
		}
	case dimensionVoice:
		switch {
		case filter.UserID != "":
			return nil, fmt.Errorf("active for voice: user filter unsupported")
		case filter.Kind != "":
			row, err = s.read.ActiveForVoiceKind(ctx, sqlc.ActiveForVoiceKindParams{VoiceID: nullString(subject.ID), Kind: filter.Kind})
		default:
			row, err = s.read.ActiveForVoice(ctx, nullString(subject.ID))
		}
	case dimensionProject:
		if filter.Kind != "" {
			return nil, fmt.Errorf("active for clip project: kind filter unsupported")
		}
		row, err = s.read.ActiveForProject(ctx, sqlc.ActiveForProjectParams{UserID: filter.UserID, ClipProjectID: nullString(subject.ID)})
	case dimensionExperiment:
		if filter.UserID != "" || filter.Kind != "" {
			return nil, fmt.Errorf("active for experiment: filters unsupported")
		}
		row, err = s.read.ActiveForExperiment(ctx, nullString(subject.ID))
	default:
		return nil, fmt.Errorf("active job lookup: unknown subject dimension %q", subject.Dimension)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select active job for %s: %w", subject.Dimension, err)
	}
	found, err := toJob(row)
	return &found, err
}

// LatestFor is the most recent job of a subject, terminal or not.
func (s *Store) LatestFor(ctx context.Context, subject job.Subject, filter job.Filter) (*job.Job, error) {
	if subject.Dimension != dimensionProject {
		return nil, fmt.Errorf("latest job lookup: unsupported subject dimension %q", subject.Dimension)
	}
	if filter.Kind != "" {
		return nil, fmt.Errorf("latest for clip project: kind filter unsupported")
	}
	row, err := s.read.LatestForProject(ctx, sqlc.LatestForProjectParams{UserID: filter.UserID, ClipProjectID: nullString(subject.ID)})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select latest job for %s: %w", subject.Dimension, err)
	}
	found, err := toJob(row)
	return &found, err
}

// ActiveUnattached guards work that carries neither a post nor a project: one per user
// and kind. Voice-only work is deliberately still visible here, as it always has been.
func (s *Store) ActiveUnattached(ctx context.Context, userID, kind string) (*job.Job, error) {
	row, err := s.read.ActiveForUserKind(ctx, sqlc.ActiveForUserKindParams{UserID: userID, Kind: kind})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select active unattached job: %w", err)
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
	cancelled, err := parseOptionalTime(row.CancelRequestedAt)
	if err != nil {
		return job.Job{}, fmt.Errorf("job %s cancel_requested_at: %w", row.ID, err)
	}
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
		CancelRequestedAt: cancelled, CancellationPolicyVersion: int(row.CancellationPolicyVersion),
		ID: row.ID, UserID: row.UserID, Kind: row.Kind, Subjects: rowSubjects(row),
		DispatchReady: row.DispatchReady != 0,
		Status:        row.Status, Stage: row.Stage.String, ProgressDone: int(row.ProgressDone),
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

// rowSubjects reads the attachments a row carries, in a fixed order so two reads of the
// same job are equal. A model experiment's id is its payload, which the generated column
// surfaces for lookups; it is not an attachment and never appears here.
func rowSubjects(row sqlc.GenerationJob) []job.Subject {
	subjects := make([]job.Subject, 0, 3)
	for _, candidate := range []job.Subject{
		{Dimension: dimensionPost, ID: row.PostSlug.String},
		{Dimension: dimensionVoice, ID: row.VoiceID.String},
		{Dimension: dimensionProject, ID: row.ClipProjectID.String},
	} {
		if candidate.ID != "" {
			subjects = append(subjects, candidate)
		}
	}
	if len(subjects) == 0 {
		return nil
	}
	return subjects
}

func (s *Store) dispatchReady(kind string) int64 {
	if s.deferred(kind) {
		return 0
	}
	return 1
}

// Activate releases one deferred job for dispatch.
func (s *Store) Activate(ctx context.Context, user, id string) (bool, error) {
	kinds, err := kindsJSON(s.kinds.Deferred)
	if err != nil {
		return false, err
	}
	n, err := s.write.Activate(ctx, sqlc.ActivateParams{UserID: user, ID: id, Kinds: kinds})
	return n == 1, err
}

// SweepUnactivated fails every deferred job boot finds still waiting for its activation.
func (s *Store) SweepUnactivated(ctx context.Context, f job.Failure) (int64, error) {
	r, p, d, err := failureColumns(&f)
	if err != nil {
		return 0, err
	}
	kinds, err := kindsJSON(s.kinds.Deferred)
	if err != nil {
		return 0, err
	}
	now := formatTime(time.Now())
	return s.write.SweepUnactivated(ctx, sqlc.SweepUnactivatedParams{ErrorReason: r, ErrorParams: p, TechnicalDetail: d, FinishedAt: nullString(now), UpdatedAt: now, Kinds: kinds})
}

// SavePayload replaces a running job's payload with its handler's result.
func (s *Store) SavePayload(ctx context.Context, id string, payload []byte, at time.Time) (bool, error) {
	n, err := s.write.SaveJobPayload(ctx, sqlc.SaveJobPayloadParams{
		Payload: string(payload), UpdatedAt: formatTime(at), ID: id,
	})
	if err != nil {
		return false, fmt.Errorf("save job payload: %w", err)
	}
	return n == 1, nil
}

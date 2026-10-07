package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store/sqlc"
)

// The storage representation belongs to this edge; domain types have no JSON/DB
// tags and the payload accepts only the validated safe product projection.
type storedRequestInspection llm.RequestInspection

func encodeRequestInspection(value llm.RequestInspection) (string, error) {
	if err := value.Validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(storedRequestInspection(value))
	return string(data), err
}

func decodeRequestInspection(raw string) (llm.RequestInspection, error) {
	var stored storedRequestInspection
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return llm.RequestInspection{}, err
	}
	value := llm.RequestInspection(stored)
	return value, value.Validate()
}

func captureOwnedPost(ctx context.Context, q *sqlc.Queries, userID, slug string) (post.Post, error) {
	value, err := ownedOriginPost(ctx, q, userID, slug)
	if err != nil {
		return post.Post{}, err
	}
	value.Images, value.Videos, _, err = originAttachments(ctx, q, slug)
	if err != nil {
		return post.Post{}, err
	}
	answers, err := q.ListPostTemplateAnswers(ctx, slug)
	if err != nil {
		return post.Post{}, err
	}
	for _, answer := range answers {
		value.TemplateAnswers = append(value.TemplateAnswers, post.TemplateAnswer{Label: answer.Label, Text: answer.Answer, Enabled: answer.Enabled != 0})
	}
	return value, nil
}

func validCaptureRun(run post.RequestCaptureRun) bool {
	return strings.TrimSpace(run.JobID) != "" && strings.TrimSpace(run.UserID) != "" && strings.TrimSpace(run.PostSlug) != "" && run.InputRevision >= 0 && run.ContentRevision >= 0 && len(run.SourceFingerprint) == 64
}

func postInspectionStage(stage string) (string, error) {
	switch stage {
	case "observe", "post-observation":
		return "post-observation", nil
	case "plan", "post-storyline":
		return "post-storyline", nil
	case "write", "post-writing":
		return "post-writing", nil
	case "revise", "post-revision":
		return "post-revision", nil
	default:
		return "", fmt.Errorf("%w: post stage is unsupported", llm.ErrInvalidInspection)
	}
}

// WritePostRequestCapture records issued evidence before canonical parsing. An
// invalid, stale or deleted capture is refused independently of canonical output.
func (s *Store) WritePostRequestCapture(ctx context.Context, run post.RequestCaptureRun, call post.RequestCaptureCall, value llm.RequestInspection) error {
	if !validCaptureRun(run) || strings.TrimSpace(call.ID) == "" || call.Sequence < 0 || len(call.AttachmentIDs) != len(call.AttachmentKinds) {
		return fmt.Errorf("%w: capture identity is invalid", llm.ErrInvalidInspection)
	}
	if value.Status != llm.InspectionCaptured && value.Status != llm.InspectionUnavailable {
		return fmt.Errorf("%w: only issued or unavailable capture may be stored", llm.ErrInvalidInspection)
	}
	if value.Status == llm.InspectionCaptured {
		value.CallID = call.ID
		value.Attachments = nil
		for i, id := range call.AttachmentIDs {
			value.Attachments = append(value.Attachments, llm.InspectionAttachment{ID: id, Kind: call.AttachmentKinds[i]})
		}
	}
	payload, err := encodeRequestInspection(value)
	if err != nil {
		return err
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	purged, err := q.PostRequestCapturePurged(ctx, sqlc.PostRequestCapturePurgedParams{PostSlug: run.PostSlug, UserID: run.UserID, JobID: run.JobID})
	if err != nil {
		return err
	}
	if purged {
		return fmt.Errorf("%w: request capture was purged", llm.ErrInvalidInspection)
	}
	found, err := captureOwnedPost(ctx, q, run.UserID, run.PostSlug)
	if err != nil {
		return err
	}
	if found.Status == post.StatusPublished {
		return post.ErrPostPublished
	}
	if found.ContentRevision != run.ContentRevision || post.RequestCaptureSourceFingerprint(found) != run.SourceFingerprint || post.StorylineFingerprint(found.Storyline) != run.PlanFingerprint {
		return post.ErrStaleContentRevision
	}
	media := make(map[string]string, len(found.Images)+len(found.Videos))
	for _, v := range found.Images {
		media[v.ID] = string(post.AttachmentPhoto)
	}
	for _, v := range found.Videos {
		media[v.ID] = string(post.AttachmentVideo)
	}
	for i, id := range call.AttachmentIDs {
		if media[id] != call.AttachmentKinds[i] {
			return post.ErrStaleContentRevision
		}
	}
	if call.AttachmentID != "" && media[call.AttachmentID] == "" {
		return post.ErrStaleContentRevision
	}
	if err := q.DeleteOtherPostStageCaptures(ctx, sqlc.DeleteOtherPostStageCapturesParams{PostSlug: run.PostSlug, Stage: value.Stage, JobID: run.JobID}); err != nil {
		return err
	}
	if err := q.WritePostRequestCapture(ctx, sqlc.WritePostRequestCaptureParams{PostSlug: run.PostSlug, UserID: run.UserID, JobID: run.JobID, CallID: call.ID, CallSequence: int64(call.Sequence), AttachmentID: call.AttachmentID, Stage: value.Stage, InputRevision: run.InputRevision, ContentRevision: run.ContentRevision, SourceFingerprint: run.SourceFingerprint, SourcePlanFingerprint: run.PlanFingerprint, Payload: payload}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) FinishPostRequestCapture(ctx context.Context, run post.RequestCaptureRun, completion *post.RequestCaptureCompletion) error {
	if !validCaptureRun(run) {
		return fmt.Errorf("%w: capture run is invalid", llm.ErrInvalidInspection)
	}
	if completion == nil {
		return nil
	}
	if completion.Result == nil && completion.PlanFingerprint == nil && len(completion.UnavailableStages) == 0 {
		return fmt.Errorf("%w: result binding is absent", llm.ErrInvalidInspection)
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	purged, err := q.PostRequestCapturePurged(ctx, sqlc.PostRequestCapturePurgedParams{PostSlug: run.PostSlug, UserID: run.UserID, JobID: run.JobID})
	if err != nil {
		return err
	}
	if purged {
		return fmt.Errorf("%w: request capture was purged", llm.ErrInvalidInspection)
	}
	found, err := captureOwnedPost(ctx, q, run.UserID, run.PostSlug)
	if err != nil {
		return err
	}
	if found.Status == post.StatusPublished {
		return post.ErrPostPublished
	}
	if post.RequestCaptureSourceFingerprint(found) != run.SourceFingerprint {
		return post.ErrStaleContentRevision
	}
	if completion.Result == nil && completion.PlanFingerprint == nil && (found.ContentRevision != run.ContentRevision || post.StorylineFingerprint(found.Storyline) != run.PlanFingerprint) {
		return post.ErrStaleContentRevision
	}
	params := sqlc.BindPostRequestCaptureParams{PostSlug: run.PostSlug, UserID: run.UserID, JobID: run.JobID, ContentRevision: run.ContentRevision, SourceFingerprint: run.SourceFingerprint}
	if completion.Result != nil {
		if found.Content == nil || post.ContentOriginIdentity(*found.Content, found.ContentRevision) != *completion.Result {
			return post.ErrStaleContentRevision
		}
		params.ResultRevision = sql.NullInt64{Int64: completion.Result.ContentRevision, Valid: true}
		params.ResultHash = sql.NullString{String: completion.Result.ContentHash, Valid: true}
	}
	if completion.PlanFingerprint != nil {
		if *completion.PlanFingerprint == "" || post.StorylineFingerprint(found.Storyline) != *completion.PlanFingerprint {
			return post.ErrStaleContentRevision
		}
		params.PlanFingerprint = sql.NullString{String: *completion.PlanFingerprint, Valid: true}
	}
	for _, stage := range completion.UnavailableStages {
		if strings.TrimSpace(stage) == "" || strings.TrimSpace(completion.UnavailableReason) == "" {
			return fmt.Errorf("%w: unavailable capture identity is absent", llm.ErrInvalidInspection)
		}
		value := llm.UnavailableRequestInspection(stage, "")
		value.UnavailableReason = completion.UnavailableReason
		payload, err := encodeRequestInspection(value)
		if err != nil {
			return err
		}
		if err := q.DeletePostStageCaptures(ctx, sqlc.DeletePostStageCapturesParams{PostSlug: run.PostSlug, UserID: run.UserID, Stage: stage}); err != nil {
			return err
		}
		if err := q.WritePostRequestCapture(ctx, sqlc.WritePostRequestCaptureParams{PostSlug: run.PostSlug, UserID: run.UserID, JobID: run.JobID, CallID: "capture-unavailable:" + stage, Stage: stage, InputRevision: run.InputRevision, ContentRevision: run.ContentRevision, SourceFingerprint: run.SourceFingerprint, SourcePlanFingerprint: run.PlanFingerprint, Payload: payload}); err != nil {
			return err
		}
	}
	if _, err := q.BindPostRequestCapture(ctx, params); err != nil {
		return err
	}
	return tx.Commit()
}

// Reads use one snapshot so source removal cannot mix an old capture with a new
// source incarnation. They never repair, execute or mutate a private payload.
func (s *Store) ReadPostRequestCaptures(ctx context.Context, userID, slug, stage string) ([]llm.RequestInspection, error) {
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q := sqlc.New(tx)
	found, err := captureOwnedPost(ctx, q, userID, slug)
	if err != nil {
		return nil, err
	}
	stage, err = postInspectionStage(stage)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListPostRequestCaptures(ctx, sqlc.ListPostRequestCapturesParams{PostSlug: slug, UserID: userID, Stage: stage})
	if err != nil {
		return nil, err
	}
	fingerprint := post.RequestCaptureSourceFingerprint(found)
	var values []llm.RequestInspection
	for _, row := range rows {
		if row.SourceFingerprint != fingerprint {
			continue
		}
		if row.ResultRevision.Valid || row.ResultHash.Valid {
			if !row.ResultRevision.Valid || !row.ResultHash.Valid || found.Content == nil || post.ContentOriginIdentity(*found.Content, found.ContentRevision) != (post.OriginResultIdentity{ContentRevision: row.ResultRevision.Int64, ContentHash: row.ResultHash.String}) {
				continue
			}
		} else if !row.PlanFingerprint.Valid {
			if row.ContentRevision != found.ContentRevision || row.SourcePlanFingerprint != post.StorylineFingerprint(found.Storyline) {
				continue
			}
		}
		if row.PlanFingerprint.Valid && post.StorylineFingerprint(found.Storyline) != row.PlanFingerprint.String {
			continue
		}
		value, err := decodeRequestInspection(row.Payload)
		if err != nil || value.Stage != stage {
			continue
		}
		values = append(values, value)
	}
	return values, nil
}

func (s *Store) ReadPostRequestInspection(ctx context.Context, userID, slug, stage string, status llm.InspectionStatus) (llm.RequestInspection, error) {
	if status != llm.InspectionCaptured {
		return llm.RequestInspection{}, fmt.Errorf("%w: store reads captured requests only", llm.ErrInvalidInspection)
	}
	values, err := s.ReadPostRequestCaptures(ctx, userID, slug, stage)
	if err != nil {
		return llm.RequestInspection{}, err
	}
	if len(values) == 0 {
		value := llm.UnavailableRequestInspection(stage, "")
		value.UnavailableReason = "capture_missing_stale_or_purged"
		return value, nil
	}
	return values[len(values)-1], nil
}

func (s *Store) PurgePostRequestCaptures(ctx context.Context, userID, slug string) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if _, err := ownedOriginPost(ctx, q, userID, slug); err != nil {
		return err
	}
	if err := q.FencePostRequestCapturePurge(ctx, sqlc.FencePostRequestCapturePurgeParams{PostSlug: slug, UserID: userID}); err != nil {
		return err
	}
	if err := q.PurgePostRequestCaptures(ctx, sqlc.PurgePostRequestCapturesParams{PostSlug: slug, UserID: userID}); err != nil {
		return err
	}
	return tx.Commit()
}

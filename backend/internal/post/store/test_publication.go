package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store/sqlc"
)

// TestPublicationGuard binds the job owner's published read port to this exact
// transaction. The composition adapter uses job/store.NewTx; this store does not
// query job-owned tables, and a missing guard never permits publication. The
// adapter must filter ordinary kinds before selecting an active row; classifying
// only the newest unfiltered row is unsafe when independent tests can coexist.
type TestPublicationGuard interface {
	HasOrdinaryWrite(context.Context, *sql.Tx, string, string) (bool, error)
}

type TestResultStore struct {
	store *Store
	jobs  TestPublicationGuard
}

func NewTestResultStore(writer, reader *sql.DB, jobs TestPublicationGuard) *TestResultStore {
	if jobs == nil {
		panic("post: transactional ordinary job guard is required")
	}
	return &TestResultStore{store: New(writer, reader), jobs: jobs}
}

func (s *TestResultStore) ApplyTestResult(ctx context.Context, in post.TestOutputPublication, now time.Time) (post.TestOutputReceipt, error) {
	var empty post.TestOutputReceipt
	if in.UserID == "" || in.TestID == "" || in.WinnerID == "" || in.RequestKey == "" || in.PostSlug == "" {
		return empty, post.ErrTestPublicationConflict
	}
	// RequestKey is not a part of the operation identity: a retry of the same
	// champion returns the original domain receipt even if the caller lost its key.
	identity := in
	identity.RequestKey = ""
	raw, err := json.Marshal(identity)
	if err != nil {
		return empty, err
	}
	digest := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(digest[:])
	tx, err := s.store.writer.BeginTx(ctx, nil)
	if err != nil {
		return empty, fmt.Errorf("begin test output publication: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit
	q := s.store.write.WithTx(tx)
	replays, err := q.GetPostTestPublications(ctx, sqlc.GetPostTestPublicationsParams{
		UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, RequestKey: in.RequestKey,
	})
	if err != nil {
		return empty, err
	}
	for _, row := range replays {
		if row.Fingerprint != fingerprint {
			return empty, post.ErrTestPublicationConflict
		}
		if err := json.Unmarshal([]byte(row.Receipt), &empty); err != nil {
			return post.TestOutputReceipt{}, fmt.Errorf("decode test publication receipt: %w", err)
		}
	}
	if len(replays) > 0 {
		return empty, nil
	}
	row, err := q.GetPost(ctx, in.PostSlug)
	if errors.Is(err, sql.ErrNoRows) {
		return empty, post.ErrNotFound
	}
	if err != nil {
		return empty, err
	}
	found, err := toPost(row)
	if err != nil {
		return empty, err
	}
	if found.UserID != in.UserID {
		return empty, post.ErrForbidden
	}
	if found.Status == post.StatusPublished {
		return empty, post.ErrPostPublished
	}
	if (found.Status != post.StatusDraft && found.Status != post.StatusReview) ||
		found.InputRevision != in.InputRevision || found.ContentRevision != in.ContentRevision ||
		in.AssignmentsHash != post.TestAssignmentsHash(found) {
		return empty, post.ErrTestPublicationConflict
	}
	busy, err := s.jobs.HasOrdinaryWrite(ctx, tx, in.UserID, in.PostSlug)
	if err != nil {
		return empty, fmt.Errorf("guard ordinary writing: %w", err)
	}
	if busy {
		return empty, post.ErrPostBusy
	}
	if !in.ContentLanguage.Valid() {
		return empty, post.ErrLanguageRequired
	}
	images, err := q.ListImagesByPost(ctx, in.PostSlug)
	if err != nil {
		return empty, err
	}
	photos := make([]post.Image, len(images))
	for i, image := range images {
		photos[i], err = toImage(image)
		if err != nil {
			return empty, err
		}
	}
	videos, err := q.ListVideosByPost(ctx, in.PostSlug)
	if err != nil {
		return empty, err
	}
	clips := make([]post.Video, len(videos))
	for i, video := range videos {
		clips[i], err = toVideo(video)
		if err != nil {
			return empty, err
		}
	}
	for _, content := range []post.PostContent{in.Content, in.Baseline} {
		if err := post.ValidateContent(content, photos, clips); err != nil {
			return empty, err
		}
	}
	if err := post.ValidateTestStoryline(in.Storyline, photos, clips); err != nil {
		return empty, err
	}
	content, err := marshalContent(in.Content)
	if err != nil {
		return empty, err
	}
	baseline, err := marshalContent(in.Baseline)
	if err != nil {
		return empty, err
	}
	if baseline != content {
		return empty, fmt.Errorf("%w: machine baseline must match applied test output", post.ErrInvalidContent)
	}
	storyline, err := marshalStoryline(in.Storyline)
	if err != nil {
		return empty, err
	}
	nouns, err := marshalNouns(in.Nouns)
	if err != nil {
		return empty, err
	}
	changed, err := q.ApplyPostTestOutput(ctx, sqlc.ApplyPostTestOutputParams{
		Content: sql.NullString{String: content, Valid: true}, MachineBaseline: sql.NullString{String: baseline, Valid: true},
		ContentLanguage: sql.NullString{String: string(in.ContentLanguage), Valid: true}, ContentNouns: nouns,
		Storyline: storyline, UpdatedAt: formatTime(now), Slug: in.PostSlug, UserID: in.UserID,
		ExpectedInputRevision: in.InputRevision, ExpectedContentRevision: in.ContentRevision,
	})
	if err != nil {
		return empty, err
	}
	if changed != 1 {
		return empty, post.ErrTestPublicationConflict
	}
	receipt := post.TestOutputReceipt{UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID,
		Action: "apply_output", RequestKey: in.RequestKey, TargetID: in.PostSlug, ResultingRevision: in.ContentRevision + 1}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return empty, err
	}
	if err := q.CreatePostTestPublication(ctx, sqlc.CreatePostTestPublicationParams{
		UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, RequestKey: in.RequestKey,
		Fingerprint: fingerprint, PostSlug: in.PostSlug, ResultingContentRevision: receipt.ResultingRevision,
		Receipt: string(encoded), CreatedAt: formatTime(now),
	}); err != nil {
		return empty, err
	}
	if err := tx.Commit(); err != nil {
		return empty, fmt.Errorf("commit test output publication: %w", err)
	}
	return receipt, nil
}

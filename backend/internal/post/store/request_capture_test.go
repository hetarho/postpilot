package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store"
)

func requestCaptureFixture() llm.RequestInspection {
	issued := testNow
	budget, no := int64(4096), false
	return llm.RequestInspection{Version: llm.RequestInspectionVersion, Status: llm.InspectionCaptured, Stage: "post-writing", Mode: "direct", PromptVersion: "write-v1", SchemaVersion: "post-v1", Fragments: []llm.RequestFragment{{ID: "memo", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "fact", Text: "private owner 🍊\nmaterial"}}, Output: llm.OutputContractInspection{Name: "post", Version: "post-v1", Schema: "{}"}, Conditions: &llm.EffectiveRequestConditions{MaxCompletionTokens: &budget, FrozenExecution: &no}, IssuedAt: &issued, Omissions: []llm.RequestOmission{{ID: "style", Reason: "no_voice"}}, Composer: "composeWriteRequest", SourceFiles: []string{"generation/request_composition.go"}}
}

func requestRun(t *testing.T, s *store.Store, slug, job string) post.RequestCaptureRun {
	t.Helper()
	ctx := context.Background()
	found, err := s.GetPost(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	found.Images, err = s.ListImages(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	found.Videos, err = s.ListVideos(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	found.TemplateAnswers, err = s.ListTemplateAnswers(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	return post.RequestCaptureRun{JobID: job, UserID: found.UserID, PostSlug: slug, InputRevision: found.InputRevision, ContentRevision: found.ContentRevision, SourceFingerprint: post.RequestCaptureSourceFingerprint(found), PlanFingerprint: post.StorylineFingerprint(found.Storyline)}
}

func capturedPost(t *testing.T, s *store.Store, slug string) llm.RequestInspection {
	t.Helper()
	value, err := s.ReadPostRequestInspection(context.Background(), "alice", slug, "write", llm.InspectionCaptured)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRequestCaptureIssuedCallsPreserveSafeExactPayloadAndOwnerSnapshot(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	if err := s.UpsertTemplateAnswers(ctx, "p", []post.TemplateAnswer{{Label: "Facts", Text: "Owner answer", Enabled: true}}, testNow); err != nil {
		t.Fatal(err)
	}
	run := requestRun(t, s, "p", "job-1")
	value := requestCaptureFixture()
	for _, call := range []post.RequestCaptureCall{{ID: "second", Sequence: 2}, {ID: "first", Sequence: 1}} {
		if err := s.WritePostRequestCapture(ctx, run, call, value); err != nil {
			t.Fatal(err)
		}
	}
	// Pure inspection reads cannot change even timestamps or private row count.
	before, err := s.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		calls, err := s.ReadPostRequestCaptures(ctx, "alice", "p", "post-writing")
		if err != nil {
			t.Fatal(err)
		}
		if len(calls) != 2 || calls[0].CallID != "first" || calls[1].CallID != "second" || calls[0].Fragments[0].Text != value.Fragments[0].Text || !reflect.DeepEqual(calls[0].Omissions, value.Omissions) || calls[0].Conditions.MaxCompletionTokens == nil || *calls[0].Conditions.MaxCompletionTokens != 4096 {
			t.Fatalf("ordered exact private calls lost: %+v", calls)
		}
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("read mutated post")
	}
	for _, check := range []struct {
		user, slug string
		want       error
	}{{"bob", "p", post.ErrForbidden}, {"alice", "missing", post.ErrNotFound}} {
		if _, err := s.ReadPostRequestCaptures(ctx, check.user, check.slug, "unknown"); !errors.Is(err, check.want) {
			t.Fatalf("owner-before-stage: %v", err)
		}
	}
	// Internal observation updates move input revision without replacing sources.
	if _, err := handle.Writer.ExecContext(ctx, "UPDATE posts SET input_revision=input_revision+1 WHERE slug='p'"); err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionCaptured {
		t.Fatalf("own observer hid call: %+v", got)
	}
	if _, err := s.UpdateDraft(ctx, "p", "alice", before.Title, "Edited owner source", nil, testNow); err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable || len(got.Fragments) != 0 {
		t.Fatal("source edit exposed frozen call over new source")
	}
}

func TestRequestCaptureBindsOnlyExactCurrentResultAndManualPlan(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	run := requestRun(t, s, "p", "job-1")
	if err := s.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: "write", Sequence: 1}, requestCaptureFixture()); err != nil {
		t.Fatal(err)
	}
	content := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Final prose"}}}
	identity, err := s.PublishGeneratedResult(ctx, "alice", "p", post.GeneratedOriginResult{Content: content, Language: post.LanguageEnglish, ExpectedContentRevision: 0}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable {
		t.Fatal("unbound call exposed over published output")
	}
	wrong := identity
	wrong.ContentRevision++
	if err := s.FinishPostRequestCapture(ctx, run, &post.RequestCaptureCompletion{Result: &wrong}); !errors.Is(err, post.ErrStaleContentRevision) {
		t.Fatalf("wrong result bound: %v", err)
	}
	plan := post.StorylineFingerprint(nil)
	if err := s.FinishPostRequestCapture(ctx, run, &post.RequestCaptureCompletion{Result: &identity, PlanFingerprint: &plan}); err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionCaptured {
		t.Fatal("exact current result missing")
	}
	manual := post.Storyline{EditedByHand: true, Paragraphs: []post.StorylineParagraph{{Text: "Edited plan"}}}
	if _, err := s.UpdateStoryline(ctx, "p", "alice", &manual, testNow); err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable {
		t.Fatal("manual plan retargeted prior request")
	}
}

func TestRequestCaptureFailedCallUnavailableMarkersLegacyAndPurgeNeverRestore(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable || got.Conditions != nil {
		t.Fatal("legacy request reconstructed")
	}
	run := requestRun(t, s, "p", "failed-job")
	if err := s.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: "failed", Sequence: 1}, requestCaptureFixture()); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPostRequestCapture(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionCaptured {
		t.Fatal("actual failed invoke witness hidden")
	}
	if err := s.FinishPostRequestCapture(ctx, run, &post.RequestCaptureCompletion{UnavailableStages: []string{"post-writing"}, UnavailableReason: "capture_persistence_failed"}); err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable || got.UnavailableReason != "capture_persistence_failed" {
		t.Fatal("capture failure reused prior issued witness")
	}
	if err := s.PurgePostRequestCaptures(ctx, "alice", "p"); err != nil {
		t.Fatal(err)
	}
	if err := s.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: "late", Sequence: 2}, requestCaptureFixture()); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatalf("purged run restored: %v", err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable || len(got.Fragments) != 0 {
		t.Fatal("purge repaired history")
	}
}

func TestRequestCaptureAttachmentWithdrawalAndAccountCascadeErasePayload(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	if err := s.CreateImage(ctx, post.Image{ID: "incarnation", PostSlug: "p", Filename: "same.jpg", Key: "private-key", Width: 10, Height: 10, Bytes: 10, CreatedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	run := requestRun(t, s, "p", "photo-job")
	call := post.RequestCaptureCall{ID: "observe", Sequence: 1, AttachmentIDs: []string{"incarnation"}, AttachmentKinds: []string{"photo"}}
	value := requestCaptureFixture()
	value.Stage = "post-observation"
	if err := s.WritePostRequestCapture(ctx, run, call, value); err != nil {
		t.Fatal(err)
	}
	if removed, err := s.DeleteImage(ctx, "incarnation"); err != nil || !removed {
		t.Fatalf("delete: %v", err)
	}
	var count int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM post_request_captures WHERE post_slug='p'").Scan(&count); err != nil || count != 0 {
		t.Fatal("withdrawn attachment prompt retained")
	}
	if err := s.CreateImage(ctx, post.Image{ID: "replacement", PostSlug: "p", Filename: "same.jpg", Key: "replacement-key", Width: 10, Height: 10, Bytes: 10, CreatedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	if err := s.WritePostRequestCapture(ctx, run, call, value); !errors.Is(err, post.ErrStaleContentRevision) {
		t.Fatalf("same filename retargeted: %v", err)
	}
	run = requestRun(t, s, "p", "new-job")
	if err := s.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: "write"}, requestCaptureFixture()); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, "DELETE FROM users WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM post_request_captures").Scan(&count); err != nil || count != 0 {
		t.Fatal("account delete kept private prompt")
	}
}

func TestRequestCapturePostDeletionAndMalformedPayloadCannotRestoreHistory(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	run := requestRun(t, s, "p", "job")
	if err := s.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: "write"}, requestCaptureFixture()); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, "UPDATE post_request_captures SET payload='{invalid-json' WHERE post_slug='p'"); err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable || len(got.Fragments) != 0 {
		t.Fatal("malformed persisted capture repaired from current rows")
	}
	if err := s.PurgePostRequestCaptures(ctx, "alice", "p"); err != nil {
		t.Fatal(err)
	}
	if deleted, err := s.DeletePost(ctx, "p", "alice"); err != nil || !deleted {
		t.Fatalf("delete post: %v", err)
	}
	for _, table := range []string{"post_request_captures", "post_request_capture_purges"} {
		var count int
		if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s cascade: count=%d err=%v", table, count, err)
		}
	}
	if err := s.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: "late"}, requestCaptureFixture()); !errors.Is(err, post.ErrNotFound) {
		t.Fatalf("deleted post late restore: %v", err)
	}
}

func TestRequestCaptureUnboundFailedWriterFencesManualPlanChanges(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	run := requestRun(t, s, "p", "failed-job")
	if err := s.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: "write"}, requestCaptureFixture()); err != nil {
		t.Fatal(err)
	}
	manual := post.Storyline{EditedByHand: true, Paragraphs: []post.StorylineParagraph{{Text: "New owner plan"}}}
	if _, err := s.UpdateStoryline(ctx, "p", "alice", &manual, testNow); err != nil {
		t.Fatal(err)
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable {
		t.Fatal("failed writer exposed over edited current plan")
	}
}

func TestRequestCapturePrivacyPurgeStillErasesPublishedPrivatePayload(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	run := requestRun(t, s, "p", "job")
	if err := s.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: "write"}, requestCaptureFixture()); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, "UPDATE posts SET status='published' WHERE slug='p'"); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPostRequestCapture(ctx, run, &post.RequestCaptureCompletion{UnavailableStages: []string{"post-writing"}, UnavailableReason: "capture_persistence_failed"}); !errors.Is(err, post.ErrPostPublished) {
		t.Fatalf("published binding: %v", err)
	}
	if err := s.PurgePostRequestCaptures(ctx, "alice", "p"); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("private erasure altered published canonical result")
	}
	if got := capturedPost(t, s, "p"); got.Status != llm.InspectionUnavailable {
		t.Fatal("published private payload survived purge")
	}
}

package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/platform/db"
)

// seedVideoTemplates gives alice two video templates, the scope a clip guideline links.
func seedVideoTemplates(t *testing.T, handle *db.DB) {
	t.Helper()
	stamp := testNow.UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice-v1", "alice-v2"} {
		if _, err := handle.Writer.Exec(
			"INSERT INTO video_templates(id,user_id,name,created_at,updated_at) VALUES(?,?,?,?,?)",
			id, "alice", id, stamp, stamp); err != nil {
			t.Fatalf("seed video template: %v", err)
		}
	}
}

func clipGuideline(id, text string, scope guideline.Scope, at time.Time, videoTemplates ...string) guideline.Guideline {
	g := newGuideline(id, "alice", text, scope, at, videoTemplates...)
	g.Kind = guideline.KindClip
	return g
}

// GUIDE-5: the cap and the duplicate rule count within a kind; a 영상 지침 may hold a 지침's text.
func TestTheCapAndTheDuplicateRuleCountWithinAKind(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("p1", "alice", "짧게", guideline.ScopeGlobal, testNow), 1, guideline.CandidateApproval{}); err != nil {
		t.Fatal(err)
	}
	// The post cap is full, and the clip kind still has room — for the same text.
	if err := s.Insert(ctx, clipGuideline("c1", "짧게", guideline.ScopeGlobal, testNow), 1, guideline.CandidateApproval{}); err != nil {
		t.Fatalf("a clip guideline with a post guideline's text: %v", err)
	}
	var capErr *guideline.AccountCapError
	if err := s.Insert(ctx, clipGuideline("c2", "길게", guideline.ScopeGlobal, testNow), 1, guideline.CandidateApproval{}); !errors.As(err, &capErr) {
		t.Fatalf("a second clip guideline past a cap of one: %v", err)
	}
	if err := s.Insert(ctx, clipGuideline("c3", "짧게", guideline.ScopeGlobal, testNow), 5, guideline.CandidateApproval{}); !errors.Is(err, guideline.ErrDuplicateText) {
		t.Fatalf("the same clip text twice: %v", err)
	}
	posts, err := s.List(ctx, "alice", guideline.KindPost)
	if err != nil || len(posts) != 1 || posts[0].ID != "p1" || posts[0].Kind != guideline.KindPost {
		t.Fatalf("post list = %+v, %v", posts, err)
	}
	clips, err := s.List(ctx, "alice", guideline.KindClip)
	if err != nil || len(clips) != 1 || clips[0].ID != "c1" || clips[0].Kind != guideline.KindClip {
		t.Fatalf("clip list = %+v, %v", clips, err)
	}
}

// GUIDE-14: a clip resolves its global clip guidelines, then those linked to its video template;
// deleting the video template cascades the link and leaves the guideline 적용 대상 없음.
func TestAClipResolvesItsGuidelinesAndAVideoTemplateDeleteCascades(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	seedVideoTemplates(t, handle)
	for _, g := range []guideline.Guideline{
		clipGuideline("c-scoped", "가격은 크게", guideline.ScopeTemplates, testNow, "alice-v1"),
		clipGuideline("c-global", "자막은 짧게", guideline.ScopeGlobal, testNow.Add(time.Minute)),
		clipGuideline("c-other", "배경음 없이", guideline.ScopeTemplates, testNow, "alice-v2"),
		newGuideline("p-global", "alice", "없는 사실 금지", guideline.ScopeGlobal, testNow),
	} {
		if err := s.Insert(ctx, g, 10, guideline.CandidateApproval{}); err != nil {
			t.Fatalf("insert %s: %v", g.ID, err)
		}
	}
	texts, err := s.ClipApplicableTexts(ctx, "alice", "alice-v1")
	if err != nil || !reflect.DeepEqual(texts, []string{"자막은 짧게", "가격은 크게"}) {
		t.Fatalf("clip texts = %v, %v", texts, err)
	}
	if texts, _ := s.ClipApplicableTexts(ctx, "alice", ""); !reflect.DeepEqual(texts, []string{"자막은 짧게"}) {
		t.Fatalf("a clip with no video template = %v", texts)
	}
	if texts, _ := s.ApplicableTexts(ctx, "alice", "", ""); !reflect.DeepEqual(texts, []string{"없는 사실 금지"}) {
		t.Fatalf("a post reads clip rows: %v", texts)
	}
	scoped, err := s.Get(ctx, "alice", "c-scoped")
	if err != nil || !reflect.DeepEqual(scoped.TemplateIDs, []string{"alice-v1"}) {
		t.Fatalf("scoped clip guideline = %+v, %v", scoped, err)
	}
	if _, err := handle.Writer.Exec("DELETE FROM video_templates WHERE id='alice-v1'"); err != nil {
		t.Fatal(err)
	}
	orphan, err := s.Get(ctx, "alice", "c-scoped")
	if err != nil || orphan.Scope != guideline.ScopeTemplates || len(orphan.TemplateIDs) != 0 {
		t.Fatalf("after the video template delete = %+v, %v", orphan, err)
	}
	// A rescope names video templates too, and a post template is not one.
	two := guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-v2"}}
	if _, err := s.Update(ctx, "alice", "c-scoped", guideline.Patch{Scope: &two}, testNow); err != nil {
		t.Fatal(err)
	}
	wrong := guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-p1"}}
	if _, err := s.Update(ctx, "alice", "c-scoped", guideline.Patch{Scope: &wrong}, testNow); !errors.Is(err, guideline.ErrTemplateNotFound) {
		t.Fatalf("a post template on a clip guideline: %v", err)
	}
}

// GUIDE-10, GUIDE-13: candidates dedupe and queue within their kind, a clip candidate names its
// project, and a project delete drops the link.
func TestCandidatesQueueWithinTheirKind(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	record := func(kind guideline.Kind, id, text, source string) bool {
		t.Helper()
		c := guideline.Candidate{ID: id, UserID: "alice", Kind: kind, Text: text, Status: guideline.CandidateStatusPending,
			Occurrences: 1, FirstSeenAt: testNow, LastSeenAt: testNow}
		if kind == guideline.KindClip {
			c.ClipID = source
		} else {
			c.PostSlug = source
		}
		recorded, err := s.RecordCandidate(ctx, c, 1)
		if err != nil {
			t.Fatal(err)
		}
		return recorded
	}
	if !record(guideline.KindPost, "p", "짧게", "post-1") || !record(guideline.KindClip, "c", "짧게", "project-1") {
		t.Fatal("the same text did not record once per kind")
	}
	// Each kind's queue is full at one; a second clip text is skipped, the first counts up.
	if record(guideline.KindClip, "c2", "길게", "project-1") {
		t.Fatal("a full clip queue took another candidate")
	}
	record(guideline.KindClip, "c3", "짧게", "project-2")
	clips, held, err := s.ListPendingCandidates(ctx, "alice", guideline.KindClip)
	if err != nil || held != 1 || clips[0].Kind != guideline.KindClip || clips[0].ClipID != "project-1" || clips[0].Occurrences != 2 {
		t.Fatalf("clip queue = %+v (%d), %v", clips, held, err)
	}
	posts, _, _ := s.ListPendingCandidates(ctx, "alice", guideline.KindPost)
	if len(posts) != 1 || posts[0].PostSlug != "post-1" {
		t.Fatalf("post queue = %+v", posts)
	}
	if err := s.DropCandidateClipID(ctx, "alice", "project-1"); err != nil {
		t.Fatal(err)
	}
	clips, _, _ = s.ListPendingCandidates(ctx, "alice", guideline.KindClip)
	if clips[0].ClipID != "" || clips[0].Text != "짧게" {
		t.Fatalf("after the project delete = %+v", clips[0])
	}
	// A clip guideline approves the clip candidate by id; a post guideline cannot.
	postGuideline := newGuideline("pg", "alice", "다른 글", guideline.ScopeGlobal, testNow)
	if err := s.Insert(ctx, postGuideline, 10, guideline.CandidateApproval{ID: clips[0].ID}); !errors.Is(err, guideline.ErrCandidateNotFound) {
		t.Fatalf("a post guideline approved a clip candidate: %v", err)
	}
	if err := s.Insert(ctx, clipGuideline("cg", "자막은 짧게", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{ID: clips[0].ID}); err != nil {
		t.Fatal(err)
	}
	if clips, _, _ := s.ListPendingCandidates(ctx, "alice", guideline.KindClip); len(clips) != 0 {
		t.Fatalf("the approved clip candidate is still pending: %+v", clips)
	}
}

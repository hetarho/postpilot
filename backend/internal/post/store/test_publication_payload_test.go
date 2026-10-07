package store_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store"
)

type publicationPayloadGuard struct {
	available bool
	calls     int
}

func (g *publicationPayloadGuard) TestPayloadAvailable(_ context.Context, tx *sql.Tx, user, test, winner string, fence uint64) (bool, error) {
	g.calls++
	if tx == nil || user != "alice" || test != "test" || winner != "champion" || fence != 0 {
		return false, nil
	}
	return g.available, nil
}

func TestPrivateTestPublicationPurgeFenceAndOriginHandoffAreAtomic(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	in := frozenPublication(t, s)
	fence := uint64(0)
	in.PrivatePayloadFence = &fence
	in.Origins = &post.OriginReview{Version: 1, Result: post.ContentOriginIdentity(in.Content, 0), Sources: []post.OriginSource{{ID: "memo", Kind: post.OriginSourceMemo, Text: "frozen input", Available: true}}, Spans: []post.OriginSpan{{Field: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, Start: 0, End: 6, Quote: "Winner", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}, ReviewState: post.OriginUnreviewed}}}
	in.Storyline.Origins = &post.PlanOriginReview{Version: 1, Result: post.PlanOriginIdentity(in.Storyline.Paragraphs), Sources: []post.OriginSource{{ID: "proposal", Kind: post.OriginSourceAIProposal, Text: "frozen plan", Available: true}}, Spans: []post.PlanOriginSpan{{ParagraphIndex: 0, Start: 0, End: 13, Quote: "Winner's plan", Category: post.OriginAIAdded, SourceRefs: []string{"proposal"}, ReviewState: post.OriginUnreviewed}}}
	before, _ := s.GetPost(context.Background(), "p")
	guard := &publicationPayloadGuard{}
	pub := store.NewFencedTestResultStore(handle.Writer, handle.Reader, publicationJobGuard{}, guard)
	if _, err := pub.ApplyTestResult(context.Background(), in, testNow); !errors.Is(err, post.ErrTestPublicationConflict) {
		t.Fatalf("purged publication accepted: %v", err)
	}
	after, _ := s.GetPost(context.Background(), "p")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed private fence changed canonical result")
	}
	guard.available = true
	receipt, err := pub.ApplyTestResult(context.Background(), in, testNow)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.GetPost(context.Background(), "p")
	if p.Status != post.StatusReview || p.MachineBaselineRevision != p.ContentRevision || p.ContentOrigins == nil || p.ContentOrigins.Result != post.ContentOriginIdentity(*p.Content, p.ContentRevision) || len(p.ContentOrigins.Spans) != 1 || p.Storyline.Origins == nil || len(p.Storyline.Origins.Spans) != 1 {
		t.Fatalf("partial content/plan/baseline/origin publication: %#v", p)
	}
	var same bool
	if err := handle.Reader.QueryRow("SELECT content=machine_baseline FROM posts WHERE slug=?", "p").Scan(&same); err != nil || !same {
		t.Fatalf("persisted baseline differs: %v", err)
	}
	guard.available = false
	manual := post.PostContent{Title: "Later manual", Blocks: []post.Block{{Type: post.BlockText, Content: "later manual"}}}
	if ok, err := s.SaveContent(context.Background(), "p", "alice", manual, p.ContentRevision, testNow.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("manual: %v %v", ok, err)
	}
	again, err := pub.ApplyTestResult(context.Background(), in, testNow.Add(time.Hour))
	if err != nil || again != receipt || guard.calls != 2 {
		t.Fatalf("lost response needs private payload again: %v %#v %d", err, again, guard.calls)
	}
	current, _ := s.GetPost(context.Background(), "p")
	if !reflect.DeepEqual(*current.Content, manual) {
		t.Fatal("duplicate champion restored old content")
	}
}

func TestPrivateTestPublicationRequiresConfiguredPayloadGuardAndBaselineEquality(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	in := frozenPublication(t, s)
	fence := uint64(0)
	in.PrivatePayloadFence = &fence
	if _, err := publicationStore(handle).ApplyTestResult(context.Background(), in, testNow); !errors.Is(err, post.ErrTestPublicationConflict) {
		t.Fatalf("missing private guard permits publication: %v", err)
	}
	in.Baseline.Title = "different baseline"
	pub := store.NewFencedTestResultStore(handle.Writer, handle.Reader, publicationJobGuard{}, &publicationPayloadGuard{available: true})
	if _, err := pub.ApplyTestResult(context.Background(), in, testNow); !errors.Is(err, post.ErrInvalidContent) {
		t.Fatalf("unequal private baseline accepted: %v", err)
	}
}

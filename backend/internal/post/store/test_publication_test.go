package store_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store"
)

type publicationJobGuard struct{}

func (publicationJobGuard) HasOrdinaryWrite(ctx context.Context, tx *sql.Tx, userID, slug string) (bool, error) {
	row, err := jobstore.NewTx(tx, jobstore.Kinds{}).ActiveFor(ctx, job.Subject{Dimension: post.JobSubject, ID: slug}, job.Filter{UserID: userID})
	if row == nil {
		return false, err
	}
	switch row.Kind {
	case job.KindGenerate, job.KindRevise, job.KindStoryline, job.KindReviseStoryline:
		return true, err
	}
	return false, err
}

func frozenPublication(t *testing.T, s *store.Store) post.TestOutputPublication {
	t.Helper()
	p, err := s.GetPost(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	content := post.PostContent{Title: "Winner", Blocks: []post.Block{{Type: post.BlockText, Content: "Winner's complete output"}}}
	return post.TestOutputPublication{UserID: "alice", TestID: "test", WinnerID: "champion", RequestKey: "request", PostSlug: "p",
		InputRevision: p.InputRevision, ContentRevision: p.ContentRevision, AssignmentsHash: post.TestAssignmentsHash(p),
		Content: content, Baseline: content, ContentLanguage: post.LanguageKorean, Nouns: []string{"winner"},
		Storyline: &post.Storyline{Paragraphs: []post.StorylineParagraph{{Text: "Winner's plan"}}}}
}

func publicationStore(s *db.DB) *store.TestResultStore {
	return store.NewTestResultStore(s.Writer, s.Reader, publicationJobGuard{})
}

func TestTestOutputReceiptReplaysAfterLaterEditsAndTargetDeletion(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	in := frozenPublication(t, s)
	publication := post.NewTestResultService(publicationStore(handle))
	ctx := context.Background()
	first, err := publication.ApplyTestResult(ctx, in)
	if err != nil || first.ResultingRevision != 1 || first.Action != "apply_output" {
		t.Fatalf("first = %+v, %v", first, err)
	}
	p, _ := s.GetPost(ctx, "p")
	if p.Status != post.StatusReview || p.ContentRevision != 1 || p.MachineBaselineRevision != 1 || p.Storyline == nil || !reflect.DeepEqual(p.ContentNouns, in.Nouns) {
		t.Fatalf("incomplete publication: %+v", p)
	}
	manual := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Later human edit"}}}
	if ok, err := s.SaveContent(ctx, "p", "alice", manual, 1, testNow.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("manual edit = %v, %v", ok, err)
	}
	if ok, err := s.UpdateDraft(ctx, "p", "alice", "Later title", "Later memo", nil, testNow.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("material edit = %v, %v", ok, err)
	}
	again, err := publication.ApplyTestResult(ctx, in)
	if err != nil || again != first {
		t.Fatalf("lost response replay = %+v, %v", again, err)
	}
	p, _ = s.GetPost(ctx, "p")
	if p.ContentRevision != 2 || !reflect.DeepEqual(*p.Content, manual) || p.Title != "Later title" {
		t.Fatalf("replay overwrote a later edit: %+v", p)
	}
	if ok, err := s.DeletePost(ctx, "p", "alice"); err != nil || !ok {
		t.Fatalf("delete = %v, %v", ok, err)
	}
	if again, err := publication.ApplyTestResult(ctx, in); err != nil || again != first {
		t.Fatalf("deleted-target replay = %+v, %v", again, err)
	}
	in.Content.Title = "Other operation"
	if _, err := publication.ApplyTestResult(ctx, in); !errors.Is(err, post.ErrTestPublicationConflict) {
		t.Fatalf("conflicting replay = %v", err)
	}
}

func TestTestOutputPublicationRefusesStaleAssignmentsOwnerAndJobsWithoutWrites(t *testing.T) {
	for _, name := range []string{"input", "content", "assignments", "finalized", "published", "foreign", "ordinary job", "guard failure"} {
		t.Run(name, func(t *testing.T) {
			s, handle := newStoreWithHandle(t)
			seedPost(t, s, "p", "alice", testNow)
			in := frozenPublication(t, s)
			pub := publicationStore(handle)
			want := post.ErrTestPublicationConflict
			switch name {
			case "input":
				in.InputRevision++
			case "content":
				in.ContentRevision++
			case "assignments":
				in.AssignmentsHash = "foreign assignments"
			case "finalized":
				if _, err := handle.Writer.Exec("UPDATE posts SET status='finalized' WHERE slug='p'"); err != nil {
					t.Fatal(err)
				}
			case "published":
				if _, err := handle.Writer.Exec("UPDATE posts SET status='published' WHERE slug='p'"); err != nil {
					t.Fatal(err)
				}
				want = post.ErrPostPublished
			case "foreign":
				in.UserID = "bob"
				want = post.ErrForbidden
			case "ordinary job":
				jobs := jobstore.New(handle.Writer, handle.Reader, jobstore.Kinds{})
				if err := jobs.Insert(context.Background(), job.Job{ID: "ordinary", UserID: "alice", Kind: job.KindGenerate, Subjects: []job.Subject{{Dimension: post.JobSubject, ID: "p"}}, CreatedAt: testNow, UpdatedAt: testNow}); err != nil {
					t.Fatal(err)
				}
				want = post.ErrPostBusy
			case "guard failure":
				pub = store.NewTestResultStore(handle.Writer, handle.Reader, failingPublicationGuard{})
				want = errPublicationGuard
			}
			before, _ := s.GetPost(context.Background(), "p")
			if _, err := pub.ApplyTestResult(context.Background(), in, testNow); !errors.Is(err, want) {
				t.Fatalf("refusal = %v, want %v", err, want)
			}
			after, _ := s.GetPost(context.Background(), "p")
			if !reflect.DeepEqual(before, after) {
				t.Fatal("refusal changed the post")
			}
			var receipts int
			if err := handle.Reader.QueryRow("SELECT count(*) FROM post_test_publications").Scan(&receipts); err != nil || receipts != 0 {
				t.Fatalf("receipts = %d, %v", receipts, err)
			}
		})
	}
}

var errPublicationGuard = errors.New("job guard unavailable")

type failingPublicationGuard struct{}

func (failingPublicationGuard) HasOrdinaryWrite(context.Context, *sql.Tx, string, string) (bool, error) {
	return false, errPublicationGuard
}

func TestTestOutputReceiptFailureRollsBackTheWholePublication(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	before, _ := s.GetPost(context.Background(), "p")
	if _, err := handle.Writer.Exec("CREATE TRIGGER reject_test_receipt BEFORE INSERT ON post_test_publications BEGIN SELECT RAISE(ABORT, 'receipt unavailable'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := publicationStore(handle).ApplyTestResult(context.Background(), frozenPublication(t, s), testNow); err == nil {
		t.Fatal("receipt failure succeeded")
	}
	after, _ := s.GetPost(context.Background(), "p")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("receipt failure partially wrote the winner")
	}
}

func TestTestOutputPublicationAndManualEditShareRevisionFence(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	base := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Base"}}}
	if _, err := s.UpdateGeneratedContent(context.Background(), "p", "alice", base, post.LanguageKorean, post.WriteAnnotations{}, testNow); err != nil {
		t.Fatal(err)
	}
	in := frozenPublication(t, s)
	manual := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Concurrent human edit"}}}
	var group sync.WaitGroup
	group.Add(2)
	var published bool
	var edited bool
	var publicationErr, editErr error
	go func() {
		defer group.Done()
		_, publicationErr = publicationStore(handle).ApplyTestResult(context.Background(), in, testNow)
		published = publicationErr == nil
	}()
	go func() {
		defer group.Done()
		edited, editErr = s.SaveContent(context.Background(), "p", "alice", manual, 1, testNow)
	}()
	group.Wait()
	if editErr != nil || (publicationErr != nil && !errors.Is(publicationErr, post.ErrTestPublicationConflict)) {
		t.Fatalf("publication %v, edit %v", publicationErr, editErr)
	}
	if published == edited {
		t.Fatalf("exactly one must win: publication %v, edit %v", published, edited)
	}
	p, _ := s.GetPost(context.Background(), "p")
	if p.ContentRevision != 2 {
		t.Fatalf("revision = %d", p.ContentRevision)
	}
	if edited && !reflect.DeepEqual(*p.Content, manual) {
		t.Fatal("stale publication overwrote concurrent human edit")
	}
}

func TestTestOutputRejectsInvalidBaselineAndStorylineWithoutReceipt(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	in := frozenPublication(t, s)
	publication := publicationStore(handle)
	bad := in
	bad.Baseline = post.PostContent{}
	if _, err := publication.ApplyTestResult(context.Background(), bad, testNow); !errors.Is(err, post.ErrInvalidContent) {
		t.Fatalf("invalid baseline = %v", err)
	}
	bad = in
	bad.Storyline = &post.Storyline{MadeWith: []string{"missing.jpg"}}
	if _, err := publication.ApplyTestResult(context.Background(), bad, testNow); err == nil {
		t.Fatal("detached storyline accepted")
	}
	var receipts int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM post_test_publications").Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("refused receipts = %d, %v", receipts, err)
	}
	p, _ := s.GetPost(context.Background(), "p")
	if p.ContentRevision != 0 || p.Content != nil || p.Storyline != nil {
		t.Fatal("invalid winner changed the post")
	}
}

func TestTestPublicationStoreRequiresTransactionalJobGuard(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("missing job guard silently allowed publication")
		}
	}()
	store.NewTestResultStore(nil, nil, nil)
}

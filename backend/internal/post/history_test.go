package post

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type historyJobs struct {
	fakeActiveJobs
	latest map[string]ActiveJob
	err    error
	calls  int
	user   string
	slugs  []string
}

func (h *historyJobs) LatestOrdinaryForPosts(_ context.Context, user string, slugs []string) (map[string]ActiveJob, error) {
	h.calls++
	h.user, h.slugs = user, slugs
	return h.latest, h.err
}

func TestHistoryReadsLatestOrdinaryJobsOnceAndSuccessMasksOldFailures(t *testing.T) {
	svc, store, _ := newTestService(t)
	seedListed(store, alice, "success", "Success", StatusReview, testNow)
	seedListed(store, alice, "failure", "Failed", StatusDraft, testNow)
	// The published bulk port returns the latest relevant job, including a later
	// success. Filtering failures before choosing latest would leak an older failure.
	jobs := &historyJobs{latest: map[string]ActiveJob{
		"success": {ID: "new-success", Status: "done"},
		"failure": {ID: "latest-failure", Status: "failed", Failure: &Failure{Reason: "PROVIDER_FAILED"}},
	}}
	svc.jobs = jobs
	page, err := svc.List(context.Background(), alice, ListQuery{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if jobs.calls != 1 || jobs.user != alice || !reflect.DeepEqual(jobs.slugs, []string{"success", "failure"}) {
		t.Fatalf("bulk read = %+v", jobs)
	}
	for _, row := range page.Summaries {
		if row.Slug == "success" && row.LatestOrdinaryFailure != nil {
			t.Fatal("success surfaced an older failure")
		}
		if row.Slug == "failure" && (row.LatestOrdinaryFailure == nil || row.LatestOrdinaryFailure.ID != "latest-failure") {
			t.Fatal("latest ordinary failure was lost")
		}
	}
}

func TestHistoryBulkFailurePropagatesAndEmptyPageDoesNotReadJobs(t *testing.T) {
	svc, store, _ := newTestService(t)
	failure := errors.New("job history unavailable")
	jobs := &historyJobs{err: failure}
	svc.jobs = jobs
	if page, err := svc.List(context.Background(), alice, ListQuery{}); err != nil || len(page.Summaries) != 0 || jobs.calls != 0 {
		t.Fatalf("empty page = %+v, %v, calls %d", page, err, jobs.calls)
	}
	seedListed(store, alice, "p", "P", StatusDraft, testNow)
	if _, err := svc.List(context.Background(), alice, ListQuery{}); !errors.Is(err, failure) {
		t.Fatalf("history failure = %v", err)
	}
}

func TestReadModelsHideActiveTestsWhilePrivacyDeletionRemainsGuarded(t *testing.T) {
	svc, _, _ := newTestService(t)
	p := mustCreatePost(t, svc, alice, "Post")
	if err := svc.SetStoryline(context.Background(), alice, p.Slug, Storyline{Paragraphs: []StorylineParagraph{{Text: "Plan"}}}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"model_experiment", "writing_test"} {
		svc.jobs = fakeActiveJobs{p.Slug: {ID: "test-job", Kind: kind, Status: "queued"}}
		found, err := svc.Get(context.Background(), alice, p.Slug)
		if err != nil || found.ActiveJob != nil {
			t.Fatalf("Get exposes %s as ordinary active work: %+v, %v", kind, found.ActiveJob, err)
		}
		page, err := svc.List(context.Background(), alice, ListQuery{})
		if err != nil || len(page.Summaries) != 1 || page.Summaries[0].ActiveJob != nil {
			t.Fatalf("List exposes %s as ordinary active work: %+v, %v", kind, page, err)
		}
		if _, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: p.Slug, Title: p.Title, Storyline: &StorylineEdit{Paragraphs: []StorylineParagraph{{Text: kind}}}}); err != nil {
			t.Fatalf("test froze future storyline input: %v", err)
		}
		if err := svc.DeletePost(context.Background(), alice, p.Slug); !errors.Is(err, ErrPostBusy) {
			t.Fatalf("privacy guard ignored %s: %v", kind, err)
		}
	}
	svc.jobs = fakeActiveJobs{p.Slug: {ID: "ordinary-job", Kind: "generate", Status: "running"}}
	if found, err := svc.Get(context.Background(), alice, p.Slug); err != nil || found.ActiveJob == nil {
		t.Fatalf("ordinary projection lost: %+v, %v", found, err)
	}
}

func TestOptionalBulkHistoryDoesNotInventFailures(t *testing.T) {
	svc, _, _ := newTestService(t)
	p := mustCreatePost(t, svc, alice, "Post")
	svc.jobs = fakeActiveJobs{p.Slug: {ID: "ordinary-job", Kind: "generate", Status: "running"}}
	page, err := svc.List(context.Background(), alice, ListQuery{})
	if err != nil || page.Summaries[0].LatestOrdinaryFailure != nil || page.Summaries[0].ActiveJob == nil {
		t.Fatalf("optional history = %+v, %v", page, err)
	}
}

type concurrentJobs struct {
	fakeActiveJobs
	ordinary *ActiveJob
}

func (j concurrentJobs) ActiveOrdinaryForPost(context.Context, string, string) (*ActiveJob, error) {
	return j.ordinary, nil
}

func TestFilteredOrdinaryPortPreventsANewerTestFromMaskingAnActiveWriter(t *testing.T) {
	svc, _, _ := newTestService(t)
	p := mustCreatePost(t, svc, alice, "Post")
	svc.jobs = concurrentJobs{fakeActiveJobs: fakeActiveJobs{p.Slug: {ID: "new-test", Kind: "writing_test", Status: "running"}},
		ordinary: &ActiveJob{ID: "older-writer", Kind: "generate", Status: "running", WritesContent: true}}
	if found, err := svc.Get(context.Background(), alice, p.Slug); err != nil || found.ActiveJob == nil || found.ActiveJob.ID != "older-writer" {
		t.Fatalf("masked ordinary job: %+v, %v", found.ActiveJob, err)
	}
	page, err := svc.List(context.Background(), alice, ListQuery{})
	if err != nil || page.Summaries[0].ActiveJob == nil || page.Summaries[0].ActiveJob.ID != "older-writer" {
		t.Fatalf("masked ordinary history: %+v, %v", page, err)
	}
	voice := aliceReview
	if _, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: p.Slug, VoiceID: &voice}); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("masked reassignment guard: %v", err)
	}
}

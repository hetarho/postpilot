package generation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// queuedObservation is the job the worker would run for the i-th StartObservation: the row's
// routing plus the payload exactly as it was encoded.
func (f *fakeJobs) queuedObservation(i int) ObservationJob {
	request := f.observations[i]
	return ObservationJob{
		ID: fmt.Sprintf("observe-%d", i), UserID: request.UserID, PostSlug: request.PostSlug,
		ObserveModel: request.ObserveModel, Payload: f.observationPayloads[i],
	}
}

func (f *fakeJobs) frozenObservation(t *testing.T, i int) observeOptions {
	t.Helper()
	options, err := decodeObservePayload(f.observationPayloads[i])
	if err != nil {
		t.Fatal(err)
	}
	return options
}

// GEN-89, GEN-9: on this RPC alone the selection keeps its presence — absent observes every
// attachment, present its intersection plus the ones with nothing to reuse — and the hold prices
// exactly that frozen set.
func TestStartObservationFreezesTheSelectionWithPresence(t *testing.T) {
	images, stored := storedSnapshot(15, "old/observer")
	// IMG_2 came back empty, so it has nothing to reuse and is always forced in.
	stored[1] = Observation{File: "IMG_2.jpg", Model: "old/observer"}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, Observations: stored}}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, newFakeModels(), fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget, testDeps())

	for index, test := range []struct {
		name      string
		requested *[]string
		want      []string
		wantCalls int
	}{
		{name: "absent observes everything", requested: nil, want: filenames(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15), wantCalls: 4},
		{name: "empty observes only what is missing", requested: stringPointer(), want: filenames(2), wantCalls: 1},
		{name: "picked plus forced, unknown dropped", requested: stringPointer("NOT_ATTACHED.jpg", "IMG_5.jpg", "IMG_1.jpg", "IMG_1.jpg"), want: filenames(1, 2, 5), wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			id, err := svc.StartObservation(context.Background(), StartObservationRequest{
				UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), ObserveFiles: test.requested,
			})
			if err != nil || id != "job" {
				t.Fatalf("start = %q, %v", id, err)
			}
			frozen := jobs.frozenObservation(t, index)
			if frozen.ObserveFiles == nil || !reflect.DeepEqual(*frozen.ObserveFiles, test.want) {
				t.Fatalf("frozen set = %v, want %v", frozen.ObserveFiles, test.want)
			}
			// The reusable snapshot: every stored entry but IMG_2's empty one.
			if len(frozen.Observations) != 14 {
				t.Fatalf("frozen snapshot holds %d entries, want the reusable 14", len(frozen.Observations))
			}
			request := jobs.observations[index]
			if request.ObserveCalls != test.wantCalls || request.ObserveModel != observeRef.String() {
				t.Fatalf("request = %+v, want %d calls", request, test.wantCalls)
			}
		})
	}
}

// GEN-89: nothing is written, so no write model, language or voice is a precondition — but an
// attached photo still needs an observe model, and the published lock and the active job hold.
func TestStartObservationPreconditions(t *testing.T) {
	images, _ := storedSnapshot(2, "old/observer")
	deleted := VoiceRef{ID: "voice-gone", Deleted: true}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: deleted, Images: images}, preserveMissingLanguages: true}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, newFakeModels(), fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget, testDeps())

	if _, err := svc.StartObservation(context.Background(), StartObservationRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String()}); err != nil {
		t.Fatalf("a post with a deleted voice and no language refused observation: %v", err)
	}
	if _, err := svc.StartObservation(context.Background(), StartObservationRequest{UserID: "alice", PostSlug: "post"}); !errors.Is(err, ErrObserveModelRequired) {
		t.Fatalf("no observe model: %v", err)
	}
	posts.input.Published = true
	if _, err := svc.StartObservation(context.Background(), StartObservationRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String()}); !errors.Is(err, ErrPostPublished) {
		t.Fatalf("published post: %v", err)
	}
	posts.input.Published = false
	jobs.err = &JobAlreadyInProgressError{ActiveID: "active-job"}
	var busy *JobAlreadyInProgressError
	if _, err := svc.StartObservation(context.Background(), StartObservationRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String()}); !errors.As(err, &busy) || busy.ActiveID != "active-job" {
		t.Fatalf("busy post: %v", err)
	}
	if jobs.enqueues != 2 {
		t.Fatalf("enqueues = %d, want the accepted start and the busy attempt", jobs.enqueues)
	}
}

// GEN-13: a post with no attachment plans no call and names no observe model, and its job makes
// no provider call and clears the stale snapshot.
func TestObservationOfAPostWithNothingAttachedClearsStaleObservations(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Observations: []Observation{{File: "GONE.jpg", Scene: "stale"}}}}
	models := observingModels(t)
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget, testDeps())

	if _, err := svc.StartObservation(context.Background(), StartObservationRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if request := jobs.observations[0]; request.ObserveModel != "" || request.ObserveCalls != 0 {
		t.Fatalf("a bare start planned %+v", request)
	}
	var progress []string
	if err := svc.Observe(context.Background(), jobs.queuedObservation(0), func(stage string, done, total int) {
		progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
	}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 0 {
		t.Fatalf("provider calls = %d, want 0", len(models.calls))
	}
	if len(posts.observationWrites) != 1 || len(posts.observationWrites[0]) != 0 {
		t.Fatalf("observations not cleared: %#v", posts.observationWrites)
	}
	if !reflect.DeepEqual(progress, []string{"observe:0/0"}) {
		t.Fatalf("progress = %v", progress)
	}
}

// GEN-11, GEN-79: the job persists the merged snapshot after every batch, carries the observed
// rotation to the post, and writes nothing but observations.
func TestObservationJobPersistsEachBatchAndWritesNothingElse(t *testing.T) {
	images, stored := storedSnapshot(6, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, Observations: stored}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if !request.HasImages() {
			t.Fatal("an observation job made a call that was not an observation")
		}
		parts := request.Messages[0].Parts
		files := strings.Split(strings.TrimPrefix(parts[len(parts)-1].Text, "files: "), ", ")
		items := make([]string, 0, len(files))
		for _, file := range files {
			items = append(items, fmt.Sprintf(`{"file":%q,"scene":"fresh","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":90}`, file))
		}
		return llm.Response{Text: `{"observations":[` + strings.Join(items, ",") + `]}`}, nil
	}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget, testDeps())

	selected := filenames(1, 2, 3, 4, 5)
	if _, err := svc.StartObservation(context.Background(), StartObservationRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), ObserveFiles: &selected}); err != nil {
		t.Fatal(err)
	}
	var progress []string
	if err := svc.Observe(context.Background(), jobs.queuedObservation(0), func(stage string, done, total int) {
		progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
	}); err != nil {
		t.Fatal(err)
	}
	if got := observeCallCount(models); got != 2 || len(models.calls) != 2 {
		t.Fatalf("calls = %d (%d observation), want 2 observation calls", len(models.calls), got)
	}
	if !reflect.DeepEqual(progress, []string{"observe:4/5", "observe:5/5"}) {
		t.Fatalf("progress = %v", progress)
	}
	if len(posts.observationWrites) != 2 {
		t.Fatalf("persists = %d, want one per batch", len(posts.observationWrites))
	}
	for i, write := range posts.observationWrites {
		if len(write) != 6 {
			t.Fatalf("persist %d holds %d entries, want the merged 6", i, len(write))
		}
	}
	final := posts.observationWrites[1]
	for i, observation := range final {
		if i < 5 && (observation.Scene != "fresh" || observation.Rotation != 90 || observation.Model != observeRef.String()) {
			t.Fatalf("re-observed entry %s = %+v", observation.File, observation)
		}
		if i == 5 && (observation.Scene != "stored-6" || observation.Rotation != 0) {
			t.Fatalf("unselected entry changed: %+v", observation)
		}
	}
	if len(posts.contents) != 0 || len(posts.storylines) != 0 {
		t.Fatalf("an observation job wrote content %+v or a storyline %+v", posts.contents, posts.storylines)
	}
}

// GEN-9: what the queued job observes was decided at enqueue; post edits afterwards cannot reach
// it, and a photo attached since then belongs to the next run.
func TestObservationJobObservesOnlyItsFrozenSet(t *testing.T) {
	images, stored := storedSnapshot(4, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, Observations: stored}}
	jobs := &fakeJobs{id: "job"}
	models := observingModels(t)
	svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget, testDeps())

	if _, err := svc.StartObservation(context.Background(), StartObservationRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), ObserveFiles: stringPointer("IMG_2.jpg"),
	}); err != nil {
		t.Fatal(err)
	}
	posts.input.Images = []Image{
		{Filename: "IMG_1.jpg", Key: "key-1"}, {Filename: "IMG_2.jpg", Key: "key-2"},
		{Filename: "IMG_3.jpg", Key: "key-3"}, {Filename: "IMG_5.jpg", Key: "key-5"},
	}
	posts.input.Observations = []Observation{{File: "IMG_1.jpg", Scene: "rewritten", Model: "other/observer"}}

	if err := svc.Observe(context.Background(), jobs.queuedObservation(0), func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if got := filesOfObserveCalls(models); !reflect.DeepEqual(got, [][]string{{"IMG_2.jpg"}}) {
		t.Fatalf("observed %v, want only the frozen IMG_2.jpg", got)
	}
	var got []string
	for _, observation := range posts.observationWrites[len(posts.observationWrites)-1] {
		got = append(got, observation.File+"="+observation.Scene)
	}
	if !reflect.DeepEqual(got, []string{"IMG_1.jpg=stored-1", "IMG_2.jpg=fresh", "IMG_3.jpg=stored-3"}) {
		t.Fatalf("persisted snapshot = %v", got)
	}

	// A post published while the job waited refuses it before any call.
	posts.input.Published = true
	before := len(models.calls)
	if err := svc.Observe(context.Background(), jobs.queuedObservation(0), func(string, int, int) {}); !errors.Is(err, ErrPostPublished) || len(models.calls) != before {
		t.Fatalf("published at dequeue: %v", err)
	}
}

// GEN-30: the payload keeps the selection's presence through encode and decode.
func TestObservePayloadPreservesPresence(t *testing.T) {
	for _, set := range []*[]string{nil, stringPointer(), stringPointer("IMG_1.jpg")} {
		raw, err := encodeObservePayload(observeOptions{ObserveFiles: set, Observations: []Observation{{File: "IMG_1.jpg", Scene: "s", Rotation: 180}}})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeObservePayload(raw)
		if err != nil {
			t.Fatal(err)
		}
		if (set == nil) != (decoded.ObserveFiles == nil) || (set != nil && !reflect.DeepEqual(*set, *decoded.ObserveFiles)) {
			t.Fatalf("presence lost: raw=%s decoded=%v", raw, decoded.ObserveFiles)
		}
		if len(decoded.Observations) != 1 || decoded.Observations[0].Rotation != 180 {
			t.Fatalf("snapshot = %+v", decoded.Observations)
		}
	}
	if _, err := decodeObservePayload([]byte(`{"origin_protocol_version":99,"observe_files":null}`)); err == nil {
		t.Fatal("an unknown origin protocol decoded")
	}
}

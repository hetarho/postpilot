package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

const reobserveBatchSize = 4

// storedSnapshot is a post that has already been observed once: one entry per photo, all
// produced by `oldModel`.
func storedSnapshot(count int, model string) (images []Image, observations []Observation) {
	for i := 1; i <= count; i++ {
		filename := fmt.Sprintf("IMG_%d.jpg", i)
		images = append(images, Image{Filename: filename, Key: fmt.Sprintf("key-%d", i)})
		observations = append(observations, Observation{
			File: filename, Scene: fmt.Sprintf("stored-%d", i), Mood: "차분", Model: model,
		})
	}
	return images, observations
}

func filenames(indexes ...int) []string {
	out := make([]string, 0, len(indexes))
	for _, i := range indexes {
		out = append(out, fmt.Sprintf("IMG_%d.jpg", i))
	}
	return out
}

// observingModels answers every observation call with a fresh scene per requested filename,
// and every write call with a minimal valid post.
func observingModels(t *testing.T) *fakeModels {
	t.Helper()
	models := newFakeModels()
	models.complete = func(ref llm.ModelRef, request llm.Request) (llm.Response, error) {
		if !request.HasImages() {
			return llm.Response{Text: `{"title":"t","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"ok"}]}`}, nil
		}
		files := strings.Split(strings.TrimPrefix(request.Messages[0].Parts[len(request.Messages[0].Parts)-1].Text, "files: "), ", ")
		items := make([]string, 0, len(files))
		for _, file := range files {
			items = append(items, fmt.Sprintf(`{"file":%q,"scene":"fresh","mood":"","visible_text":"","objects":[],"people_present":false}`, file))
		}
		return llm.Response{Text: `{"observations":[` + strings.Join(items, ",") + `]}`}, nil
	}
	return models
}

func observeCallCount(models *fakeModels) int {
	count := 0
	for _, call := range models.calls {
		if call.request.HasImages() {
			count++
		}
	}
	return count
}

func filesOfObserveCalls(models *fakeModels) [][]string {
	var out [][]string
	for _, call := range models.calls {
		if !call.request.HasImages() {
			continue
		}
		parts := call.request.Messages[0].Parts
		out = append(out, strings.Split(strings.TrimPrefix(parts[len(parts)-1].Text, "files: "), ", "))
	}
	return out
}

func stringPointer(values ...string) *[]string {
	out := append([]string{}, values...)
	return &out
}

// A2 · A9 — the common case: the writing stage failed, the photos did not change, the picker
// is confirmed untouched. No observation call, no snapshot write at all, and the observation
// stage reports complete before the writer starts.
func TestReuseEverythingMakesNoObservationCallAndLeavesTheSnapshotUntouched(t *testing.T) {
	images, stored := storedSnapshot(15, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images}}
	models := observingModels(t)
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, reobserveBatchSize, testReasoningPolicy, testBudget)

	var progress []string
	err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: stringPointer(), Observations: stored,
	}, func(stage string, done, total int) {
		progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := observeCallCount(models); got != 0 {
		t.Fatalf("observation calls = %d, want 0", got)
	}
	if len(posts.observationWrites) != 0 {
		t.Fatalf("the stored snapshot was rewritten: %#v", posts.observationWrites)
	}
	if !reflect.DeepEqual(progress, []string{"observe:0/0", "write:0/1", "write:1/1"}) {
		t.Fatalf("progress = %v", progress)
	}
	if len(posts.contents) != 1 {
		t.Fatalf("the writing stage did not run: %+v", posts.contents)
	}
}

// A2 — the reused observations are what the writer is actually given, not an empty set.
func TestReuseEverythingWritesFromTheStoredObservations(t *testing.T) {
	images, stored := storedSnapshot(3, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images}}
	models := observingModels(t)
	var writePrompt string
	models.complete = func(ref llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.HasImages() {
			t.Fatal("reuse-everything made an observation call")
		}
		writePrompt = request.Messages[0].Parts[0].Text
		return llm.Response{Text: `{"title":"t","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"ok"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, reobserveBatchSize, testReasoningPolicy, testBudget)
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: stringPointer(), Observations: stored,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if !strings.Contains(writePrompt, fmt.Sprintf("stored-%d", i)) {
			t.Fatalf("write prompt lost reused observation %d: %s", i, writePrompt)
		}
	}
}

// A3 · A4 — five of fifteen at batch size four: two calls, the snapshot complete from the
// FIRST persist, the ten unselected entries untouched and the five selected ones replaced.
func TestPartialReobservationReplacesOnlyTheSelectedEntries(t *testing.T) {
	images, stored := storedSnapshot(15, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images}}
	models := observingModels(t)
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, reobserveBatchSize, testReasoningPolicy, testBudget)

	selected := filenames(2, 5, 7, 11, 14)
	var progress []string
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: &selected, Observations: stored,
	}, func(stage string, done, total int) {
		progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
	}); err != nil {
		t.Fatal(err)
	}
	if got := observeCallCount(models); got != 2 {
		t.Fatalf("observation calls = %d, want 2 (5 photos at batch %d)", got, reobserveBatchSize)
	}
	// A9: the total is what is being observed, not what is attached.
	if !reflect.DeepEqual(progress, []string{"observe:4/5", "observe:5/5", "write:0/1", "write:1/1"}) {
		t.Fatalf("progress = %v", progress)
	}
	if len(posts.observationWrites) != 2 {
		t.Fatalf("persists = %d, want 2", len(posts.observationWrites))
	}
	// A4: complete already after the first batch — the contact sheet never shrinks.
	for i, write := range posts.observationWrites {
		if len(write) != 15 {
			t.Fatalf("persist %d holds %d entries, want 15", i, len(write))
		}
	}
	final := posts.observationWrites[len(posts.observationWrites)-1]
	reobserved := map[string]struct{}{}
	for _, file := range selected {
		reobserved[file] = struct{}{}
	}
	for i, observation := range final {
		want := fmt.Sprintf("stored-%d", i+1)
		wantModel := "old/observer"
		if _, ok := reobserved[observation.File]; ok {
			want, wantModel = "fresh", observeRef.String()
		}
		if observation.Scene != want || observation.Model != wantModel {
			t.Fatalf("entry %s = %+v, want scene %q by %q", observation.File, observation, want, wantModel)
		}
	}
}

// A5 (server side) — a photo with nothing to reuse is observed whatever the client asked for.
// The picker states the rule; the run enforces it, because writing from a photo nothing has
// ever looked at is the failure this whole change exists to avoid.
func TestStartForcesPhotosWithNothingToReuse(t *testing.T) {
	images, stored := storedSnapshot(3, "old/observer")
	// IMG_2 was observed and came back empty; IMG_4 was attached after the last observation.
	stored[1] = Observation{File: "IMG_2.jpg", Model: "old/observer"}
	images = append(images, Image{Filename: "IMG_4.jpg", Key: "key-4"})
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, Observations: stored}}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget)

	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: stringPointer(),
	}); err != nil {
		t.Fatal(err)
	}
	frozen := jobs.generations[0]
	if frozen.ObserveFiles == nil {
		t.Fatal("Start left the observation set unfrozen")
	}
	if !reflect.DeepEqual(*frozen.ObserveFiles, []string{"IMG_2.jpg", "IMG_4.jpg"}) {
		t.Fatalf("frozen set = %v, want the two photos with nothing to reuse", *frozen.ObserveFiles)
	}
	// T006: the credit hold prices the frozen set, not the attached count.
	if frozen.ObserveCalls != 1 {
		t.Fatalf("ObserveCalls = %d, want 1 (2 photos at batch %d)", frozen.ObserveCalls, reobserveBatchSize)
	}
	// The unknown name is dropped, exactly as an unattached observation would be.
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: stringPointer("NOT_ATTACHED.jpg", "IMG_1.jpg", "IMG_1.jpg"),
	}); err != nil {
		t.Fatal(err)
	}
	frozen = jobs.generations[1]
	if !reflect.DeepEqual(*frozen.ObserveFiles, []string{"IMG_1.jpg", "IMG_2.jpg", "IMG_4.jpg"}) {
		t.Fatalf("frozen set = %v, want the asked photo plus the two forced ones", *frozen.ObserveFiles)
	}
}

// T006 — the hold and the work must agree: reusing everything costs no observation call, so
// it must not be held for one either.
func TestStartPricesTheHoldOverTheFrozenSet(t *testing.T) {
	images, stored := storedSnapshot(15, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, Observations: stored}}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget)

	for _, test := range []struct {
		name      string
		requested *[]string
		wantCalls int
	}{
		{name: "reuse everything", requested: stringPointer(), wantCalls: 0},
		{name: "reuse ten of fifteen", requested: &[]string{"IMG_1.jpg", "IMG_2.jpg", "IMG_3.jpg", "IMG_4.jpg", "IMG_5.jpg"}, wantCalls: 2},
		{name: "no picker answer", requested: nil, wantCalls: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			jobs.generations = nil
			if _, err := svc.Start(context.Background(), StartRequest{
				UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
				ObserveFiles: test.requested,
			}); err != nil {
				t.Fatal(err)
			}
			if got := jobs.generations[0].ObserveCalls; got != test.wantCalls {
				t.Fatalf("ObserveCalls = %d, want %d", got, test.wantCalls)
			}
		})
	}
}

// A7 — the selection is frozen at enqueue. Attaching, deleting or switching the observation
// model after Start returns cannot change what the dequeued run observes.
func TestPostEditsAfterEnqueueCannotChangeWhatTheRunObserves(t *testing.T) {
	images, stored := storedSnapshot(4, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, Observations: stored}}
	jobs := &fakeJobs{id: "job"}
	models := observingModels(t)
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget)

	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: stringPointer("IMG_2.jpg"),
	}); err != nil {
		t.Fatal(err)
	}
	frozen := jobs.generations[0]

	// The post moves on while the job waits: a photo is added, another is deleted, and the
	// stored snapshot is replaced by a different model's work.
	posts.input.Images = []Image{
		{Filename: "IMG_1.jpg", Key: "key-1"}, {Filename: "IMG_2.jpg", Key: "key-2"},
		{Filename: "IMG_3.jpg", Key: "key-3"}, {Filename: "IMG_5.jpg", Key: "key-5"},
	}
	posts.input.Observations = []Observation{{File: "IMG_1.jpg", Scene: "rewritten", Model: "other/observer"}}

	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", ObserveModel: frozen.ObserveModel, WriteModel: frozen.WriteModel,
		ObserveFiles: frozen.ObserveFiles, Observations: frozen.Observations,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if got := filesOfObserveCalls(models); !reflect.DeepEqual(got, [][]string{{"IMG_2.jpg"}}) {
		t.Fatalf("observed %v, want only the frozen IMG_2.jpg", got)
	}
	final := posts.observationWrites[len(posts.observationWrites)-1]
	// IMG_4 was deleted so it drops out; IMG_5 arrived after the freeze, so this run neither
	// observes it nor invents an entry for it. IMG_1 and IMG_3 keep the FROZEN text, not the
	// text the post picked up afterwards.
	var got []string
	for _, observation := range final {
		got = append(got, observation.File+"="+observation.Scene)
	}
	if !reflect.DeepEqual(got, []string{"IMG_1.jpg=stored-1", "IMG_2.jpg=fresh", "IMG_3.jpg=stored-3"}) {
		t.Fatalf("persisted snapshot = %v", got)
	}
}

// A10 — the pre-picker path is untouched: a payload with no frozen set observes every attached
// photo in ceil(N/batch) calls, and the zero-photo path still clears the snapshot.
func TestAbsentFrozenSetObservesEveryPhoto(t *testing.T) {
	images, _ := storedSnapshot(9, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images}}
	models := observingModels(t)
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, reobserveBatchSize, testReasoningPolicy, testBudget)

	var progress []string
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	}, func(stage string, done, total int) {
		progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
	}); err != nil {
		t.Fatal(err)
	}
	if got := observeCallCount(models); got != 3 {
		t.Fatalf("observation calls = %d, want 3", got)
	}
	if !reflect.DeepEqual(progress, []string{"observe:4/9", "observe:8/9", "observe:9/9", "write:0/1", "write:1/1"}) {
		t.Fatalf("progress = %v", progress)
	}
	// Byte for byte the pre-change persist sequence: 4, then 8, then 9 entries.
	if len(posts.observationWrites) != 3 ||
		len(posts.observationWrites[0]) != 4 || len(posts.observationWrites[1]) != 8 || len(posts.observationWrites[2]) != 9 {
		t.Fatalf("incremental writes changed shape: %d %d %d",
			len(posts.observationWrites[0]), len(posts.observationWrites[1]), len(posts.observationWrites[2]))
	}
}

func TestZeroPhotoPathStillClearsTheSnapshot(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice}}
	models := observingModels(t)
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget)

	// Even a client that sends a picker answer for a photoless post freezes nothing.
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: stringPointer("IMG_1.jpg"),
	}); err != nil {
		t.Fatal(err)
	}
	if jobs.generations[0].ObserveFiles != nil || jobs.generations[0].ObserveCalls != 0 {
		t.Fatalf("zero-photo start froze a selection: %+v", jobs.generations[0])
	}
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if observeCallCount(models) != 0 {
		t.Fatal("zero-photo generation made an observation call")
	}
	if len(posts.observationWrites) != 1 || len(posts.observationWrites[0]) != 0 {
		t.Fatalf("observations not cleared: %#v", posts.observationWrites)
	}
}

// T004 — the payload must round-trip all THREE states of the frozen set. A plain []string with
// omitempty would collapse "observe nothing" into "absent", which decodes as "observe
// everything": the silent double-spend this contract exists to stop.
func TestGenerationPayloadPreservesFrozenSetPresence(t *testing.T) {
	for _, test := range []struct {
		name string
		set  *[]string
	}{
		{name: "absent"},
		{name: "present and empty", set: stringPointer()},
		{name: "present with names", set: stringPointer("IMG_1.jpg", "IMG_2.jpg")},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := EncodeGenerationPayload(GenerationOptions{
				TargetLanguage: LanguageKorean, ObserveFiles: test.set,
				Observations: []Observation{{File: "IMG_1.jpg", Scene: "s", Objects: []string{"o"}, Model: "old/observer"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeGenerationPayload(raw)
			if err != nil {
				t.Fatal(err)
			}
			if (test.set == nil) != (decoded.ObserveFiles == nil) {
				t.Fatalf("presence lost: raw=%s decoded=%v", raw, decoded.ObserveFiles)
			}
			if test.set != nil && !reflect.DeepEqual(*test.set, *decoded.ObserveFiles) {
				t.Fatalf("frozen set = %v, want %v", *decoded.ObserveFiles, *test.set)
			}
			if !reflect.DeepEqual(decoded.Observations, []Observation{
				{File: "IMG_1.jpg", Scene: "s", Objects: []string{"o"}, Model: "old/observer"},
			}) {
				t.Fatalf("carried observations = %+v", decoded.Observations)
			}
		})
	}
	// A payload written before this contract existed decodes as "observe everything".
	decoded, err := DecodeGenerationPayload([]byte(`{"target_language":"ko"}`))
	if err != nil || decoded.ObserveFiles != nil {
		t.Fatalf("legacy payload decoded to %v (err %v)", decoded.ObserveFiles, err)
	}
}

// A8 — the A/B write comparison shares the picker and the same reuse, zero-observation case
// included. The observe-stage A/B is deliberately not part of this: it compares observation
// models, so reusing an observation there would compare one model against the other's work.
func TestWriteComparisonHonorsTheSameReuse(t *testing.T) {
	images, stored := storedSnapshot(15, "old/observer")
	for _, test := range []struct {
		name        string
		requested   *[]string
		wantCalls   int
		wantPersist int
	}{
		{name: "reuse everything", requested: stringPointer(), wantCalls: 0, wantPersist: 0},
		{name: "reuse ten of fifteen", requested: &[]string{"IMG_1.jpg", "IMG_2.jpg", "IMG_3.jpg", "IMG_4.jpg", "IMG_5.jpg"}, wantCalls: 2, wantPersist: 2},
		{name: "no picker answer", requested: nil, wantCalls: 4, wantPersist: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, Observations: stored}}
			models := observingModels(t)
			svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, reobserveBatchSize, testReasoningPolicy, testBudget)

			raw, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", observeRef, nil, test.requested)
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := svc.PrepareWriteInput(context.Background(), raw, func(string, int, int) {})
			if err != nil {
				t.Fatal(err)
			}
			if got := observeCallCount(models); got != test.wantCalls {
				t.Fatalf("observation calls = %d, want %d", got, test.wantCalls)
			}
			if len(posts.observationWrites) != test.wantPersist {
				t.Fatalf("persists = %d, want %d", len(posts.observationWrites), test.wantPersist)
			}
			var snapshot experimentSnapshot
			if err := json.Unmarshal(prepared, &snapshot); err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Observations) != 15 {
				t.Fatalf("prepared snapshot holds %d observations, want 15", len(snapshot.Observations))
			}
			// The candidates read the snapshot's own set; the frozen post must not carry a
			// second, divergent copy of the same fact.
			if snapshot.Post.Observations != nil {
				t.Fatalf("the frozen post carries a duplicate snapshot: %+v", snapshot.Post.Observations)
			}
		})
	}
}

// A photo confirmed between the enqueue and the dequeue is outside the frozen decision: it is
// not observed, and it must not reach the WRITE prompt either. A filename with no observation
// beside it is exactly the "write from a photo nothing has looked at" case.
func TestAPhotoAttachedAfterEnqueueNeverReachesTheWritePrompt(t *testing.T) {
	images, stored := storedSnapshot(2, "old/observer")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, Observations: stored}}
	jobs := &fakeJobs{id: "job"}
	models := observingModels(t)
	var writePrompt string
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.HasImages() {
			t.Fatal("reuse-everything made an observation call")
		}
		writePrompt = request.Messages[0].Parts[0].Text
		return llm.Response{Text: `{"title":"t","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"ok"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, reobserveBatchSize, testReasoningPolicy, testBudget)

	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: stringPointer(),
	}); err != nil {
		t.Fatal(err)
	}
	frozen := jobs.generations[0]
	posts.input.Images = append(append([]Image(nil), images...), Image{Filename: "LATE.jpg", Key: "key-late"})

	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", ObserveModel: frozen.ObserveModel, WriteModel: frozen.WriteModel,
		ObserveFiles: frozen.ObserveFiles, Observations: frozen.Observations,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(writePrompt, "LATE.jpg") {
		t.Fatalf("the write prompt was shown an unobserved photo: %s", writePrompt)
	}
	for i := 1; i <= 2; i++ {
		if !strings.Contains(writePrompt, fmt.Sprintf("IMG_%d.jpg", i)) {
			t.Fatalf("the write prompt lost an observed photo: %s", writePrompt)
		}
	}
}

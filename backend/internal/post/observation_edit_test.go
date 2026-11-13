package post

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func observationEditFixture(t *testing.T) (*Service, *fakeStore, Post) {
	t.Helper()
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "성수")
	attachPhoto(t, svc, blobs, p.Slug, "a.jpg")
	mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 5_000)
	if err := svc.SetObservations(ctx, alice, p.Slug, []Observation{
		{File: "a.jpg", Scene: "가게 앞", Mood: "밝음", VisibleText: "OPEN", Objects: []string{"간판"}, Model: "m/a"},
		{File: "clip.mp4", Scene: "주방", Events: []string{"면을 삶는다"}, Speech: "어서 오세요", Model: "m/a"},
	}); err != nil {
		t.Fatal(err)
	}
	found, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	return svc, store, found
}

func text(value string) *string { return &value }

func texts(values ...string) *[]string { return &values }

// POST-126, GEN-90: a save replaces the edited fields, marks the entry with the cumulative set
// of fields the owner changed, and moves no status, content revision or baseline.
func TestADraftSaveStoresTheOwnersObservationCorrections(t *testing.T) {
	svc, _, p := observationEditFixture(t)
	ctx := context.Background()
	saved, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: p.Slug, Title: p.Title, Memo: p.Memo, Observations: []ObservationEdit{
		{File: "a.jpg", Scene: text("가게 입구"), Mood: text("밝음"), Objects: texts("간판", "화분")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	photo := saved.Observations[0]
	want := Observation{File: "a.jpg", Scene: "가게 입구", Mood: "밝음", VisibleText: "OPEN", Objects: []string{"간판", "화분"}, Model: "m/a",
		OwnerEdited: true, OwnerFields: []string{ObservationFieldScene, ObservationFieldObjects}}
	if !reflect.DeepEqual(photo, want) {
		t.Fatalf("edited entry = %+v, want %+v", photo, want)
	}
	if saved.Observations[1].OwnerEdited {
		t.Fatalf("an untouched entry was marked: %+v", saved.Observations[1])
	}
	if saved.Status != p.Status || saved.ContentRevision != p.ContentRevision || saved.MachineBaselineRevision != p.MachineBaselineRevision {
		t.Fatalf("the edit moved the post: %+v", saved)
	}
	if saved.InputRevision <= p.InputRevision {
		t.Fatalf("input revision %d did not advance from %d (POST-111)", saved.InputRevision, p.InputRevision)
	}

	// A later edit adds to the set; a video's events are editable, and clearing is a change.
	again, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: p.Slug, Title: p.Title, Memo: p.Memo, Observations: []ObservationEdit{
		{File: "a.jpg", VisibleText: text("")},
		{File: "clip.mp4", Events: texts("면을 건진다", "그릇에 담는다")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Observations[0].OwnerFields; !reflect.DeepEqual(got, []string{ObservationFieldScene, ObservationFieldObjects, ObservationFieldVisibleText}) {
		t.Fatalf("cumulative fields = %v", got)
	}
	video := again.Observations[1]
	if !video.OwnerEdited || !reflect.DeepEqual(video.Events, []string{"면을 건진다", "그릇에 담는다"}) || video.Speech != "어서 오세요" ||
		!reflect.DeepEqual(video.OwnerFields, []string{ObservationFieldEvents}) {
		t.Fatalf("video entry = %+v", video)
	}

	// An identical save is no edit: nothing is written and the revision stays.
	same, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: p.Slug, Title: p.Title, Memo: p.Memo, Observations: []ObservationEdit{
		{File: "clip.mp4", Events: texts("면을 건진다", "그릇에 담는다")},
	}})
	if err != nil || same.InputRevision != again.InputRevision {
		t.Fatalf("an identical save = %d, %v; want revision %d", same.InputRevision, err, again.InputRevision)
	}
}

// An edit that leaves every field as it stands sets no mark.
func TestAnUnchangedObservationEditSetsNoMark(t *testing.T) {
	svc, _, p := observationEditFixture(t)
	saved, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: p.Slug, Title: p.Title, Memo: p.Memo, Observations: []ObservationEdit{
		{File: "a.jpg", Scene: text("가게 앞"), Objects: texts("간판")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Observations[0].OwnerEdited || saved.Observations[0].OwnerFields != nil || saved.InputRevision != p.InputRevision {
		t.Fatalf("an unchanged edit marked the entry: %+v", saved.Observations[0])
	}
}

// POST-126, POST-74, GEN-89: an edit is refused whole — an unknown filename, one named twice,
// events on a photo, a create, a published post, or while a job that writes the observations
// targets the post — and nothing else in the save is applied.
func TestADraftSaveRefusesAnObservationEditItCannotTake(t *testing.T) {
	for name, test := range map[string]struct {
		edits   []ObservationEdit
		busy    string
		publish bool
		check   func(error) bool
	}{
		"an unknown filename": {
			edits: []ObservationEdit{{File: "ghost.jpg", Scene: text("x")}},
			check: func(err error) bool { return errors.Is(err, ErrObservationEditInvalid) },
		},
		"a filename twice": {
			edits: []ObservationEdit{{File: "a.jpg", Scene: text("x")}, {File: "a.jpg", Mood: text("y")}},
			check: func(err error) bool { return errors.Is(err, ErrObservationEditInvalid) },
		},
		"events on a photo": {
			edits: []ObservationEdit{{File: "a.jpg", Events: texts("x")}},
			check: func(err error) bool { return errors.Is(err, ErrObservationEditInvalid) },
		},
		"an observation job": {
			edits: []ObservationEdit{{File: "a.jpg", Scene: text("x")}}, busy: "observe",
			check: func(err error) bool { return errors.Is(err, ErrPostBusy) },
		},
		"a generation job": {
			edits: []ObservationEdit{{File: "a.jpg", Scene: text("x")}}, busy: "generate",
			check: func(err error) bool { return errors.Is(err, ErrPostBusy) },
		},
		"a storyline job": {
			edits: []ObservationEdit{{File: "a.jpg", Scene: text("x")}}, busy: "storyline",
			check: func(err error) bool { return errors.Is(err, ErrPostBusy) },
		},
		"a published post": {
			edits: []ObservationEdit{{File: "a.jpg", Scene: text("x")}}, publish: true,
			check: func(err error) bool { return errors.Is(err, ErrPostPublished) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc, store, p := observationEditFixture(t)
			if test.busy != "" {
				svc.jobs = fakeActiveJobs{p.Slug: {ID: "job-1", Kind: test.busy, Status: "running"}}
			}
			if test.publish {
				store.mu.Lock()
				stored := store.posts[p.Slug]
				stored.Status = StatusPublished
				store.posts[p.Slug] = stored
				store.mu.Unlock()
			}
			_, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: p.Slug, Title: "바뀐 제목", Memo: p.Memo, Observations: test.edits})
			if !test.check(err) {
				t.Fatalf("err = %v", err)
			}
			after, getErr := svc.Get(context.Background(), alice, p.Slug)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if after.Title != p.Title || !reflect.DeepEqual(after.Observations, p.Observations) {
				t.Fatalf("a refused edit changed the post: %+v", after)
			}
		})
	}

	// A revision writes no observation, so it holds no edit back.
	svc, _, p := observationEditFixture(t)
	svc.jobs = fakeActiveJobs{p.Slug: {ID: "job-1", Kind: "revise", Status: "running"}}
	if _, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: p.Slug, Title: p.Title, Memo: p.Memo,
		Observations: []ObservationEdit{{File: "a.jpg", Scene: text("x")}}}); err != nil {
		t.Fatalf("an edit during a revision: %v", err)
	}

	// A post being created has no observation to edit.
	language := LanguageKorean
	if _, err := svc.SaveDraft(context.Background(), alice, DraftSave{Title: "새 글", TargetLanguage: &language,
		Observations: []ObservationEdit{{File: "a.jpg", Scene: text("x")}}}); !errors.Is(err, ErrObservationEditInvalid) {
		t.Fatalf("a create with an edit = %v", err)
	}
}

// POST-18: deleting the attachment deletes its edited entry too.
func TestDeletingAnAttachmentDeletesItsEditedObservation(t *testing.T) {
	svc, _, p := observationEditFixture(t)
	ctx := context.Background()
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: p.Slug, Title: p.Title, Memo: p.Memo, Observations: []ObservationEdit{
		{File: "a.jpg", Scene: text("가게 입구")},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteImage(ctx, alice, p.Images[0].ID); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Observations) != 1 || after.Observations[0].File != "clip.mp4" {
		t.Fatalf("observations after the delete = %+v", after.Observations)
	}
}

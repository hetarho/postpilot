package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
)

func observationRequest(key string, photos ...string) (experiment.TestStart, experiment.TestPlan) {
	request, plan := writingRequest(experiment.FactorModel, experiment.StageObserve, 2, key)
	request.Output = experiment.OutputObservation
	request.Input = experiment.TestInput{TargetLanguage: "ko", PhotoIDs: photos}
	for i := range plan.Calls {
		plan.Calls[i].Stage = experiment.StageObserve
	}
	return request, plan
}

// Admission writes the output kind and attaches its photos in the same transaction; a photo
// that cannot be attached refuses the whole admission and writes nothing.
func TestAdmitObservationTestAttachesItsPhotos(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	future := time.Now().UTC().Add(time.Hour)
	for _, id := range []string{"p1", "p2", "held"} {
		readyTestPhoto(t, store, id, "alice", future)
	}
	readyTestPhoto(t, store, "bobs", "bob", future)
	if err := store.CreateTestPhoto(ctx, newTestPhoto("pending", "alice", future)); err != nil {
		t.Fatal(err)
	}
	other := admitModelTest(t, store, "post-test")
	if other.Output != experiment.OutputPost || len(other.Photos) != 0 {
		t.Fatalf("a post test reads back as a post with no photos: %q %v", other.Output, other.Photos)
	}
	if err := experimentstore.AttachTestPhotosTx(ctx, handle.Writer, "alice", other.ID, []string{"held"}); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		var n int
		if err := handle.Writer.QueryRowContext(ctx, `SELECT COUNT(*) FROM writing_tests`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	for name, photos := range map[string][]string{
		"a pending photo":       {"p1", "pending"},
		"another owner's photo": {"p1", "bobs"},
		"an attached photo":     {"p1", "held"},
		"an unknown photo":      {"p1", "missing"},
	} {
		request, plan := observationRequest("refused-"+name, photos...)
		now := time.Now().UTC()
		quote := experiment.TestQuote{Key: request.QuoteKey, UserID: request.UserID, Fingerprint: experiment.WritingTestStartFingerprint(request), Start: request, Plan: plan, Credits: plan.EstimateCredits, CreatedAt: now, ExpiresAt: now.Add(time.Hour), Retention: 30 * 24 * time.Hour}
		if err := store.PutTestQuote(ctx, quote); err != nil {
			t.Fatal(err)
		}
		if _, _, err := store.AdmitTest(ctx, request, plan); !errors.Is(err, experiment.ErrTestMaterialInvalid) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if count() != before {
		t.Fatal("a refused admission must write no test")
	}

	request, plan := observationRequest("observe", "p1", "p2")
	found := admitWriting(t, store, request, plan)
	if found.Output != experiment.OutputObservation || !slices.Equal(photoIDs(found.Photos), []string{"p1", "p2"}) {
		t.Fatalf("admitted observation test: output %q photos %v", found.Output, photoIDs(found.Photos))
	}
	again, err := store.GetTest(ctx, "alice", found.ID)
	if err != nil || again.Output != experiment.OutputObservation || len(again.Photos) != 2 || again.Photos[0].Key == "" {
		t.Fatalf("read back: %v %q %v", err, again.Output, again.Photos)
	}
	if replayed, fresh, err := store.AdmitTest(ctx, request, plan); err != nil || fresh || replayed.ID != found.ID {
		t.Fatalf("a replayed admission returns the same test without attaching again: %v %v", fresh, err)
	}
}

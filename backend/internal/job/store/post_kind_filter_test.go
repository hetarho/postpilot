package store_test

import (
	"testing"

	"github.com/postpilot/backend/internal/job"
)

func TestActivePostKindFilterSelectsOnlyOwnedRequestedWork(t *testing.T) {
	store, _ := subjectHarness(t)
	insert(t, store, job.Job{ID: "ordinary", UserID: "alice", Kind: job.KindGenerate, Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})
	for _, filter := range []job.Filter{{Kind: job.KindGenerate}, {UserID: "alice", Kind: job.KindGenerate}} {
		found, err := store.ActiveFor(t.Context(), job.Subject{Dimension: postSubject, ID: "post-alice"}, filter)
		if err != nil || found == nil || found.ID != "ordinary" {
			t.Fatal(found, err)
		}
	}
	for _, filter := range []job.Filter{{Kind: job.KindRevise}, {UserID: "bob", Kind: job.KindGenerate}} {
		found, err := store.ActiveFor(t.Context(), job.Subject{Dimension: postSubject, ID: "post-alice"}, filter)
		if err != nil || found != nil {
			t.Fatal("kind or owner filter ignored", found, err)
		}
	}
}

package post

import (
	"context"
	"errors"
	"testing"
)

// POST-63: the tag count is presence-aware on save, bounded 1–10, and never touches the
// content lifecycle.
func TestSaveGenerationOptionsTagCount(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := mustCreatePost(t, svc, alice, "Tags")
	if created.TagCount != 4 {
		t.Fatalf("a new post reads tag count %d, want the default 4", created.TagCount)
	}

	seven := 7
	saved, err := svc.SaveGenerationOptions(context.Background(), alice, created.Slug, nil, &seven)
	if err != nil || saved.TagCount != 7 || saved.TargetLength != nil {
		t.Fatalf("saved = %+v err=%v", saved, err)
	}
	if saved.ContentRevision != created.ContentRevision || saved.Status != created.Status {
		t.Fatalf("an option change moved the lifecycle: %+v", saved)
	}

	// Absent keeps the stored count while the length is replaced.
	length := 900
	kept, err := svc.SaveGenerationOptions(context.Background(), alice, created.Slug, &length, nil)
	if err != nil || kept.TagCount != 7 || kept.TargetLength == nil || *kept.TargetLength != 900 {
		t.Fatalf("kept = %+v err=%v", kept, err)
	}

	for _, bad := range []int{0, -1, 11} {
		_, err := svc.SaveGenerationOptions(context.Background(), alice, created.Slug, nil, &bad)
		if !errors.Is(err, ErrInvalidTagCount) {
			t.Fatalf("tag count %d: err = %v, want ErrInvalidTagCount", bad, err)
		}
	}
	after, err := svc.Get(context.Background(), alice, created.Slug)
	if err != nil || after.TagCount != 7 || after.TargetLength == nil || *after.TargetLength != 900 {
		t.Fatalf("a refused save changed the row: %+v err=%v", after, err)
	}
}

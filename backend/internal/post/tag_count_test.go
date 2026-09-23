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
	saved, err := svc.SaveGenerationOptions(context.Background(), alice, created.Slug, nil, &seven, nil, nil)
	if err != nil || saved.TagCount != 7 || saved.TargetLength != nil {
		t.Fatalf("saved = %+v err=%v", saved, err)
	}
	if saved.ContentRevision != created.ContentRevision || saved.Status != created.Status {
		t.Fatalf("an option change moved the lifecycle: %+v", saved)
	}

	// Absent keeps the stored count while the length is replaced.
	length := 900
	kept, err := svc.SaveGenerationOptions(context.Background(), alice, created.Slug, &length, nil, nil, nil)
	if err != nil || kept.TagCount != 7 || kept.TargetLength == nil || *kept.TargetLength != 900 {
		t.Fatalf("kept = %+v err=%v", kept, err)
	}

	for _, bad := range []int{0, -1, 11} {
		_, err := svc.SaveGenerationOptions(context.Background(), alice, created.Slug, nil, &bad, nil, nil)
		if !errors.Is(err, ErrInvalidTagCount) {
			t.Fatalf("tag count %d: err = %v, want ErrInvalidTagCount", bad, err)
		}
	}
	after, err := svc.Get(context.Background(), alice, created.Slug)
	if err != nil || after.TagCount != 7 || after.TargetLength == nil || *after.TargetLength != 900 {
		t.Fatalf("a refused save changed the row: %+v err=%v", after, err)
	}
}

// MEM-18/POST-71: the memory opt-in rides this save, is presence-aware like the tag count,
// and changes nothing else about the post — no status, no revision, no baseline.
func TestSaveGenerationOptionsUseMemory(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "Jeju")
	if created.UseMemory {
		t.Fatal("a new post starts with the option on")
	}
	before, err := svc.Get(ctx, alice, created.Slug)
	if err != nil {
		t.Fatal(err)
	}

	on := true
	saved, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, nil, nil, &on, nil)
	if err != nil || !saved.UseMemory {
		t.Fatalf("save = %+v, %v", saved.UseMemory, err)
	}
	if saved.Status != before.Status || saved.ContentRevision != before.ContentRevision ||
		saved.MachineBaselineRevision != before.MachineBaselineRevision {
		t.Fatalf("the option save moved the post's state: %+v", saved)
	}

	// Absent keeps the stored flag, exactly as an absent tag count keeps the stored count.
	length := 1200
	kept, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, &length, nil, nil, nil)
	if err != nil || !kept.UseMemory {
		t.Fatalf("an absent flag cleared the option: %+v, %v", kept.UseMemory, err)
	}
	off := false
	cleared, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, &length, nil, &off, nil)
	if err != nil || cleared.UseMemory {
		t.Fatalf("the option could not be turned off: %+v, %v", cleared.UseMemory, err)
	}
}

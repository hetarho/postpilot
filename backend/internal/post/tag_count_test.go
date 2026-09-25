package post

import (
	"context"
	"errors"
	"testing"
)

// POST-63: the tag count is bounded 1–10 and never touches the content lifecycle. The save is a
// whole set (POST-89), so each save names the count it means.
func TestSaveGenerationOptionsTagCount(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "Tags")
	if created.TagCount != 4 {
		t.Fatalf("a new post reads tag count %d, want the default 4", created.TagCount)
	}

	saved, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, optionsSet(t, svc, alice, created.Slug, func(s *GenerationOptionsSet) { s.TagCount = 7 }))
	if err != nil || saved.TagCount != 7 || saved.TargetLength != nil {
		t.Fatalf("saved = %+v err=%v", saved, err)
	}
	if saved.ContentRevision != created.ContentRevision || saved.Status != created.Status {
		t.Fatalf("an option change moved the lifecycle: %+v", saved)
	}

	// The length changes, and the count stays as the set says.
	length := 900
	kept, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, optionsSet(t, svc, alice, created.Slug, func(s *GenerationOptionsSet) { s.TargetLength = &length }))
	if err != nil || kept.TagCount != 7 || kept.TargetLength == nil || *kept.TargetLength != 900 {
		t.Fatalf("kept = %+v err=%v", kept, err)
	}

	for _, bad := range []int{0, -1, 11} {
		_, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, optionsSet(t, svc, alice, created.Slug, func(s *GenerationOptionsSet) { s.TagCount = bad }))
		if !errors.Is(err, ErrInvalidTagCount) {
			t.Fatalf("tag count %d: err = %v, want ErrInvalidTagCount", bad, err)
		}
	}
	after, err := svc.Get(ctx, alice, created.Slug)
	if err != nil || after.TagCount != 7 || after.TargetLength == nil || *after.TargetLength != 900 {
		t.Fatalf("a refused save changed the row: %+v err=%v", after, err)
	}
}

// MEM-18/POST-71: the memory opt-in rides the brief's save and changes nothing else about the
// post — no status, no revision, no baseline.
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

	saved, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, optionsSet(t, svc, alice, created.Slug, func(s *GenerationOptionsSet) { s.UseMemory = true }))
	if err != nil || !saved.UseMemory {
		t.Fatalf("save = %+v, %v", saved.UseMemory, err)
	}
	if saved.Status != before.Status || saved.ContentRevision != before.ContentRevision ||
		saved.MachineBaselineRevision != before.MachineBaselineRevision {
		t.Fatalf("the option save moved the post's state: %+v", saved)
	}

	cleared, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, optionsSet(t, svc, alice, created.Slug, func(s *GenerationOptionsSet) { s.UseMemory = false }))
	if err != nil || cleared.UseMemory {
		t.Fatalf("the option could not be turned off: %+v, %v", cleared.UseMemory, err)
	}
}

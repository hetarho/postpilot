package post

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// POST-89: the brief's run options land as one set in one write — the ticks normalized, the 분야
// riding the same statement rather than AssignField — and change nothing of the lifecycle.
func TestSaveGenerationOptionsWritesTheWholeSetInOneWrite(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "제주")
	before := store.posts[created.Slug]

	length := 1500
	saved, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, GenerationOptionsSet{
		TargetLength: &length, TagCount: 7, UseMemory: true,
		QualityRules: []string{QualityRuleComposition, QualityRuleTitleSaturation, QualityRuleComposition}, Field: "cafe",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.TargetLength == nil || *saved.TargetLength != 1500 || saved.TagCount != 7 || !saved.UseMemory ||
		!reflect.DeepEqual(saved.QualityRules, []string{QualityRuleTitleSaturation, QualityRuleComposition}) || saved.Field != "cafe" {
		t.Fatalf("saved = %+v", saved.GenerationOptions())
	}
	if store.optionWrites != 1 || store.fieldAssignments != 0 {
		t.Fatalf("%d option writes and %d 분야 assignments, want one write and none", store.optionWrites, store.fieldAssignments)
	}
	after := store.posts[created.Slug]
	if after.Status != before.Status || after.ContentRevision != before.ContentRevision || after.MachineBaselineRevision != before.MachineBaselineRevision {
		t.Fatalf("the options save moved the lifecycle: %+v", after)
	}

	cleared, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, GenerationOptionsSet{TagCount: 4, QualityRules: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.TargetLength != nil || cleared.TagCount != 4 || cleared.UseMemory || cleared.QualityRules != nil || cleared.Field != "" {
		t.Fatalf("cleared = %+v", cleared.GenerationOptions())
	}
	if store.optionWrites != 2 {
		t.Fatalf("the clearing save took %d writes", store.optionWrites-1)
	}
}

// Every member is checked before anything is written: one bad member refuses the whole set.
func TestSaveGenerationOptionsRefusesABadMemberAndWritesNothing(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "제주")
	zero, negative := 0, -1
	for name, test := range map[string]struct {
		change func(*GenerationOptionsSet)
		want   error
	}{
		"length 0":        {func(s *GenerationOptionsSet) { s.TargetLength = &zero }, ErrInvalidContent},
		"length -1":       {func(s *GenerationOptionsSet) { s.TargetLength = &negative }, ErrInvalidContent},
		"count 0":         {func(s *GenerationOptionsSet) { s.TagCount = 0 }, ErrInvalidTagCount},
		"count 11":        {func(s *GenerationOptionsSet) { s.TagCount = 11 }, ErrInvalidTagCount},
		"an unknown tick": {func(s *GenerationOptionsSet) { s.QualityRules = []string{"score"} }, ErrQualityRuleInvalid},
		"an unknown 분야":   {func(s *GenerationOptionsSet) { s.Field = "space_travel" }, ErrFieldNotFound},
	} {
		before := store.posts[created.Slug]
		writes := store.optionWrites
		set := optionsSet(t, svc, alice, created.Slug, func(s *GenerationOptionsSet) { s.UseMemory = true; test.change(s) })
		if _, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, set); !errors.Is(err, test.want) {
			t.Errorf("%s: err = %v, want %v", name, err, test.want)
		}
		if after := store.posts[created.Slug]; !reflect.DeepEqual(after, before) || store.optionWrites != writes {
			t.Errorf("%s: a refused set wrote something: %+v", name, after)
		}
	}
}

// POST-74: a published post refuses the whole set, ticks and 분야 included, and nothing lands.
func TestAnOptionsSaveOnAPublishedPostIsLocked(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
		t.Fatal(err)
	}
	before := store.posts[finalized.Slug]
	set := before.GenerationOptions()
	set.QualityRules, set.Field = []string{QualityRuleComposition}, "cafe"
	if _, err := svc.SaveGenerationOptions(ctx, alice, finalized.Slug, set); !errors.Is(err, ErrPostPublished) {
		t.Fatalf("err = %v, want ErrPostPublished", err)
	}
	if after := store.posts[finalized.Slug]; !reflect.DeepEqual(after, before) {
		t.Fatalf("the lock let the set through: %+v", after)
	}
}

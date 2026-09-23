package post

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestNormalizeQualityRules(t *testing.T) {
	for name, test := range map[string]struct {
		in   []string
		want []string
	}{
		"nothing ticked": {nil, []string{}},
		"collapsed and ordered": {
			[]string{QualityRuleComposition, QualityRuleTitleSaturation, QualityRuleComposition},
			[]string{QualityRuleTitleSaturation, QualityRuleComposition},
		},
		"all four in reverse": {
			[]string{QualityRuleComposition, QualityRuleInPostRepetition, QualityRuleCrossPostPhrases, QualityRuleTitleSaturation},
			[]string{QualityRuleTitleSaturation, QualityRuleCrossPostPhrases, QualityRuleInPostRepetition, QualityRuleComposition},
		},
	} {
		got, err := NormalizeQualityRules(test.in)
		if err != nil || got == nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: %q, %v; want %q", name, got, err, test.want)
		}
	}
	for _, bad := range [][]string{{"score"}, {""}, {QualityRuleComposition, "TITLE_SATURATION"}} {
		if got, err := NormalizeQualityRules(bad); !errors.Is(err, ErrQualityRuleInvalid) || got != nil {
			t.Errorf("%q = %q, %v; want ErrQualityRuleInvalid", bad, got, err)
		}
	}
}

// POST-81: absent keeps the ticks, present replaces them, present and empty clears them, and a
// set equal to the stored one in another order is the no-op the other options already are.
func TestTicksSaveKeepClearAndCollapse(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	clock := testNow
	svc.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	created := mustCreatePost(t, svc, alice, "제주")
	ticks := func(ids ...string) *[]string { return &ids }

	saved, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, nil, nil, nil, ticks(QualityRuleComposition, QualityRuleTitleSaturation, QualityRuleComposition))
	if want := []string{QualityRuleTitleSaturation, QualityRuleComposition}; err != nil || !reflect.DeepEqual(saved.QualityRules, want) {
		t.Fatalf("ticked = %q, %v; want %q", saved.QualityRules, err, want)
	}
	kept, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, nil, nil, nil, nil)
	if err != nil || !reflect.DeepEqual(kept.QualityRules, saved.QualityRules) {
		t.Fatalf("absent = %q, %v", kept.QualityRules, err)
	}

	stamp := store.posts[created.Slug].UpdatedAt
	if _, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, nil, nil, nil, ticks(QualityRuleComposition, QualityRuleTitleSaturation)); err != nil {
		t.Fatal(err)
	}
	if !store.posts[created.Slug].UpdatedAt.Equal(stamp) {
		t.Fatal("the same set in another order was written again")
	}

	replaced, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, nil, nil, nil, ticks(QualityRuleInPostRepetition))
	if err != nil || !reflect.DeepEqual(replaced.QualityRules, []string{QualityRuleInPostRepetition}) {
		t.Fatalf("replaced = %q, %v", replaced.QualityRules, err)
	}
	cleared, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, nil, nil, nil, ticks())
	if err != nil || cleared.QualityRules != nil {
		t.Fatalf("cleared = %q, %v", cleared.QualityRules, err)
	}
}

// A tick that names none of the four is refused before anything in the request is applied.
func TestAnUnknownTickIsRefusedAndNothingLands(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "제주")
	before := store.posts[created.Slug]
	length, count, memory := 1200, 7, true
	bad := []string{QualityRuleTitleSaturation, "score"}
	if _, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, &length, &count, &memory, &bad); !errors.Is(err, ErrQualityRuleInvalid) {
		t.Fatalf("err = %v, want ErrQualityRuleInvalid", err)
	}
	if after := store.posts[created.Slug]; !reflect.DeepEqual(after, before) {
		t.Fatalf("a refused save applied an option: %+v", after)
	}
}

// POST-82: ticks are an option of the next run. Saving them on a finalized post moves nothing of
// its lifecycle, and it stays learnable.
func TestTicksAreAnOptionSave(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	before := store.posts[finalized.Slug]
	ticks := []string{QualityRuleTitleSaturation}
	saved, err := svc.SaveGenerationOptions(ctx, alice, finalized.Slug, before.TargetLength, nil, nil, &ticks)
	if err != nil || !reflect.DeepEqual(saved.QualityRules, ticks) {
		t.Fatalf("ticks = %q, %v", saved.QualityRules, err)
	}
	after := store.posts[finalized.Slug]
	if after.Status != StatusFinalized || after.ContentRevision != before.ContentRevision ||
		after.MachineBaselineRevision != before.MachineBaselineRevision || after.FinalizedRevision != before.FinalizedRevision {
		t.Fatalf("a tick save moved the lifecycle: %+v", after)
	}
	if _, err := svc.LearningSnapshot(ctx, alice, finalized.Slug); err != nil {
		t.Fatalf("the post stopped being learnable: %v", err)
	}
}

func TestATickSaveOnAPublishedPostIsLocked(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
		t.Fatal(err)
	}
	before := store.posts[finalized.Slug]
	ticks := []string{QualityRuleComposition}
	if _, err := svc.SaveGenerationOptions(ctx, alice, finalized.Slug, nil, nil, nil, &ticks); !errors.Is(err, ErrPostPublished) {
		t.Fatalf("err = %v, want ErrPostPublished", err)
	}
	if after := store.posts[finalized.Slug]; !reflect.DeepEqual(after, before) {
		t.Fatalf("the lock let the ticks through: %+v", after)
	}
}

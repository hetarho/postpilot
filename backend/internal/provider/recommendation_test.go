package provider_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
)

var (
	// graded is registered to every stage with a classification at each.
	graded  = llm.ModelRef{ProviderID: "openrouter", ModelID: "graded"}
	graded2 = llm.ModelRef{ProviderID: "openrouter", ModelID: "graded-two"}
	// writerOnly is registered to write and analyze but not to photo-analysis.
	writerOnly = llm.ModelRef{ProviderID: "openrouter", ModelID: "writer-only"}
	// ungraded is registered everywhere but classified nowhere.
	ungraded = llm.ModelRef{ProviderID: "openrouter", ModelID: "ungraded"}
)

func operatorCatalog() fakeCatalog {
	everywhere := map[string]string{"observe": "balanced", "write": "top", "analyze": "value"}
	return fakeCatalog{
		graded:     {Ref: graded, Vision: true, Stages: allStages, Levels: everywhere},
		graded2:    {Ref: graded2, Vision: true, Stages: allStages, Levels: everywhere},
		writerOnly: {Ref: writerOnly, Stages: textStages, Levels: map[string]string{"write": "value", "analyze": "value"}},
		ungraded:   {Ref: ungraded, Vision: true, Stages: allStages},
	}
}

func completeDraft(id, label string) provider.RecommendationSet {
	return provider.RecommendationSet{ID: id, Label: label, Selections: []provider.RecommendationStageSelection{
		{Stage: provider.StageObserve, Active: graded, CandidateA: graded, CandidateB: graded2},
		{Stage: provider.StageAnalyze, Active: graded},
		{Stage: provider.StageWrite, Active: graded2, CandidateA: graded2, CandidateB: graded},
	}}
}

// untouchedSelections is what every operator write must leave behind (MODEL-71).
func untouchedSelections(t *testing.T, store *fakeStore, before map[string]provider.Selection) {
	t.Helper()
	if len(store.lastBatch) != 0 || len(store.deleted) != 0 || len(store.rows) != len(before) {
		t.Fatalf("an operator write touched selections: batch=%+v deleted=%v rows=%+v", store.lastBatch, store.deleted, store.rows)
	}
	for stage, selection := range before {
		if store.rows[stage] != selection {
			t.Fatalf("selection %s changed to %+v", stage, store.rows[stage])
		}
	}
}

// MODEL-69: an empty id creates a set at the end with a server id; an id replaces that set
// whole. The label is stored trimmed, and neither write touches an account's selections.
func TestSaveRecommendationSetCreatesAndReplacesWithoutTouchingSelections(t *testing.T) {
	chosen := provider.Selection{Stage: provider.StageWrite, Slot: provider.SlotActive, Ref: writerOnly}
	store := &fakeStore{rows: map[string]provider.Selection{"write": chosen}, sets: []provider.RecommendationSet{completeDraft("first", "First")}}
	before := map[string]provider.Selection{"write": chosen}
	svc := provider.NewService(store, operatorCatalog(), fakeCredits{})

	created, err := svc.SaveRecommendationSet(context.Background(), completeDraft("", "  Second  "))
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.ID == "first" || created.Label != "Second" {
		t.Fatalf("created = %+v", created)
	}
	if len(store.sets) != 2 || store.sets[1].ID != created.ID || store.sets[1].Label != "Second" {
		t.Fatalf("sets after create = %+v", store.sets)
	}

	replacement := completeDraft("first", "First, revised")
	replacement.Selections[2].CandidateB = writerOnly
	replaced, err := svc.SaveRecommendationSet(context.Background(), replacement)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.ID != "first" || store.sets[0].Label != "First, revised" || store.sets[0].Selections[2].CandidateB != writerOnly {
		t.Fatalf("replaced = %+v, stored = %+v", replaced, store.sets[0])
	}
	// The stored set is in the product's stage order with analyze's candidates empty.
	if stages := []provider.Stage{store.sets[0].Selections[0].Stage, store.sets[0].Selections[1].Stage, store.sets[0].Selections[2].Stage}; stages[0] != provider.StageObserve || stages[1] != provider.StageAnalyze || stages[2] != provider.StageWrite {
		t.Fatalf("stage order = %v", stages)
	}

	if _, err := svc.SaveRecommendationSet(context.Background(), completeDraft("nobody", "Ghost")); !errors.Is(err, provider.ErrRecommendationNotFound) {
		t.Fatalf("replacing an unknown set = %v", err)
	}
	untouchedSelections(t, store, before)
}

// MODEL-70: the draft is refused whole, and the refusal names every offending field with its
// cause in one answer — the operator fixes the form once, not one field per attempt.
func TestSaveRecommendationSetNamesEveryOffendingField(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	svc := provider.NewService(store, operatorCatalog(), fakeCredits{})
	gone := llm.ModelRef{ProviderID: "openrouter", ModelID: "gone"}
	draft := provider.RecommendationSet{Label: "   ", Selections: []provider.RecommendationStageSelection{
		// observe: A is registered for writing only, B repeats A.
		{Stage: provider.StageObserve, Active: ungraded, CandidateA: writerOnly, CandidateB: writerOnly},
		// write: the active model is not in the catalog; B is missing.
		{Stage: provider.StageWrite, Active: gone, CandidateA: graded},
		// analyze is left out entirely.
	}}

	_, err := svc.SaveRecommendationSet(context.Background(), draft)
	var refusal *provider.SetDraftRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a SetDraftRefusal", err)
	}
	want := map[string]string{
		"label":               provider.DraftRequired,
		"observe_active":      provider.DraftUnclassified,
		"observe_candidate_a": provider.DraftUnregistered,
		"observe_candidate_b": provider.DraftDuplicate,
		"analyze_active":      provider.DraftRequired,
		"write_active":        provider.DraftUnregistered,
		"write_candidate_b":   provider.DraftRequired,
	}
	if len(refusal.Fields) != len(want) {
		t.Fatalf("fields = %v, want %v", refusal.Fields, want)
	}
	for field, cause := range want {
		if refusal.Fields[field] != cause {
			t.Errorf("%s = %q, want %q", field, refusal.Fields[field], cause)
		}
	}
	params := refusal.Params()
	if refusal.Reason() != provider.ReasonSetInvalid || params["fields"] != "analyze_active,label,observe_active,observe_candidate_a,observe_candidate_b,write_active,write_candidate_b" {
		t.Fatalf("reason %q params %v", refusal.Reason(), params)
	}
	if len(store.sets) != 0 {
		t.Fatalf("a refused draft was written: %+v", store.sets)
	}
}

// The label is 1–60 runes after trimming, counted as characters rather than bytes.
func TestSaveRecommendationSetBoundsTheLabelInRunes(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	svc := provider.NewService(store, operatorCatalog(), fakeCredits{})
	if _, err := svc.SaveRecommendationSet(context.Background(), completeDraft("", strings.Repeat("가", provider.MaxRecommendationLabelRunes))); err != nil {
		t.Fatalf("a 60-rune label was refused: %v", err)
	}
	_, err := svc.SaveRecommendationSet(context.Background(), completeDraft("", strings.Repeat("가", provider.MaxRecommendationLabelRunes+1)))
	var refusal *provider.SetDraftRefusal
	if !errors.As(err, &refusal) || refusal.Fields["label"] != provider.DraftTooLong || len(refusal.Fields) != 1 {
		t.Fatalf("61 runes = %v", err)
	}
}

// MODEL-70: no plan or balance is read at save — a set of top-grade models is a valid draft
// whoever will be able to apply it.
func TestSaveRecommendationSetIgnoresPlanAndBalance(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	svc := provider.NewService(store, operatorCatalog(), refusingCredits{}).WithModelGrades()
	if _, err := svc.SaveRecommendationSet(context.Background(), completeDraft("", "Top")); err != nil {
		t.Fatalf("save read the operator's plan or balance: %v", err)
	}
}

// MODEL-69: at most ten sets. The eleventh is refused with the limit as a param.
func TestSaveRecommendationSetRefusesTheEleventh(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	svc := provider.NewService(store, operatorCatalog(), fakeCredits{})
	for i := 0; i < provider.MaxRecommendationSets; i++ {
		if _, err := svc.SaveRecommendationSet(context.Background(), completeDraft("", "Set")); err != nil {
			t.Fatalf("set %d: %v", i+1, err)
		}
	}
	_, err := svc.SaveRecommendationSet(context.Background(), completeDraft("", "One too many"))
	var limit *provider.SetLimitError
	if !errors.As(err, &limit) || !errors.Is(err, provider.ErrRecommendationLimit) || limit.Params()["limit"] != "10" || limit.Reason() != provider.ReasonSetLimit {
		t.Fatalf("eleventh = %v", err)
	}
	if len(store.sets) != provider.MaxRecommendationSets {
		t.Fatalf("sets = %d", len(store.sets))
	}
}

// Delete and move answer an unknown id as not found; neither touches selections.
func TestDeleteAndMoveRecommendationSets(t *testing.T) {
	chosen := provider.Selection{Stage: provider.StageObserve, Slot: provider.SlotActive, Ref: graded}
	store := &fakeStore{rows: map[string]provider.Selection{"observe": chosen}, sets: []provider.RecommendationSet{
		completeDraft("a", "A"), completeDraft("b", "B"), completeDraft("c", "C"),
	}}
	svc := provider.NewService(store, operatorCatalog(), fakeCredits{})
	ctx := context.Background()

	if err := svc.MoveRecommendationSet(ctx, "c", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.MoveRecommendationSet(ctx, "a", true); err != nil {
		t.Fatalf("moving the first set up: %v", err)
	}
	sets, err := svc.RecommendationSets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if order := sets[0].ID + sets[1].ID + sets[2].ID; order != "acb" {
		t.Fatalf("order = %s, want acb", order)
	}
	if err := svc.DeleteRecommendationSet(ctx, "c"); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"delete": svc.DeleteRecommendationSet(ctx, "c"),
		"move":   svc.MoveRecommendationSet(ctx, "c", false),
	} {
		if !errors.Is(err, provider.ErrRecommendationNotFound) {
			t.Errorf("%s of a deleted set = %v", name, err)
		}
	}
	if len(store.sets) != 2 {
		t.Fatalf("sets = %+v", store.sets)
	}
	untouchedSelections(t, store, map[string]provider.Selection{"observe": chosen})
}

// refusingCredits fails any read, so a test proves the service never asked.
type refusingCredits struct{}

func (refusingCredits) ForCalls([]provider.PlannedCall) int { return -1 }
func (refusingCredits) Balance(context.Context, string) (int, bool, error) {
	return 0, false, errors.New("the operator's balance was read")
}

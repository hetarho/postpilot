package modelcatalog_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
)

// document builds a paste for the sections given, in the order given.
func document(sections ...[]string) string {
	var b strings.Builder
	b.WriteString(modelcatalog.DocumentVersionLine + "\n")
	for _, section := range sections {
		b.WriteString("[" + section[0] + "]\n")
		for _, id := range section[1:] {
			b.WriteString(id + "\n")
		}
	}
	return b.String()
}

func planFor(t *testing.T, plan modelcatalog.DocumentPlan, purpose modelcatalog.Purpose) modelcatalog.DocumentPurposePlan {
	t.Helper()
	for _, p := range plan.Purposes {
		if p.Purpose == purpose {
			return p
		}
	}
	t.Fatalf("no plan for %s in %v", purpose, plan.Purposes)
	return modelcatalog.DocumentPurposePlan{}
}

func TestPreviewDocumentReportsTheDiffAndWritesNothing(t *testing.T) {
	store := newFakeStore(
		curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting),
		curated("x-ai/grok-4.6", modelcatalog.PurposeWriting),
	)
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("anthropic/claude-sonnet-5", 10),
		candidate("x-ai/grok-4.6", 20),
		candidate("z-ai/glm-5.3", 30),
	}}
	svc := newService(t, store, upstream)

	plan, err := svc.PreviewDocument(context.Background(), document(
		[]string{"writing", "anthropic/claude-sonnet-5", "z-ai/glm-5.3"},
	))
	if err != nil {
		t.Fatal(err)
	}
	writing := planFor(t, plan, modelcatalog.PurposeWriting)
	if !reflect.DeepEqual(writing.Register, []string{"z-ai/glm-5.3"}) {
		t.Errorf("register = %v, want the new id", writing.Register)
	}
	if !reflect.DeepEqual(writing.Deregister, []string{"x-ai/grok-4.6"}) {
		t.Errorf("deregister = %v — a section is the purpose's complete membership", writing.Deregister)
	}
	if !reflect.DeepEqual(writing.Unchanged, []string{"anthropic/claude-sonnet-5"}) {
		t.Errorf("unchanged = %v", writing.Unchanged)
	}
	if plan.Applied {
		t.Error("a preview never applies")
	}
	if store.syncs != 0 || store.refreshes != 0 {
		t.Errorf("preview wrote something: syncs=%d refreshes=%d", store.syncs, store.refreshes)
	}
	if row := store.rows["x-ai/grok-4.6"]; !slices.Contains(row.Purposes, modelcatalog.PurposeWriting) {
		t.Error("preview must not deregister anything")
	}
}

func TestApplyDocumentSyncsTheNamedPurposeWhole(t *testing.T) {
	store := newFakeStore(
		curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting),
		curated("x-ai/grok-4.6", modelcatalog.PurposeWriting, modelcatalog.PurposePhotoAnalysis),
	)
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("anthropic/claude-sonnet-5", 10),
		candidate("x-ai/grok-4.6", 20),
		candidate("z-ai/glm-5.3", 30),
	}}
	svc := newService(t, store, upstream)

	plan, err := svc.ApplyDocument(context.Background(), document(
		[]string{"writing", "anthropic/claude-sonnet-5", "z-ai/glm-5.3"},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied {
		t.Fatalf("not applied: %+v", plan)
	}
	if got := store.rows["z-ai/glm-5.3"]; !slices.Contains(got.Purposes, modelcatalog.PurposeWriting) {
		t.Error("a listed id the document names must be registered")
	}
	grok := store.rows["x-ai/grok-4.6"]
	if slices.Contains(grok.Purposes, modelcatalog.PurposeWriting) {
		t.Error("an id the section omits must be deregistered")
	}
	// A purpose the document gives no section for is untouched, even on a model the
	// document did touch for another purpose.
	if !slices.Contains(grok.Purposes, modelcatalog.PurposePhotoAnalysis) {
		t.Error("a purpose with no section must be left alone")
	}
}

func TestApplyDocumentCreatesARowForAnUncuratedID(t *testing.T) {
	store := newFakeStore()
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{candidate("z-ai/glm-5.3-flash", 30)}}
	svc := newService(t, store, upstream)

	if _, err := svc.ApplyDocument(context.Background(), document(
		[]string{"writing", "z-ai/glm-5.3-flash"},
	)); err != nil {
		t.Fatal(err)
	}
	row, ok := store.rows["z-ai/glm-5.3-flash"]
	if !ok {
		t.Fatal("an id nobody curated must get a row from the live snapshot — that is the point of the paste")
	}
	if row.Label != "z-ai/glm-5.3-flash label" || !row.Listed {
		t.Errorf("row was not snapshotted from the candidate: %+v", row)
	}
}

func TestApplyDocumentDropsTheEffortWithTheRegistration(t *testing.T) {
	row := curated("x-ai/grok-4.6", modelcatalog.PurposeWriting)
	row.Reasoning = map[modelcatalog.Purpose]llm.ReasoningEffort{modelcatalog.PurposeWriting: llm.ReasoningLow}
	store := newFakeStore(row, curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("x-ai/grok-4.6", 20), candidate("anthropic/claude-sonnet-5", 10),
	}}
	svc := newService(t, store, upstream)

	if _, err := svc.ApplyDocument(context.Background(), document(
		[]string{"writing", "anthropic/claude-sonnet-5"},
	)); err != nil {
		t.Fatal(err)
	}
	// Same write as an operator's uncheck: the effort is a column on the registration row.
	if effort := store.rows["x-ai/grok-4.6"].Reasoning[modelcatalog.PurposeWriting]; effort != llm.ReasoningUnspecified {
		t.Errorf("effort = %q, want it gone with the registration", effort)
	}
}

func TestDocumentRefusesWholeOnAnyBadLine(t *testing.T) {
	blind := candidate("deepseek/deepseek-v4-flash-0731", 40)
	blind.Vision = false
	store := newFakeStore(curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("anthropic/claude-sonnet-5", 10), blind,
	}}
	svc := newService(t, store, upstream)

	cases := map[string]struct {
		text  string
		cause string
	}{
		"an id the source does not offer": {
			text:  document([]string{"writing", "anthropic/claude-sonnet-5"}, []string{"photo-analysis", "openai/gpt-9"}),
			cause: modelcatalog.IssueUnknownModel,
		},
		"a model that fails the purpose gate": {
			text:  document([]string{"photo-analysis", "deepseek/deepseek-v4-flash-0731"}),
			cause: modelcatalog.IssueIneligible,
		},
		"a malformed line": {
			text:  document([]string{"writing", "| pasted | row |"}),
			cause: modelcatalog.IssueMalformedLine,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan, err := svc.ApplyDocument(context.Background(), tc.text)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Applied {
				t.Fatal("a document with any issue is never applied")
			}
			if len(plan.Issues) == 0 || plan.Issues[len(plan.Issues)-1].Cause != tc.cause {
				t.Fatalf("issues = %v, want one caused by %s", plan.Issues, tc.cause)
			}
			if store.syncs != 0 {
				t.Error("nothing may be written when anything was refused")
			}
			if !slices.Contains(store.rows["anthropic/claude-sonnet-5"].Purposes, modelcatalog.PurposeWriting) {
				t.Error("the valid half of a refused document must not be applied either")
			}
		})
	}
}

func TestDocumentTellsUnlistedFromUnknown(t *testing.T) {
	store := newFakeStore(curated("x-ai/grok-4.5", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{candidate("x-ai/grok-4.6", 20)}}
	svc := newService(t, store, upstream)

	plan, err := svc.PreviewDocument(context.Background(), document([]string{"writing", "x-ai/grok-4.5"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Issues) != 1 || plan.Issues[0].Cause != modelcatalog.IssueUnlisted {
		t.Fatalf("issues = %v, want unlisted_model — the operator has a row, the source does not", plan.Issues)
	}
}

func TestDocumentRefusesWhenTheCatalogCannotBeRead(t *testing.T) {
	store := newFakeStore(curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{err: errors.New("dial tcp: no route to host")}
	svc := newService(t, store, upstream)

	for _, tc := range []struct {
		name string
		call func(string) (modelcatalog.DocumentPlan, error)
	}{
		{"preview", func(text string) (modelcatalog.DocumentPlan, error) {
			return svc.PreviewDocument(context.Background(), text)
		}},
		{"apply", func(text string) (modelcatalog.DocumentPlan, error) {
			return svc.ApplyDocument(context.Background(), text)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := tc.call(document([]string{"writing", "anthropic/claude-sonnet-5"}))
			if err != nil {
				t.Fatal(err)
			}
			if plan.FetchError == "" {
				t.Error("an unreadable catalog must be reported — this path cannot degrade to stored rows")
			}
			if plan.Applied || store.syncs != 0 {
				t.Error("nothing may be written when the catalog could not be read")
			}
			if strings.Contains(plan.FetchError, "dial tcp") {
				t.Error("the provider's prose must not reach the browser")
			}
		})
	}
}

func TestExportDocumentRoundTripsToNoChange(t *testing.T) {
	sonnet := curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting)
	sonnet.Levels = map[modelcatalog.Purpose]modelcatalog.Level{modelcatalog.PurposeWriting: modelcatalog.LevelTop}
	gemini := curated("google/gemini-3.8-flash", modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeStyleAnalysis)
	// One graded registration and one deliberately ungraded, so the round trip proves both
	// halves: a level survives the render, and an absent one is not invented.
	gemini.Levels = map[modelcatalog.Purpose]modelcatalog.Level{modelcatalog.PurposePhotoAnalysis: modelcatalog.LevelValue}
	store := newFakeStore(sonnet, gemini)
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("anthropic/claude-sonnet-5", 10), candidate("google/gemini-3.8-flash", 20),
	}}
	svc := newService(t, store, upstream)

	exported, err := svc.ExportDocument(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := svc.PreviewDocument(context.Background(), exported)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Issues) != 0 {
		t.Fatalf("the exported document must be valid, got %v", plan.Issues)
	}
	for _, purpose := range plan.Purposes {
		if len(purpose.Register) > 0 || len(purpose.Deregister) > 0 || len(purpose.Relevel) > 0 {
			t.Errorf("%s: pasting an export straight back must change nothing, got +%v -%v ~%v",
				purpose.Purpose, purpose.Register, purpose.Deregister, purpose.Relevel)
		}
	}
	// Every purpose is present, so the export is a complete statement rather than a partial
	// edit — an empty section really does deregister.
	if len(plan.Purposes) != len(modelcatalog.Purposes) {
		t.Errorf("purposes = %d, want all five", len(plan.Purposes))
	}
}

// T093/MODEL-54: a document that re-grades a registration it keeps reports it as its own
// change — not as "unchanged", and not buried in the deregistration list.
func TestPreviewDocumentReportsRelevelWithoutWriting(t *testing.T) {
	sonnet := curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting)
	sonnet.Levels = map[modelcatalog.Purpose]modelcatalog.Level{modelcatalog.PurposeWriting: modelcatalog.LevelBalanced}
	graded := curated("x-ai/grok-4.6", modelcatalog.PurposeWriting)
	graded.Levels = map[modelcatalog.Purpose]modelcatalog.Level{modelcatalog.PurposeWriting: modelcatalog.LevelTop}
	store := newFakeStore(sonnet, graded, curated("z-ai/glm-5.3", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("anthropic/claude-sonnet-5", 10), candidate("x-ai/grok-4.6", 20), candidate("z-ai/glm-5.3", 30),
	}}
	svc := newService(t, store, upstream)

	text := modelcatalog.DocumentVersionLine + "\n[writing]\n" +
		// balanced → top
		"anthropic/claude-sonnet-5 top\n" +
		// top → unset: an id-only line clears a grade, and that must be visible
		"x-ai/grok-4.6\n" +
		// unset → unset: genuinely nothing to say
		"z-ai/glm-5.3\n"
	plan, err := svc.PreviewDocument(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	writing := planFor(t, plan, modelcatalog.PurposeWriting)
	want := []modelcatalog.LevelChange{
		{ModelID: "anthropic/claude-sonnet-5", From: modelcatalog.LevelBalanced, To: modelcatalog.LevelTop},
		{ModelID: "x-ai/grok-4.6", From: modelcatalog.LevelTop, To: ""},
	}
	if !reflect.DeepEqual(writing.Relevel, want) {
		t.Errorf("relevel = %v, want %v", writing.Relevel, want)
	}
	if !reflect.DeepEqual(writing.Unchanged, []string{"z-ai/glm-5.3"}) {
		t.Errorf("unchanged = %v, want only the model whose grade did not move", writing.Unchanged)
	}
	if len(writing.Register) != 0 || len(writing.Deregister) != 0 {
		t.Errorf("a re-grade is not a registration change: +%v -%v", writing.Register, writing.Deregister)
	}
	// A preview writes nothing, levels included.
	if got := store.rows["anthropic/claude-sonnet-5"].Levels[modelcatalog.PurposeWriting]; got != modelcatalog.LevelBalanced {
		t.Errorf("preview wrote a level: %q", got)
	}
	if store.syncs != 0 {
		t.Errorf("syncs = %d, want none", store.syncs)
	}
}

// T093/MODEL-59: applying writes the grade for every id the section lists — setting one on
// a registration that had none, and clearing one whose line no longer names it.
func TestApplyDocumentSetsAndClearsLevels(t *testing.T) {
	graded := curated("x-ai/grok-4.6", modelcatalog.PurposeWriting)
	graded.Levels = map[modelcatalog.Purpose]modelcatalog.Level{modelcatalog.PurposeWriting: modelcatalog.LevelTop}
	store := newFakeStore(graded, curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("x-ai/grok-4.6", 20), candidate("anthropic/claude-sonnet-5", 10),
		candidate("z-ai/glm-5.3-flash", 30),
	}}
	svc := newService(t, store, upstream)

	text := modelcatalog.DocumentVersionLine + "\n[writing]\n" +
		"anthropic/claude-sonnet-5 premium\n" + // already registered, gains a grade
		"x-ai/grok-4.6\n" + // already registered and graded, loses it
		"z-ai/glm-5.3-flash value\n" // brand new, arrives graded
	plan, err := svc.ApplyDocument(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied {
		t.Fatalf("not applied: %v", plan.Issues)
	}
	if got := store.rows["anthropic/claude-sonnet-5"].Levels[modelcatalog.PurposeWriting]; got != modelcatalog.LevelPremium {
		t.Errorf("sonnet level = %q, want premium", got)
	}
	if got, ok := store.rows["x-ai/grok-4.6"].Levels[modelcatalog.PurposeWriting]; ok {
		t.Errorf("grok level = %q, want cleared by its id-only line", got)
	}
	if got := store.rows["z-ai/glm-5.3-flash"].Levels[modelcatalog.PurposeWriting]; got != modelcatalog.LevelValue {
		t.Errorf("new registration level = %q, want value", got)
	}
	// A new registration is a Register and nothing else — its grade is part of arriving.
	writing := planFor(t, plan, modelcatalog.PurposeWriting)
	if !reflect.DeepEqual(writing.Register, []string{"z-ai/glm-5.3-flash"}) {
		t.Errorf("register = %v", writing.Register)
	}
}

// T093/MODEL-53: a bad grade refuses the WHOLE document, so the lines around it are not
// half-applied — the same rule every other cause follows.
func TestDocumentRefusesWholeOnABadLevel(t *testing.T) {
	store := newFakeStore(curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("anthropic/claude-sonnet-5", 10), candidate("z-ai/glm-5.3-flash", 30),
	}}
	svc := newService(t, store, upstream)

	text := modelcatalog.DocumentVersionLine + "\n[writing]\n" +
		"anthropic/claude-sonnet-5 top\n" +
		"z-ai/glm-5.3-flash legendary\n"
	plan, err := svc.ApplyDocument(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Applied {
		t.Fatal("a document with a bad level must not apply")
	}
	if len(plan.Issues) != 1 || plan.Issues[0].Cause != modelcatalog.IssueUnknownLevel || plan.Issues[0].Line != 4 {
		t.Fatalf("issues = %v, want one unknown_level on line 4", plan.Issues)
	}
	if store.syncs != 0 {
		t.Errorf("syncs = %d, want none — the valid line above must not land either", store.syncs)
	}
	if _, ok := store.rows["anthropic/claude-sonnet-5"].Levels[modelcatalog.PurposeWriting]; ok {
		t.Error("the accepted line's grade was written despite the refusal")
	}
}

// T093/MODEL-20: a deregistration takes the grade with it, exactly as it takes the effort.
func TestApplyDocumentDropsTheLevelWithTheRegistration(t *testing.T) {
	graded := curated("x-ai/grok-4.6", modelcatalog.PurposeWriting)
	graded.Levels = map[modelcatalog.Purpose]modelcatalog.Level{modelcatalog.PurposeWriting: modelcatalog.LevelTop}
	store := newFakeStore(graded, curated("anthropic/claude-sonnet-5", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("x-ai/grok-4.6", 20), candidate("anthropic/claude-sonnet-5", 10),
	}}
	svc := newService(t, store, upstream)

	if _, err := svc.ApplyDocument(context.Background(), document(
		[]string{"writing", "anthropic/claude-sonnet-5"},
	)); err != nil {
		t.Fatal(err)
	}
	if got, ok := store.rows["x-ai/grok-4.6"].Levels[modelcatalog.PurposeWriting]; ok {
		t.Errorf("level = %q, want it gone with the registration", got)
	}
}

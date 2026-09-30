package modelcatalog_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/modelcatalog"
)

// The ids a set block uses in these tests: `eyes` and `sight` observe, `pen` and `quill` write.
const setBlock = "set Balanced\n" +
	"observe vendor/eyes vendor/eyes vendor/sight\n" +
	"analyze vendor/pen\n" +
	"write vendor/pen vendor/pen vendor/quill\n"

func withRecommendations(sections string, sets string) string {
	return modelcatalog.DocumentVersionLine + "\n" + sections + "[recommendations]\n" + sets
}

func causesOf(issues []modelcatalog.DocumentIssue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, issue.Cause)
	}
	return out
}

// MODEL-72: no section leaves the sets alone (nil); an empty section is an explicit empty list.
func TestParseDocumentDistinguishesNoSectionFromAnEmptyOne(t *testing.T) {
	doc, issues := modelcatalog.ParseDocument(modelcatalog.DocumentVersionLine + "\n[writing]\nvendor/pen\n")
	if len(issues) != 0 || doc.Recommendations != nil {
		t.Fatalf("no section: recommendations = %v, issues = %v", doc.Recommendations, issues)
	}
	doc, issues = modelcatalog.ParseDocument(withRecommendations("", ""))
	if len(issues) != 0 || doc.Recommendations == nil || len(*doc.Recommendations) != 0 {
		t.Fatalf("empty section: recommendations = %v, issues = %v", doc.Recommendations, issues)
	}
}

func TestParseDocumentReadsSetBlocks(t *testing.T) {
	text := withRecommendations("[writing]\nvendor/pen top\n",
		setBlock+"\n# a note\nset   Free start · 무료  \nobserve vendor/a vendor/a vendor/b\nanalyze vendor/c\nwrite vendor/c vendor/c vendor/d\n")
	doc, issues := modelcatalog.ParseDocument(text)
	if len(issues) != 0 {
		t.Fatalf("issues = %v", issues)
	}
	if len(doc.Sections) != 1 || doc.Recommendations == nil || len(*doc.Recommendations) != 2 {
		t.Fatalf("doc = %+v", doc)
	}
	first, second := (*doc.Recommendations)[0], (*doc.Recommendations)[1]
	if first.Label != "Balanced" || second.Label != "Free start · 무료" {
		t.Fatalf("labels = %q, %q (the rest of the set line, trimmed)", first.Label, second.Label)
	}
	observe, ok := first.Stage("observe")
	if !ok || !reflect.DeepEqual(observe.IDs, []string{"vendor/eyes", "vendor/eyes", "vendor/sight"}) {
		t.Fatalf("observe = %+v", observe)
	}
}

// Every problem in the section is reported at once, each on its own line.
func TestParseDocumentRefusesMalformedSets(t *testing.T) {
	for name, tc := range map[string]struct {
		sets  string
		cause string
	}{
		"empty label":           {"set\n", modelcatalog.IssueSetLabel},
		"label too long":        {"set " + strings.Repeat("가", modelcatalog.MaxDocumentSetLabelRunes+1) + "\n", modelcatalog.IssueSetLabel},
		"repeated label":        {setBlock + setBlock, modelcatalog.IssueDuplicateSet},
		"stage before a set":    {"observe vendor/a vendor/a vendor/b\n", modelcatalog.IssueOrphanStage},
		"repeated stage":        {setBlock + "analyze vendor/pen\n", modelcatalog.IssueDuplicateStage},
		"missing stage":         {"set Partial\nobserve vendor/a vendor/a vendor/b\nwrite vendor/c vendor/c vendor/d\n", modelcatalog.IssueMissingStage},
		"unknown stage":         {"set X\nlisten vendor/a\n", modelcatalog.IssueMalformedLine},
		"wrong id count":        {"set X\nanalyze vendor/a vendor/b\n", modelcatalog.IssueMalformedLine},
		"not a model id":        {"set X\nanalyze claude\n", modelcatalog.IssueMalformedLine},
		"a bare id line":        {"vendor/a\n", modelcatalog.IssueMalformedLine},
		"second section header": {setBlock + "[recommendations]\n", modelcatalog.IssueDuplicateSection},
	} {
		t.Run(name, func(t *testing.T) {
			_, issues := modelcatalog.ParseDocument(withRecommendations("", tc.sets))
			found := false
			for _, issue := range issues {
				found = found || issue.Cause == tc.cause
			}
			if !found {
				t.Fatalf("issues = %v, want %s", issues, tc.cause)
			}
		})
	}
	// The missing stage is named on the set line.
	_, issues := modelcatalog.ParseDocument(withRecommendations("", "set Partial\nobserve vendor/a vendor/a vendor/b\nwrite vendor/c vendor/c vendor/d\n"))
	if len(issues) != 1 || issues[0].Text != "analyze" || issues[0].Line != 3 {
		t.Fatalf("missing stage issue = %+v", issues)
	}
}

// MODEL-69: an eleventh set is refused on its own line.
func TestParseDocumentRefusesTheEleventhSet(t *testing.T) {
	var sets strings.Builder
	for i := 0; i <= modelcatalog.MaxDocumentSets; i++ {
		sets.WriteString(strings.Replace(setBlock, "Balanced", "Set "+string(rune('A'+i)), 1))
	}
	doc, issues := modelcatalog.ParseDocument(withRecommendations("", sets.String()))
	if !reflect.DeepEqual(causesOf(issues), []string{modelcatalog.IssueSetLimit}) || len(*doc.Recommendations) != modelcatalog.MaxDocumentSets {
		t.Fatalf("issues = %v, sets = %d", issues, len(*doc.Recommendations))
	}
}

// MODEL-55: the export ends with every set in order and parses back to them.
func TestRenderDocumentAppendsTheSetsAndParsesBack(t *testing.T) {
	sets := []modelcatalog.StoredSet{
		{ID: "a", Label: "Balanced · August 2026", Observe: [3]string{"vendor/eyes", "vendor/eyes", "vendor/sight"}, Analyze: "vendor/pen", Write: [3]string{"vendor/pen", "vendor/pen", "vendor/quill"}},
		{ID: "b", Label: "Free", Observe: [3]string{"vendor/a", "vendor/a", "vendor/b"}, Analyze: "vendor/c", Write: [3]string{"vendor/c", "vendor/c", "vendor/d"}},
	}
	text := modelcatalog.RenderDocument(nil, sets)
	if !strings.HasSuffix(text, "[recommendations]\nset Balanced · August 2026\nobserve vendor/eyes vendor/eyes vendor/sight\nanalyze vendor/pen\nwrite vendor/pen vendor/pen vendor/quill\n\nset Free\nobserve vendor/a vendor/a vendor/b\nanalyze vendor/c\nwrite vendor/c vendor/c vendor/d\n") {
		t.Fatalf("rendered:\n%s", text)
	}
	doc, issues := modelcatalog.ParseDocument(text)
	if len(issues) != 0 || len(*doc.Recommendations) != 2 {
		t.Fatalf("issues = %v", issues)
	}
	if empty := modelcatalog.RenderDocument(nil, nil); !strings.HasSuffix(empty, "\n[recommendations]\n") {
		t.Fatalf("no set still renders an empty section:\n%s", empty)
	}
}

// documentCatalog is a catalog where `eyes`/`sight` are graded photo-analysis models and
// `pen`/`quill` are graded for writing and style analysis. `draft` is offered but not curated.
func documentCatalog() (*fakeStore, *fakeUpstream) {
	graded := func(id string, level modelcatalog.Level, purposes ...modelcatalog.Purpose) modelcatalog.Model {
		row := curated(id, purposes...)
		row.Levels = map[modelcatalog.Purpose]modelcatalog.Level{}
		for _, purpose := range purposes {
			row.Levels[purpose] = level
		}
		return row
	}
	store := newFakeStore(
		graded("vendor/eyes", modelcatalog.LevelValue, modelcatalog.PurposePhotoAnalysis),
		graded("vendor/sight", modelcatalog.LevelTop, modelcatalog.PurposePhotoAnalysis),
		graded("vendor/pen", modelcatalog.LevelValue, modelcatalog.PurposeWriting, modelcatalog.PurposeStyleAnalysis),
		graded("vendor/quill", modelcatalog.LevelTop, modelcatalog.PurposeWriting),
	)
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("vendor/eyes", 1), candidate("vendor/sight", 2), candidate("vendor/pen", 3),
		candidate("vendor/quill", 4), candidate("vendor/draft", 5),
	}}
	return store, upstream
}

func balanced(id string) modelcatalog.StoredSet {
	return modelcatalog.StoredSet{ID: id, Label: "Balanced", Observe: [3]string{"vendor/eyes", "vendor/eyes", "vendor/sight"}, Analyze: "vendor/pen", Write: [3]string{"vendor/pen", "vendor/pen", "vendor/quill"}}
}

func TestApplyDocumentReplacesTheSetsByLabel(t *testing.T) {
	store, upstream := documentCatalog()
	store.sets = []modelcatalog.StoredSet{
		{ID: "old", Label: "Old", Observe: [3]string{"vendor/eyes", "vendor/eyes", "vendor/sight"}, Analyze: "vendor/pen", Write: [3]string{"vendor/pen", "vendor/pen", "vendor/quill"}},
		balanced("kept"),
	}
	svc := newService(t, store, upstream)
	if _, err := svc.PreviewDocument(context.Background(), withRecommendations("", "")); err != nil || store.setWrites != 0 || len(store.sets) != 2 {
		t.Fatalf("a preview wrote the sets: %v, writes = %d", err, store.setWrites)
	}
	plan, err := svc.ApplyDocument(context.Background(), withRecommendations("",
		"set Fresh\nobserve vendor/sight vendor/sight vendor/eyes\nanalyze vendor/pen\nwrite vendor/quill vendor/quill vendor/pen\n\n"+
			strings.Replace(setBlock, "write vendor/pen vendor/pen vendor/quill", "write vendor/quill vendor/quill vendor/pen", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied || len(plan.Issues) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	got := plan.Recommendations
	if got == nil || !reflect.DeepEqual(got.Added, []string{"Fresh"}) || !reflect.DeepEqual(got.Removed, []string{"Old"}) ||
		!reflect.DeepEqual(got.Changed, []string{"Balanced"}) || len(got.Unchanged) != 0 {
		t.Fatalf("recommendation plan = %+v", got)
	}
	if len(store.sets) != 2 || store.sets[0].Label != "Fresh" || store.sets[0].ID != "new-1" ||
		store.sets[1].ID != "kept" || store.sets[1].Write != [3]string{"vendor/quill", "vendor/quill", "vendor/pen"} {
		t.Fatalf("stored sets = %+v", store.sets)
	}
}

// MODEL-73: a set is checked against what the SAME document leaves registered — a model it
// registers may be used, one it deregisters may not.
func TestDocumentSetsAreValidatedAgainstTheDocumentsOwnRegistrations(t *testing.T) {
	store, upstream := documentCatalog()
	svc := newService(t, store, upstream)
	usesDraft := "set Draft\nobserve vendor/eyes vendor/eyes vendor/sight\nanalyze vendor/pen\nwrite vendor/draft vendor/draft vendor/pen\n"

	plan, err := svc.PreviewDocument(context.Background(), withRecommendations("", usesDraft))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(causesOf(plan.Issues), []string{modelcatalog.IssueSlotUnregistered, modelcatalog.IssueSlotUnregistered}) {
		t.Fatalf("an unregistered model was accepted: %v", plan.Issues)
	}

	plan, err = svc.ApplyDocument(context.Background(), withRecommendations("[writing]\nvendor/pen value\nvendor/quill top\nvendor/draft balanced\n", usesDraft))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied {
		t.Fatalf("registering and using a model in one paste was refused: %v", plan.Issues)
	}

	plan, err = svc.PreviewDocument(context.Background(), withRecommendations("[writing]\nvendor/pen value\nvendor/quill top\n",
		strings.Replace(usesDraft, "Draft", "Draft two", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Issues) == 0 || plan.Issues[0].Cause != modelcatalog.IssueSlotUnregistered || plan.Issues[0].Text != "vendor/draft" {
		t.Fatalf("deregistering and using a model in one paste was accepted: %v", plan.Issues)
	}
}

func TestDocumentSetSlotCauses(t *testing.T) {
	store, upstream := documentCatalog()
	svc := newService(t, store, upstream)
	plan, err := svc.PreviewDocument(context.Background(), withRecommendations("[style-analysis]\nvendor/pen\n",
		"set X\nobserve vendor/eyes vendor/sight vendor/sight\nanalyze vendor/pen\nwrite vendor/pen vendor/pen vendor/quill\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(causesOf(plan.Issues), []string{modelcatalog.IssueSlotDuplicate, modelcatalog.IssueSlotUnclassified}) {
		t.Fatalf("issues = %v", plan.Issues)
	}
	if plan.Issues[0].Line != 6 || plan.Issues[1].Line != 7 || plan.Issues[1].Text != "vendor/pen" {
		t.Fatalf("issues point at %+v", plan.Issues)
	}
}

// A stored set whose model has since been deregistered is kept as it is when the document
// names it unchanged — so an export pasted back is no change (MODEL-73).
func TestAnExportedDocumentWithAStaleSetPreviewsAsNoChange(t *testing.T) {
	store, upstream := documentCatalog()
	stale := balanced("kept")
	stale.Write[2] = "vendor/retired"
	store.sets = []modelcatalog.StoredSet{stale}
	svc := newService(t, store, upstream)

	exported, err := svc.ExportDocument(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := svc.ApplyDocument(context.Background(), exported)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied || len(plan.Issues) != 0 {
		t.Fatalf("the export was refused: %v", plan.Issues)
	}
	if got := plan.Recommendations; got == nil || !reflect.DeepEqual(got.Unchanged, []string{"Balanced"}) || got.Reordered {
		t.Fatalf("recommendation plan = %+v", got)
	}
	if store.setWrites != 0 {
		t.Fatalf("an unchanged set list was rewritten %d times", store.setWrites)
	}
}

func TestADocumentWithoutTheSectionLeavesTheSets(t *testing.T) {
	store, upstream := documentCatalog()
	store.sets = []modelcatalog.StoredSet{balanced("kept")}
	svc := newService(t, store, upstream)
	plan, err := svc.ApplyDocument(context.Background(), modelcatalog.DocumentVersionLine+"\n[writing]\nvendor/pen value\nvendor/quill top\n")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied || plan.Recommendations != nil || store.setWrites != 0 || len(store.sets) != 1 {
		t.Fatalf("plan = %+v, writes = %d, sets = %+v", plan, store.setWrites, store.sets)
	}
}

func TestPlanRecommendationsReportsAReorder(t *testing.T) {
	a := balanced("a")
	b := balanced("b")
	b.Label = "Second"
	doc, issues := modelcatalog.ParseDocument(withRecommendations("",
		strings.Replace(setBlock, "Balanced", "Second", 1)+setBlock))
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	plan, next, issues := modelcatalog.PlanRecommendations(*doc.Recommendations, []modelcatalog.StoredSet{a, b}, nil)
	if len(issues) != 0 || !plan.Reordered || len(plan.Unchanged) != 2 || next[0].ID != "b" || next[1].ID != "a" {
		t.Fatalf("plan = %+v next = %+v issues = %v", plan, next, issues)
	}
}

// A failed set write is a failed document: nothing reports applied.
func TestApplyDocumentFailsWhenTheSetWriteFails(t *testing.T) {
	store, upstream := documentCatalog()
	svc := newService(t, store, upstream)
	store.syncErr = errors.New("disk full")
	if _, err := svc.ApplyDocument(context.Background(), withRecommendations("", setBlock)); err == nil {
		t.Fatal("a failed write reported success")
	}
}

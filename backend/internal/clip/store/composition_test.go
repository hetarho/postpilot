package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
)

type ownedPlanWriter struct{ *plannerFake }

func TestOldRoleStyleDraftCanBeReadAndExplicitlyCorrected(t *testing.T) {
	s, raw, d := setup(t)
	valid := `<clip version="1"><text id="badge" kind="fixed" role="badge">기록</text><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`
	invalid := strings.Replace(valid, `role="badge"`, `role="badge" style="simple"`, 1)
	template, err := s.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "historical style", CompositionBody: valid})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "saved draft", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Writer.Exec("UPDATE video_templates SET composition_body=? WHERE id=?", invalid, template.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Writer.Exec("UPDATE clip_projects SET composition_snapshot_json=json_set(composition_snapshot_json,'$.body',?) WHERE id=?", invalid, p.ID); err != nil {
		t.Fatal(err)
	}
	read, err := s.GetProject(t.Context(), "alice", p.ID)
	if err != nil || read.Composition.Snapshot.Body != invalid {
		t.Fatal("historical draft could not reopen", err)
	}
	if _, err = raw.GetTemplate(t.Context(), "alice", template.ID); err != nil {
		t.Fatal("old template not readable", err)
	}
	if _, err = s.UpdateTemplate(t.Context(), "alice", template.ID, clip.TemplatePatch{CompositionBody: &invalid}); err == nil {
		t.Fatal("old conflict could be saved again")
	}
	if _, err = s.UpdateTemplate(t.Context(), "alice", template.ID, clip.TemplatePatch{CompositionBody: &valid}); err != nil {
		t.Fatal("explicit correction failed", err)
	}
	fixed, err := s.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CompositionInputs: &read.Composition.Inputs})
	if err != nil || fixed.Composition.Snapshot.Body != valid {
		t.Fatal("corrected template not applied", err)
	}
}

func (ownedPlanWriter) CompositionPlanVersion() int { return clip.CompositionPlanVersion }
func (p ownedPlanWriter) Flow(ctx context.Context, model llm.ModelRef, in clip.PlanningInput) (clip.EditPlan, llm.Usage, error) {
	plan, usage, err := p.plannerFake.Plan(ctx, model, in)
	if err != nil {
		return plan, usage, err
	}
	plan.Cuts[0].Copies = nil
	doc, problem := composition.Parse(in.Composition.Snapshot.Body, config.ClipCompositionLimits())
	if problem != nil {
		return plan, usage, problem
	}
	cut := plan.Cuts[0]
	plan.Portable = &clip.PortablePlan{Snapshot: in.Composition.Snapshot, Inputs: in.Composition.Inputs, Cuts: []composition.Cut{{ID: cut.ID, SectionID: "shot", SourceID: cut.SourceID, StartMS: cut.StartMS, EndMS: cut.EndMS}}}
	resolved, problem := composition.Resolve(doc, composition.Inputs{Cuts: plan.Portable.Cuts}, config.ClipCompositionLimits(), 30000)
	if problem != nil {
		return plan, usage, problem
	}
	for _, e := range resolved.Elements {
		e.Text = "원래 작성한 문장"
		plan.Portable.Elements = append(plan.Portable.Elements, clip.PortableText{Resolved: e, Pace: "steady"})
	}
	return plan, usage, nil
}

type ownedPlanRenderer struct{ *rendererFake }

func (ownedPlanRenderer) CompositionPlanVersion() int { return clip.CompositionPlanVersion }

// The layout is where the executor decides what the plan finally says: it runs
// before any render, which is the only place a generation reaches now that it
// stops at the plan (CLIP-151), and it is what the draft preview then draws.
func (ownedPlanRenderer) LayoutComposition(_ context.Context, p clip.EditPlan, _ []clip.RenderSource) (clip.EditPlan, []clip.CompositionElement, error) {
	if p.Portable == nil || p.Disclosure != "" || p.Hook != "" || len(p.Facts) != 0 {
		return p, nil, errors.New("injected legacy content into native layout")
	}
	p.Portable.Elements[0].Resolved.Text = "짧은 문장"
	p.Portable.Elements[0].FallbackReason = "shorter_copy"
	p.Portable.Elements[0].Placement = &clip.CompositionPlacement{Style: "memo", Position: "top", StartMS: 120, EndMS: 14880}
	return p, nil, nil
}
func (r ownedPlanRenderer) Render(ctx context.Context, ws clip.MediaWorkspace, p clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) (clip.RenderedVideo, error) {
	if p.Portable == nil || p.Disclosure != "" || p.Hook != "" || len(p.Facts) != 0 {
		return clip.RenderedVideo{}, errors.New("injected legacy content into native render")
	}
	return r.rendererFake.Render(ctx, ws, p, sources, load)
}
func TestGenerationPersistsTheRenderedOwnedPlanAndItsStyles(t *testing.T) {
	h := generationSetup(t)
	h.service = clip.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, ownedPlanWriter{h.planner}, ownedPlanRenderer{h.renderer}, generationJobs{h.queue}, h.cfg).WithFinisher(generationFinisher{h.store}).WithCredits(&quotePricing{}, nil)
	h.projects.SetGeneration(h.service)
	body := `<clip version="1"><repeat for="scenes"><scene id="shot"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene></repeat><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`
	template, err := legacyTemplate(t, h.store, "alice", "owned-render", body), error(nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{VideoTemplateID: &template.ID})
	if err != nil {
		t.Fatal(err)
	}
	h.project = p
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	p, err = h.store.GetProject(t.Context(), "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Portable.Elements[0].Resolved.Text != "짧은 문장" {
		t.Fatal("stored pre-render plan or live template styles")
	}
}

const nativeBody = `<clip version="1"><field id="a" label="가격"/><field id="b" label="가격" required="true"/><group id="menu"><field id="price" label="가격"/></group><repeat for="scenes"><scene id="shot" scope="scene"><text id="copy" kind="ai" role="caption" basis="cut">장면만 설명</text></scene></repeat><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`

func TestNativeCompositionOwnedRoundTripAndRequiredIDs(t *testing.T) {
	s, st, d := setup(t)
	ctx := context.Background()
	body := "\n" + nativeBody + "\n"
	template, err := legacyTemplate(t, st, "alice", "native", body), error(nil)
	if err != nil || template.CompositionBody != body || template.CompositionLegacy {
		t.Fatal(template, err)
	}
	inputs := clip.CompositionInputs{Values: map[string]string{"a": "  12,000원 🥣\n", "b": ""}, Items: map[string][]composition.Item{"menu": {{ID: "dish-a", Values: map[string]string{"price": "9,000원"}}, {ID: "dish-b", Values: map[string]string{"price": "15,000원"}}}}}
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "draft", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000, CompositionInputs: &inputs})
	if err != nil {
		t.Fatal(err)
	}
	if err := clip.RequiredAnswers(template, p, config.ClipCompositionLimits()); err == nil {
		t.Fatal("blank required ID passed")
	}
	inputs.Values["b"] = "18,000원"
	p, err = s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &inputs})
	if err != nil {
		t.Fatal(err)
	}
	if err := clip.RequiredAnswers(template, p, config.ClipCompositionLimits()); err != nil {
		t.Fatal("legacy category/disclosure gate survived", err)
	}
	restarted := clip.NewService(store.New(d.Writer, d.Reader), config.ClipLimits())
	got, err := restarted.GetProject(ctx, "alice", p.ID)
	if err != nil || !reflect.DeepEqual(got.Composition, p.Composition) || got.Composition.Snapshot.Body != body {
		t.Fatal(got.Composition, err)
	}
	if _, err := s.UpdateProject(ctx, "bob", p.ID, clip.ProjectPatch{CompositionInputs: &inputs}); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("foreign update", err)
	}
	if _, err := s.CreateProject(ctx, "bob", clip.ProjectInput{Language: "ko", Title: "foreign", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000}); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("foreign template", err)
	}
	cleared := clip.CompositionInputs{Values: map[string]string{}, Items: map[string][]composition.Item{}}
	got, err = s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &cleared})
	if err != nil || len(got.Composition.Inputs.Values) != 0 || len(got.Composition.Inputs.Items) != 0 {
		t.Fatal("clear presence", got.Composition, err)
	}
	before := got.EditPlanRevision
	got, err = s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &cleared})
	if err != nil || got.EditPlanRevision != before {
		t.Fatal("no-op invalidated revision", err)
	}
}

func TestNativeSnapshotSurvivesTemplateEditAndDeletion(t *testing.T) {
	s, st, _ := setup(t)
	ctx := context.Background()
	template, err := legacyTemplate(t, st, "alice", "native", nativeBody), error(nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "draft", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal(err)
	}
	before := p.Composition
	changed := strings.Replace(nativeBody, "장면만 설명", "다른 구성", 1)
	if _, err = st.UpdateTemplate(ctx, "alice", template.ID, clip.TemplatePatch{CompositionBody: &changed}, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetProject(ctx, "alice", p.ID)
	if err != nil || !reflect.DeepEqual(before, got.Composition) {
		t.Fatal("template edit changed snapshot", err)
	}
	if _, err = s.DeleteTemplate(ctx, "alice", template.ID); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetProject(ctx, "alice", p.ID)
	if err != nil || got.VideoTemplateID != "" || !reflect.DeepEqual(before, got.Composition) {
		t.Fatal("detach lost snapshot", err)
	}
}

func TestLegacyConversionKeepsProjectChoicesResultsAndFrozenRecipe(t *testing.T) {
	s, raw, d := setup(t)
	ctx := context.Background()
	template, first := create(t, s)
	second, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "hidden", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 30000, Disclosure: "ad", HideDisclosure: true})
	if err != nil {
		t.Fatal(err)
	}
	result := clip.Result{Key: "unchanged.mp4", ContentType: "video/mp4", Bytes: 123, DurationMS: 15000, CreatedAt: time.Now()}
	if err = raw.SaveGeneration(ctx, "alice", first.ID, "[]", "plan", result); err != nil {
		t.Fatal(err)
	}
	// Old rows have no new columns populated. Read conversion must not write.
	if _, err = d.Writer.Exec(`UPDATE video_templates SET composition_body=NULL, preset='' WHERE id=?`, template.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Writer.Exec(`UPDATE clip_projects SET composition_snapshot_json=NULL, composition_inputs_json=NULL WHERE user_id='alice'`); err != nil {
		t.Fatal(err)
	}
	first, err = s.GetProject(ctx, "alice", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err = s.GetProject(ctx, "alice", second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.Composition.Snapshot.Body, "협찬") || strings.Contains(second.Composition.Snapshot.Body, "legacy-disclosure") || strings.Count(first.Composition.Snapshot.Body, `role="hook"`) != 1 || strings.Count(first.Composition.Snapshot.Body, `role="ending"`) != 1 {
		t.Fatal("visibility or empty cards changed", first.Composition, second.Composition)
	}
	var count int
	if err = d.Reader.QueryRow(`SELECT count(*) FROM clip_projects WHERE composition_snapshot_json IS NOT NULL`).Scan(&count); err != nil || count != 0 {
		t.Fatal("read conversion wrote rows", count, err)
	}
	newGuide := "new shared template"
	if _, err = s.UpdateTemplate(ctx, "alice", template.ID, clip.TemplatePatch{CutGuidance: &newGuide}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DeleteTemplate(ctx, "alice", template.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetProject(ctx, "alice", first.ID)
	if err != nil || got.EditPlan != "plan" || got.Analysis != "[]" || got.EditPlanRevision != first.EditPlanRevision || got.RenderedPlanRevision != first.RenderedPlanRevision || !reflect.DeepEqual(got.Result, first.Result) || !reflect.DeepEqual(got.Composition, first.Composition) {
		t.Fatal("migration/detach changed retained output", got, err)
	}
	ad := "ad"
	got, err = s.UpdateProject(ctx, "alice", got.ID, clip.ProjectPatch{Disclosure: &ad})
	if err != nil || got.Composition.Snapshot.LegacyRecipe.CutGuidance == newGuide {
		t.Fatal("project write used edited shared recipe", err)
	}
	again, err := s.GetProject(ctx, "alice", second.ID)
	if err != nil || !reflect.DeepEqual(again.Composition, second.Composition) {
		t.Fatal("shared disclosure leaked", err)
	}
}

func TestCompositionAssociationHoldsToObservationsAndBindsTheQuote(t *testing.T) {
	doc, err := composition.Parse(nativeBody, config.ClipCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	input := clip.CompositionInputs{Values: map[string]string{"b": "3"}, Items: map[string][]composition.Item{"menu": {{ID: "dish", Values: map[string]string{"price": "9000"}}}}, Associations: []clip.SourceAssociation{{GroupID: "menu", ItemID: "dish", SourceID: "source", Fingerprint: "sha", StartMS: 100, EndMS: 500}}}
	if err := clip.ValidateCompositionInputs(doc, input, config.ClipCompositionLimits(), true); err != nil {
		t.Fatal(err)
	}
	a := []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "sha", Info: clip.MediaInfo{DurationMS: 1000}}}, Segments: []clip.Segment{{StartMS: 0, EndMS: 700}}}}
	b, _ := json.Marshal(a)
	p := clip.Project{Analysis: string(b), Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: 1, Body: nativeBody}, Inputs: input}}
	if err := clip.ValidateSourceAssociations(p, input.Associations); err != nil {
		t.Fatal(err)
	}
	base := clip.QuoteInputDigest(p, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{})
	input.Associations[0].EndMS = 600
	if next := clip.QuoteInputDigest(p, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{}); next == base {
		t.Fatal("association omitted from quote")
	}
	// A range the observations contradict is still refused: this source was
	// looked at, and 800 ms is past the 700 ms that was observed.
	input.Associations[0].EndMS = 800
	if err := clip.ValidateSourceAssociations(p, input.Associations); err == nil {
		t.Fatal("unobserved range accepted")
	}
	// A whole-source binding spans every segment by design (CLIP-123).
	input.Associations[0].StartMS, input.Associations[0].EndMS = 0, 1000
	if err := clip.ValidateSourceAssociations(p, input.Associations); err != nil {
		t.Fatal("whole-source binding refused", err)
	}
	// A binding matching no retained observation is ignored rather than
	// refused: before generation there are none, and a replaced source simply
	// stops matching (CLIP-123).
	input.Associations[0].Fingerprint = "other"
	if err := clip.ValidateSourceAssociations(p, input.Associations); err != nil {
		t.Fatal("a binding for an unobserved source was refused", err)
	}
	input.Values["menu.price"] = "smuggled"
	if err := clip.ValidateCompositionInputs(doc, input, config.ClipCompositionLimits(), true); err == nil {
		t.Fatal("group field escaped scope")
	}
}

func TestUnsupportedCompositionRefusesQuoteBeforeMediaOrCreditWork(t *testing.T) {
	h := generationSetup(t)
	ctx := context.Background()
	body := `<clip version="1"><repeat for="scenes"><scene id="shot" scope="scene"/></repeat><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`
	if _, err := h.store.UpdateTemplate(ctx, "alice", h.template.ID, clip.TemplatePatch{CompositionBody: &body}, time.Now()); err != nil {
		t.Fatal(err)
	}
	_, err := h.service.Quote(ctx, "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	// Old answers have no IDs in the new empty-field composition; clear them
	// by selecting its native snapshot through an independently saved project.
	if err == nil {
		t.Fatal("unsupported native workflow was quoted")
	}
	template, err := legacyTemplate(t, h.store, "alice", "native-capability", body), error(nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := h.projects.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "native", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.service.Quote(ctx, "alice", p.ID, h.batch.ID, "p/o", "p/w")
	if !errors.Is(err, clip.ErrCompositionUnavailable) {
		t.Fatal("missing capability was not authoritative", err)
	}
	assertNoQuoteWork(t, h)
}

func TestLegacyEscapingKeepsMaximumValidGuidanceReadable(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	r := recipe()
	r.CutGuidance = strings.Repeat("&", config.ClipLimits().GuidanceChars)
	template, err := s.CreateTemplate(ctx, "alice", r)
	if err != nil {
		t.Fatal(err)
	}
	values, err := s.ListTemplates(ctx, "alice")
	if err != nil || len(values) != 1 || values[0].CompositionBody != template.CompositionBody {
		t.Fatal("XML expansion hid an existing template", err)
	}
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "maximum guide", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetProject(ctx, "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTemplate(ctx, "alice", clip.Recipe{Name: "authored too large", CompositionBody: template.CompositionBody}); err == nil {
		t.Fatal("legacy expansion allowance enlarged native authoring")
	}
}

func TestConvertedLegacyFieldsAcceptTypedUpdatesWithoutLosingHistoricalAnswers(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	_, p := create(t, s)
	input := clip.CompositionInputs{Values: map[string]string{clip.LegacyFieldID("장소"): "  부산 & 바다\n"}}
	got, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &input})
	if err != nil {
		t.Fatal(err)
	}
	answers := map[string]string{}
	for _, a := range got.Answers {
		answers[a.Label] = a.Text
	}
	if answers["장소"] != "  부산 & 바다\n" || answers["이전 질문"] != "보존" || got.Composition.Inputs.Values[clip.LegacyFieldID("장소")] != answers["장소"] {
		t.Fatal("typed and legacy inputs diverged", got)
	}
	cleared := clip.CompositionInputs{}
	got, err = s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &cleared})
	if err != nil || got.Composition.Inputs.Values[clip.LegacyFieldID("장소")] != "" {
		t.Fatal("clear did not reach legacy fields", err)
	}
}

type compositionPlanner struct{ *plannerFake }

func (compositionPlanner) CompositionPlanVersion() int { return clip.CompositionPlanVersion }

type compositionRenderer struct{ *rendererFake }

func (compositionRenderer) CompositionPlanVersion() int { return clip.CompositionPlanVersion }

func TestNativeQuoteInvalidatesGroupedValuesAndFreezesAcceptedComposition(t *testing.T) {
	h := generationSetup(t)
	ctx := context.Background()
	h.service = clip.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, compositionPlanner{h.planner}, compositionRenderer{h.renderer}, generationJobs{h.queue}, h.cfg).WithFinisher(generationFinisher{h.store}).WithCredits(&quotePricing{}, nil)
	h.projects.SetGeneration(h.service)
	template, err := legacyTemplate(t, h.store, "alice", "native-quote", nativeBody), error(nil)
	if err != nil {
		t.Fatal(err)
	}
	inputs := clip.CompositionInputs{Values: map[string]string{"b": "required"}, Items: map[string][]composition.Item{"menu": {{ID: "dish", Values: map[string]string{"price": "9,000원"}}}}}
	p, err := h.projects.UpdateProject(ctx, "alice", h.project.ID, clip.ProjectPatch{VideoTemplateID: &template.ID, CompositionInputs: &inputs})
	if err != nil {
		t.Fatal(err)
	}
	h.project = p
	q := quote(t, h)
	inputs.Items["menu"][0].Values["price"] = "12,000원"
	if _, err = h.projects.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &inputs}); err != nil {
		t.Fatal(err)
	}
	if _, err = accept(h, q); !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal("changed item reused approval", err)
	}
	q = quote(t, h)
	jobID, err := accept(h, q)
	if err != nil {
		t.Fatal(err)
	}
	j, err := h.jobs.GetByID(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Version     int
		Composition *clip.ProjectComposition
	}
	if json.Unmarshal(j.Payload, &payload) != nil || payload.Version != 5 || payload.Composition == nil || payload.Composition.Snapshot.Body != nativeBody || payload.Composition.Snapshot.Legacy || payload.Composition.Inputs.Items["menu"][0].Values["price"] != "12,000원" {
		t.Fatal("accepted payload lost composition")
	}
	changed := strings.Replace(nativeBody, "장면만 설명", "새 구성", 1)
	if _, err = h.store.UpdateTemplate(ctx, "alice", template.ID, clip.TemplatePatch{CompositionBody: &changed}, time.Now()); err != nil {
		t.Fatal(err)
	}
	again, err := h.jobs.GetByID(ctx, jobID)
	if err != nil || !reflect.DeepEqual(again.Payload, j.Payload) {
		t.Fatal("template edit rewrote accepted job", err)
	}
	assertNoQuoteWork(t, h)
}

func TestApplyingCurrentTemplateInputsPreservesPriorRenderedResult(t *testing.T) {
	s, raw, _ := setup(t)
	ctx := t.Context()
	template, err := legacyTemplate(t, raw, "alice", "editable", nativeBody), error(nil)
	if err != nil {
		t.Fatal(err)
	}
	input := clip.CompositionInputs{Values: map[string]string{"b": "1인분 12000원"}, Items: map[string][]composition.Item{"menu": {{ID: "retained-item", Values: map[string]string{"price": "9000원"}}}}}
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "before", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000, CompositionInputs: &input})
	if err != nil {
		t.Fatal(err)
	}
	result := clip.Result{Key: "prior-result.mp4", ContentType: "video/mp4", Bytes: 123, DurationMS: 15000, CreatedAt: time.Now()}
	if err = raw.SaveGeneration(ctx, "alice", p.ID, "[]", "prior-plan", result); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetProject(ctx, "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(nativeBody, `label="가격"`, `label="가격 안내"`, 1)
	changed = strings.Replace(changed, `</clip>`, `<field id="new" label="추가 정보"/></clip>`, 1)
	if _, err = raw.UpdateTemplate(ctx, "alice", template.ID, clip.TemplatePatch{CompositionBody: &changed}, time.Now()); err != nil {
		t.Fatal(err)
	}
	read, err := s.GetProject(ctx, "alice", p.ID)
	if err != nil || read.Composition.Snapshot.Body != nativeBody {
		t.Fatal("template read applied unsaved changes", err)
	}
	input.Values["new"] = "new fact"
	updated, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &input})
	if err != nil {
		t.Fatal("current field rejected by old snapshot", err)
	}
	if updated.Composition.Snapshot.Body != changed || !reflect.DeepEqual(updated.Composition.Inputs, input) || updated.EditPlanRevision <= before.EditPlanRevision {
		t.Fatal("current inputs were not applied")
	}
	if !reflect.DeepEqual(updated.Result, before.Result) || updated.EditPlan != before.EditPlan || updated.RenderedPlanRevision != before.RenderedPlanRevision {
		t.Fatal("applying inputs replaced previous output")
	}
	input.Values["unknown"] = "must not be saved"
	if _, err = s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &input}); err == nil {
		t.Fatal("unknown field accepted")
	}
	after, err := s.GetProject(ctx, "alice", p.ID)
	if err != nil || !reflect.DeepEqual(after.Composition, updated.Composition) {
		t.Fatal("invalid edit replaced saved input", err)
	}
}

// A project with no plan has nothing for changed inputs to stale, and CLIP-36 reads any revision
// above zero as an editing state. Raising one here opened the owner at step 2, which offers no way
// to upload the originals step 1 collects, so the clip could never be generated at all.
func TestSavingInputsBeforeAnyGenerationLeavesTheProjectADraft(t *testing.T) {
	s, raw, _ := setup(t)
	ctx := t.Context()
	template := legacyTemplate(t, raw, "alice", "editable", nativeBody)
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "before", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal(err)
	}
	if p.EditPlanRevision != 0 {
		t.Fatalf("a fresh project already carries revision %d", p.EditPlanRevision)
	}
	input := clip.CompositionInputs{Values: map[string]string{"b": "1인분 12000원"}, Items: map[string][]composition.Item{"menu": {{ID: "item", Values: map[string]string{"price": "9000원"}}}}}
	saved, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &input})
	if err != nil {
		t.Fatal("inputs refused", err)
	}
	if !reflect.DeepEqual(saved.Composition.Inputs, input) {
		t.Fatal("inputs were not saved")
	}
	if saved.EditPlanRevision != 0 || saved.RenderedPlanRevision != 0 {
		t.Fatalf("saving inputs staled a plan that does not exist: %d/%d", saved.EditPlanRevision, saved.RenderedPlanRevision)
	}
	// A second edit is the one that used to pile up: every pause in the form raised it again.
	input.Values["a"] = "another fact"
	again, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &input})
	if err != nil || again.EditPlanRevision != 0 {
		t.Fatal("a further edit left the draft", err)
	}
	// Once a plan exists the same save must stale it, which is what CLIP-139 asks for.
	result := clip.Result{Key: "result.mp4", ContentType: "video/mp4", Bytes: 123, DurationMS: 15000, CreatedAt: time.Now()}
	if err = raw.SaveGeneration(ctx, "alice", p.ID, "[]", "plan", result); err != nil {
		t.Fatal(err)
	}
	generated, err := s.GetProject(ctx, "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	input.Values["a"] = "changed after the render"
	staled, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{CompositionInputs: &input})
	if err != nil || staled.EditPlanRevision != generated.EditPlanRevision+1 {
		t.Fatal("inputs changed after a render did not stale it", err)
	}
}

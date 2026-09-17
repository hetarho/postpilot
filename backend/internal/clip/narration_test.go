package clip_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

// narrationFixture is the creation fixture with one narration caption saved on
// it: two cuts of observed footage, and a sentence that owns 2–5s of the output
// timeline without belonging to either of them.
func narrationFixture(t *testing.T) (clip.Project, clip.EditPlan) {
	t.Helper()
	p, plan := creationFixture(t)
	plan.Portable.Elements = append(plan.Portable.Elements, clip.NarrationCaption("narration-1", "고기부터 올렸어요", 2000, 5000))
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	p.EditPlan = raw
	saved, err := clip.DecodeEditPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	saved.Portable.NativeEditing = true
	return p, saved
}

func narrationOf(plan clip.EditPlan, id string) (clip.PortableText, bool) {
	for _, text := range plan.Portable.Elements {
		if text.Resolved.InstanceID == id {
			return text, text.Scope == clip.NarrationScope
		}
	}
	return clip.PortableText{}, false
}

func planProblem(t *testing.T, err error) *composition.Problem {
	t.Helper()
	var problem *composition.Problem
	if !errors.As(err, &problem) {
		t.Fatal("not an identified refusal", err)
	}
	return problem
}

func TestNarrationRidesThePlanWithoutADeclaration(t *testing.T) {
	p, plan := narrationFixture(t)
	saved, ok := narrationOf(plan, "narration-1")
	if !ok {
		t.Fatal("the narration caption did not survive the envelope", plan.Portable.Elements)
	}
	r := saved.Resolved
	if r.CutID != "" || r.Element.Kind != "ai" || r.Element.Role != "caption" || r.Element.Basis != "output-start" {
		t.Fatal("a narration caption was stored as a cut-bound declaration", r)
	}
	if r.StartMS != 2000 || r.EndMS != 5000 || r.Text != "고기부터 올렸어요" {
		t.Fatal("the absolute interval or the sentence changed", r)
	}
	// The snapshot declares the template's fixed regions and nothing else, so
	// this caption is admitted on its own shape.
	if strings.Contains(plan.Portable.Snapshot.Body, "narration-1") {
		t.Fatal("the caption was written into the template snapshot")
	}
	if strings.Contains(p.EditPlan, `"Version":6`) == false {
		t.Fatal("the caption did not ride the version-6 envelope")
	}
	// Every earlier envelope still decodes: none of its readers moved.
	legacy, _ := correctionFixture(t)
	if _, err := clip.DecodeEditPlan(legacy.EditPlan); err != nil {
		t.Fatal("a version-4 plan stopped decoding", err)
	}
}

func TestNarrationValidationRefusals(t *testing.T) {
	for name, mutate := range map[string]func(*clip.EditPlan){
		"beyond the output": func(p *clip.EditPlan) {
			end := p.DurationMS + 1
			p.Portable.Elements[2].Resolved.Element.EndMS = &end
			p.Portable.Elements[2].Resolved.EndMS = end
		},
		"overlapping another caption": func(p *clip.EditPlan) {
			p.Portable.Elements = append(p.Portable.Elements, clip.NarrationCaption("narration-2", "두 번째", 4000, 6000))
		},
		"bound to a cut": func(p *clip.EditPlan) {
			p.Portable.Elements[2].Resolved.CutID = "first"
		},
		"carrying an identity the server did not mint": func(p *clip.EditPlan) {
			p.Portable.Elements[2].Resolved.InstanceID = "owner-caption"
			p.Portable.Elements[2].Resolved.Element.ID = "owner-caption"
		},
		"resolved away from its own declaration": func(p *clip.EditPlan) {
			p.Portable.Elements[2].Resolved.StartMS = 1000
		},
	} {
		_, plan := narrationFixture(t)
		mutate(&plan)
		if _, err := clip.EncodeEditPlan(plan); err == nil {
			t.Fatal("an invalid narration caption was stored: " + name)
		}
	}
	// Two captions that merely touch do not overlap.
	_, plan := narrationFixture(t)
	plan.Portable.Elements = append(plan.Portable.Elements, clip.NarrationCaption("narration-2", "두 번째", 5000, 7000))
	if _, err := clip.EncodeEditPlan(plan); err != nil {
		t.Fatal("adjacent narration intervals were read as an overlap", err)
	}
}

func TestNarrationKeepsItsOutputIntervalThroughEveryFootageEdit(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	for name, edit := range map[string]func(*clip.CorrectionPlan){
		"reorder": func(d *clip.CorrectionPlan) {
			d.Cuts[0], d.Cuts[1] = d.Cuts[1], d.Cuts[0]
		},
		"add": func(d *clip.CorrectionPlan) {
			d.Cuts = append(d.Cuts, addedCut(ownerCut, "first", 20000, 30000))
			d.DurationMS = 30000
		},
		"split": func(d *clip.CorrectionPlan) {
			second := d.Cuts[1]
			d.Cuts[0].EndMS = 5000
			split := clip.CorrectionCut{ID: ownerCut, StartMS: 5000, EndMS: 10000, VolumePermille: 1000,
				Creation: &clip.CutCreation{Kind: clip.CutSplit, OriginID: "first"}}
			d.Cuts = []clip.CorrectionCut{d.Cuts[0], split, second}
		},
		"trim": func(d *clip.CorrectionPlan) {
			d.Cuts[1].StartMS, d.Cuts[1].EndMS = 12000, 20000
			d.DurationMS = 18000
		},
		"rate": func(d *clip.CorrectionPlan) {
			d.Cuts[1].PlaybackRatePermille = 2000
			d.DurationMS = 15000
		},
	} {
		p, plan := narrationFixture(t)
		draft := creationDraft(plan)
		edit(&draft)
		next, err := clip.ApplyCorrection(cfg, p, draft)
		if err != nil {
			t.Fatal(name, err)
		}
		saved, ok := narrationOf(next, "narration-1")
		if !ok || saved.Resolved.StartMS != 2000 || saved.Resolved.EndMS != 5000 || saved.Resolved.CutID != "" {
			t.Fatal("a footage edit moved the narration: "+name, saved.Resolved)
		}
	}
	// Trimming the output below a caption's end is the one case CLIP-67 refuses
	// rather than retimes, and it names the caption the owner has to correct.
	p, plan := narrationFixture(t)
	draft := creationDraft(plan)
	draft.Cuts[0].EndMS = 2000
	draft.Cuts[1].StartMS, draft.Cuts[1].EndMS = 10000, 11000
	draft.DurationMS = 3000
	_, err := clip.ApplyCorrection(cfg, p, draft)
	problem := planProblem(t, err)
	if problem.Reason != clip.NoticeCaptionOutsideOutput || problem.ElementID != "narration-1" {
		t.Fatal("a caption outside the shortened output was not identified", problem)
	}
}

func ownerCaption(text string, start, end int) clip.CorrectionText {
	return clip.CorrectionText{Narration: true, Kind: "ai", Role: "caption", Text: text,
		Style: "auto", Position: "auto", Align: "center", Basis: "output-start", StartMS: &start, EndMS: &end,
		Creation: &clip.TextCreation{Kind: clip.CutAdd}}
}

func TestOwnerAddsEditsAndRemovesANarrationCaption(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	p, plan := narrationFixture(t)
	draft := creationDraft(plan)
	draft.Elements = append(draft.Elements, ownerCaption("직접 쓴 자막", 8000, 11000))
	next, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	added, ok := narrationOf(next, "narration-2")
	if !ok || added.Resolved.Text != "직접 쓴 자막" || added.Resolved.StartMS != 8000 || added.Resolved.EndMS != 11000 {
		t.Fatal("the added caption is not the one the owner asked for", added.Resolved)
	}
	if !added.OwnerEdited {
		t.Fatal("an owner-written caption was left for the writer to ground")
	}
	raw, err := clip.EncodeEditPlan(next)
	if err != nil || strings.Contains(raw, "Creation") {
		t.Fatal("creation provenance reached the stored plan", err)
	}
	// The projection hands it back as an ordinary caption, so a resave corrects
	// it rather than creating a second one.
	for _, e := range clip.CorrectionFromPlan(next).Elements {
		if e.Creation != nil {
			t.Fatal("the projection claimed a caption was still being created", e.InstanceID)
		}
		if e.InstanceID == "narration-2" && !e.Narration {
			t.Fatal("the projection lost the caption's scope", e)
		}
	}
	p.EditPlan = raw

	draft = creationDraft(next)
	for i, e := range draft.Elements {
		if e.InstanceID != "narration-1" {
			continue
		}
		moved, until := 12000, 16000
		draft.Elements[i].Text, draft.Elements[i].StartMS, draft.Elements[i].EndMS = "다시 쓴 자막", &moved, &until
		draft.Elements[i].Pace = "rapid"
		draft.Elements[i].Phrases = []clip.EditablePhrase{{Text: "다시", StartMS: 12000, EndMS: 12600}, {Text: "쓴 자막", StartMS: 12600, EndMS: 13400}}
	}
	edited, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := narrationOf(edited, "narration-1")
	if saved.Resolved.Text != "다시 쓴 자막" || saved.Resolved.StartMS != 12000 || saved.Resolved.EndMS != 16000 || !saved.OwnerEdited {
		t.Fatal("the owner's edit was not applied", saved.Resolved, saved.OwnerEdited)
	}
	// Phrase times are absolute for a caption that owns no cut, exactly as its
	// own interval is.
	if len(saved.Phrases) != 2 || saved.Phrases[0].StartMS != 12000 {
		t.Fatal("phrase times were read against a cut", saved.Phrases)
	}

	// Removal is absence from the draft.
	draft = creationDraft(edited)
	kept := []clip.CorrectionText{}
	for _, e := range draft.Elements {
		if e.InstanceID != "narration-1" {
			kept = append(kept, e)
		}
	}
	draft.Elements = kept
	removed, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := narrationOf(removed, "narration-1"); ok {
		t.Fatal("a removed caption stayed on the timeline")
	}
	// Its identity is retired, never handed to a different sentence.
	p.EditPlan, err = clip.EncodeEditPlan(removed)
	if err != nil {
		t.Fatal(err)
	}
	draft = creationDraft(removed)
	draft.Elements = append(draft.Elements, ownerCaption("세 번째", 14000, 17000))
	again, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := narrationOf(again, "narration-3"); !ok {
		t.Fatal("a retired narration identity was reused", again.Portable.Elements)
	}
}

func TestNarrationCreationRefusals(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	for name, broken := range map[string]func(clip.CorrectionText) clip.CorrectionText{
		"an unknown creation kind": func(e clip.CorrectionText) clip.CorrectionText {
			e.Creation = &clip.TextCreation{Kind: "invent"}
			return e
		},
		"a caption that is not narration": func(e clip.CorrectionText) clip.CorrectionText {
			e.Narration = false
			return e
		},
		"an identity the plan already holds": func(e clip.CorrectionText) clip.CorrectionText {
			e.InstanceID = "narration-1"
			return e
		},
		"a cut of its own": func(e clip.CorrectionText) clip.CorrectionText {
			e.CutID = "first"
			return e
		},
		"no interval at all": func(e clip.CorrectionText) clip.CorrectionText {
			e.StartMS, e.EndMS = nil, nil
			return e
		},
		"an interval past the end": func(e clip.CorrectionText) clip.CorrectionText {
			return ownerCaption(e.Text, 18000, 25000)
		},
		"an interval another caption already owns": func(e clip.CorrectionText) clip.CorrectionText {
			return ownerCaption(e.Text, 4000, 6000)
		},
	} {
		p, plan := narrationFixture(t)
		draft := creationDraft(plan)
		draft.Elements = append(draft.Elements, broken(ownerCaption("새 자막", 8000, 11000)))
		if _, err := clip.ApplyCorrection(cfg, p, draft); err == nil {
			t.Fatal("a caption was created with " + name)
		}
	}
	// The overlap refusal names the caption the owner submitted, and nothing
	// was retimed to make room for it.
	p, plan := narrationFixture(t)
	draft := creationDraft(plan)
	draft.Elements = append(draft.Elements, ownerCaption("새 자막", 4000, 6000))
	_, err := clip.ApplyCorrection(cfg, p, draft)
	problem := planProblem(t, err)
	if problem.Reason != clip.NoticeCaptionOverlap || problem.ElementID != "narration-2" {
		t.Fatal("the overlap refusal did not name the caption", problem)
	}
}

func TestGroundNarrationChecksFactsWithoutAnItemRule(t *testing.T) {
	facts := []composition.Fact{
		{FieldID: "price", GroupID: "menu", ItemID: "galbi", Value: "1인분 18,000원"},
		{FieldID: "name", GroupID: "menu", ItemID: "galbi", Value: "살치살"},
		{FieldID: "verdict", Value: "고소했어요"},
	}
	for name, c := range map[string]struct {
		text       string
		instructed bool
		reason     string
	}{
		// A caption belongs to no item, so another item's fact grounds it and
		// naming an item is not a cross-item claim any more (CLIP-137).
		"a number another item's fact states":      {"살치살은 1인분 18,000원", false, ""},
		"a number no fact states":                  {"1인분 21,000원이에요", false, "unsupported_number_unit"},
		"a price basis no fact states":             {"1인분 18,000원 인당", false, "unsupported_price_basis"},
		"an experience nobody asked for":           {"국물이 고소하고 맛있었어요", false, "unsupported_experience"},
		"an experience an instruction asked for":   {"국물이 고소하고 맛있었어요", true, ""},
		"an experience the owner wrote down":       {"고소했어요", false, ""},
		"a number pointed at the dish on screen":   {"이 메뉴는 1인분 18,000원", false, ""},
		"a sentence that claims nothing checkable": {"고기를 올렸어요", false, ""},
	} {
		if reason := clip.GroundNarration(c.text, facts, c.instructed); reason != c.reason {
			t.Fatal(name+": got "+reason+", wanted", c.reason)
		}
	}
}

func TestAPlanWithoutSectionsResolvesOnlyItsFixedRegions(t *testing.T) {
	limits := config.ClipCompositionLimits()
	body := `<clip version="1">` +
		`<field id="place" label="상호" required="true">가게 이름</field>` +
		`<group id="menu" label="메뉴" min="1"><field id="name" label="이름" required="true">메뉴 이름</field></group>` +
		`<text id="hook" kind="fixed" role="hook"><row><value field="place"/></row></text>` +
		`<text id="ending" kind="fixed" role="ending"><row>또 갈래요</row></text></clip>`
	doc, problem := composition.ParseTemplate(body, limits)
	if problem != nil {
		t.Fatal(problem)
	}
	inputs := clip.CompositionInputs{
		Values: map[string]string{"place": "성수 곱창"},
		Items:  map[string][]composition.Item{"menu": {{ID: "galbi", Values: map[string]string{"name": "살치살"}}}},
	}
	// A cut of a plan written today belongs to no section, group or item: the
	// footage flow is ordered by the writer, not repeated by a template.
	cuts := []composition.Cut{
		{ID: "first", SourceID: "a", StartMS: 0, EndMS: 10000, PlaybackRatePermille: 1000},
		{ID: "second", SourceID: "a", StartMS: 10000, EndMS: 20000, PlaybackRatePermille: 1000},
	}
	timeline, fallbacks, err := clip.ResolveSelectedComposition(doc, inputs, cuts, limits, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	// No section means no item-bound element, so none of the notices the
	// retired scene-bound copy recorded has anything to describe.
	if len(fallbacks) != 0 {
		t.Fatal("a plan with no sections recorded an item notice", fallbacks)
	}
	if len(timeline.Elements) != 2 || timeline.DurationMS != 20000 {
		t.Fatal("the fixed regions are not all that resolved", timeline)
	}
	for _, e := range timeline.Elements {
		if e.CutID != "" || e.ItemID != "" {
			t.Fatal("a fixed region was bound to a cut", e)
		}
	}
}

package store_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func text(v string) *string { return &v }

// observations reads the project's stored observations.
func observations(t *testing.T, h *generationHarness) (clip.Project, []clip.SourceAnalysis) {
	t.Helper()
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	analyses, err := clip.RetainedObservations(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, analyses
}

// editRefusal is the observation and field a correction was refused over.
func editRefusal(err error) (clip.ObservationEditError, bool) {
	var refused *clip.ObservationEditError
	if !errors.As(err, &refused) {
		return clip.ObservationEditError{}, false
	}
	return *refused, true
}

// CLIP-235: a correction replaces only the descriptive text of the scene it names, records the
// fields it changed, marks the scene and its source edited by hand, and changes no plan,
// storyline, render or confirmation.
func TestOwnerObservationEditsReplaceOnlyDescriptiveText(t *testing.T) {
	h := generationSetup(t)
	h.planner.speech = "어서 오세요"
	if _, err := analyzed(t, h); err != nil {
		t.Fatal(err)
	}
	before, observed := observations(t, h)
	first := h.batch.Sources[0].ID
	id := clip.ObservationID(first, 1)
	saved, err := h.projects.SaveObservationEdits(t.Context(), "alice", h.project.ID, []clip.ObservationEdit{{
		ObservationID: id, Event: text("  갈비를 굽는다 "), Action: text("집게로 뒤집는다"), Speech: text("어서 오세요, 두 분이세요?"),
		Subjects: []string{"갈비", "숯불"}, SubjectsSet: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	after, stored := observations(t, h)
	if !slices.EqualFunc(saved, stored, func(a, b clip.SourceAnalysis) bool {
		return a.Source.ID == b.Source.ID && len(a.Segments) == len(b.Segments)
	}) {
		t.Fatal("the save answered other observations than it stored")
	}
	got, was := stored[0].Segments[1], observed[0].Segments[1]
	if got.Event != "갈비를 굽는다" || got.Action != "집게로 뒤집는다" || got.Speech != "어서 오세요, 두 분이세요?" || !slices.Equal(got.Subjects, []string{"갈비", "숯불"}) {
		t.Fatalf("the correction was not saved as written: %+v", got)
	}
	if got.StartMS != was.StartMS || got.EndMS != was.EndMS || got.Scene != was.Scene || got.Motion != was.Motion || got.Quality != was.Quality || got.Certainty != was.Certainty || got.Usability != was.Usability || got.ReadableText != was.ReadableText || got.Subject != was.Subject || !slices.Equal(got.CaptionSafe, was.CaptionSafe) || got.Focal != was.Focal {
		t.Fatalf("a measured field changed: %+v was %+v", got, was)
	}
	if !got.OwnerEdited || !slices.Equal(got.OwnerFields, []string{"action", "event", "speech", "subjects"}) || !stored[0].OwnerEdited {
		t.Fatalf("the correction is not marked by hand: %v %v %v", got.OwnerEdited, got.OwnerFields, stored[0].OwnerEdited)
	}
	if stored[0].Segments[0].OwnerEdited || stored[1].OwnerEdited || stored[1].Segments[0].OwnerEdited {
		t.Fatal("a scene the owner did not correct is marked by hand")
	}
	if after.EditPlan != before.EditPlan || after.EditPlanRevision != before.EditPlanRevision || after.Storyline != nil || after.Result != nil || after.Confirmed != nil {
		t.Fatal("a correction changed more than the observations")
	}
	// Correcting one field again keeps the record of the others.
	if _, err := h.projects.SaveObservationEdits(t.Context(), "alice", h.project.ID, []clip.ObservationEdit{{ObservationID: id, Event: text("갈비를 다시 굽는다")}}); err != nil {
		t.Fatal(err)
	}
	if _, stored = observations(t, h); stored[0].Segments[1].Event != "갈비를 다시 굽는다" || len(stored[0].Segments[1].OwnerFields) != 4 {
		t.Fatalf("a second correction lost the first: %+v", stored[0].Segments[1])
	}
}

// CLIP-235, CLIP-129: an unknown id, emptied speech, speech added to a silent scene and text past
// the observation's bounds are refused with the field they concern, and nothing is stored.
func TestOwnerObservationEditsRefuseWhatTheyCannotChange(t *testing.T) {
	h := generationSetup(t)
	h.planner.speech = "어서 오세요"
	if _, err := analyzed(t, h); err != nil {
		t.Fatal(err)
	}
	before, _ := observations(t, h)
	first := h.batch.Sources[0].ID
	long := string(make([]rune, h.cfg.Analysis.MaxTextRunes+1))
	for name, c := range map[string]struct {
		edit         clip.ObservationEdit
		field, check string
	}{
		"unknown id":      {clip.ObservationEdit{ObservationID: first + "/9", Event: text("x")}, "observation_id", "observation_unknown"},
		"unknown source":  {clip.ObservationEdit{ObservationID: "nowhere/0", Event: text("x")}, "observation_id", "observation_unknown"},
		"cleared speech":  {clip.ObservationEdit{ObservationID: first + "/0", Speech: text("  ")}, "speech", "speech_audibility"},
		"text past bound": {clip.ObservationEdit{ObservationID: first + "/0", Event: &long}, "", "observe_text_length"},
	} {
		_, err := h.projects.SaveObservationEdits(t.Context(), "alice", h.project.ID, []clip.ObservationEdit{c.edit})
		refused, ok := editRefusal(err)
		if !ok || refused.Field != c.field || refused.Check != c.check || refused.ObservationID != c.edit.ObservationID {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if after, _ := observations(t, h); after.Analysis != before.Analysis {
		t.Fatal("a refused correction stored something")
	}
	// A scene that observed no speech is not given any.
	silent := generationSetup(t)
	if _, err := analyzed(t, silent); err != nil {
		t.Fatal(err)
	}
	_, err := silent.projects.SaveObservationEdits(t.Context(), "alice", silent.project.ID, []clip.ObservationEdit{{ObservationID: clip.ObservationID(silent.batch.Sources[0].ID, 0), Speech: text("안녕하세요")}})
	if refused, ok := editRefusal(err); !ok || refused.Field != "speech" {
		t.Fatal("speech was added to a silent scene", err)
	}
}

// CLIP-235, CLIP-160: a project with a running job and a finalized one refuse corrections.
func TestOwnerObservationEditsWaitForARunningJobAndEndAtFinalization(t *testing.T) {
	h := generationSetup(t)
	if _, err := analyzed(t, h); err != nil {
		t.Fatal(err)
	}
	edit := []clip.ObservationEdit{{ObservationID: clip.ObservationID(h.batch.Sources[0].ID, 0), Event: text("고친 장면")}}
	q, err := h.service.QuoteAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", []string{h.batch.Sources[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.StartAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", []string{h.batch.Sources[0].ID}, approve(q)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.projects.SaveObservationEdits(t.Context(), "alice", h.project.ID, edit); !errors.Is(err, clip.ErrBusy) {
		t.Fatal("a correction was saved under a running job", err)
	}
	done := generationSetup(t)
	if _, err := analyzed(t, done); err != nil {
		t.Fatal(err)
	}
	at := "2026-10-06T00:00:00Z"
	if _, err := done.db.Writer.Exec(`UPDATE clip_projects SET result_id='result',result_key='result-key',result_content_type='video/mp4',result_bytes=1,result_duration_ms=15000,result_created_at=?,edit_plan_revision=1,rendered_plan_revision=1,finalized_at=?,finalized_plan_revision=1,finalized_result_key='result-key',source_access_revoked_at=? WHERE id=?`, at, at, at, done.project.ID); err != nil {
		t.Fatal(err)
	}
	edit[0].ObservationID = clip.ObservationID(done.batch.Sources[0].ID, 0)
	if _, err := done.projects.SaveObservationEdits(t.Context(), "alice", done.project.ID, edit); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal("a finalized project took a correction", err)
	}
	if _, err := done.projects.SaveObservationEdits(t.Context(), "bob", done.project.ID, edit); err == nil {
		t.Fatal("another account corrected the project")
	}
}

// observedUnder makes the kept chunks look observed by another model, as if the owner had
// chosen a different analysis model since (CLIP-93).
func observedUnder(t *testing.T, h *generationHarness, observe llm.ModelRef) {
	t.Helper()
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state == nil {
		t.Fatal("no recovery", err)
	}
	state.Observe = observe
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec(`UPDATE clip_recovery_states SET state_json=? WHERE project_id=?`, string(raw), h.project.ID); err != nil {
		t.Fatal(err)
	}
}

// CLIP-235, CLIP-93, CLIP-31: a source corrected by hand counts as analyzed after the observe
// model changes — no chunk of it is quoted or observed — and the storyline and flow calls read
// its corrected text, while the uncorrected source is observed again.
func TestACorrectedSourceSurvivesAModelChangeAndReachesTheWriters(t *testing.T) {
	h := generationSetup(t)
	if _, err := analyzed(t, h); err != nil {
		t.Fatal(err)
	}
	first, second := h.batch.Sources[0].ID, h.batch.Sources[1].ID
	if _, err := h.projects.SaveObservationEdits(t.Context(), "alice", h.project.ID, []clip.ObservationEdit{{ObservationID: clip.ObservationID(first, 0), Event: text("주인이 고친 장면")}}); err != nil {
		t.Fatal(err)
	}
	observedUnder(t, h, llm.ModelRef{ProviderID: "p", ModelID: "older"})
	q, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if q.Pricing.ObservationCalls != 1 || q.Pricing.ReusedChunks != 2 {
		t.Fatalf("the corrected source was quoted for analysis again: %+v", q.Pricing)
	}
	id, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	observed := h.planner.observe
	// Only the uncorrected source is read from its original: the fake probe answers its length.
	h.media.durations = []int{h.batch.Sources[1].DurationMS}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.observe-observed != 1 {
		t.Fatal("the storyline observed", h.planner.observe-observed, "chunks instead of the uncorrected source's one")
	}
	corrected := func(analyses []clip.SourceAnalysis) bool {
		at := slices.IndexFunc(analyses, func(a clip.SourceAnalysis) bool { return a.Source.ID == first })
		return at >= 0 && analyses[at].OwnerEdited && analyses[at].Segments[0].Event == "주인이 고친 장면" && analyses[at].Segments[1].Event == "scene"
	}
	if len(h.planner.storylines) != 1 || !corrected(h.planner.storylines[0].Analyses) {
		t.Fatal("the storyline call did not read the corrected text")
	}
	if _, stored := observations(t, h); !corrected(stored) || stored[1].Source.ID != second || stored[1].OwnerEdited {
		t.Fatal("the storyline's save lost the correction")
	}
	// Building along the storyline: the flow reads the corrected text too, and still observes nothing.
	build, err := h.service.QuoteFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if build.Pricing.ObservationCalls != 0 {
		t.Fatalf("building observed again: %+v", build.Pricing)
	}
	if id, err = h.service.StartFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(build)); err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if !corrected(h.planner.input.Analyses) {
		t.Fatal("the flow call did not read the corrected text")
	}
}

// CLIP-235, CLIP-234: only 다시 관찰하기 replaces a corrected source, observing every chunk of it
// again and dropping the corrections.
func TestReanalysisReplacesACorrectedSource(t *testing.T) {
	h := generationSetup(t)
	if _, err := analyzed(t, h); err != nil {
		t.Fatal(err)
	}
	first := h.batch.Sources[0].ID
	if _, err := h.projects.SaveObservationEdits(t.Context(), "alice", h.project.ID, []clip.ObservationEdit{{ObservationID: clip.ObservationID(first, 0), Event: text("주인이 고친 장면")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := analyzed(t, h, first); err != nil {
		t.Fatal(err)
	}
	_, stored := observations(t, h)
	if stored[0].OwnerEdited || stored[0].Segments[0].OwnerEdited || stored[0].Segments[0].Event != "scene" {
		t.Fatalf("reanalysis kept the correction: %+v", stored[0])
	}
}

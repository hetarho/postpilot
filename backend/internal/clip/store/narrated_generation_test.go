package store_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

type narratedFixture struct {
	quotes  int
	writer  *sql.DB
	voice   clip.SpeechVoice
	calls   int
	failAt  int
	removed bool
	audio   llm.EncodedAudio
	stage   *plannerFake
}

func (f *narratedFixture) ResolveClipVoice(_ context.Context, owner, id string) (clip.SpokenVoiceBinding, error) {
	if f.removed || owner != "alice" || id != f.voice.Binding.ID {
		return clip.SpokenVoiceBinding{}, clip.ErrNotFound
	}
	return f.voice.Binding, nil
}
func (f *narratedFixture) ResolveSpeechVoice(ctx context.Context, owner string, _ plan.Plan, id string) (clip.SpeechVoice, error) {
	_, e := f.ResolveClipVoice(ctx, owner, id)
	return f.voice, e
}
func (*narratedFixture) PlanOf(context.Context, string) (plan.Plan, error) { return plan.Master, nil }
func (*narratedFixture) Budget(_ context.Context, _ string, _ plan.Plan, id string, rev int64, _ string, scope string, in llm.SpeechInput, n int) (usage.UnitBudget, error) {
	return usage.UnitBudget{PolicyID: id, Revision: rev, ScopeDigest: scope, Ref: in.Ref, Operation: in.Operation, InputDigest: in.Digest, Count: n, InputCharacters: in.InputCharacters, ParametersDigest: in.ParametersDigest, Tariffs: []usage.UnitTariff{{Unit: llm.SpeechUnitCharacters, USDPerUnit: "0.0001", Multiplier: "1", MaximumUnits: "1000", UnitsPerInputCharacter: "1"}}, Source: "https://example.com/prices", BoundsSource: "https://example.com/bounds", CheckedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Complete: true}, nil
}
func (f *narratedFixture) QuoteUnits(_ context.Context, owner string, _ plan.Plan, kind string, b []usage.UnitBudget) (usage.UnitQuote, error) {
	f.quotes++
	return usage.UnitQuote{ID: "initial-quote-" + strconv.Itoa(f.quotes), UserID: owner, Kind: kind, Calls: b, Rate: quoteRate, ExpiresAt: time.Now().Add(5 * time.Minute)}, nil
}
func (*narratedFixture) ReservationForUnits(context.Context, string, plan.Plan, string, usage.UnitApproval, []usage.UnitBudget) (*usage.Reservation, error) {
	return nil, errors.New("no nested speech approval")
}
func (*narratedFixture) AdmissionForJob(context.Context, string) (usage.Admission, bool, error) {
	return usage.Admission{AdmittedPlan: plan.Master}, true, nil
}
func (f *narratedFixture) SynthesizeSpeech(_ context.Context, in llm.SpeechRequest) (llm.SpeechResponse, error) {
	f.calls++
	f.stage.stages = append(f.stage.stages, "speech")
	if f.calls == f.failAt {
		return llm.SpeechResponse{}, errors.New("supplier fixture failed")
	}
	return llm.SpeechResponse{Audio: f.audio}, nil
}
func (*narratedFixture) PutClipSpeechAudio(context.Context, string, []byte) error { return nil }
func (*narratedFixture) DeleteClipSpeechAudio(context.Context, string) error      { return nil }
func (f *narratedFixture) WriteSpeech(ctx context.Context, fn func(clipapp.SpeechTxPorts) error) error {
	tx, e := f.writer.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = fn(clipapp.SpeechTxPorts{Store: clipstore.NewTx(tx), Jobs: jobstore.NewTx(tx, jobKindsForTest())}); e != nil {
		return e
	}
	return tx.Commit()
}

type narratedPlanner struct {
	*plannerFake
	scripts int
}

func (p *narratedPlanner) SpokenScript(ctx context.Context, ref llm.ModelRef, in clip.PlanningInput) (clip.SpokenDraft, llm.Usage, error) {
	p.scripts++
	p.stages = append(p.stages, "script")
	policy, e := clipapp.ConsumePolicy(ctx, "alice", p.id, ref.String(), 32768, "write")
	if e != nil || policy != in.Policy {
		return clip.SpokenDraft{}, llm.Usage{}, clip.ErrCreditAllowance
	}
	s := &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "장면을 자연스럽게 소개해요.", ObservationIDs: []string{clip.ObservationID(in.Analyses[0].Source.ID, 0)}}}}
	if in.FollowStoryline != nil {
		s = in.FollowStoryline
	}
	d, e := clip.NewSpokenDraft([]string{"첫 번째 문장", "두 번째 문장"}, s)
	return d, llm.Usage{}, e
}

func narratedSetup(t *testing.T) (*generationHarness, *narratedFixture, *narratedPlanner) {
	h := generationSetup(t)
	bytes, e := os.ReadFile("../../llm/testdata/speech-tone.mp3")
	if e != nil {
		t.Fatal(e)
	}
	audio, e := llm.InspectSpeechAudio(t.Context(), bytes)
	if e != nil {
		t.Fatal(e)
	}
	f := &narratedFixture{writer: h.db.Writer, audio: audio, stage: h.planner, voice: clip.SpeechVoice{Binding: clip.SpokenVoiceBinding{ID: "confirmed", Digest: strings.Repeat("a", 64)}, ProfileID: "profile", ProfileRevision: 1, Model: llm.ModelRef{ProviderID: "p", ModelID: "speech"}, Handle: "private", Settings: llm.SpeechSettings{Speed: 1}}}
	speech := clipapp.NewSpeechService(clipapp.SpeechDeps{Store: h.store, Voices: f, Plans: f, Prices: f, Ledger: f, Models: f, Objects: f, Queue: h.queue, Transactions: f})
	p := &narratedPlanner{plannerFake: h.planner}
	deps := generationDeps(generationFinisher{h.store}, &quotePricing{}, nil)
	deps.Voices = f
	deps.Speech = speech
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, p, h.renderer, h.clipJobs(), h.cfg, deps)
	if _, e = h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Dubbing: &clip.DubbingOptions{Enabled: true, VoiceID: "confirmed"}}); e != nil {
		t.Fatal(e)
	}
	return h, f, p
}
func narratedRun(t *testing.T, h *generationHarness) error {
	j, e := h.jobs.PickNextQueued(t.Context(), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	ctx := usage.WithWork(t.Context(), usage.Work{UserID: j.UserID, Kind: j.Kind, JobID: j.ID})
	e = h.service.Run(ctx, j.UserID, j.ID, h.project.ID, j.Payload, func(stage string, done, total int) {
		if err := h.jobs.UpdateProgress(ctx, j.ID, stage, done, total, time.Now()); err != nil {
			t.Fatal(err)
		}
	})
	status := "done"
	var failure *job.Failure
	if e != nil {
		status = "failed"
		failure = &job.Failure{Reason: "CLIP_PROCESSING_FAILED"}
	}
	if err := h.jobs.Finish(t.Context(), j.ID, status, failure, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := h.sources.ReleaseAttempt(t.Context(), j.UserID, j.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	return e
}
func TestNarratedGenerationRunsScriptSpeechFlowAndRetainsMeasuredCheckpoints(t *testing.T) {
	h, f, p := narratedSetup(t)
	q := quote(t, h)
	if q.Pricing.Dubbing == nil || q.Pricing.Dubbing.Units[0].TotalInputCharacters != 2000 || q.Pricing.Dubbing.Units[0].Count != 32 {
		t.Fatal("not bounded", q)
	}
	h.start(t)
	if e := narratedRun(t, h); e != nil {
		t.Fatal(e)
	}
	if strings.Join(p.stages, ",") != "script,speech,speech,flow" || p.scripts != 1 || p.narrations != 0 || f.calls != 2 {
		t.Fatal("wrong workflow", p.stages)
	}
	current, e := h.store.GetProject(t.Context(), "alice", h.project.ID)
	if e != nil {
		t.Fatal(e)
	}
	edit, e := clip.DecodeEditPlan(current.EditPlan)
	if e != nil || clip.NarrationReadiness(edit) != nil || len(h.objects.results) != 0 {
		t.Fatal("not a measured editable draft", edit, e)
	}
	if edit.Narration.Segments[1].StartMS != edit.Narration.Segments[0].Speech.DurationMS() {
		t.Fatal("audio time changed")
	}
	state, e := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if e != nil || state.Spoken == nil || !state.PlanReady {
		t.Fatal(state, e)
	}
}
func TestNarratedGenerationFailureReusesScriptAndPartialAudioUnderNewApproval(t *testing.T) {
	h, f, p := narratedSetup(t)
	f.failAt = 2
	h.start(t)
	if e := narratedRun(t, h); e == nil {
		t.Fatal("injected speech failure ignored")
	}
	state, e := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if e != nil || state.Spoken == nil || state.Spoken.Narration.Segments[0].Speech == nil || state.Spoken.Narration.Segments[1].Speech != nil {
		t.Fatal("partial checkpoint lost", state, e)
	}
	q := quote(t, h)
	if q.Pricing.Dubbing.QuoteID == state.Pricing.Dubbing.QuoteID {
		t.Fatal("consumed partial speech quote reused")
	}
	if !q.Pricing.SkipFlow || q.Pricing.PlanCalls() != 1+q.Pricing.Narration.ResponseRetries || q.Pricing.ObservationCalls != 0 {
		t.Fatal("script/analysis repriced", q.Pricing)
	}
	f.failAt = 0
	h.start(t)
	if e = narratedRun(t, h); e != nil {
		t.Fatal(e)
	}
	if p.scripts != 1 || f.calls != 3 || p.flows != 1 {
		t.Fatal("completed work replayed", p.scripts, f.calls, p.flows)
	}
}

func TestNarratedStorylineFirstStopsThenBuildsWithoutPaidCaptionWriting(t *testing.T) {
	h, f, p := narratedSetup(t)
	q, e := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if e != nil {
		t.Fatal(e)
	}
	if q.Pricing.Dubbing != nil || q.Pricing.PlanCalls() != 1+q.Pricing.Plan.ResponseRetries {
		t.Fatal("storyline quoted speech", q)
	}
	id, e := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if e != nil {
		t.Fatal(e)
	}
	h.planner.id = id
	if e = h.run(t); e != nil {
		t.Fatal(e)
	}
	if p.scripts != 0 || f.calls != 0 || p.flows != 0 {
		t.Fatal("storyline ran beyond its one writing call")
	}
	q, e = h.service.QuoteFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if e != nil {
		t.Fatal(e)
	}
	id, e = h.service.StartFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if e != nil {
		t.Fatal(e)
	}
	h.planner.id = id
	if e = narratedRun(t, h); e != nil {
		t.Fatal(e)
	}
	if p.scripts != 1 || f.calls != 2 || p.flows != 1 || p.narrations != 0 {
		t.Fatal("storyline build wrong workflow")
	}
}

func TestCompletedNarratedDraftNeedsNoSupplierVoiceAfterItsRemoval(t *testing.T) {
	h, f, _ := narratedSetup(t)
	h.start(t)
	if e := narratedRun(t, h); e != nil {
		t.Fatal(e)
	}
	f.removed = true
	q := quote(t, h)
	if !q.Pricing.RenderOnly() || len(q.Pricing.Dubbing.Units) != 0 || q.Pricing.MaxCredits != 0 {
		t.Fatal("retained audio asked for speech", q)
	}
	h.start(t)
	if e := narratedRun(t, h); e != nil {
		t.Fatal(e)
	}
	if f.calls != 2 {
		t.Fatal("ready audio resynthesized")
	}
}

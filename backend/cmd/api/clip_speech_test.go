package main

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice/spoken"
	"os"
	"reflect"
	"testing"
	"time"
)

type fixtureClipSpeechVoices struct{ f *spokenGenerationFixture }

func (a fixtureClipSpeechVoices) ResolveSpeechVoice(ctx context.Context, owner string, tier plan.Plan, id string) (clipapp.SpeechVoice, error) {
	v, e := a.f.library.GetVoice(ctx, owner, id)
	if e != nil {
		return clipapp.SpeechVoice{}, e
	}
	if v.RemovedAt != nil {
		return clipapp.SpeechVoice{}, spoken.ErrNotFound
	}
	p, e := a.f.profiles.ResolveSpokenProfile(ctx, owner, tier, v.Profile.ID, v.Profile.Revision, "")
	if e != nil || p != v.Profile {
		return clipapp.SpeechVoice{}, spoken.ErrConflict
	}
	b, e := (clipSpokenVoices{a.f.library}).ResolveClipVoice(ctx, owner, id)
	return clipapp.SpeechVoice{Binding: b, ProfileID: p.ID, ProfileRevision: p.Revision, Model: p.Synthesis, Handle: v.Handle, Settings: p.Settings}, e
}

type fixtureClipSynth struct {
	*spokenTestProvider
	failAt  int
	corrupt bool
}

func (p *fixtureClipSynth) SynthesizeSpeech(ctx context.Context, r llm.SpeechRequest) (llm.SpeechResponse, error) {
	out, e := p.spokenTestProvider.SynthesizeSpeech(ctx, r)
	if p.corrupt {
		out.Audio.Bytes = []byte("corrupt despite good metadata")
	}
	if p.failAt == p.speech {
		return out, errors.New("supplier failed after reporting usage")
	}
	return out, e
}

type clipSpeechFixture struct {
	f       *spokenGenerationFixture
	s       *clipapp.SpeechService
	store   *clipstore.Store
	synth   *fixtureClipSynth
	project string
	voice   spoken.Voice
}

func newClipSpeechFixture(t *testing.T) *clipSpeechFixture {
	f := spokenGenerationFresh(t)
	v := f.confirmed("clip-voice")
	b, e := (clipSpokenVoices{f.library}).ResolveClipVoice(t.Context(), "alice", v.ID)
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile("../../internal/llm/testdata/speech-tone.mp3")
	if e != nil {
		t.Fatal(e)
	}
	audio, e := llm.InspectSpeechAudio(t.Context(), data)
	if e != nil {
		t.Fatal(e)
	}
	f.provider.fixtureAudio = &audio
	edit := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut-1", SourceID: "source", Fingerprint: "source-proof", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}}, Narration: &clip.NarrationPlan{Enabled: true, VoiceID: v.ID, BindingDigest: b.Digest, VolumePermille: 1000, Segments: []clip.SpokenSegment{{ID: "spoken-1", Text: "첫 번째 문장", TextRevision: 1, InputHash: clip.SpokenInputHash("첫 번째 문장"), EndMS: 5000}, {ID: "spoken-2", Text: "두 번째 문장", TextRevision: 1, InputHash: clip.SpokenInputHash("두 번째 문장"), StartMS: 5000, EndMS: 10000}}}}
	raw, e := clip.EncodeEditPlan(edit)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, e = f.handle.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,language,target_duration_ms,edit_plan_json,edit_plan_revision,created_at,updated_at,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at) VALUES('speech-project','alice','Speech','vertical','ko',15000,?,1,?,?,'prior.mp4','video/mp4',8,15000,?)`, raw, now, now, now)
	if e != nil {
		t.Fatal(e)
	}
	st := clipstore.New(f.handle.Writer, f.handle.Reader)
	synth := &fixtureClipSynth{spokenTestProvider: f.provider}
	s := clipapp.NewSpeechService(clipapp.SpeechDeps{Store: st, Voices: fixtureClipSpeechVoices{f}, Plans: f.admission.plans, Prices: f.profiles, Ledger: f.ledger, Models: usage.SpeechMeter{Provider: synth, Ledger: f.ledger}, Objects: fixtureClipObjects{f.objects}, Queue: f.queue, Transactions: clipSpeechTransactions{f.handle.Writer}})
	return &clipSpeechFixture{f, s, st, synth, "speech-project", v}
}
func (h *clipSpeechFixture) plan(t *testing.T) (clip.Project, clip.EditPlan) {
	t.Helper()
	p, e := h.store.GetProject(t.Context(), "alice", h.project)
	if e != nil {
		t.Fatal(e)
	}
	edit, e := clip.DecodeEditPlan(p.EditPlan)
	if e != nil {
		t.Fatal(e)
	}
	return p, edit
}
func (h *clipSpeechFixture) start(t *testing.T, key string) string {
	t.Helper()
	p, _ := h.plan(t)
	q, e := h.s.Quote(t.Context(), "alice", h.project, p.EditPlanRevision)
	if e != nil {
		t.Fatal(e)
	}
	id, e := h.s.Start(t.Context(), "alice", h.project, p.EditPlanRevision, key, usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1})
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func (h *clipSpeechFixture) run(t *testing.T, id string) error {
	t.Helper()
	j, e := h.f.jobs.PickNextQueued(t.Context(), time.Now())
	if e != nil || j.ID != id {
		t.Fatalf("pick %+v %v", j, e)
	}
	e = metered(h.s.Run)(t.Context(), j, func(string, int, int) {})
	current, x := h.f.jobs.GetByID(t.Context(), id)
	if x != nil {
		t.Fatal(x)
	}
	status := job.StatusDone
	var failure *job.Failure
	if current.CancelRequestedAt != nil {
		status = job.StatusCancelled
	} else if e != nil {
		status = job.StatusFailed
		n := jobReporting{}.Failure(e)
		failure = &n
	}
	if x = h.f.jobs.Finish(t.Context(), id, status, failure, time.Now()); x != nil {
		t.Fatal(x)
	}
	current, _ = h.f.jobs.GetByID(t.Context(), id)
	h.f.admission.Settle(t.Context(), id, status)
	h.f.admission.Settle(t.Context(), id, status)
	if x = h.s.OnTerminal(t.Context(), current); x != nil {
		t.Fatal(x)
	}
	return e
}
func TestClipSpeechSelectiveReuseAndImmutableTiming(t *testing.T) {
	h := newClipSpeechFixture(t)
	id := h.start(t, "first")
	_, before := h.plan(t)
	if e := h.run(t, id); e != nil {
		t.Fatal(e)
	}
	p, edit := h.plan(t)
	if h.f.provider.speech != 2 || clip.NarrationReadiness(edit) != nil || p.Result.Key != "prior.mp4" || !reflect.DeepEqual(before.Cuts, edit.Cuts) {
		t.Fatal("speech or prior scene/result changed")
	}
	if len(edit.Narration.Segments[0].Speech.Timing) != 0 {
		t.Fatal("missing alignment became precise")
	}
	q, e := h.s.Quote(t.Context(), "alice", h.project, p.EditPlanRevision)
	if e != nil || q.ID != "" || len(q.SegmentIDs) != 0 {
		t.Fatal("unchanged quote", q, e)
	}
	oldAsset := edit.Narration.Segments[1].Speech.AssetID
	in := clip.CorrectionFromPlan(edit)
	in.Narration.Segments[0].Text = "변경된 첫 문장"
	next := edit
	if e = clip.CorrectNarration(edit, in, &next); e != nil {
		t.Fatal(e)
	}
	raw, _ := clip.EncodeEditPlan(next)
	p, e = h.store.SaveCorrection(t.Context(), "alice", h.project, p.EditPlanRevision, raw, nil)
	if e != nil {
		t.Fatal(e)
	}
	q, e = h.s.Quote(t.Context(), "alice", h.project, p.EditPlanRevision)
	if e != nil || !reflect.DeepEqual(q.SegmentIDs, []string{"spoken-1"}) {
		t.Fatal(q, e)
	}
	id = h.start(t, "changed")
	if e = h.run(t, id); e != nil {
		t.Fatal(e)
	}
	_, edit = h.plan(t)
	if h.f.provider.speech != 3 || edit.Narration.Segments[1].Speech.AssetID != oldAsset {
		t.Fatal("unchanged speech synthesized")
	}
}
func TestClipSpeechPartialFailureKeepsCompleted(t *testing.T) {
	h := newClipSpeechFixture(t)
	h.synth.failAt = 2
	id := h.start(t, "partial")
	if e := h.run(t, id); e == nil {
		t.Fatal("failure accepted")
	}
	p, edit := h.plan(t)
	if edit.Narration.Segments[0].Speech == nil || edit.Narration.Segments[1].Speech != nil || p.Result.Key != "prior.mp4" {
		t.Fatal("partial speech/result lost")
	}
	q, e := h.s.Quote(t.Context(), "alice", h.project, p.EditPlanRevision)
	if e != nil || !reflect.DeepEqual(q.SegmentIDs, []string{"spoken-2"}) {
		t.Fatal(q, e)
	}
}
func TestClipSpeechDecodeStorageAndFit(t *testing.T) {
	for _, kind := range []string{"decode", "storage", "fit"} {
		t.Run(kind, func(t *testing.T) {
			h := newClipSpeechFixture(t)
			if kind == "decode" {
				h.synth.corrupt = true
			}
			if kind == "storage" {
				h.f.objects.fail = true
			}
			if kind == "fit" {
				p, edit := h.plan(t)
				edit.Narration.Segments[0].EndMS = 1
				raw, _ := clip.EncodeEditPlan(edit)
				if _, e := h.store.SaveCorrection(t.Context(), "alice", h.project, p.EditPlanRevision, raw, nil); e != nil {
					t.Fatal(e)
				}
			}
			id := h.start(t, kind)
			e := h.run(t, id)
			p, edit := h.plan(t)
			if kind == "fit" {
				if e != nil || edit.Narration.Segments[0].Speech == nil || edit.Narration.Segments[0].EndMS != 1 || clip.NarrationReadiness(edit) == nil {
					t.Fatal("fit changed or audio discarded", e)
				}
			} else if e == nil || edit.Narration.Segments[0].Speech != nil {
				t.Fatal("invalid media published", e)
			}
			if p.Result.Key != "prior.mp4" {
				t.Fatal("prior result removed")
			}
		})
	}
}
func TestClipSpeechPublicationStopAndRevisionRaces(t *testing.T) {
	for _, kind := range []string{"cancel", "revision"} {
		t.Run(kind, func(t *testing.T) {
			h := newClipSpeechFixture(t)
			id := h.start(t, kind)
			h.f.objects.afterPut = func() {
				if kind == "cancel" {
					if _, e := h.f.queue.Cancel(t.Context(), "alice", job.Subject{Dimension: clip.JobSubject, ID: h.project}, id); e != nil {
						t.Fatal(e)
					}
				} else {
					if _, e := h.f.handle.Writer.Exec("UPDATE clip_projects SET edit_plan_revision=edit_plan_revision+1 WHERE id=?", h.project); e != nil {
						t.Fatal(e)
					}
				}
			}
			if e := h.run(t, id); e == nil {
				t.Fatal("stopped/obsolete publication accepted")
			}
			_, edit := h.plan(t)
			if edit.Narration.Segments[0].Speech != nil || h.f.provider.speech != 1 {
				t.Fatal("late speech replaced current or extra call")
			}
		})
	}
}
func TestClipSpeechIdempotentForeignAndProfileDrift(t *testing.T) {
	h := newClipSpeechFixture(t)
	p, _ := h.plan(t)
	q, e := h.s.Quote(t.Context(), "alice", h.project, p.EditPlanRevision)
	if e != nil {
		t.Fatal(e)
	}
	a := usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1}
	id, e := h.s.Start(t.Context(), "alice", h.project, p.EditPlanRevision, "same", a)
	if e != nil {
		t.Fatal(e)
	}
	again, e := h.s.Start(t.Context(), "alice", h.project, p.EditPlanRevision, "same", a)
	if e != nil || again != id {
		t.Fatal("duplicate start", again, e)
	}
	if _, e = h.s.Quote(t.Context(), "bob", h.project, p.EditPlanRevision); e == nil {
		t.Fatal("foreign project")
	}
	h.f.profiles.unavailable = true
	if e = h.run(t, id); e == nil || h.f.provider.speech != 0 {
		t.Fatal("drift made paid call", e)
	}
}

func TestClipSpeechRestartNeverRepeatsClaimedCalls(t *testing.T) {
	h := newClipSpeechFixture(t)
	id := h.start(t, "interrupted")
	j, e := h.f.jobs.PickNextQueued(t.Context(), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	r, e := h.store.GetSpeechRun(t.Context(), "alice", string(j.Payload))
	if e != nil {
		t.Fatal(e)
	}
	if e = h.store.ClaimSpeechCall(t.Context(), r, r.Calls[0]); e != nil {
		t.Fatal(e)
	}
	if e = h.s.Recover(t.Context()); e != nil {
		t.Fatal(e)
	}
	current, e := h.f.jobs.GetByID(t.Context(), id)
	if e != nil || current.Status != job.StatusFailed {
		t.Fatal(current, e)
	}
	h.f.admission.Settle(t.Context(), id, current.Status)
	h.f.admission.Settle(t.Context(), id, current.Status)
	if e = h.s.Recover(t.Context()); e != nil {
		t.Fatal(e)
	}
	if h.f.provider.speech != 0 {
		t.Fatal("restart synthesized")
	}
	var state string
	if e = h.f.handle.Reader.QueryRow("SELECT state FROM clip_speech_segments WHERE job_id=? AND segment_id='spoken-1'", id).Scan(&state); e != nil || state != "unresolved" {
		t.Fatal(state, e)
	}
}
func TestClipSpeechVoiceReplacementAndRemovedVoice(t *testing.T) {
	h := newClipSpeechFixture(t)
	if e := h.run(t, h.start(t, "original")); e != nil {
		t.Fatal(e)
	}
	p, edit := h.plan(t)
	v := h.f.confirmed("replacement")
	b, e := (clipSpokenVoices{h.f.library}).ResolveClipVoice(t.Context(), "alice", v.ID)
	if e != nil {
		t.Fatal(e)
	}
	edit.Narration.VoiceID, edit.Narration.BindingDigest = v.ID, b.Digest
	raw, _ := clip.EncodeEditPlan(edit)
	p, e = h.store.SaveCorrection(t.Context(), "alice", h.project, p.EditPlanRevision, raw, nil)
	if e != nil {
		t.Fatal(e)
	}
	q, e := h.s.Quote(t.Context(), "alice", h.project, p.EditPlanRevision)
	if e != nil || len(q.SegmentIDs) != 2 {
		t.Fatal("voice change did not stale all", q, e)
	}
	if _, e = h.f.library.RemoveVoice(t.Context(), "alice", v.ID, v.Revision, "remove"); e != nil {
		t.Fatal(e)
	}
	if _, e = h.s.Quote(t.Context(), "alice", h.project, p.EditPlanRevision); e == nil {
		t.Fatal("removed voice synthesized")
	}
	_, edit = h.plan(t)
	if edit.Narration.Segments[0].Speech == nil || edit.Narration.Segments[0].Speech.VoiceID != h.voice.ID {
		t.Fatal("prior audio removed")
	}
}
func TestClipSpeechRepeatedTextHasBoundedReusableBudget(t *testing.T) {
	h := newClipSpeechFixture(t)
	p, edit := h.plan(t)
	first := edit.Narration.Segments[0]
	for i := range edit.Narration.Segments {
		edit.Narration.Segments[i].Text = first.Text
		edit.Narration.Segments[i].InputHash = first.InputHash
	}
	raw, _ := clip.EncodeEditPlan(edit)
	if _, e := h.store.SaveCorrection(t.Context(), "alice", h.project, p.EditPlanRevision, raw, nil); e != nil {
		t.Fatal(e)
	}
	if e := h.run(t, h.start(t, "same-words")); e != nil {
		t.Fatal(e)
	}
	if h.f.provider.speech != 2 {
		t.Fatal("duplicate budget malformed")
	}
}

func TestClipSpeechRPCAuthenticationAndReadiness(t *testing.T) {
	h := newClipSpeechFixture(t)
	rpc := cliprpc.NewHandler(nil).WithSpeech(h.s)
	if _, e := rpc.QuoteClipSpeech(t.Context(), connect.NewRequest(&v1.QuoteClipSpeechRequest{ProjectId: h.project, ExpectedRevision: 1})); connect.CodeOf(e) != connect.CodeUnauthenticated {
		t.Fatal("missing authentication", e)
	}
	if _, e := rpc.GetClipSpeechReadiness(auth.WithUser(t.Context(), "bob"), connect.NewRequest(&v1.GetClipSpeechReadinessRequest{ProjectId: h.project})); connect.CodeOf(e) != connect.CodeNotFound {
		t.Fatal("foreign readiness", e)
	}
	ctx := auth.WithUser(t.Context(), "alice")
	q, e := rpc.QuoteClipSpeech(ctx, connect.NewRequest(&v1.QuoteClipSpeechRequest{ProjectId: h.project, ExpectedRevision: 1}))
	if e != nil || len(q.Msg.SegmentIds) != 2 {
		t.Fatal(q, e)
	}
	n := q.Msg.MaximumCredits
	started, e := rpc.StartClipSpeech(ctx, connect.NewRequest(&v1.StartClipSpeechRequest{ProjectId: h.project, ExpectedRevision: 1, IdempotencyKey: "rpc-speech", QuoteId: q.Msg.QuoteId, ApprovedMaxCredits: &n, CancellationPolicyVersion: 1}))
	if e != nil {
		t.Fatal(e)
	}
	if e = h.run(t, started.Msg.JobId); e != nil {
		t.Fatal(e)
	}
	ready, e := rpc.GetClipSpeechReadiness(ctx, connect.NewRequest(&v1.GetClipSpeechReadinessRequest{ProjectId: h.project}))
	if e != nil || !ready.Msg.RenderReady || ready.Msg.Segments[0].State != "ready" || ready.Msg.Segments[0].HasCharacterTiming {
		t.Fatal(ready, e)
	}
}

type fixtureClipObjects struct{ *spokenTestObjects }

func (o fixtureClipObjects) PutClipSpeechAudio(ctx context.Context, key string, b []byte) error {
	return o.PutSpokenAudio(ctx, key, b)
}
func (o fixtureClipObjects) DeleteClipSpeechAudio(ctx context.Context, key string) error {
	return o.DeleteSpokenAudio(ctx, key)
}

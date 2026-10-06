package store_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/clip/workerclient"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"io"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

type analysisObjects struct {
	*artifactObjects
	data    map[string][]byte
	deleted []string
	listed  []clip.StoredObject
}

func (o *analysisObjects) Download(_ context.Context, key string, w io.Writer, limit int64) (int64, error) {
	data, ok := o.data[key]
	if !ok {
		return 0, clip.ErrNotFound
	}
	n, e := w.Write(data)
	return int64(n), e
}
func (o *analysisObjects) Delete(_ context.Context, key string) error {
	o.deleted = append(o.deleted, key)
	delete(o.data, key)
	delete(o.objects, key)
	return nil
}
func (o *analysisObjects) ListAnalysisCopies(context.Context) ([]clip.StoredObject, error) {
	return o.listed, nil
}

type analysisFixture struct {
	h       *generationHarness
	a       *clipapp.AnalysisPreparations
	objects *analysisObjects
	now     time.Time
	limits  clip.AnalysisPreparationLimits
	bind    clipapp.Binder
	q       clip.GenerationQuote
	p       clip.AnalysisPreparation
}

func newAnalysisFixture(t *testing.T) *analysisFixture {
	t.Helper()
	h := generationSetup(t)
	f := &analysisFixture{h: h, now: time.Now().UTC(), objects: &analysisObjects{artifactObjects: &artifactObjects{objects: map[string]clip.SourceObjectInfo{}}, data: map[string][]byte{}}}
	f.bind = func(tx *sql.Tx) clipapp.Ports {
		c := store.NewTx(tx)
		j := jobstore.NewTx(tx, jobKindsForTest())
		return clipapp.Ports{Clips: c, Jobs: j, Waits: j, Media: c, Analysis: c}
	}
	cfg := clip.DefaultMediaConfig(clip.Environment{})
	f.limits = clip.DefaultAnalysisPreparationLimits(clip.Environment{PutTTL: time.Minute, OrphanMinAge: time.Second})
	f.limits.Stages = clip.MediaStageLimits{LeaseTTL: time.Second, WaitTimeout: time.Minute, StageTimeout: 2 * time.Minute, MaxAttempts: 3}
	f.a = clipapp.NewAnalysisPreparations(h.db.Writer, f.bind, h.store, f.objects, clipapp.NewAnalysisJobs(h.queue), cfg, f.limits, func() time.Time { return f.now })
	deps := generationDeps(generationFinisher{h.store}, &quotePricing{}, nil)
	deps.AnalysisPreparations = f.a
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, h.planner, h.renderer, h.clipJobs(), h.cfg, deps)
	f.q = quote(t, h)
	return f
}
func (f *analysisFixture) input() clip.AnalysisPreparationInput {
	in := clip.AnalysisPreparationInput{ProjectID: f.h.project.ID, BatchID: f.h.batch.ID, QuoteID: f.q.ID, ExpectedRevision: f.h.project.EditPlanRevision, ProfileVersion: clip.BrowserAnalysisProfileVersion}
	for _, s := range f.h.batch.Sources {
		in.Originals = append(in.Originals, clip.BrowserOriginalMeasurement{SourceID: s.ID, Fingerprint: s.Fingerprint, Info: clip.MediaInfo{DurationMS: s.DurationMS, Width: s.Width, Height: s.Height, FrameRateNumerator: 30, FrameRateDenominator: 1, DecodedFrames: s.DurationMS * 30 / 1000, CadenceVerified: true}})
	}
	return in
}
func (f *analysisFixture) begin(t *testing.T) {
	t.Helper()
	p, e := f.a.Begin(t.Context(), "alice", f.input())
	if e != nil {
		t.Fatal(e)
	}
	f.p = p
}
func (f *analysisFixture) start(t *testing.T) string {
	t.Helper()
	id, e := f.h.service.Start(t.Context(), "alice", f.h.project.ID, f.h.batch.ID, "p/o", "p/w", clip.QuoteApproval{AnalysisPreparationID: f.p.ID, CancellationPolicyVersion: clip.CancellationPolicyVersion, QuoteID: f.q.ID, MaxCredits: &f.q.Pricing.MaxCredits})
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func (f *analysisFixture) upload(t *testing.T) {
	t.Helper()
	body := []byte("copy")
	digest := sha256.Sum256(body)
	for _, c := range f.p.Copies {
		access, _, e := f.a.Reserve(t.Context(), "alice", f.p.ID, c.Slot, int64(len(body)), hex.EncodeToString(digest[:]))
		if e != nil {
			t.Fatal(e)
		}
		key := strings.TrimPrefix(access.URL, "https://private.test/")
		f.objects.data[key] = body
		f.objects.objects[key] = clip.SourceObjectInfo{Bytes: int64(len(body)), ContentType: "video/mp4"}
	}
}
func analysisProfile() clip.MediaWorkerProfile {
	return clip.MediaWorkerProfile{WorkerID: "verify", Operation: clip.MediaVerifyAnalysis, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.AnalysisVerificationRenderer, AssetVersion: clip.AnalysisVerificationAssets, Profile: clip.AnalysisVerificationProfile, RuntimeManifest: `{"AcceptedOperations":["verify_analysis"]}`}
}

func TestBrowserPreparationOwnsQuoteAndNeverFallsBack(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	if f.p.Originals[0].Provenance != clip.BrowserOriginalProvenance {
		t.Fatal("client claims relabelled")
	}
	if f.p.Sources[0].OriginalMeasurementProvenance != clip.BrowserOriginalProvenance {
		t.Fatal("browser recovery source lost client provenance")
	}
	bound, e := f.h.store.GetQuote(t.Context(), "alice", f.q.ID)
	if e != nil || bound.InputDigest == f.q.InputDigest || bound.AnalysisPreparationID != f.p.ID {
		t.Fatalf("bound=%+v error=%v", bound, e)
	}
	if _, e = accept(f.h, f.q); !errors.Is(e, clip.ErrQuoteChanged) {
		t.Fatal("browser quote entered legacy native route", e)
	}
	for _, user := range []string{"bob", "foreign"} {
		if _, _, e = f.a.Reserve(t.Context(), user, f.p.ID, f.p.Copies[0].Slot, 4, strings.Repeat("a", 64)); e == nil {
			t.Fatal("foreign reserved copy")
		}
	}
	id := f.start(t)
	j, e := f.h.jobs.PickNextQueued(t.Context(), f.now)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.h.service.Run(t.Context(), "alice", id, f.h.project.ID, j.Payload, nil); !errors.Is(e, job.ErrYield) {
		t.Fatal("missing copies did not park", e)
	}
	if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 || f.h.media.probes != 0 {
		t.Fatal("pending browser work held/called/transcoded")
	}
	if _, e = f.a.Complete(t.Context(), "alice", f.p.ID); e == nil {
		t.Fatal("missing intervals submitted")
	}
}
func TestBrowserPreparationSubmissionOwnsFiniteVerificationClocks(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	f.start(t)
	f.now = f.now.Add(10 * time.Minute) // longer than StageTimeout, shorter than page TTL
	f.upload(t)
	submitted, e := f.a.Complete(t.Context(), "alice", f.p.ID)
	if e != nil {
		t.Fatal(e)
	}
	want := f.now.Add(f.limits.Stages.StageTimeout)
	if !submitted.DeadlineAt.Equal(want) || !submitted.QueueDeadlineAt.Equal(f.now.Add(f.limits.Stages.WaitTimeout)) {
		t.Fatalf("stage clocks started during browser upload: %+v", submitted)
	}
	f.now = f.now.Add(10 * time.Second)
	again, e := f.a.Complete(t.Context(), "alice", f.p.ID)
	if e != nil || !again.DeadlineAt.Equal(want) {
		t.Fatal("poll extended stage deadline", again, e)
	}
	work, e := f.a.Claim(t.Context(), analysisProfile())
	if e != nil || work == nil {
		t.Fatal("live delayed handoff was unclaimable", e)
	}
	bad := work.Credentials
	bad.Token = "forged"
	if _, e = f.a.Renew(t.Context(), bad, 1); !errors.Is(e, clip.ErrMediaLeaseLost) {
		t.Fatal("forged lease renewed", e)
	}
	f.now = f.now.Add(2 * time.Second)
	next, e := f.a.Claim(t.Context(), analysisProfile())
	if e != nil || next == nil || next.Credentials.Token == work.Credentials.Token {
		t.Fatal("expired lease not independently reclaimed", e)
	}
	if _, e = f.a.Read(t.Context(), work.Credentials, submitted.Copies[0].Slot); !errors.Is(e, clip.ErrMediaLeaseLost) {
		t.Fatal("old lease read copies", e)
	}
	if e = f.a.Fail(t.Context(), next.Credentials, clip.MediaFailureWorkerLost); e != nil {
		t.Fatal(e)
	}
	if replacement, e := f.a.Claim(t.Context(), analysisProfile()); e != nil || replacement == nil || replacement.Credentials.AttemptID == next.Credentials.AttemptID {
		t.Fatal("reported worker loss did not release retry capacity", e)
	}
}
func TestBrowserPreparationImmutableSlotsAndHeadRecheck(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	f.start(t)
	slot := f.p.Copies[0].Slot
	first, _, e := f.a.Reserve(t.Context(), "alice", f.p.ID, slot, 4, strings.Repeat("a", 64))
	if e != nil {
		t.Fatal(e)
	}
	same, _, e := f.a.Reserve(t.Context(), "alice", f.p.ID, slot, 4, strings.Repeat("a", 64))
	if e != nil || first.URL != same.URL || same.Headers["If-None-Match"] != "*" {
		t.Fatal("reservation identity changed", e)
	}
	if _, _, e = f.a.Reserve(t.Context(), "alice", f.p.ID, slot, 4, strings.Repeat("b", 64)); e == nil {
		t.Fatal("copy digest overwritten")
	}
	if _, _, e = f.a.Reserve(t.Context(), "alice", f.p.ID, "unowned/slot", 4, strings.Repeat("a", 64)); e == nil {
		t.Fatal("unowned slot accepted")
	}
	if _, _, e = f.a.Reserve(t.Context(), "alice", f.p.ID, f.p.Copies[1].Slot, 8<<20+1, strings.Repeat("a", 64)); e == nil {
		t.Fatal("oversized copy accepted")
	}
	if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 {
		t.Fatal("reservation admitted paid work")
	}
}
func TestBrowserPreparationRejectsForgedClientBindingsBeforeWork(t *testing.T) {
	for _, name := range []string{"duration", "source", "profile", "revision", "owner"} {
		t.Run(name, func(t *testing.T) {
			f := newAnalysisFixture(t)
			in := f.input()
			user := "alice"
			switch name {
			case "duration":
				in.Originals[0].Info.DurationMS = 900000
			case "source":
				in.Originals[0].SourceID = "foreign"
			case "profile":
				in.ProfileVersion = "unqualified-v999"
			case "revision":
				in.ExpectedRevision++
			case "owner":
				user = "bob"
			}
			if _, e := f.a.Begin(t.Context(), user, in); e == nil {
				t.Fatal("forged preparation admitted")
			}
			var n int
			if e := f.h.db.Reader.QueryRow("SELECT COUNT(*) FROM clip_analysis_preparations").Scan(&n); e != nil || n != 0 {
				t.Fatal("invalid preparation persisted", n, e)
			}
			if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 || f.h.media.probes != 0 {
				t.Fatal("invalid preparation did paid/native work")
			}
		})
	}
}
func TestBrowserPreparationRejectedReceiptDoesNotWakeOrReserve(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	id := f.start(t)
	j, e := f.h.jobs.PickNextQueued(t.Context(), f.now)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.h.service.Run(t.Context(), "alice", id, f.h.project.ID, j.Payload, nil); !errors.Is(e, job.ErrYield) {
		t.Fatal(e)
	}
	f.upload(t)
	p, e := f.a.Complete(t.Context(), "alice", f.p.ID)
	if e != nil {
		t.Fatal(e)
	}
	work, e := f.a.Claim(t.Context(), analysisProfile())
	if e != nil || work == nil {
		t.Fatal(e)
	}
	var task clip.AnalysisVerificationTask
	if json.Unmarshal([]byte(work.Payload), &task) != nil {
		t.Fatal("bad task")
	}
	receipt, _ := json.Marshal(clip.AnalysisVerificationResult{Version: 1, ProfileVersion: p.ProfileVersion, ManifestDigest: task.ManifestDigest})
	if e = f.a.CompleteVerification(t.Context(), work.Credentials, string(receipt)); e == nil {
		t.Fatal("missing decoded coverage accepted")
	}
	wait, e := f.h.jobs.Continuation(t.Context(), id)
	if e != nil || wait.State != job.ContinuationWaiting {
		t.Fatal("bad receipt woke parent", wait, e)
	}
	if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 {
		t.Fatal("bad receipt admitted paid work")
	}
}

func safeAnalysisReceipt(t *testing.T, work *clip.MediaWork) string {
	t.Helper()
	var task clip.AnalysisVerificationTask
	if json.Unmarshal([]byte(work.Payload), &task) != nil {
		t.Fatal("task decode")
	}
	out := clip.AnalysisVerificationResult{Version: 1, ProfileVersion: task.ProfileVersion, ManifestDigest: task.ManifestDigest}
	for _, c := range task.Copies {
		out.Copies = append(out.Copies, clip.AnalysisCopyVerification{Slot: c.Slot, Digest: c.Digest, Bytes: c.Bytes, Provenance: clip.AnalysisCopyProvenance, VideoPackets: c.DurationMS * 15 / 1000, Info: clip.MediaInfo{DurationMS: c.DurationMS, ContainerDurationMS: c.DurationMS, VideoDurationMS: c.DurationMS, DecodedDurationMS: c.DurationMS, Width: c.Width, Height: c.Height, FrameRateNumerator: 15, FrameRateDenominator: 1, CadenceVerified: true, DecodedFrames: c.DurationMS * 15 / 1000, PixelFormat: "yuv420p", SampleAspectRatio: "1:1", Streams: []clip.MediaStream{{Kind: "video", Codec: "h264"}}}})
	}
	data, e := json.Marshal(out)
	if e != nil {
		t.Fatal(e)
	}
	return string(data)
}
func TestBrowserPreparationAcceptedHandoffIsIdempotentAndQualityGated(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	id := f.start(t)
	first, e := f.h.jobs.PickNextQueued(t.Context(), f.now)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.h.service.Run(t.Context(), "alice", id, f.h.project.ID, first.Payload, nil); !errors.Is(e, job.ErrYield) {
		t.Fatal(e)
	}
	f.upload(t)
	if _, e = f.a.Complete(t.Context(), "alice", f.p.ID); e != nil {
		t.Fatal(e)
	}
	work, e := f.a.Claim(t.Context(), analysisProfile())
	if e != nil || work == nil {
		t.Fatal(e)
	}
	receipt := safeAnalysisReceipt(t, work)
	for i := 0; i < 2; i++ {
		if e = f.a.CompleteVerification(t.Context(), work.Credentials, receipt); e != nil {
			t.Fatal("identical receipt was not idempotent", e)
		}
	}
	wait, e := f.h.jobs.Continuation(t.Context(), id)
	if e != nil || wait.State != job.ContinuationReady {
		t.Fatal("accepted receipt did not wake its parent", wait, e)
	}
	candidate, e := f.h.jobs.PickNextQueued(t.Context(), f.now)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.h.service.Run(t.Context(), "alice", id, f.h.project.ID, candidate.Payload, nil); !errors.Is(e, clip.ErrAnalysisProfileUnqualified) {
		t.Fatal("unqualified profile dispatched", e)
	}
	if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 || f.h.planner.plans != 0 || f.h.media.probes != 0 {
		t.Fatal("quality gate issued paid/native work")
	}
	p, e := f.h.store.GetAnalysisPreparation(t.Context(), "alice", f.p.ID)
	if e != nil || p.Originals[0].Provenance != clip.BrowserOriginalProvenance || p.Copies[0].State != "verified" {
		t.Fatal("copy proof overwrote original client provenance", p, e)
	}
}
func TestBrowserPreparationCancelFencesLateReportsAndCleansOnce(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	id := f.start(t)
	first, e := f.h.jobs.PickNextQueued(t.Context(), f.now)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.h.service.Run(t.Context(), "alice", id, f.h.project.ID, first.Payload, nil); !errors.Is(e, job.ErrYield) {
		t.Fatal(e)
	}
	f.upload(t)
	if _, e = f.a.Complete(t.Context(), "alice", f.p.ID); e != nil {
		t.Fatal(e)
	}
	work, e := f.a.Claim(t.Context(), analysisProfile())
	if e != nil || work == nil {
		t.Fatal(e)
	}
	if _, e = f.a.Cancel(t.Context(), "alice", f.p.ID); e != nil {
		t.Fatal(e)
	}
	if e = f.a.CompleteVerification(t.Context(), work.Credentials, safeAnalysisReceipt(t, work)); e == nil {
		t.Fatal("late report published after cancellation")
	}
	f.now = f.now.Add(2 * time.Minute)
	for i := 0; i < 2; i++ {
		f.now = f.now.Add(2 * time.Second)
		if e = f.a.Reconcile(t.Context()); e != nil {
			t.Fatal(e)
		}
		if e = f.a.Cleanup(t.Context()); e != nil {
			t.Fatal(e)
		}
	}
	if len(f.objects.deleted) != len(f.p.Copies) {
		t.Fatal("private copies did not clean exactly once", f.objects.deleted)
	}
	current, e := f.h.jobs.GetByID(t.Context(), id)
	if e != nil || current.Status != job.StatusCancelled {
		t.Fatal("waiting cancellation did not settle", current.Status, e)
	}
	if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 {
		t.Fatal("cancelled preparation dispatched or reserved")
	}
}

func TestBrowserVerificationCapacityRemainsIndependentAndFinite(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	f.start(t)
	f.upload(t)
	if _, e := f.a.Complete(t.Context(), "alice", f.p.ID); e != nil {
		t.Fatal(e)
	}
	// An occupied native-render lease does not enter the separate verification
	// occupancy query or grant a verifier any native claim authority.
	project, err := f.h.projects.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "native occupied", Ratio: "vertical", Language: "ko", TargetDurationMS: 30000})
	if err != nil {
		t.Fatal(err)
	}
	_, e := f.h.db.Writer.Exec("INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('native-occupied','alice',?,'render_clip','running',?,?)", project.ID, f.now.Format(time.RFC3339Nano), f.now.Format(time.RFC3339Nano))
	if e != nil {
		t.Fatal(e)
	}
	in := clip.MediaStageInput{ID: "native-stage", ParentJobID: "native-occupied", UserID: "alice", ProjectID: project.ID, Operation: clip.MediaRender, ContractVersion: 3, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Payload: `{"native":true}`, Limits: clip.DefaultMediaStageLimits(clip.Environment{})}
	in.InputDigest = clip.MediaPayloadDigest(in.Payload)
	if _, e = f.h.store.CreateMediaStage(t.Context(), in, f.now); e != nil {
		t.Fatal(e)
	}
	native := mediaProfile()
	if _, e = f.h.store.ClaimMediaStage(t.Context(), native, f.now); e != nil {
		t.Fatal(e)
	}
	router := clipapp.NewMediaWorkerRouter(clipapp.NewMediaWorker(f.h.store, protocolArtifacts{}, func() time.Time { return f.now }), f.a)
	api := httptest.NewServer(cliprpc.NewMediaWorkerServerWithRoles("", map[string]string{"worker-1": "native-token", "verify": "verify-token", "verify-other": "other-token"}, map[string]string{"verify": clip.AnalysisVerificationRole, "verify-other": clip.AnalysisVerificationRole}, router).Handler)
	defer api.Close()
	nativeClient := workerclient.New(api.URL, "worker-1", "native-token")
	verifyClient := workerclient.New(api.URL, "verify", "verify-token")
	if status, e := verifyClient.StatusForProfile(t.Context(), analysisProfile()); e != nil || status != (clip.MediaRuntimeStatus{Waiting: 1}) {
		t.Fatal("verifier status advertised native occupancy", status, e)
	}
	if _, e := verifyClient.Status(t.Context()); !errors.Is(e, clip.ErrMediaIncompatible) {
		t.Fatal("verifier accepted native health identity", e)
	}
	work, e := f.a.Claim(t.Context(), analysisProfile())
	if e != nil || work == nil {
		t.Fatal("native render monopolized verification", e)
	}
	for name, client := range map[string]*workerclient.Client{"native": nativeClient, "verify": verifyClient} {
		profile := native
		if name == "verify" {
			profile = analysisProfile()
		}
		if status, e := client.StatusForProfile(t.Context(), profile); e != nil || status != (clip.MediaRuntimeStatus{Active: 1, OwnActive: 1}) {
			t.Fatal(name, "status mixed independent worker occupancy", status, e)
		}
	}
	if status, e := workerclient.New(api.URL, "verify-other", "other-token").StatusForProfile(t.Context(), analysisProfile()); e != nil || status != (clip.MediaRuntimeStatus{Active: 1}) {
		t.Fatal("verifier own-active scope", status, e)
	}
	if _, e := nativeClient.StatusForProfile(t.Context(), analysisProfile()); !errors.Is(e, clip.ErrMediaIncompatible) {
		t.Fatal("native identity accepted analysis profile", e)
	}
	if another, e := f.a.Claim(t.Context(), analysisProfile()); e != nil || another != nil {
		t.Fatal("duplicate active verification exceeded finite capacity", e)
	}
	wrong := analysisProfile()
	wrong.Profile = clip.MediaCPUProfile
	if _, e = f.a.Claim(t.Context(), wrong); e == nil {
		t.Fatal("unqualified worker profile accepted")
	}
}

func TestBrowserVerificationRejectsLiveFenceChangesBeforePublishing(t *testing.T) {
	for name, statement := range map[string]string{
		"revision":       "UPDATE clip_projects SET edit_plan_revision=edit_plan_revision+1 WHERE id=?",
		"deletion":       "UPDATE clip_projects SET deleting=1 WHERE id=?",
		"finalized":      "UPDATE clip_projects SET finalized_at='2026-10-06T00:00:00Z',finalized_plan_revision=1,edit_plan_revision=1,rendered_plan_revision=1,finalized_result_key='retained-result',result_key='retained-result',result_id='retained',source_access_revoked_at='2026-10-06T00:00:00Z' WHERE id=?",
		"source revoked": "UPDATE clip_projects SET source_access_revoked_at='2026-10-06T00:00:00Z' WHERE id=?",
	} {
		t.Run(name, func(t *testing.T) {
			f := newAnalysisFixture(t)
			f.begin(t)
			id := f.start(t)
			j, e := f.h.jobs.PickNextQueued(t.Context(), f.now)
			if e != nil {
				t.Fatal(e)
			}
			if e = f.h.service.Run(t.Context(), "alice", id, f.h.project.ID, j.Payload, nil); !errors.Is(e, job.ErrYield) {
				t.Fatal(e)
			}
			f.upload(t)
			if _, e = f.a.Complete(t.Context(), "alice", f.p.ID); e != nil {
				t.Fatal(e)
			}
			work, e := f.a.Claim(t.Context(), analysisProfile())
			if e != nil || work == nil {
				t.Fatal("initial lease", e)
			}
			if name == "deletion" {
				// Existing project deletion first cancels active parents; its SQL
				// busy guard prevents reversing that lifecycle order.
				if _, e = f.h.db.Writer.Exec("UPDATE generation_jobs SET status='cancelled',cancel_requested_at=? WHERE id=?", f.now.Format(time.RFC3339Nano), id); e != nil {
					t.Fatal(e)
				}
			}
			if _, e = f.h.db.Writer.Exec(statement, f.h.project.ID); e != nil {
				t.Fatal(e)
			}
			if e = f.a.CompleteVerification(t.Context(), work.Credentials, safeAnalysisReceipt(t, work)); e == nil {
				t.Fatal("changed live fence published")
			}
			if _, e = f.a.Read(t.Context(), work.Credentials, f.p.Copies[0].Slot); e == nil {
				t.Fatal("changed live fence read")
			}
			if _, e = f.a.Renew(t.Context(), work.Credentials, 1); e == nil {
				t.Fatal("changed live fence renewed")
			}
			wait, e := f.h.jobs.Continuation(t.Context(), id)
			if e != nil || wait.State != job.ContinuationWaiting {
				t.Fatal("stale preparation woke parent", wait, e)
			}
			if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 || f.h.media.probes != 0 {
				t.Fatal("stale input authorized paid/native work")
			}
		})
	}
}

type analysisSettlementRetry struct {
	clipapp.AnalysisPreparationJobs
	fail bool
}

func (j *analysisSettlementRetry) AcknowledgeWaitCancellation(ctx context.Context, id, key string) (bool, error) {
	if j.fail {
		j.fail = false
		return false, errors.New("temporary settlement failure")
	}
	return j.AnalysisPreparationJobs.AcknowledgeWaitCancellation(ctx, id, key)
}

func TestBrowserPreparationRestartRetriesSettlementAndLateOrphanCleanup(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	id := f.start(t)
	j, e := f.h.jobs.PickNextQueued(t.Context(), f.now)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.h.service.Run(t.Context(), "alice", id, f.h.project.ID, j.Payload, nil); !errors.Is(e, job.ErrYield) {
		t.Fatal(e)
	}
	f.upload(t)
	if _, e = f.a.Cancel(t.Context(), "alice", f.p.ID); e != nil {
		t.Fatal(e)
	}
	retry := &analysisSettlementRetry{AnalysisPreparationJobs: clipapp.NewAnalysisJobs(f.h.queue), fail: true}
	restarted := clipapp.NewAnalysisPreparations(f.h.db.Writer, f.bind, f.h.store, f.objects, retry, clip.DefaultMediaConfig(clip.Environment{}), f.limits, func() time.Time { return f.now })
	if e = restarted.Reconcile(t.Context()); e == nil {
		t.Fatal("lost external settlement failure")
	}
	var reconciled sql.NullString
	if e = f.h.db.Reader.QueryRow("SELECT reconciled_at FROM clip_analysis_preparations WHERE id=?", f.p.ID).Scan(&reconciled); e != nil || reconciled.Valid {
		t.Fatal("failed settlement dropped durable reconciliation", e)
	}
	for range 2 {
		if e = restarted.Reconcile(t.Context()); e != nil {
			t.Fatal(e)
		}
	}
	current, e := f.h.jobs.GetByID(t.Context(), id)
	if e != nil || current.Status != job.StatusCancelled {
		t.Fatal("restart did not settle waiting parent", current.Status, e)
	}
	f.now = f.now.Add(f.limits.PutTTL + f.limits.OrphanGrace + time.Second)
	if e = restarted.Cleanup(t.Context()); e != nil {
		t.Fatal(e)
	}
	if len(f.objects.deleted) != len(f.p.Copies) {
		t.Fatal("reserved copies leaked", f.objects.deleted)
	}
	key := f.objects.deleted[0]
	// A late object with a retired immutable key is recoverable through the
	// bounded orphan sweep, without reopening the session or its paid parent.
	f.objects.data[key] = []byte("late")
	f.objects.listed = []clip.StoredObject{{Key: key, Modified: f.now.Add(-2 * f.limits.OrphanGrace)}}
	if e = restarted.SweepOrphans(t.Context()); e != nil {
		t.Fatal(e)
	}
	f.now = f.now.Add(f.limits.OrphanGrace + time.Second)
	for range 2 {
		if e = restarted.Cleanup(t.Context()); e != nil {
			t.Fatal(e)
		}
	}
	if len(f.objects.deleted) != len(f.p.Copies)+1 {
		t.Fatal("late orphan deletion was lost or replayed", f.objects.deleted)
	}
	if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 {
		t.Fatal("recovery replayed paid work")
	}
}

func markBrowserRecoveryProvenance(t *testing.T, h *generationHarness) {
	t.Helper()
	r, e := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if e != nil || r == nil || len(r.Sources) == 0 || len(r.Chunks) == 0 {
		t.Fatal("missing accepted observation fixture", e)
	}
	for i := range r.Sources {
		r.Sources[i].OriginalMeasurementProvenance = clip.BrowserOriginalProvenance
	}
	raw, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	// Persist the browser checkpoint fixture as it would have been written
	// during its original run; normal SaveRecovery refuses stopped parents.
	if _, e = h.db.Writer.Exec("UPDATE clip_recovery_states SET state_json=? WHERE project_id=? AND user_id='alice'", string(raw), h.project.ID); e != nil {
		t.Fatal(e)
	}
}

func TestBrowserRecoveryNativeHostReprobesWithoutRepeatingPaidObservations(t *testing.T) {
	h := generationSetup(t)
	h.planner.errorPlan = errPreparationCrash
	h.start(t)
	if e := h.run(t); !errors.Is(e, errPreparationCrash) {
		t.Fatal(e)
	}
	markBrowserRecoveryProvenance(t, h)
	beforeObserve, beforeProbes := h.planner.observe, h.media.probes
	h.planner.errorPlan = nil
	q := quote(t, h)
	if q.Pricing.ObservationCalls != 0 || q.Pricing.ReusedChunks != beforeObserve {
		t.Fatal("lost compatible paid observation reuse", q.Pricing)
	}
	h.start(t)
	if e := h.run(t); e != nil {
		t.Fatal(e)
	}
	if h.media.probes <= beforeProbes || h.planner.observe != beforeObserve {
		t.Fatal("browser claims skipped native original probe or repeated paid observations", h.media.probes, h.planner.observe)
	}
	r, e := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if e != nil || r == nil {
		t.Fatal(e)
	}
	for _, s := range r.Sources {
		if s.OriginalMeasurementProvenance != "" {
			t.Fatal("actual native probe retained client provenance")
		}
	}
}

func TestBrowserRecoveryNativeRemoteReprobesWithoutRepeatingPaidObservations(t *testing.T) {
	g := remoteGenerationSetup(t)
	id := g.h.start(t)
	if e := g.runJob(t, g.pick(t)); !errors.Is(e, job.ErrYield) {
		t.Fatal(e)
	}
	auth, raw := g.preparedReceipt(t)
	if e := g.artifacts.Complete(t.Context(), auth, raw); e != nil {
		t.Fatal(e)
	}
	g.h.planner.errorPlan = errPreparationCrash
	if e := g.runJob(t, g.pick(t)); !errors.Is(e, errPreparationCrash) {
		t.Fatal(e)
	}
	if e := g.h.jobs.Finish(t.Context(), id, job.StatusFailed, &job.Failure{Reason: "JOB_INTERRUPTED"}, time.Now()); e != nil {
		t.Fatal(e)
	}
	if e := g.h.sources.ReleaseAttempt(t.Context(), "alice", id, time.Now()); e != nil {
		t.Fatal(e)
	}
	markBrowserRecoveryProvenance(t, g.h)
	before := g.h.planner.observe
	g.h.planner.errorPlan = nil
	q := quote(t, g.h)
	if q.Pricing.ObservationCalls != 0 || q.Pricing.ReusedChunks != before {
		t.Fatal("lost accepted observation reuse", q.Pricing)
	}
	next := g.h.start(t)
	if e := g.runJob(t, g.pick(t)); !errors.Is(e, job.ErrYield) {
		t.Fatal(e)
	}
	stage, e := g.h.store.MediaStageForJob(t.Context(), next, clip.MediaPrepare)
	if e != nil {
		t.Fatal(e)
	}
	task, e := mediacodec.DecodeTask(stage.Payload)
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range task.Sources {
		if !reflect.DeepEqual(s.Info, clip.MediaInfo{}) || len(s.ReusedChunks) != 0 {
			t.Fatal("browser original claim entered native v3 cache shortcut")
		}
	}
	auth, raw = g.preparedReceipt(t)
	result, e := mediacodec.DecodeResult(raw)
	if e != nil || len(result.Outputs) == 0 {
		t.Fatal("native preparation did not independently verify originals", e)
	}
	if e = g.artifacts.Complete(t.Context(), auth, raw); e != nil {
		t.Fatal(e)
	}
	if e = g.runJob(t, g.pick(t)); e != nil {
		t.Fatal(e)
	}
	if g.h.planner.observe != before || g.h.media.probes != 0 {
		t.Fatal("native remote continuation repeated paid observations or read originals on API", g.h.planner.observe)
	}
}

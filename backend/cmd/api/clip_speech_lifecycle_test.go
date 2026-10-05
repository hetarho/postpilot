package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice/spoken"
)

func speechFixtureSources(t *testing.T, h *clipSpeechFixture) (*clipapp.SourceService, *finalizationObjects, *time.Time, clip.SourceBatch) {
	t.Helper()
	now := time.Now().UTC()
	objects := &finalizationObjects{objects: map[string]clip.SourceObjectInfo{}}
	sources := clipapp.NewSourceService(h.store, objects, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: 6 * time.Hour, PutTTL: 10 * time.Minute}), func() time.Time { return now })
	u, err := sources.Create(t.Context(), "alice", h.project, []clip.SourceMetadata{{Filename: "source.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: 15000, Width: 640, Height: 640, Fingerprint: strings.Repeat("a", 64)}})
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[u.Batch.Sources[0].Key] = clip.SourceObjectInfo{Bytes: 100, ContentType: "video/mp4"}
	b, err := sources.Confirm(t.Context(), "alice", u.Batch.ID, u.Batch.Sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return sources, objects, &now, b
}

func TestClipSpeechLifecycleSourceExpiryRemovedVoiceAndNoSynthesis(t *testing.T) {
	h := newClipSpeechFixture(t)
	if err := h.run(t, h.start(t, "lifetime")); err != nil {
		t.Fatal(err)
	}
	_, edit := h.plan(t)
	ref := edit.Narration.Segments[0].Speech
	ticket, err := h.s.SpeechAccess(t.Context(), "alice", h.project, ref.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	sources, objects, now, _ := speechFixtureSources(t, h)
	*now = now.Add(25 * time.Hour)
	if err = sources.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(objects.objects) != 0 {
		t.Fatal("expired originals retained")
	}
	if _, err = h.f.library.RemoveVoice(t.Context(), "alice", h.voice.ID, h.voice.Revision, "remove"); err != nil {
		t.Fatal(err)
	}
	if err = h.f.library.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = h.s.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if data, err := h.s.ReadSpeechPlayback(t.Context(), "alice", ticket.ID); err != nil || len(data) == 0 {
		t.Fatal("speech followed source/voice lifetime", err)
	}
	_, edit = h.plan(t)
	if err = clip.NarrationReadiness(edit); err != nil || h.f.provider.speech != 2 {
		t.Fatal("removed voice rebound existing speech", err)
	}
	// Display changes and replay remain separate from speech input and admission.
	edit.Cuts[0].Copies = []clip.Copy{{Text: "independent displayed words", Style: "bold", Anchor: "bottom", Align: "center"}}
	raw, _ := clip.EncodeEditPlan(edit)
	p, _ := h.plan(t)
	if _, err = h.store.SaveCorrection(t.Context(), "alice", h.project, p.EditPlanRevision, raw, nil); err != nil {
		t.Fatal(err)
	}
	if h.f.provider.speech != 2 {
		t.Fatal("caption edit synthesized")
	}
}

func TestClipSpeechLifecycleFinalizationKeepsExactResultAndOnlyUsedAssets(t *testing.T) {
	h := newClipSpeechFixture(t)
	if err := h.run(t, h.start(t, "finalize")); err != nil {
		t.Fatal(err)
	}
	p, edit := h.plan(t)
	used, err := h.store.GetSpeechAsset(t.Context(), "alice", h.project, edit.Narration.Segments[0].Speech.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	unused := used
	unused.ID = "unused-speech"
	unused.Speech.AssetID = unused.ID
	unused.ObjectKey = clip.SpeechAudioPrefix + unused.ID + ".mp3"
	if err = h.store.InsertSpeechAsset(t.Context(), unused); err != nil {
		t.Fatal(err)
	}
	h.f.objects.data[unused.ObjectKey] = h.f.objects.data[used.ObjectKey]
	sources, objects, _, batch := speechFixtureSources(t, h)
	src := batch.Sources[0]
	edit.Cuts[0].SourceID, edit.Cuts[0].Fingerprint = src.ID, src.Fingerprint
	edit.SourceAudio = &clip.SourceAudioSettings{Values: []clip.SourceAudioSetting{{SourceID: src.ID, Fingerprint: src.Fingerprint, RetainOriginal: false}}}
	raw, _ := clip.EncodeEditPlan(edit)
	analysis, _ := json.Marshal([]clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: src.ID, Fingerprint: src.Fingerprint, Info: clip.MediaInfo{DurationMS: 15000, Width: 640, Height: 640}}}}})
	result := clip.Result{Key: clip.ResultPrefix + "alice/" + h.project + "/narrated.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: 15000, CreatedAt: time.Now(), Speech: clip.RequestedSpeech(edit)}
	if err = h.store.SaveGeneration(t.Context(), "alice", h.project, string(analysis), raw, result); err != nil {
		t.Fatal(err)
	}
	p, _ = h.plan(t)
	f := clipapp.NewFinalizer(h.f.handle.Writer, clipTxPorts(h.f.ledger, nil, h.f.admission.plans), h.store, clip.DefaultRenderConfig(clip.Environment{}), nil)
	service := clipapp.NewService(h.store, clip.DefaultLimits(), sources, f)
	req := clip.FinalizationRequest{UserID: "alice", ProjectID: h.project, ExpectedRevision: p.EditPlanRevision, ExpectedResultID: p.Result.ID}
	final, err := service.FinalizeProject(t.Context(), req)
	if err != nil || final.Finalized == nil || clip.SpeechFingerprint(final.Result.Speech) != clip.SpeechFingerprint(result.Speech) {
		t.Fatal("narrated finalization", final, err)
	}
	if _, err = h.store.GetSpeechAsset(t.Context(), "alice", h.project, unused.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("unused speech retained", err)
	}
	if _, err = h.store.GetSpeechAsset(t.Context(), "alice", h.project, used.ID); err != nil {
		t.Fatal("confirmed speech collected", err)
	}
	if err = h.s.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.f.objects.data[unused.ObjectKey]; ok {
		t.Fatal("unused audio not cleaned")
	}
	if _, ok := h.f.objects.data[used.ObjectKey]; !ok {
		t.Fatal("confirmed audio deleted")
	}
	if _, err = h.f.library.GetVoice(t.Context(), "alice", h.voice.ID); err != nil {
		t.Fatal("clip finalization deleted voice", err)
	}
	if _, err = service.FinalizeProject(t.Context(), req); err != nil {
		t.Fatal("repeat confirmation", err)
	}
	clipapp.NewGenerationService(h.store, service, sources, objects, nil, nil, nil, nil, clip.GenerationConfig{ReadTTL: time.Minute, CleanupTimeout: time.Second, OrphanMinAge: time.Hour}, neutralGenerationDeps())
	read, err := service.GetProject(t.Context(), "alice", h.project)
	if err != nil || read.Result.DownloadURL == "" || read.Result.ViewURL == "" {
		t.Fatal("final result depends on voice or original access", err)
	}
	if err = h.store.InsertSpeechAsset(t.Context(), unused); err == nil {
		t.Fatal("late asset restored finalized editing")
	}
	if _, err = h.s.Quote(t.Context(), "alice", h.project, final.EditPlanRevision); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal("finalized speech admitted", err)
	}
	if len(objects.objects) != 0 || h.f.provider.speech != 2 {
		t.Fatal("finalization retained originals or invoked synthesis")
	}
}

type failingClipSpeechDelete struct {
	clipapp.SpeechObjects
	fail bool
}

func (o *failingClipSpeechDelete) DeleteClipSpeechAudio(ctx context.Context, key string) error {
	if o.fail {
		return errors.New("storage unavailable")
	}
	return o.SpeechObjects.DeleteClipSpeechAudio(ctx, key)
}

func TestClipSpeechLifecycleProjectAndAccountDeletionCleanupRetry(t *testing.T) {
	for _, kind := range []string{"project", "account"} {
		t.Run(kind, func(t *testing.T) {
			h := newClipSpeechFixture(t)
			if err := h.run(t, h.start(t, "delete")); err != nil {
				t.Fatal(err)
			}
			_, edit := h.plan(t)
			ref := edit.Narration.Segments[0].Speech
			ticket, err := h.s.SpeechAccess(t.Context(), "alice", h.project, ref.AssetID)
			if err != nil {
				t.Fatal(err)
			}
			if err = h.s.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if kind == "project" {
				err = h.store.DeleteProject(t.Context(), "alice", h.project)
			} else {
				_, err = authstore.New(h.f.handle.Writer, h.f.handle.Reader).DeleteAllUsers(t.Context())
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = h.s.ReadSpeechPlayback(t.Context(), "alice", ticket.ID); !errors.Is(err, clip.ErrNotFound) {
				t.Fatal("deleted owner/project authorized", err)
			}
			pending, err := h.store.PendingSpeechCleanup(t.Context(), time.Now().Add(time.Hour))
			if err != nil || len(pending) != 2 {
				t.Fatal("cascade lost durable speech intents", pending, err)
			}
			failing := &failingClipSpeechDelete{SpeechObjects: h.s.Objects, fail: true}
			h.s.Objects = failing
			if err = h.s.Cleanup(t.Context(), time.Now().Add(time.Hour)); err == nil {
				t.Fatal("failed cleanup disappeared")
			}
			failing.fail = false
			for range 2 {
				if err = h.s.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			for key := range h.f.objects.data {
				if strings.HasPrefix(key, clip.SpeechAudioPrefix) {
					t.Fatal("deleted speech survived", key)
				}
			}
			if kind == "project" {
				if _, err = h.f.library.GetVoice(t.Context(), "alice", h.voice.ID); err != nil {
					t.Fatal("project deleted account voice", err)
				}
			} else {
				if err = h.f.library.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
				if len(h.f.objects.data) != 0 {
					t.Fatal("account retained private audio")
				}
			}
		})
	}
}

func TestClipSpeechLifecycleLateUploadCannotRestoreDeletedProject(t *testing.T) {
	h := newClipSpeechFixture(t)
	id := h.start(t, "late-delete")
	j, err := h.f.jobs.PickNextQueued(t.Context(), time.Now())
	if err != nil || j.ID != id {
		t.Fatal(err)
	}
	h.f.objects.afterPut = func() {
		if _, err := h.f.queue.Cancel(t.Context(), "alice", job.Subject{Dimension: clip.JobSubject, ID: h.project}, id); err != nil {
			t.Fatal(err)
		}
		if err := h.f.jobs.Finish(t.Context(), id, job.StatusCancelled, nil, time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := h.f.handle.Writer.Exec("DELETE FROM clip_projects WHERE id=? AND user_id='alice'", h.project); err != nil {
			t.Fatal(err)
		}
	}
	if err = metered(h.s.Run)(t.Context(), j, func(string, int, int) {}); err == nil {
		t.Fatal("late upload published")
	}
	if _, err = h.store.GetProject(t.Context(), "alice", h.project); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("project resurrected", err)
	}
	if err = h.s.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	for key := range h.f.objects.data {
		if strings.HasPrefix(key, clip.SpeechAudioPrefix) {
			t.Fatal("late object escaped journal", key)
		}
	}
	if h.f.provider.speech != 1 {
		t.Fatal("late paid call retried")
	}
	if _, err = h.f.queue.Snapshot(t.Context(), "alice", job.Subject{Dimension: clip.JobSubject, ID: h.project}, id); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("deleted job restored", err)
	}
}

type speechCleanupDeletionRace struct {
	clipapp.SpeechStorage
	h       *clipSpeechFixture
	deleted bool
}

func (s *speechCleanupDeletionRace) SpeechAssetRetained(ctx context.Context, id string) (bool, error) {
	retained, err := s.SpeechStorage.SpeechAssetRetained(ctx, id)
	if err == nil && retained && !s.deleted {
		s.deleted = true
		err = s.h.store.DeleteProject(ctx, "alice", s.h.project)
	}
	return retained, err
}
func TestClipSpeechLifecycleCleanupReadCannotLoseDeletion(t *testing.T) {
	h := newClipSpeechFixture(t)
	if err := h.run(t, h.start(t, "cleanup-race")); err != nil {
		t.Fatal(err)
	}
	h.s.Store = &speechCleanupDeletionRace{SpeechStorage: h.store, h: h}
	for range 2 {
		if err := h.s.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	for key := range h.f.objects.data {
		if strings.HasPrefix(key, clip.SpeechAudioPrefix) {
			t.Fatal("conditional close lost cascade intent", key)
		}
	}
}

func TestClipSpeechLifecycleJournalFailurePreventsUpload(t *testing.T) {
	h := newClipSpeechFixture(t)
	id := h.start(t, "journal-failure")
	if _, err := h.f.handle.Writer.Exec(`CREATE TRIGGER fail_speech_journal BEFORE INSERT ON clip_speech_cleanup BEGIN SELECT RAISE(ABORT,'fixture journal unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if err := h.run(t, id); err == nil {
		t.Fatal("unrecoverable upload attempted")
	}
	for key := range h.f.objects.data {
		if strings.HasPrefix(key, clip.SpeechAudioPrefix) {
			t.Fatal("upload preceded durable intent", key)
		}
	}
	if h.f.provider.speech != 1 {
		t.Fatal("failed journal caused paid retry")
	}
}

func TestClipSpeechLifecycleUnpublishedUploadRecoversAfterRestart(t *testing.T) {
	h := newClipSpeechFixture(t)
	id := h.start(t, "row-failure")
	if _, err := h.f.handle.Writer.Exec(`CREATE TRIGGER fail_speech_row BEFORE INSERT ON clip_speech_assets BEGIN SELECT RAISE(ABORT,'fixture row unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if err := h.run(t, id); err == nil {
		t.Fatal("failed publication succeeded")
	}
	count := 0
	for key := range h.f.objects.data {
		if strings.HasPrefix(key, clip.SpeechAudioPrefix) {
			count++
		}
	}
	if count != 1 {
		t.Fatal("fixture did not leave an uploaded object", count)
	}
	h.f.handle.Close()
	reopened, err := db.Open(h.f.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	h.s.Store = clipstore.New(reopened.Writer, reopened.Reader)
	if err = h.s.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	for key := range h.f.objects.data {
		if strings.HasPrefix(key, clip.SpeechAudioPrefix) {
			t.Fatal("restart lost pre-PUT intent", key)
		}
	}
	if h.f.provider.speech != 1 {
		t.Fatal("cleanup retried synthesis")
	}
}

type refusedNarratedProfile struct{ calls int }

func (p *refusedNarratedProfile) ResolveNarratedSpokenProfile(context.Context, string, plan.Plan, string, int64) (spoken.Profile, error) {
	p.calls++
	return spoken.Profile{}, modelcatalog.ErrSpeechProfileUnavailable
}
func TestClipSpeechLifecycleUnqualifiedExportProfileRefusesBeforePaidWork(t *testing.T) {
	h := newClipSpeechFixture(t)
	gate := &refusedNarratedProfile{}
	h.s.Voices = clipSpeechVoices{voices: clipSpokenVoices{h.f.library}, profiles: gate}
	p, _ := h.plan(t)
	if _, err := h.s.Quote(t.Context(), "alice", h.project, p.EditPlanRevision); !errors.Is(err, modelcatalog.ErrSpeechProfileUnavailable) {
		t.Fatal("missing export qualification admitted speech", err)
	}
	if gate.calls != 1 || h.f.provider.speech != 0 {
		t.Fatal("profile gate was bypassed or paid work started")
	}
}

package clip_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestNativeFinalizationUsesSavedEvidenceWithoutPixelsOrLayout(t *testing.T) {
	p, plan := nativeHistoryFixture(t)
	p.UserID, p.ID, p.EditPlanRevision, p.RenderedPlanRevision = "owner", "project", 2, 2
	p.Result = &clip.Result{ID: "result", Key: "clip-results/result.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: plan.DurationMS, CreatedAt: time.Now()}
	req := clip.FinalizationRequest{UserID: p.UserID, ProjectID: p.ID, ExpectedRevision: 2, ExpectedResultID: "result"}
	for _, kind := range []clip.RenderKind{"", clip.RenderServer, clip.RenderBrowser} {
		p.Result.Kind = kind
		if err := clip.ValidateFinalization(p, req, clip.DefaultRenderConfig(clip.Environment{})); err != nil {
			t.Fatal(kind, err)
		}
	}
	plan.Portable.Elements[0].StaleEvidence = true
	var err error
	p.EditPlan, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := clip.ValidateFinalization(p, req, clip.DefaultRenderConfig(clip.Environment{})); !errors.Is(err, clip.ErrFinalizationInvalid) {
		t.Fatal("stale evidence finalized", err)
	}
}

func TestNarratedFinalizationRequiresExactCurrentOutputSpeech(t *testing.T) {
	p, plan := nativeHistoryFixture(t)
	p.UserID, p.ID, p.EditPlanRevision, p.RenderedPlanRevision = "owner", "project", 2, 2
	text := "spoken sentence"
	ref := clip.SpeechRef{AssetID: "speech-one", VoiceID: "voice", BindingDigest: strings.Repeat("a", 64), InputHash: clip.SpokenInputHash(text), SettingsHash: strings.Repeat("b", 64), AudioHash: strings.Repeat("c", 64), ProfileID: "profile", ProfileRevision: 1, Samples: 44100, SampleRate: 44100, Channels: 2}
	plan.Narration = &clip.NarrationPlan{Enabled: true, VoiceID: ref.VoiceID, BindingDigest: ref.BindingDigest, VolumePermille: 1000, Segments: []clip.SpokenSegment{{ID: "spoken-1", Text: text, TextRevision: 1, InputHash: ref.InputHash, StartMS: 1000, EndMS: 2000, Speech: &ref}}}
	p.EditPlan, _ = clip.EncodeEditPlan(plan)
	p.Result = &clip.Result{ID: "result", Key: "clip-results/result.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: plan.DurationMS, CreatedAt: time.Now()}
	req := clip.FinalizationRequest{UserID: p.UserID, ProjectID: p.ID, ExpectedRevision: 2, ExpectedResultID: "result"}
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	if err := clip.ValidateFinalization(p, req, cfg); !errors.Is(err, clip.ErrFinalizationInvalid) {
		t.Fatal("missing speech finalized", err)
	}
	p.Result.Speech = clip.RequestedSpeech(plan)
	if err := clip.ValidateFinalization(p, req, cfg); err != nil {
		t.Fatal("matching speech refused", err)
	}
	p.Result.Speech[0].Speech.AudioHash = strings.Repeat("d", 64)
	if err := clip.ValidateFinalization(p, req, cfg); !errors.Is(err, clip.ErrFinalizationInvalid) {
		t.Fatal("different speech finalized", err)
	}
}

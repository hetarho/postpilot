package store_test

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"strings"
	"testing"
	"time"
)

func TestClipSpeechAssetsOwnershipReuseAndPublicationCAS(t *testing.T) {
	ctx := context.Background()
	h, p, _ := completedNativeClip(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	text := "자막과 다른 더빙 대본"
	hash := clip.SpokenInputHash(text)
	binding := strings.Repeat("a", 64)
	a := clip.SpeechAsset{ID: "speech-one", OwnerID: "alice", ProjectID: p.ID, ObjectKey: "clip/speech/private.mp3", Text: text, CreatedAt: time.Now(), Speech: clip.SpeechRef{AssetID: "speech-one", VoiceID: "voice", BindingDigest: binding, InputHash: hash, SettingsHash: strings.Repeat("b", 64), AudioHash: strings.Repeat("c", 64), ProfileID: "profile", ProfileRevision: 1, Samples: 44100, SampleRate: 44100, Channels: 2}}
	foreign := a
	foreign.ID = "foreign"
	foreign.Speech.AssetID = "foreign"
	foreign.OwnerID = "bob"
	foreign.ObjectKey = "another.mp3"
	if err := h.store.InsertSpeechAsset(ctx, foreign); err == nil {
		t.Fatal("foreign project accepted")
	}
	if err := h.store.InsertSpeechAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.GetSpeechAsset(ctx, "bob", p.ID, a.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("foreign read", err)
	}
	if _, err := h.store.GetSpeechAsset(ctx, "alice", "another-project", a.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("foreign project read", err)
	}
	if got, err := h.store.FindSpeechAsset(ctx, "alice", p.ID, hash, binding); err != nil || got.ID != a.ID {
		t.Fatal("reuse", err)
	}
	plan.Narration = &clip.NarrationPlan{Enabled: true, VoiceID: "voice", BindingDigest: binding, VolumePermille: 1000, Segments: []clip.SpokenSegment{{ID: "spoken-1", Text: text, TextRevision: 1, InputHash: hash, EndMS: 2000}}}
	raw, _ := clip.EncodeEditPlan(plan)
	p, err = h.store.SaveCorrection(ctx, "alice", p.ID, p.EditPlanRevision, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldRevision := p.EditPlanRevision
	p, err = h.store.PublishSpeechAsset(ctx, "alice", p.ID, "", oldRevision, "spoken-1", hash, binding, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	ready, _ := clip.DecodeEditPlan(p.EditPlan)
	if err := clip.NarrationReadiness(ready); err != nil {
		t.Fatal(err)
	}
	ready.Narration.VolumePermille = 900
	raw, _ = clip.EncodeEditPlan(ready)
	p, err = h.store.SaveCorrection(ctx, "alice", p.ID, p.EditPlanRevision, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.PublishSpeechAsset(ctx, "alice", p.ID, "", oldRevision, "spoken-1", hash, binding, a.ID); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("late job replaced plan", err)
	}
	ready.Narration.Segments[0].Speech.AudioHash = strings.Repeat("d", 64)
	raw, _ = clip.EncodeEditPlan(ready)
	if _, err := h.store.SaveCorrection(ctx, "alice", p.ID, p.EditPlanRevision, raw, nil); err == nil {
		t.Fatal("client forged provenance")
	}
	// A later generated sentence and then an owner undo reuse the original
	// retained asset, even after a different ready asset occupied the segment.
	current, _ := clip.DecodeEditPlan(p.EditPlan)
	b := a
	b.ID, b.ObjectKey, b.Text = "speech-two", "clip/speech/two.mp3", "새 더빙 문장"
	b.Speech.AssetID, b.Speech.InputHash = b.ID, clip.SpokenInputHash(b.Text)
	b.Speech.AudioHash = strings.Repeat("e", 64)
	if err := h.store.InsertSpeechAsset(ctx, b); err != nil {
		t.Fatal(err)
	}
	for _, desired := range []clip.SpeechAsset{b, a} {
		in := clip.CorrectionFromPlan(current)
		in.Narration.Segments[0].Text = desired.Text
		next := current
		if err := clip.CorrectNarration(current, in, &next); err != nil {
			t.Fatal(err)
		}
		raw, _ = clip.EncodeEditPlan(next)
		p, err = h.store.SaveCorrection(ctx, "alice", p.ID, p.EditPlanRevision, raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		current, _ = clip.DecodeEditPlan(p.EditPlan)
		if current.Narration.Segments[0].Speech.AssetID != desired.ID || !clip.CompatibleSpeech(current.Narration, current.Narration.Segments[0]) {
			t.Fatal("exact retained input was not reused")
		}
	}
}

func TestClipForeignVoiceRefusedBeforeSave(t *testing.T) {
	h, p, draft := completedNativeClip(t)
	draft.Narration = &clip.NarrationPlan{Enabled: true, VoiceID: "foreign", VolumePermille: 1000, Segments: []clip.SpokenSegment{{Creation: true, Text: "문장", EndMS: 1000}}}
	if _, err := h.service.SaveCorrection(context.Background(), "alice", p.ID, p.EditPlanRevision, draft); !errors.Is(err, clip.ErrCompositionUnavailable) {
		t.Fatal("unresolved foreign voice accepted", err)
	}
	current, err := h.projects.GetProject(context.Background(), "alice", p.ID)
	if err != nil || current.EditPlanRevision != p.EditPlanRevision {
		t.Fatal("refusal changed plan", err)
	}
}

func TestNarratedResultProvenancePersistsAndRejectsOmission(t *testing.T) {
	h, p, _ := completedNativeClip(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	text := "immutable sentence"
	hash := clip.SpokenInputHash(text)
	binding := strings.Repeat("a", 64)
	a := clip.SpeechAsset{ID: "speech-result", OwnerID: "alice", ProjectID: p.ID, ObjectKey: "clip/speech/result.mp3", Text: text, Bytes: 100, CreatedAt: time.Now(), Speech: clip.SpeechRef{AssetID: "speech-result", VoiceID: "voice", BindingDigest: binding, InputHash: hash, SettingsHash: strings.Repeat("b", 64), AudioHash: strings.Repeat("c", 64), ProfileID: "profile", ProfileRevision: 1, Samples: 44100, SampleRate: 44100, Channels: 2}}
	if err = h.store.InsertSpeechAsset(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	plan.Narration = &clip.NarrationPlan{Enabled: true, VoiceID: "voice", BindingDigest: binding, VolumePermille: 800, Segments: []clip.SpokenSegment{{ID: "spoken-1", Text: text, InputHash: hash, TextRevision: 1, EndMS: 2000, Speech: &a.Speech}}}
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	p, err = h.store.SaveCorrection(t.Context(), "alice", p.ID, p.EditPlanRevision, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := clip.Result{Key: "clip/results/narrated.mp4", ContentType: "video/mp4", Bytes: 1000, DurationMS: p.TargetDurationMS, CreatedAt: time.Now(), Speech: clip.RequestedSpeech(plan)}
	omitted := result
	omitted.Speech = nil
	if err = h.store.SaveRender(t.Context(), "alice", p.ID, p.EditPlanRevision, omitted); !errors.Is(err, clip.ErrInvalidMedia) {
		t.Fatal("omitted audio published", err)
	}
	if err = h.store.SaveRender(t.Context(), "alice", p.ID, p.EditPlanRevision, result); err != nil {
		t.Fatal(err)
	}
	got, err := h.store.GetProject(t.Context(), "alice", p.ID)
	if err != nil || got.Result == nil || len(got.Result.Speech) != 1 || got.Result.Speech[0].Speech.AssetID != a.ID {
		t.Fatal("provenance lost", got, err)
	}
	if err = h.store.SaveGeneration(t.Context(), "alice", p.ID, "analysis", raw, omitted); !errors.Is(err, clip.ErrInvalidMedia) {
		t.Fatal("generation bypass", err)
	}
}

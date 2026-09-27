package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"github.com/postpilot/backend/internal/clip"
)

// guidelineSource is the guideline context as the clip one sees it: whatever the owner's
// 영상 지침 are right now, and every read it was asked for.
type guidelineSource struct {
	value clip.VideoGuidelines
	reads []string
}

func (g *guidelineSource) ForClip(_ context.Context, user, templateID, language string) (clip.VideoGuidelines, error) {
	g.reads = append(g.reads, user+"|"+templateID+"|"+language)
	return g.value, nil
}

// withGuidelines rebuilds the generation side with a 영상 지침 source.
func (h *generationHarness) withGuidelines(source clipapp.VideoGuidelineSource) {
	deps := generationDeps(generationFinisher{h.store}, &quotePricing{}, nil)
	deps.Guidelines = source
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, h.planner, h.renderer, h.clipJobs(), h.cfg, deps)
}

var (
	firstGuidelines  = clip.VideoGuidelines{Defaults: []string{"입력한 사실만 쓰기"}, Owner: []string{"자막에 가격을 적지 않기"}}
	secondGuidelines = clip.VideoGuidelines{Defaults: []string{"입력한 사실만 쓰기"}, Owner: []string{"자막은 두 줄까지"}}
)

// QUOTA-45, GUIDE-15: a clip quote binds the 영상 지침 it was taken under, so a change before
// the start refuses the approval; the start freezes them, and the run reads only its payload
// however the owner's guidelines move afterwards.
func TestAClipQuoteBindsItsVideoGuidelinesAndTheStartFreezesThem(t *testing.T) {
	h := generationSetup(t)
	source := &guidelineSource{value: firstGuidelines}
	h.withGuidelines(source)
	ctx := t.Context()

	q, err := h.service.Quote(ctx, "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if q.Pricing.GuidelinesDigest == "" || q.Pricing.GuidelinesDigest != firstGuidelines.Digest() {
		t.Fatal("the quote did not bind the 영상 지침", q.Pricing.GuidelinesDigest)
	}
	// The project's own video template and language are what the guidelines were read for.
	if len(source.reads) == 0 || source.reads[0] != "alice|"+h.project.VideoTemplateID+"|ko" {
		t.Fatal("the 영상 지침 were read for another project", source.reads)
	}
	source.value = secondGuidelines
	approval := clip.QuoteApproval{CancellationPolicyVersion: clip.CancellationPolicyVersion, QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits}
	if _, err := h.service.Start(ctx, "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approval); !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal("a 영상 지침 change between the quote and the start was approved", err)
	}

	id := h.start(t)
	j, err := h.jobs.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var payload clip.GenerationPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(payload.Guidelines, secondGuidelines) {
		t.Fatal("the start did not freeze the 영상 지침 it approved", payload.Guidelines)
	}
	// Edited after the start: the run still writes under what it froze.
	source.value = clip.VideoGuidelines{Owner: []string{"나중에 바꾼 지침"}}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.planner.input.Guidelines, secondGuidelines) {
		t.Fatal("the writer did not read the frozen 영상 지침", h.planner.input.Guidelines)
	}
}

// A project with no 영상 지침 quotes exactly as before they existed: no digest, and nothing in
// the frozen payload.
func TestAClipWithNoVideoGuidelinesFreezesNone(t *testing.T) {
	h := generationSetup(t)
	h.withGuidelines(&guidelineSource{})
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if q.Pricing.GuidelinesDigest != "" {
		t.Fatal("an empty 영상 지침 set moved the quote", q.Pricing.GuidelinesDigest)
	}
	id := h.start(t)
	j, err := h.jobs.GetByID(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(j.Payload, &raw); err != nil {
		t.Fatal(err)
	}
	if _, present := raw["Guidelines"]; present {
		t.Fatal("an empty 영상 지침 set was written into the payload", string(raw["Guidelines"]))
	}
}

// A revision binds and freezes the 영상 지침 the same way (CLIP-131, QUOTA-45).
func TestAClipRevisionBindsAndFreezesItsVideoGuidelines(t *testing.T) {
	h := revisionReady(t)
	source := &guidelineSource{value: firstGuidelines}
	h.withGuidelines(source)
	ctx := t.Context()

	q, err := h.service.QuoteRevision(ctx, "alice", h.project.ID, "더 짧게", clip.RevisionNarration, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	source.value = secondGuidelines
	approval := clip.QuoteApproval{CancellationPolicyVersion: clip.CancellationPolicyVersion, QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits}
	if _, err := h.service.StartRevision(ctx, "alice", h.project.ID, "더 짧게", clip.RevisionNarration, "p/o", "p/w", approval); !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal("a 영상 지침 change between the revision quote and its start was approved", err)
	}

	startRevision(t, h, "더 짧게", clip.RevisionNarration)
	source.value = clip.VideoGuidelines{}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if len(h.planner.revisions) != 1 || !reflect.DeepEqual(h.planner.revisions[0].Guidelines, secondGuidelines) {
		t.Fatal("the revision did not write under the 영상 지침 it froze", h.planner.revisions)
	}
}

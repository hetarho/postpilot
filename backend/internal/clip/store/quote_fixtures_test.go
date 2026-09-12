package store_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

type quotePricing struct {
	inputRate   string
	budgetDelta int
}

func (p *quotePricing) Freeze(_ context.Context, o, w llm.ModelRef, count int) (clip.GenerationPricing, error) {
	rate := p.inputRate
	if rate == "" {
		rate = "0.1"
	}
	a := llm.CallPolicy{Ref: o, Stage: "observe", CompletionTokens: 8192 + p.budgetDelta, InputUSDPerMillion: rate, OutputUSDPerMillion: "0.7"}
	b := llm.CallPolicy{Ref: w, Stage: "write", CompletionTokens: 32768, InputUSDPerMillion: rate, OutputUSDPerMillion: "0.7"}
	a.Pricing = llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionInlineStatic, Endpoint: "leaf", RequiredParameters: "max_tokens", PromptUSDPerMillion: rate, CompletionUSDPerMillion: "0.7", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0"}
	b.Pricing = a.Pricing
	b.Pricing.Delivery = llm.ExecutionTextOnly
	credits, err := usage.ClipCredits([]usage.PricedCall{{Policy: a, Count: count}, {Policy: b, Count: 1}})
	return clip.GenerationPricing{Version: clip.PricingPolicyVersion, Observe: a, Plan: b, ObservationCalls: count, MaxCredits: credits}, err
}
func startApproved(ctx context.Context, s *clip.GenerationService, user, id, batch, o, w string) (string, error) {
	q, err := s.Quote(ctx, user, id, batch, o, w)
	if err != nil {
		return "", err
	}
	return s.Start(ctx, user, id, batch, o, w, clip.QuoteApproval{QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits})
}
func (j generationJobs) Latest(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	r, err := j.q.LatestClipSnapshot(ctx, user, id)
	if err != nil || r == nil {
		return nil, err
	}
	return &clip.ClipJob{ID: r.ID, Kind: r.Kind, Status: r.Status, Stage: r.Stage, Payload: r.Payload, DispatchReady: r.DispatchReady}, nil
}

// Correction and outbox tests use a retained result fixture, independent of the
// temporarily unavailable AI runner. This makes their zero-credit checks exact.
func seedCompletedGeneration(t *testing.T, h *generationHarness) {
	t.Helper()
	analyses := []clip.SourceAnalysis{}
	for i, s := range h.batch.Sources {
		info := clip.MediaInfo{DurationMS: h.media.durations[i], Width: s.Width, Height: s.Height}
		analyses = append(analyses, clip.SourceAnalysis{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: info}, Filename: s.Filename}, Segments: []clip.Segment{{EndMS: info.DurationMS, Event: "scene", Quality: "usable", Focal: clip.Point{X: .5, Y: .5}}}})
	}
	s := h.batch.Sources[0]
	p := clip.EditPlan{Ratio: h.project.Ratio, DurationMS: h.project.TargetDurationMS, Cuts: []clip.Cut{{ID: "cut", SourceID: s.ID, Fingerprint: s.Fingerprint, EndMS: h.project.TargetDurationMS, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "서울", Style: "clean", Anchor: "bottom", Align: "center"}}}}}
	encoded, err := clip.EncodeEditPlan(p, h.template.CopyStyles)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(analyses)
	if err != nil {
		t.Fatal(err)
	}
	r := clip.Result{Key: clip.ResultPrefix + "alice/" + h.project.ID + "/fixture.mp4", ContentType: "video/mp4", Bytes: 5, DurationMS: p.DurationMS, CreatedAt: time.Now()}
	if err = h.store.SaveGeneration(context.Background(), "alice", h.project.ID, string(raw), encoded, r); err != nil {
		t.Fatal(err)
	}
	h.objects.info[r.Key] = clip.SourceObjectInfo{Bytes: 5, ContentType: "video/mp4"}
}

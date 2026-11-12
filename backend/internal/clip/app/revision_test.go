package app

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func revisionPricingService() *GenerationService {
	pricing := NewPricing(&fakeFreezer{}, Budgets{ObserveCompletionTokens: 8192, FlowCompletionTokens: 32768, NarrationCompletionTokens: 32768}, &changingRates{rate: testQuoteRate})
	return &GenerationService{pricing: pricing}
}

// An auto request is approved at the two writing calls a flow target prices, the most it
// may need, and never at the one a narration target prices (CLIP-132).
func TestAnAutoRevisionIsApprovedAtTwoWritingCalls(t *testing.T) {
	s := revisionPricingService()
	priced := map[string]clip.GenerationPricing{}
	for _, target := range []string{clip.RevisionAuto, clip.RevisionFlow, clip.RevisionNarration} {
		p, err := s.revisionPricing(t.Context(), "p/w", "p/o", target, clip.VideoGuidelines{}, nil)
		if err != nil {
			t.Fatal(target, err)
		}
		priced[target] = p
	}
	auto, flow, narration := priced[clip.RevisionAuto], priced[clip.RevisionFlow], priced[clip.RevisionNarration]
	// Each priced call carries its correction allowance, so a call counts 1 + retries attempts.
	if auto.FlowCalls() == 0 || auto.NarrationCalls() == 0 || auto.ObservationCalls != 0 || auto.MaxCredits != flow.MaxCredits || auto.PlanCalls() != flow.PlanCalls() {
		t.Fatal("auto was not priced as the two calls a flow target prices", auto.PlanCalls(), auto.MaxCredits, flow.MaxCredits)
	}
	// A narration-only answer settles one call's confirmed usage, which stays under the
	// approved maximum; nothing here reserves less or refunds on its own (QUOTA-46).
	if narration.FlowCalls() != 0 || 2*narration.PlanCalls() != auto.PlanCalls() || narration.MaxCredits >= auto.MaxCredits {
		t.Fatal("one call is not below the two-call approval", narration.MaxCredits, auto.MaxCredits)
	}
}

// The selected scene is part of what was asked: another scene, or none, is another quote,
// and a scene the quoted plan does not hold is refused (CLIP-131).
func TestARevisionQuoteBindsItsSelectedScene(t *testing.T) {
	p := clip.Project{UserID: "alice", ID: "clip", EditPlan: `{"cuts":[]}`, EditPlanRevision: 3}
	whole := clip.RevisionInputDigest(p, "이 장면만", clip.RevisionAuto, 0, "p/w", clip.GenerationPricing{})
	first := clip.RevisionInputDigest(p, "이 장면만", clip.RevisionAuto, 1, "p/w", clip.GenerationPricing{})
	second := clip.RevisionInputDigest(p, "이 장면만", clip.RevisionAuto, 2, "p/w", clip.GenerationPricing{})
	if whole == first || first == second || whole == second {
		t.Fatal("the digest does not bind the selected scene")
	}
	if clip.RevisionInputDigest(p, "이 장면만", clip.RevisionAuto, 1, "p/w", clip.GenerationPricing{}) != first {
		t.Fatal("the same scene digested differently")
	}
	plan := clip.EditPlan{Cuts: []clip.Cut{{ID: "a"}, {ID: "b"}}}
	for cut, want := range map[int]bool{0: true, 1: true, 2: true, 3: false, -1: false} {
		if clip.ValidRevisionScope(cut, plan) != want {
			t.Fatal("scene", cut, "outside or inside the plan's cut count was judged wrongly")
		}
	}
}

// The request record names auto, and the kinds earlier requests were stored under still
// read (CLIP-133).
func TestAnAutoRevisionIsRecordedAsItsOwnKind(t *testing.T) {
	if clip.RevisionRequestKind(clip.RevisionAuto) != "revision:auto" {
		t.Fatal(clip.RevisionRequestKind(clip.RevisionAuto))
	}
	for _, kind := range []string{"revision:auto", "revision:flow", "revision:narration", "revision:both", clip.RequestInstruction, clip.RequestStoryline} {
		if !clip.ValidRequestKind(kind) {
			t.Fatal("a stored kind no longer parses", kind)
		}
	}
	if clip.ValidRequestKind("revision:everything") || clip.ValidRequestKind("revision:") {
		t.Fatal("an undeclared revision kind parses")
	}
}

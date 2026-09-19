package app

import (
	"context"
	"errors"
	"sync"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// admissionRefusal names the model on an admission failure and leaves every
// other error alone.
func admissionRefusal(model llm.ModelRef, err error) error {
	if _, ok := clip.EligibilityOf(err); !ok {
		return err
	}
	var named *clip.ModelAdmissionError
	if errors.As(err, &named) {
		return err
	}
	return &clip.ModelAdmissionError{Model: model, Err: err}
}

// eligibilityConcurrency bounds how many models are qualified at once; the
// adapter bounds its own document reads underneath.
const eligibilityConcurrency = 4

// ListAnalysisEligibility answers CLIP-44 for every registered observe model,
// in registry order, without a model call or a write. A model without video
// input is answered from the catalog alone; every other model is qualified
// against its current endpoint document. A qualification that fails for a
// reason other than admission (a disabled provider, a read that could not
// complete) has no current proof and reports the endpoint as unavailable; only
// the caller's own cancellation propagates.
func (s *GenerationService) ListAnalysisEligibility(ctx context.Context) ([]clip.ModelEligibility, error) {
	if s.admission == nil {
		return nil, clip.ErrPricingUnavailable
	}
	candidates := s.admission.ObserveModels()
	out := make([]clip.ModelEligibility, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, eligibilityConcurrency)
	for i, c := range candidates {
		out[i] = clip.ModelEligibility{Model: c.Ref, Status: clip.EligibilityVideoInputAbsent}
		if !c.VideoInput {
			continue
		}
		wg.Add(1)
		go func(i int, ref llm.ModelRef) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				out[i].Status = clip.EligibilityInlineEndpointUnavailable
				return
			}
			err := s.admission.QualifyObserve(ctx, ref)
			switch status, ok := clip.EligibilityOf(err); {
			case err == nil:
				out[i].Status = clip.EligibilityEligible
			case ok:
				out[i].Status = status
			default:
				out[i].Status = clip.EligibilityInlineEndpointUnavailable
			}
		}(i, c.Ref)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

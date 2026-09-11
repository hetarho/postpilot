package clip

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/postpilot/backend/internal/llm"
)

// EligibilityStatus is the one stable answer the clip context gives for a
// registered observe model (CLIP-30, CLIP-44): eligible, or exactly one of the
// four reasons it is not. The zero value is not a status — a reader that meets
// it has an unspecified wire value and must refuse it, never read it as
// eligible (see ParseEligibility).
type EligibilityStatus string

const (
	EligibilityEligible                      EligibilityStatus = "eligible"
	EligibilityVideoInputAbsent              EligibilityStatus = "video_input_absent"
	EligibilityInlineEndpointUnavailable     EligibilityStatus = "inline_endpoint_unavailable"
	EligibilityRequiredParametersUnsupported EligibilityStatus = "required_parameters_unsupported"
	EligibilityPriceCeilingUnavailable       EligibilityStatus = "price_ceiling_unavailable"
)

// ParseEligibility accepts the five statuses and nothing else.
func ParseEligibility(value string) (EligibilityStatus, error) {
	switch s := EligibilityStatus(value); s {
	case EligibilityEligible, EligibilityVideoInputAbsent, EligibilityInlineEndpointUnavailable, EligibilityRequiredParametersUnsupported, EligibilityPriceCeilingUnavailable:
		return s, nil
	}
	return "", fmt.Errorf("%w: clip analysis eligibility %q", ErrInvalid, value)
}

// The four registered product reasons (LANG-21, LANG-24). Each carries the
// display-safe `model` parameter and nothing else.
const (
	reasonModelVideoInputAbsent              = "CLIP_MODEL_VIDEO_INPUT_ABSENT"
	reasonModelInlineEndpointUnavailable     = "CLIP_MODEL_INLINE_ENDPOINT_UNAVAILABLE"
	reasonModelRequiredParametersUnsupported = "CLIP_MODEL_REQUIRED_PARAMETERS_UNSUPPORTED"
	reasonModelPriceCeilingUnavailable       = "CLIP_MODEL_PRICE_CEILING_UNAVAILABLE"
)

// FailureReason is the registered product reason for a refusing status; the
// eligible status has none.
func (s EligibilityStatus) FailureReason() string {
	switch s {
	case EligibilityVideoInputAbsent:
		return reasonModelVideoInputAbsent
	case EligibilityInlineEndpointUnavailable:
		return reasonModelInlineEndpointUnavailable
	case EligibilityRequiredParametersUnsupported:
		return reasonModelRequiredParametersUnsupported
	case EligibilityPriceCeilingUnavailable:
		return reasonModelPriceCeilingUnavailable
	}
	return ""
}

// EligibilityOf reads the llm boundary's admission failure into a status. Any
// other error is not an admission answer and reports false.
func EligibilityOf(err error) (EligibilityStatus, bool) {
	var admission *llm.AdmissionError
	if !errors.As(err, &admission) {
		return "", false
	}
	switch admission {
	case llm.ErrVideoInputAbsent:
		return EligibilityVideoInputAbsent, true
	case llm.ErrInlineEndpointUnavailable:
		return EligibilityInlineEndpointUnavailable, true
	case llm.ErrRequiredParametersUnsupported:
		return EligibilityRequiredParametersUnsupported, true
	case llm.ErrPriceCeilingUnavailable:
		return EligibilityPriceCeilingUnavailable, true
	}
	return "", false
}

// ModelEligibility is one row of the read-only eligibility answer.
type ModelEligibility struct {
	Model  llm.ModelRef
	Status EligibilityStatus
}

// AnalysisCandidate is a registered observe model as the registry lists it:
// the ref and the catalog's raw video modality, which is the only admission
// decided without reading an endpoint document.
type AnalysisCandidate struct {
	Ref        llm.ModelRef
	VideoInput bool
}

// AnalysisAdmission is the consumer-owned port behind CLIP-30. ObserveModels
// lists every registered observe model in registry order; QualifyObserve
// freezes the exact bounded analysis request for one of them without a model
// call or a write, returning nil or one of the llm admission failures.
type AnalysisAdmission interface {
	ObserveModels() []AnalysisCandidate
	QualifyObserve(context.Context, llm.ModelRef) error
}

// ModelAdmissionError is an admission failure that knows which model it is
// about, so the product reason can name it. It carries the model ref and the
// fixed llm sentence — never a provider payload, price or key.
type ModelAdmissionError struct {
	Model llm.ModelRef
	Err   error
}

func (e *ModelAdmissionError) Error() string { return "clip analysis model refused: " + e.Err.Error() }
func (e *ModelAdmissionError) Unwrap() error { return e.Err }
func (e *ModelAdmissionError) Failure() llm.Failure {
	status, _ := EligibilityOf(e.Err)
	return llm.Failure{Reason: status.FailureReason(), Params: map[string]string{"model": e.Model.String()}}
}

// admissionRefusal names the model on an admission failure and leaves every
// other error alone.
func admissionRefusal(model llm.ModelRef, err error) error {
	if _, ok := EligibilityOf(err); !ok {
		return err
	}
	var named *ModelAdmissionError
	if errors.As(err, &named) {
		return err
	}
	return &ModelAdmissionError{Model: model, Err: err}
}

// WithAdmission attaches the qualifier the eligibility list reads.
func (s *GenerationService) WithAdmission(admission AnalysisAdmission) *GenerationService {
	s.admission = admission
	return s
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
func (s *GenerationService) ListAnalysisEligibility(ctx context.Context) ([]ModelEligibility, error) {
	if s.admission == nil {
		return nil, ErrPricingUnavailable
	}
	candidates := s.admission.ObserveModels()
	out := make([]ModelEligibility, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, eligibilityConcurrency)
	for i, c := range candidates {
		out[i] = ModelEligibility{Model: c.Ref, Status: EligibilityVideoInputAbsent}
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
				out[i].Status = EligibilityInlineEndpointUnavailable
				return
			}
			err := s.admission.QualifyObserve(ctx, ref)
			switch status, ok := EligibilityOf(err); {
			case err == nil:
				out[i].Status = EligibilityEligible
			case ok:
				out[i].Status = status
			default:
				out[i].Status = EligibilityInlineEndpointUnavailable
			}
		}(i, c.Ref)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

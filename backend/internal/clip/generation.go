package clip

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

var ErrBusy = errors.New("clip busy")

// This is the durable application snapshot, not the public project projection. No URL
// or source byte is serialized. Model choices and every recipe input are frozen here.
// GenerationPayloadVersion is bumped whenever the frozen shape changes, and
// every reader checks it: an approval frozen under an older shape is refused
// rather than run with fields it never carried. Version 3 added the disclosure
// and the resolved CTA, so a job approved before the owner could choose a
// campaign type cannot render a clip that carries one. Version 4 freezes the
// project composition. Version 5 freezes the observation language; accepted
// version-3/4 jobs retain Korean, the legacy project language.
const GenerationPayloadVersion = 5

type GenerationPayload struct {
	Language                         string
	Recovery                         *RecoveryState
	Composition                      *ProjectComposition
	Version                          int
	ProjectID, Ratio, Observe, Write string
	TargetDurationMS                 int
	Template                         Recipe
	Answers                          []Answer
	// Frozen with the approval, like the template and the answers: the badge's
	// campaign type and the resolved closing CTA. Version 3 exists because of
	// them — a job approved before the disclosure was a choice cannot render a
	// clip that carries one, and is refused rather than rendered without it.
	Disclosure, CTA string
	// The project's own instruction (CLIP-121), frozen with the answers and the
	// composition so editing it mid-flight changes nothing in flight. A payload
	// written before it existed decodes as none, which is today's behaviour.
	Instruction string
	// The project's caption pace and accent (CLIP-139), frozen with the rest of
	// the render inputs. They are deliberately absent from planRecoveryDigest
	// and from the quote: changing either re-renders the same plan and costs no
	// writing call. A payload written before they existed decodes as empty,
	// which is the frozen document's own value — today's behaviour.
	CaptionPace, Accent string
	// The rest of the project's design selection (CLIP-139, CLIP-142), frozen
	// the same way and absent from the digest for the same reason. A payload
	// written before it moved onto the project decodes as empty, which reads as
	// the shared defaults -- today's behaviour.
	IntroPreset, OutroPreset string
	CaptionStyles            []string
	HideDisclosure           bool
	Batch                    SourceBatch
	Approval                 *GenerationApproval
}

// design is the selection this frozen run renders with, in one value so a run
// and a rerender cannot read it differently.
func (p GenerationPayload) Design() ProjectDesign {
	return ProjectDesign{CaptionPace: p.CaptionPace, Accent: p.Accent, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, CaptionStyles: p.CaptionStyles}
}

type StageFailure struct {
	Stage string
	Cause error
}

// The worker logs Error(), so redaction must apply here as well as to the public
// Failure projection. Unwrap retains the original cause for classification/tests.
func (e *StageFailure) Error() string        { return fmt.Sprintf("clip %s: %s", e.Stage, e.Failure().Reason) }
func (e *StageFailure) Unwrap() error        { return e.Cause }
func (e *StageFailure) FailureStage() string { return e.Stage }
func (e *StageFailure) Failure() llm.Failure {
	f := llm.NormalizeFailure(e.Cause)
	var credits *plan.InsufficientCreditsError
	var layout interface{ LayoutReason() string }
	var facts *MissingFactsError
	var admission *ModelAdmissionError
	var element *composition.Problem
	var media *MediaStageFailure
	switch {
	case errors.As(e.Cause, &media):
		reason := "CLIP_PROCESSING_FAILED"
		switch media.Code {
		case MediaFailureWaitExpired:
			reason = "CLIP_MEDIA_UNAVAILABLE"
		case MediaFailureAttemptsExhausted:
			reason = "CLIP_MEDIA_RETRY_EXHAUSTED"
		case MediaFailureDeadlineExceeded:
			reason = "CLIP_MEDIA_TIMEOUT"
		case MediaFailureWorkspaceLimit:
			reason = "CLIP_WORKSPACE_LIMIT"
		case MediaFailureInputTooLarge:
			reason = "CLIP_INPUT_TOO_LARGE"
		case MediaFailureAnalysisTooLarge:
			reason = "CLIP_ANALYSIS_TOO_LARGE"
		case MediaFailureInvalidInput:
			reason = "CLIP_INVALID_MEDIA"
		case MediaFailureInvalidOutput:
			reason = "CLIP_PROCESSING_FAILED"
		}
		f = llm.Failure{Reason: reason}
	case errors.As(e.Cause, &element):
		f = llm.Failure{Reason: "CLIP_COMPOSITION_INVALID", Params: element.FailureParams()}
	case errors.Is(e.Cause, ErrCompositionUnavailable):
		f = llm.Failure{Reason: "CLIP_COMPOSITION_UNAVAILABLE"}
	// The model's own admission answer comes first: it unwraps to the generic
	// unsupported error, which must not swallow the specific reason.
	case errors.As(e.Cause, &admission):
		f = admission.Failure()
	case errors.Is(e.Cause, ErrQuoteRequired):
		f = llm.Failure{Reason: "CLIP_QUOTE_REQUIRED"}
	case errors.Is(e.Cause, ErrQuoteExpired):
		f = llm.Failure{Reason: "CLIP_QUOTE_EXPIRED"}
	case errors.Is(e.Cause, ErrQuoteChanged):
		f = llm.Failure{Reason: "CLIP_QUOTE_CHANGED"}
	case errors.Is(e.Cause, ErrPricingUnavailable):
		f = llm.Failure{Reason: "CLIP_MODEL_PRICING_UNAVAILABLE"}
	case errors.Is(e.Cause, ErrWorkspaceLimit):
		f = llm.Failure{Reason: "CLIP_WORKSPACE_LIMIT"}
	case errors.Is(e.Cause, ErrInputTooLarge):
		f = llm.Failure{Reason: "CLIP_INPUT_TOO_LARGE"}
	case errors.Is(e.Cause, ErrAnalysisTooLarge):
		f = llm.Failure{Reason: "CLIP_ANALYSIS_TOO_LARGE"}
	case errors.Is(e.Cause, ErrModelInputUnsupported):
		f = llm.Failure{Reason: "CLIP_MODEL_INPUT_UNSUPPORTED"}
	case errors.As(e.Cause, &credits):
		f = llm.Failure{Reason: "INSUFFICIENT_CREDITS", Params: map[string]string{"required": fmt.Sprint(credits.Required), "balance": fmt.Sprint(credits.Balance), "renews_at": credits.RenewsAt.Format(time.RFC3339)}}
	case errors.Is(e.Cause, ErrInvalidMedia):
		f = llm.Failure{Reason: "CLIP_INVALID_MEDIA", TechnicalDetail: "Source verification failed before analysis."}
	case errors.Is(e.Cause, ErrDisclosureRequired):
		f = llm.Failure{Reason: reasonDisclosureRequired}
	case errors.Is(e.Cause, ErrTargetDurationRequired):
		f = llm.Failure{Reason: reasonTargetDuration}
	case errors.As(e.Cause, &facts):
		f = llm.Failure{Reason: reasonFactsRequired, Params: map[string]string{"labels": strings.Join(facts.Labels, ", ")}}
	case errors.Is(e.Cause, ErrCopyTooLong):
		f = llm.Failure{Reason: "CLIP_COPY_TOO_LONG"}
	// A plan that was read and validated but cannot fill the floor is the
	// footage's shortfall, not the model's (CLIP-120).
	case errors.Is(e.Cause, ErrInsufficientFootage):
		f = llm.Failure{Reason: reasonInsufficientFootage}
	// One reason per verifier check, so the correction step can point at what
	// failed instead of saying the plan is invalid (CDS-52, LANG-21).
	case errors.As(e.Cause, &layout) && layout.LayoutReason() != "":
		f = llm.Failure{Reason: layout.LayoutReason()}
	case errors.Is(e.Cause, ErrInvalid):
		f = llm.Failure{Reason: "CLIP_INVALID_INPUT"}
	case errors.Is(e.Cause, ErrPlanConflict):
		f = llm.Failure{Reason: "CLIP_PLAN_CONFLICT"}
	case errors.Is(e.Cause, ErrSourceState), errors.Is(e.Cause, ErrNotFound):
		f = llm.Failure{Reason: "CLIP_SOURCE_UNAVAILABLE"}
	}
	if f.Reason == "UNKNOWN_FAILURE" {
		f = llm.Failure{Reason: "CLIP_PROCESSING_FAILED", TechnicalDetail: "Clip processing failed in stage " + e.Stage + "."}
	}
	if f.Params == nil {
		f.Params = map[string]string{}
	}
	// The durable job already owns Stage. Adding it to reason-specific params
	// would violate the frontend's strict failure allowlist (including credits).
	return f
}

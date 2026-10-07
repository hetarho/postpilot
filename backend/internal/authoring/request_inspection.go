package authoring

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

type RequestInspectionInput struct {
	SessionID      string
	Kind           Kind
	Revision       uint32
	Mode           Mode
	Prompt         string
	Model          llm.ModelRef
	CandidateCount int
	Status         llm.InspectionStatus
}

type RequestInspectionModels interface {
	PrepareAuthoringRequest(context.Context, string, llm.ModelRef, llm.Request) (llm.RequestInspection, error)
}
type RequestInspectionSelections interface {
	ModelForInspection(context.Context, string, string) (llm.ModelRef, bool, error)
}
type RequestCapture struct {
	UserID, SessionID, OperationID, JobID string
	Kind                                  Kind
	BaseRevision                          uint32
	Inspection                            llm.RequestInspection
}
type RequestInspectionSnapshot struct {
	Session  Session
	Active   *Operation
	Captured *llm.RequestInspection
}
type RequestCaptureStore interface {
	ReadAuthoringInspectionSnapshot(context.Context, string, string, Kind, uint32, Mode) (RequestInspectionSnapshot, error)
	WriteAuthoringRequestCapture(context.Context, RequestCapture) error
	PurgeAuthoringRequestCaptures(context.Context, string, string) error
}
type RequestInspectionDependencies struct {
	Captures   RequestCaptureStore
	Models     RequestInspectionModels
	Selections RequestInspectionSelections
}

// The ordinary unconfigured service remains legal; production opts into private
// evidence with all behavior ports explicitly supplied by the composition root.
func NewInspectedService(s *Service, dependencies RequestInspectionDependencies) *Service {
	if s == nil || dependencies.Captures == nil || dependencies.Models == nil || dependencies.Selections == nil {
		panic("authoring: request inspection ports are required")
	}
	s.inspection = &dependencies
	return s
}

func unavailableAuthoring(kind Kind, mode Mode, reason string) llm.RequestInspection {
	in := llm.UnavailableRequestInspection("setting-authoring", string(kind)+"/"+string(mode))
	in.UnavailableReason = reason
	return in
}

// InspectRequest reads a single private snapshot directly. In particular it does
// not use Get, which is a recovery action that can reconcile a completed job.
func (s *Service) InspectRequest(ctx context.Context, owner string, input RequestInspectionInput) (llm.RequestInspection, error) {
	if s.inspection == nil {
		return unavailableAuthoring(input.Kind, input.Mode, "Request inspection is not configured."), nil
	}
	snapshot, err := s.inspection.Captures.ReadAuthoringInspectionSnapshot(ctx, owner, input.SessionID, input.Kind, input.Revision, input.Mode)
	if err != nil {
		return llm.RequestInspection{}, err
	}
	state := snapshot.Session
	if !input.Mode.Valid() || input.Status != llm.InspectionCurrent && input.Status != llm.InspectionPrepared && input.Status != llm.InspectionCaptured {
		return llm.RequestInspection{}, llm.ErrInvalidInspection
	}
	if input.Status == llm.InspectionCaptured {
		if snapshot.Captured == nil {
			return unavailableAuthoring(state.Kind, input.Mode, "No matching captured request exists for this session revision."), nil
		}
		return *snapshot.Captured, nil
	}
	if !utf8.ValidString(input.Prompt) || utf8.RuneCountInString(input.Prompt) > MaxPromptChars {
		return llm.RequestInspection{}, ErrInvalid
	}
	if _, err = NormalizeCandidateCount(input.CandidateCount); err != nil {
		return llm.RequestInspection{}, err
	}
	ref := input.Model
	var frozen operationInput
	if snapshot.Active != nil && snapshot.Active.Mode == input.Mode {
		if err = json.Unmarshal(snapshot.Active.Payload, &frozen); err != nil || !receiptMatches(snapshot.Active.Payload, *snapshot.Active) || frozen.Kind != state.Kind || frozen.Mode != input.Mode || frozen.CompletionTokens <= 0 || !utf8.ValidString(frozen.Prompt) || utf8.RuneCountInString(frozen.Prompt) > MaxPromptChars {
			return unavailableAuthoring(state.Kind, input.Mode, "The active private request payload is unavailable."), nil
		}
		// An admitted request is immutable. A prospective edit of it is not the
		// request associated with this active session revision.
		if input.Prompt != "" && input.Prompt != frozen.Prompt || input.CandidateCount != 0 && normalizedCount(input.CandidateCount) != normalizedCount(frozen.CandidateCount) {
			return unavailableAuthoring(state.Kind, input.Mode, "An active request already freezes different material."), nil
		}
		if snapshot.Active.JobID != "" {
			job, e := s.jobs.Get(ctx, owner, snapshot.Active.JobID)
			if e != nil {
				return unavailableAuthoring(state.Kind, input.Mode, "The active model selection is unavailable."), nil
			}
			provider, model, ok := strings.Cut(job.WriteModel, "/")
			if !ok {
				return unavailableAuthoring(state.Kind, input.Mode, "The active model selection is unavailable."), nil
			}
			activeRef := llm.ModelRef{ProviderID: provider, ModelID: model}
			if ref != (llm.ModelRef{}) && ref != activeRef {
				return unavailableAuthoring(state.Kind, input.Mode, "The active request freezes a different model."), nil
			}
			ref = activeRef
		}
	} else {
		if state.ActiveRequestID != "" {
			return unavailableAuthoring(state.Kind, input.Mode, "A different authoring mode is active."), nil
		}
		if input.Status == llm.InspectionPrepared && state.Phase == "saving" || input.Status == llm.InspectionPrepared && state.Phase == "saved" {
			return unavailableAuthoring(state.Kind, input.Mode, "This revision is already being published or saved."), nil
		}
		if input.Mode == Refine && currentSource(state) == nil {
			return unavailableAuthoring(state.Kind, input.Mode, "Refinement requires a current draft."), nil
		}
		if input.Mode == Refine && input.Status == llm.InspectionPrepared && strings.TrimSpace(input.Prompt) == "" {
			return unavailableAuthoring(state.Kind, input.Mode, "A prepared refinement requires an explicit owner instruction."), nil
		}
	}
	if ref == (llm.ModelRef{}) {
		var found bool
		ref, found, err = s.inspection.Selections.ModelForInspection(ctx, owner, llm.StageNameWrite)
		if err != nil {
			return llm.RequestInspection{}, err
		}
		if !found {
			return unavailableAuthoring(state.Kind, input.Mode, "No eligible writing model is selected."), nil
		}
	}
	info, err := s.eligible(ref)
	if err != nil {
		return unavailableAuthoring(state.Kind, input.Mode, "The writing model cannot prepare this authoring request."), nil
	}
	if snapshot.Active == nil {
		frozen, err = s.freezeInput(state, Operation{}, Start{Mode: input.Mode, Prompt: input.Prompt, RequestedCandidateCount: input.CandidateCount}, info)
		if err != nil {
			return unavailableAuthoring(state.Kind, input.Mode, "The current private material cannot be prepared."), nil
		}
	}
	request := prepareAuthoringRequest(frozen, info)
	out, err := s.inspection.Models.PrepareAuthoringRequest(ctx, owner, ref, request)
	if err != nil {
		return unavailableAuthoring(state.Kind, input.Mode, "The selected model cannot prepare these conditions."), nil
	}
	if out.Status == llm.InspectionUnavailable {
		return unavailableAuthoring(state.Kind, input.Mode, "No safe request projection is available."), nil
	}
	out.Status = input.Status
	out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "execution-admission", Reason: "This unissued view makes no live provider, credit or hold admission claim. Execution checks happen only when the owner starts work.", Activation: "current configuration or prepared preview", SourceFiles: []string{"internal/authoring/request_inspection.go"}})
	if snapshot.Active != nil {
		out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "active-job-execution-options", Reason: "This preview preserves the active request material and uses currently configured model conditions. The job's frozen admission options are not claimed by this unissued view; captured evidence records the actual effective invocation.", Activation: "active frozen request material", SourceFiles: []string{"internal/authoring/request_inspection.go"}})
	}
	if snapshot.Active == nil && input.Mode == Refine && input.Prompt == "" {
		out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "refinement-instruction", Reason: "Current configuration contains no prospective owner instruction.", Activation: "current configuration only"})
	}
	return out, out.Validate()
}

func prepareAuthoringRequest(in operationInput, info llm.ModelInfo) llm.Request {
	request := llm.Request{Composition: authoringComposition(in), System: systemMessage(in), Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(modelMessage(in))}}}, Stage: llm.StageNameWrite, Reasoning: llm.ReasoningLow, MaxTokens: in.CompletionTokens}
	if info.StructuredOutput {
		request.JSONSchema = kindResponseSchema(in.Kind, in.Mode, in.CandidateCount)
	}
	return request
}

func (s *Service) captureAuthoringRequest(ctx context.Context, run Run, in operationInput, response llm.Response, callErr error) {
	if s.inspection == nil {
		return
	}
	out := response.Inspection
	if witness, found := llm.RequestInspectionFromError(callErr); found {
		out = &witness
	}
	witness := unavailableAuthoring(in.Kind, in.Mode, "The issued call has no valid safe request witness.")
	if out != nil && out.Status == llm.InspectionCaptured && out.Validate() == nil && out.Stage == "setting-authoring" && out.Mode == string(in.Kind)+"/"+string(in.Mode) {
		witness = *out
		witness.CallID = run.ID
	}
	// A canceled issued call may still produce a private witness. The owner and
	// operation purge/revision fences are rechecked atomically by the writer.
	_ = s.inspection.Captures.WriteAuthoringRequestCapture(context.WithoutCancel(ctx), RequestCapture{UserID: run.UserID, SessionID: in.SessionID, OperationID: in.OperationID, JobID: run.ID, Kind: in.Kind, BaseRevision: in.BaseRevision, Inspection: witness})
}

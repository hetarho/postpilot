package rpc

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice/spoken"
	spokenapp "github.com/postpilot/backend/internal/voice/spoken/app"
	"time"
)

type GenerationHandler struct{ svc *spokenapp.GenerationService }

var _ postpilotv1connect.SpokenVoiceGenerationServiceHandler = (*GenerationHandler)(nil)

func NewGenerationHandler(s *spokenapp.GenerationService) *GenerationHandler {
	return &GenerationHandler{s}
}
func workFailure(err error) error {
	if access, ok := usage.ModelAccessFailure(err); ok {
		return rpcserver.AppErrorFrom(connect.CodeFailedPrecondition, access)
	}
	var credits *plan.InsufficientCreditsError
	if errors.As(err, &credits) {
		return rpcserver.AppErrorFrom(connect.CodeResourceExhausted, credits)
	}
	if errors.Is(err, spokenapp.ErrQualificationOnly) {
		return connect.NewError(connect.CodePermissionDenied, errors.New("owned qualification session required"))
	}
	if errors.Is(err, usage.ErrUnitApproval) || errors.Is(err, usage.ErrUnitPricing) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("voice estimate changed or unavailable; request a new estimate"))
	}
	return failure(err)
}
func workApproval(p *v1.SpokenWorkApproval) usage.UnitApproval {
	a := usage.UnitApproval{QuoteID: p.GetQuoteId(), CancellationPolicyVersion: int(p.GetCancellationPolicyVersion())}
	if p != nil && p.ApprovedMaxCredits != nil {
		n := int(*p.ApprovedMaxCredits)
		a.ApprovedMaxCredits = &n
	}
	return a
}
func designInput(p *v1.SpokenDesignQuoteRequest) spokenapp.GenerationInput {
	return spokenapp.GenerationInput{Kind: spoken.JobKindDesign, DraftID: p.GetDraftId(), Revision: p.GetExpectedRevision()}
}
func confirmationInput(p *v1.SpokenConfirmationQuoteRequest) spokenapp.GenerationInput {
	return spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: p.GetDraftId(), Revision: p.GetExpectedRevision(), CandidateID: p.GetCandidateId()}
}
func probeInput(p *v1.SpokenProbeQuoteRequest) spokenapp.GenerationInput {
	return spokenapp.GenerationInput{Kind: spoken.JobKindProbe, VoiceID: p.GetVoiceId(), Revision: p.GetExpectedRevision(), QualificationSessionID: p.GetQualificationSessionId(), Texts: [2]string{p.GetFirstText(), p.GetSecondText()}}
}
func (h *GenerationHandler) quote(ctx context.Context, in spokenapp.GenerationInput) (*connect.Response[v1.SpokenWorkQuote], error) {
	owner, tier, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	q, err := h.svc.Quote(ctx, owner, tier, in)
	if err != nil {
		return nil, workFailure(err)
	}
	expires := ""
	if !q.ExpiresAt.IsZero() {
		expires = q.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	return connect.NewResponse(&v1.SpokenWorkQuote{QuoteId: q.ID, MaximumCredits: int32(q.MaximumCredits), ExpiresAt: expires, ApprovalRequired: q.ApprovalRequired, ExistingVoiceId: q.ExistingVoiceID}), nil
}
func (h *GenerationHandler) response(ctx context.Context, o spoken.Operation, err error) (*connect.Response[v1.SpokenOperationResponse], error) {
	if err != nil {
		return nil, workFailure(err)
	}
	out := &v1.SpokenOperation{Id: o.ID, Kind: o.Kind, State: o.State, JobId: o.JobID, DraftId: o.DraftID, VoiceId: o.VoiceID, CandidateId: o.CandidateID, ResultId: o.ResultID, FailureReason: o.FailureReason}
	if !o.CreatedAt.IsZero() {
		out.CreatedAt = o.CreatedAt.UTC().Format(time.RFC3339Nano)
		out.UpdatedAt = o.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	if o.Kind == spoken.JobKindProbe {
		for i, id := range o.AssetIDs {
			if id == "" {
				continue
			}
			a, e := h.svc.Operations.GetAsset(ctx, o.OwnerID, id)
			if e != nil {
				return nil, workFailure(e)
			}
			p, e := h.svc.Operations.GetProbeAudio(ctx, o.OwnerID, o.VoiceID, o.SpeechInputs[i])
			if e != nil {
				return nil, workFailure(e)
			}
			out.ProbeSamples = append(out.ProbeSamples, &v1.SpokenProbeSample{AssetId: id, DurationMs: a.DurationMS(), HasCharacterTiming: len(p.Timing) > 0})
		}
	}
	return connect.NewResponse(&v1.SpokenOperationResponse{Operation: out}), nil
}
func (h *GenerationHandler) start(ctx context.Context, in spokenapp.GenerationInput, key string, a *v1.SpokenWorkApproval) (*connect.Response[v1.SpokenOperationResponse], error) {
	owner, tier, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	o, err := h.svc.Start(ctx, owner, tier, in, key, workApproval(a))
	return h.response(ctx, o, err)
}
func (h *GenerationHandler) QuoteVoiceCandidates(ctx context.Context, r *connect.Request[v1.SpokenDesignQuoteRequest]) (*connect.Response[v1.SpokenWorkQuote], error) {
	return h.quote(ctx, designInput(r.Msg))
}
func (h *GenerationHandler) StartVoiceCandidates(ctx context.Context, r *connect.Request[v1.SpokenDesignStartRequest]) (*connect.Response[v1.SpokenOperationResponse], error) {
	return h.start(ctx, designInput(r.Msg.Input), r.Msg.IdempotencyKey, r.Msg.Approval)
}
func (h *GenerationHandler) QuoteVoiceConfirmation(ctx context.Context, r *connect.Request[v1.SpokenConfirmationQuoteRequest]) (*connect.Response[v1.SpokenWorkQuote], error) {
	return h.quote(ctx, confirmationInput(r.Msg))
}
func (h *GenerationHandler) StartVoiceConfirmation(ctx context.Context, r *connect.Request[v1.SpokenConfirmationStartRequest]) (*connect.Response[v1.SpokenOperationResponse], error) {
	return h.start(ctx, confirmationInput(r.Msg.Input), r.Msg.IdempotencyKey, r.Msg.Approval)
}
func (h *GenerationHandler) QuoteVoiceReuseProbe(ctx context.Context, r *connect.Request[v1.SpokenProbeQuoteRequest]) (*connect.Response[v1.SpokenWorkQuote], error) {
	return h.quote(ctx, probeInput(r.Msg))
}
func (h *GenerationHandler) StartVoiceReuseProbe(ctx context.Context, r *connect.Request[v1.SpokenProbeStartRequest]) (*connect.Response[v1.SpokenOperationResponse], error) {
	return h.start(ctx, probeInput(r.Msg.Input), r.Msg.IdempotencyKey, r.Msg.Approval)
}
func (h *GenerationHandler) GetSpokenOperation(ctx context.Context, r *connect.Request[v1.SpokenIDRequest]) (*connect.Response[v1.SpokenOperationResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	o, err := h.svc.Get(ctx, owner, r.Msg.Id)
	return h.response(ctx, o, err)
}
func (h *GenerationHandler) CancelSpokenOperation(ctx context.Context, r *connect.Request[v1.SpokenIDRequest]) (*connect.Response[v1.SpokenOperationResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	o, err := h.svc.Cancel(ctx, owner, r.Msg.Id)
	return h.response(ctx, o, err)
}
func (h *GenerationHandler) RetrySpokenPublication(ctx context.Context, r *connect.Request[v1.SpokenIDRequest]) (*connect.Response[v1.SpokenOperationResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	o, err := h.svc.RetryPublication(ctx, owner, r.Msg.Id)
	return h.response(ctx, o, err)
}

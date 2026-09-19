package app

import (
	"context"
	"encoding/json"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) Accounting(ctx context.Context, user, id string) (*clip.Accounting, error) {
	if _, err := s.projects.store.GetProject(ctx, user, id); err != nil {
		return nil, err
	}
	r, ok := s.jobs.(generationJobReader)
	if !ok {
		return nil, nil
	}
	j, err := r.Latest(ctx, user, id)
	if err != nil || j == nil {
		return nil, err
	}
	return s.accountingForJob(ctx, user, id, j)
}

type clipJobSnapshotReader interface {
	Snapshot(context.Context, string, string, string) (*clip.ClipJob, error)
}

func (s *GenerationService) AccountingForAttempt(ctx context.Context, user, project, id string) (*clip.Accounting, error) {
	if _, err := s.projects.store.GetProject(ctx, user, project); err != nil {
		return nil, err
	}
	r, ok := s.jobs.(clipJobSnapshotReader)
	if !ok {
		return nil, clip.ErrNotFound
	}
	j, err := r.Snapshot(ctx, user, project, id)
	if err != nil || j == nil {
		return nil, err
	}
	return s.accountingForJob(ctx, user, project, j)
}

// chargedClipKind names the clip work that reserves credits against an approved
// ceiling: the generation and the owner's revision request (CLIP-19, CLIP-132).
// A render spends none and has nothing to disclose.
func chargedClipKind(kind string) bool {
	return kind == "generate_clip" || kind == "revise_clip"
}

// chargedApproval reads the approval a charged job froze into its own payload.
// The two kinds freeze different documents — a generation's inputs, a revision's
// request — so each is read under its own rules rather than one shape being
// forced onto both.
func chargedApproval(kind string, payload []byte, user, id string) (clip.GenerationApproval, bool) {
	switch kind {
	case "generate_clip":
		var p clip.GenerationPayload
		if json.Unmarshal(payload, &p) != nil || !clip.SupportedGenerationPayload(p.Version) || p.ProjectID != id || p.Batch.UserID != user || p.Approval == nil || p.Approval.MaxCredits < 0 {
			return clip.GenerationApproval{}, false
		}
		return *p.Approval, true
	case "revise_clip":
		var p revisionJobPayload
		if json.Unmarshal(payload, &p) != nil || p.Version != revisionPayloadVersion || p.ProjectID != id || p.Batch.UserID != user || p.Approval == nil || p.Approval.MaxCredits < 0 {
			return clip.GenerationApproval{}, false
		}
		return *p.Approval, true
	}
	return clip.GenerationApproval{}, false
}

func (s *GenerationService) accountingForJob(ctx context.Context, user, id string, j *clip.ClipJob) (*clip.Accounting, error) {
	if !chargedClipKind(j.Kind) {
		return nil, nil
	}
	out := &clip.Accounting{JobID: j.ID, Status: "unavailable"}
	approval, ok := chargedApproval(j.Kind, j.Payload, user, id)
	if !ok {
		return out, nil
	}
	approved := approval.MaxCredits
	out.ApprovedMax = &approved
	if s.accounting == nil {
		return out, nil
	}
	ledger, err := s.accounting.ForJob(ctx, user, j.ID)
	if err != nil {
		return nil, err
	}
	terminal := j.Status == "done" || j.Status == "failed" || j.Status == "cancelled"
	if ledger == nil {
		zero := 0
		out.Status, out.Reserved = "not_reserved", &zero
		if terminal {
			out.FinalCharge, out.Refund, out.Settled = &zero, &zero, true
			if approval.Pricing.CancellationPolicyVersion != 0 {
				out.NominalReservation, out.ConfirmedCharge, out.CancellationFee = &zero, &zero, &zero
				out.CancellationPolicyVersion = approval.Pricing.CancellationPolicyVersion
				out.SettlementReason = j.Status
				if j.Status == "done" {
					out.SettlementReason = "succeeded"
				}
			}
		}
		return out, nil
	}
	if ledger.ApprovedMax == nil || *ledger.ApprovedMax != approved {
		return out, nil
	}
	ledger.JobID = j.ID
	switch {
	case ledger.Exempt:
		ledger.Status = "exempt"
	case ledger.Settled:
		ledger.Status = "settled"
	case terminal:
		ledger.Status = "settling"
	default:
		ledger.Status = "reserved"
	}
	return ledger, nil
}

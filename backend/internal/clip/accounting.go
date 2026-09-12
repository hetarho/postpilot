package clip

import (
	"context"
	"encoding/json"
)

type Accounting struct {
	CancellationPolicyVersion                                int
	SettlementReason                                         string
	NominalReservation, ConfirmedCharge, CancellationFee     *int
	ShadowConfirmedCharge, ShadowCancellationFee             *int
	JobID, Status                                            string
	ApprovedMax, Reserved, FinalCharge, Refund, ShadowCharge *int
	Exempt, Settled                                          bool
}
type AccountingReader interface {
	ForJob(context.Context, string, string) (*Accounting, error)
}

func (s *GenerationService) Accounting(ctx context.Context, user, id string) (*Accounting, error) {
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
	Snapshot(context.Context, string, string, string) (*ClipJob, error)
}

func (s *GenerationService) AccountingForAttempt(ctx context.Context, user, project, id string) (*Accounting, error) {
	if _, err := s.projects.store.GetProject(ctx, user, project); err != nil {
		return nil, err
	}
	r, ok := s.jobs.(clipJobSnapshotReader)
	if !ok {
		return nil, ErrNotFound
	}
	j, err := r.Snapshot(ctx, user, project, id)
	if err != nil || j == nil {
		return nil, err
	}
	return s.accountingForJob(ctx, user, project, j)
}
func (s *GenerationService) accountingForJob(ctx context.Context, user, id string, j *ClipJob) (*Accounting, error) {
	if j.Kind != "generate_clip" {
		return nil, nil
	}
	out := &Accounting{JobID: j.ID, Status: "unavailable"}
	var p generationPayload
	if json.Unmarshal(j.Payload, &p) != nil || p.Version != generationPayloadVersion || p.ProjectID != id || p.Batch.UserID != user || p.Approval == nil || p.Approval.MaxCredits < 0 {
		return out, nil
	}
	approved := p.Approval.MaxCredits
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
			if p.Approval.Pricing.CancellationPolicyVersion != 0 {
				out.NominalReservation, out.ConfirmedCharge, out.CancellationFee = &zero, &zero, &zero
				out.CancellationPolicyVersion = p.Approval.Pricing.CancellationPolicyVersion
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

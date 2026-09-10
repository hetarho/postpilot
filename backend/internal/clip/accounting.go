package clip

import (
	"context"
	"encoding/json"
)

type Accounting struct {
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
	if j.Kind != "generate_clip" {
		return nil, nil
	}
	out := &Accounting{JobID: j.ID, Status: "unavailable"}
	var p generationPayload
	if json.Unmarshal(j.Payload, &p) != nil || p.Version != 2 || p.ProjectID != id || p.Batch.UserID != user || p.Approval == nil || p.Approval.MaxCredits < 0 {
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
	terminal := j.Status == "done" || j.Status == "failed"
	if ledger == nil {
		zero := 0
		out.Status, out.Reserved = "not_reserved", &zero
		if terminal {
			out.FinalCharge, out.Refund, out.Settled = &zero, &zero, true
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

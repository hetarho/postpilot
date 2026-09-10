package clip

import "encoding/json"

// AttemptIdentity survives transient source cleanup without exposing source
// metadata or bytes. The caller has already obtained this owned durable job.
type AttemptIdentity struct{ JobID, BatchID, QuoteID string }

func IdentifyAttempt(user, project, job, kind string, payload []byte) *AttemptIdentity {
	var p struct {
		Version   int
		ProjectID string
		Batch     SourceBatch
		Approval  *GenerationApproval
	}
	if json.Unmarshal(payload, &p) != nil || job == "" || p.ProjectID != project || p.Batch.ProjectID != project || p.Batch.UserID != user || p.Batch.ID == "" {
		return nil
	}
	a := &AttemptIdentity{JobID: job, BatchID: p.Batch.ID}
	switch kind {
	case "generate_clip":
		if p.Version != 2 || p.Approval == nil || p.Approval.QuoteID == "" {
			return nil
		}
		a.QuoteID = p.Approval.QuoteID
	case "render_clip":
		if p.Version != 1 {
			return nil
		}
	default:
		return nil
	}
	return a
}

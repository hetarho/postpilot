package clip

import (
	"context"
	"errors"
	"time"
)

var (
	ErrFinalized            = errors.New("clip project is finalized")
	ErrFinalizationConflict = errors.New("clip finalization revision or result changed")
	ErrFinalizationInvalid  = errors.New("clip has no valid matching result to finalize")
)

type Finalization struct {
	At           time.Time
	PlanRevision int
	ResultID     string
}
type FinalizationRequest struct {
	UserID, ProjectID string
	ExpectedRevision  int
	ExpectedResultID  string
}
type ProjectFinalizer interface {
	Finalize(context.Context, FinalizationRequest) (Project, error)
}

// ValidateFinalization uses retained metadata only. Confirmation cannot require
// source pixels or re-render an already successful, matching export.
func ValidateFinalization(p Project, req FinalizationRequest, cfg RenderConfig) error {
	if p.UserID != req.UserID || p.ID != req.ProjectID {
		return ErrNotFound
	}
	if req.ExpectedRevision <= 0 || req.ExpectedResultID == "" {
		return ErrFinalizationConflict
	}
	if p.Finalized != nil {
		if p.Finalized.PlanRevision != req.ExpectedRevision || p.Finalized.ResultID != req.ExpectedResultID {
			return ErrFinalizationConflict
		}
		return nil
	}
	if p.EditPlanRevision != req.ExpectedRevision || p.RenderedPlanRevision != req.ExpectedRevision {
		return ErrFinalizationConflict
	}
	if p.Result == nil || p.Result.Key == "" || p.Result.ID == "" || p.Result.ContentType != "video/mp4" || p.Result.Bytes <= 0 || p.Result.DurationMS <= 0 || p.Result.CreatedAt.IsZero() {
		return ErrFinalizationInvalid
	}
	if p.Result.ID != req.ExpectedResultID {
		return ErrFinalizationConflict
	}
	plan, err := DecodeEditPlan(p.EditPlan)
	if err != nil || ValidateCompositionEvidence(plan) != nil {
		return ErrFinalizationInvalid
	}
	validated, err := ApplyCorrection(cfg, p, CorrectionFromPlan(plan))
	if err != nil || ValidateCompositionEvidence(validated) != nil {
		return ErrFinalizationInvalid
	}
	return nil
}

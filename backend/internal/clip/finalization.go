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

func (s *Service) SetFinalizer(f ProjectFinalizer) { s.finalizer = f }

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
	plan, _, err := DecodeEditPlan(p.EditPlan)
	if err != nil || ValidateCompositionEvidence(plan) != nil {
		return ErrFinalizationInvalid
	}
	validated, _, err := ApplyCorrection(cfg, p, CorrectionFromPlan(plan))
	if err != nil || ValidateCompositionEvidence(validated) != nil {
		return ErrFinalizationInvalid
	}
	return nil
}

func (s *Service) FinalizeProject(ctx context.Context, req FinalizationRequest) (Project, error) {
	if s.finalizer == nil {
		return Project{}, ErrFinalizationInvalid
	}
	p, err := s.finalizer.Finalize(ctx, req)
	if err != nil {
		return Project{}, err
	}
	// The transaction has already fenced originals and persisted cleanup. Storage
	// outages cannot undo confirmation; the source sweeper owns further retries.
	if s.sources != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.sources.RevokeProject(cleanup, req.UserID, req.ProjectID)
	}
	return p, nil
}

func (s *GenerationService) FinalizationRefusal(p Project, busy bool) string {
	if p.Finalized != nil {
		return "finalized"
	}
	if busy {
		return "busy"
	}
	if p.Result == nil || p.Result.ID == "" {
		return "missing_render"
	}
	err := ValidateFinalization(p, FinalizationRequest{p.UserID, p.ID, p.EditPlanRevision, p.Result.ID}, s.cfg.Render)
	if errors.Is(err, ErrFinalizationConflict) {
		return "stale_render"
	}
	if err != nil {
		return "invalid_plan"
	}
	return ""
}

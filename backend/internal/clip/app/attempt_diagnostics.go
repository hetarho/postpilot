package app

import (
	"context"
	"errors"
	"log/slog"
	"regexp"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) checkpoint(ctx context.Context, user, project string, c clip.AttemptCheckpoint) error {
	store, ok := s.store.(clip.AttemptCheckpointStore)
	if !ok {
		return nil
	}
	c.Version = 1
	c.Diagnostic.Check = clip.SafeAttemptCheck(c.Diagnostic.Check)
	c.Diagnostic.Values = clip.SafeAttemptValues(c.Diagnostic.Values)
	c.Diagnostic.Phase = clip.SafeAttemptPhase(c.Diagnostic.Phase)
	if err := store.SaveAttemptCheckpoint(ctx, user, project, c); err != nil {
		// Preserve interruption identity so the worker leaves recovery to its boot
		// sweep instead of reporting a storage failure during normal shutdown.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		slog.Warn("clip checkpoint unavailable", "job", c.JobID, "stage", c.Stage)
		return clip.ErrAttemptCheckpointUnavailable
	}
	return nil
}

func (s *GenerationService) AttemptCheckpoint(ctx context.Context, user, project, job string) (*clip.AttemptCheckpoint, error) {
	if _, err := s.projects.store.GetProject(ctx, user, project); err != nil {
		return nil, err
	}
	store, ok := s.store.(clip.AttemptCheckpointStore)
	if !ok {
		return nil, nil
	}
	return store.GetAttemptCheckpoint(ctx, user, project, job)
}

var diagnosticElementID = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func logAttemptDiagnostic(job, stage string, d clip.AttemptDiagnostic) {
	attrs := []any{"job", job, "stage", stage, "validation_phase", clip.SafeAttemptPhase(d.Phase), "validation_check", clip.SafeAttemptCheck(d.Check)}
	if d.Line > 0 && d.Line <= 16000 {
		attrs = append(attrs, "line", d.Line)
	}
	if diagnosticElementID.MatchString(d.ElementID) {
		attrs = append(attrs, "element_id", d.ElementID)
	}
	// Check is supplied only by the owned parser; the public diagnostic projection
	// still normalizes it against its vocabulary before display.
	for key, value := range clip.SafeAttemptValues(d.Values) {
		attrs = append(attrs, key, value)
	}
	slog.Info("clip attempt diagnostic", attrs...)
	for _, r := range d.Ranges {
		slog.Info("clip candidate range", "job", job, "cut", r.Cut, "source", r.Source, "start_ms", r.StartMS, "end_ms", r.EndMS, "range_valid", r.Valid)
	}
}

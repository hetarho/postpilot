package experiment

import (
	"context"
	"errors"
	"log/slog"

	"github.com/postpilot/backend/internal/llm"
)

// Handle runs one durable experiment job. The queue owns durability; this method owns
// stage orchestration and persists each candidate independently as it finishes.
func (s *Service) Handle(ctx context.Context, experimentID string, progress Progress) error {
	found, err := s.store.Get(ctx, experimentID)
	if err != nil {
		return err
	}
	if len(found.InputSnapshot) == 0 {
		_ = s.store.FailUnfinished(ctx, found.ID, normalizeFailure(ErrSnapshotUnavailable), s.now())
		return ErrSnapshotUnavailable
	}
	if err := s.store.SetStatus(ctx, found.ID, StatusRunning, nil); err != nil {
		return err
	}
	if found.Stage == StageWrite {
		prepared, err := s.runner.PrepareWrite(ctx, found, progress)
		if err != nil {
			_ = s.store.FailUnfinished(ctx, found.ID, normalizeFailure(err), s.now())
			return err
		}
		if prepared.TargetLanguage == nil || found.TargetLanguage == nil || *prepared.TargetLanguage != *found.TargetLanguage {
			_ = s.store.FailUnfinished(ctx, found.ID, normalizeFailure(ErrLanguageRequired), s.now())
			return ErrLanguageRequired
		}
		frozen, hash, err := FreezeSnapshot(prepared)
		if err != nil {
			_ = s.store.FailUnfinished(ctx, found.ID, normalizeFailure(err), s.now())
			return err
		}
		if err := s.store.SetSnapshot(ctx, found.ID, frozen, hash); err != nil {
			_ = s.store.FailUnfinished(ctx, found.ID, normalizeFailure(err), s.now())
			return err
		}
		found.InputSnapshot = frozen.Content
		found.InputHash = hash
		found.PromptVersion = frozen.PromptVersion
	}
	if err := s.runCandidates(ctx, found, progress); err != nil {
		_ = s.store.FailUnfinished(ctx, found.ID, normalizeFailure(err), s.now())
		return err
	}
	updated, err := s.store.Get(ctx, found.ID)
	if err != nil {
		return err
	}
	status, err := StatusAfterCandidates(updated.Candidates)
	if err != nil {
		return err
	}
	finished := s.now()
	if err := s.store.SetStatus(ctx, found.ID, status, &finished); err != nil {
		return err
	}
	if status == StatusFailed {
		return errAllCandidatesFailed
	}
	return nil
}

func (s *Service) runCandidate(ctx context.Context, found Experiment, candidate Candidate, progress Progress) error {
	started := s.now()
	if err := s.store.StartCandidate(ctx, found.ID, candidate.ID, started); err != nil {
		return err
	}
	candidate.Status = CandidateRunning
	result, runErr := s.runner.RunCandidate(ctx, found, candidate, progress)
	finished := s.now()
	candidate.FinishedAt = &finished
	candidate.Usage.LatencyMS = max(0, finished.Sub(started).Milliseconds())
	model, _ := s.catalog.Resolve(candidate.Model)
	usage := ResolveCost(result.Usage, model)
	usage.LatencyMS = candidate.Usage.LatencyMS
	candidate.Usage = usage
	candidate.Output = append([]byte(nil), result.Output...)
	if runErr != nil {
		candidate.Status = CandidateFailed
		failure := normalizeFailure(runErr)
		candidate.Failure = &failure
		slog.Error("experiment candidate failed",
			"experiment_id", found.ID,
			"candidate_id", candidate.ID,
			"model", candidate.Model.String(),
			"err", diagnosticError(runErr),
		)
	} else {
		candidate.Status = CandidateSucceeded
		candidate.Failure = nil
	}
	return s.store.CompleteCandidate(ctx, candidate)
}

func diagnosticError(err error) error {
	var providerErr *llm.ProviderError
	if errors.As(err, &providerErr) {
		return err
	}
	if errors.Is(err, llm.ErrBadOutput) {
		return llm.ErrBadOutput
	}
	return err
}

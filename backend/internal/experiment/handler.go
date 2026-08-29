package experiment

import (
	"context"
	"errors"
)

// Handle runs one durable experiment job. The queue owns durability; this method owns
// stage orchestration and persists each candidate independently as it finishes.
func (s *Service) Handle(ctx context.Context, experimentID string, progress Progress) error {
	found, err := s.store.Get(ctx, experimentID)
	if err != nil {
		return err
	}
	if len(found.InputSnapshot) == 0 {
		return ErrSnapshotUnavailable
	}
	if err := s.store.SetStatus(ctx, found.ID, StatusRunning, nil); err != nil {
		return err
	}
	if found.Stage == StageWrite {
		prepared, err := s.runner.PrepareWrite(ctx, found, progress)
		if err != nil {
			_ = s.store.FailUnfinished(ctx, found.ID, userFacingError(err), s.now())
			return err
		}
		frozen, hash, err := FreezeSnapshot(prepared)
		if err != nil {
			_ = s.store.FailUnfinished(ctx, found.ID, userFacingError(err), s.now())
			return err
		}
		if err := s.store.SetSnapshot(ctx, found.ID, frozen, hash); err != nil {
			_ = s.store.FailUnfinished(ctx, found.ID, "비교 입력을 저장하지 못했어요. 다시 시도해 주세요.", s.now())
			return err
		}
		found.InputSnapshot = frozen.Content
		found.InputHash = hash
		found.PromptVersion = frozen.PromptVersion
	}
	if err := s.runCandidates(ctx, found, progress); err != nil {
		_ = s.store.FailUnfinished(ctx, found.ID, "비교 결과를 저장하지 못했어요. 다시 시도해 주세요.", s.now())
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
		return errors.New("두 모델 모두 결과를 만들지 못했어요. 실패한 후보를 다시 시도해 주세요")
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
		candidate.Error = userFacingError(runErr)
	} else {
		candidate.Status = CandidateSucceeded
		candidate.Error = ""
	}
	return s.store.CompleteCandidate(ctx, candidate)
}

func userFacingError(err error) string {
	if errors.Is(err, ErrSnapshotUnavailable) {
		return ErrSnapshotUnavailable.Error()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "모델 호출 시간이 초과됐어요. 다시 시도해 주세요."
	}
	return "모델 호출에 실패했어요. 다시 시도해 주세요."
}

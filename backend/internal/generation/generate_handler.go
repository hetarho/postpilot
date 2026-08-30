package generation

import (
	"context"
	"errors"
	"fmt"
)

// Generate handles one durable generate job. Each model call completes before its
// corresponding post write, so no SQLite transaction spans provider latency.
func (s *Service) Generate(ctx context.Context, job GenerateJob, progress Progress) error {
	post, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("load generation input: %w", err)
	}
	if _, err := frozenVoice(post, job.VoiceID); err != nil {
		return err
	}
	// Generation options are frozen when the job is enqueued. A later options edit
	// must not change the prompt of work that is already waiting in the queue.
	post.TargetLength = cloneOptionalInt(job.TargetLength)
	// An empty observe model records that StartGeneration accepted a zero-photo input.
	// Photos attached while the queued job waits belong to the next generation; without
	// this snapshot bit the accepted job would fail later for lacking a vision model.
	if job.ObserveModel == "" {
		post.Images = nil
	}
	var observations []Observation
	if len(post.Images) == 0 {
		progress("observe", 0, 0)
		if err := s.posts.SetObservations(ctx, post.UserID, post.Slug, nil); err != nil {
			return fmt.Errorf("clear observations: %w", err)
		}
	} else {
		observeModel, ok := parseModelRef(job.ObserveModel)
		if !ok {
			return ErrObserveModelRequired
		}
		observations, err = s.observe(ctx, post, observeModel, progress)
		if err != nil {
			return err
		}
	}
	writeModel, ok := parseModelRef(job.WriteModel)
	if !ok {
		return ErrWriteModelRequired
	}
	progress("write", 0, 1)
	content, err := s.write(ctx, post, observations, writeModel)
	if err != nil {
		return err
	}
	if err := s.posts.SetGeneratedContent(ctx, post.UserID, post.Slug, content); err != nil {
		return fmt.Errorf("persist generated content: %w", err)
	}
	progress("write", 1, 1)
	return nil
}

func cloneOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func providerCallError(stage string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s 모델 호출 시간이 초과됐어요: %w", stage, err)
	}
	return fmt.Errorf("%s 모델 호출 실패: %w", stage, err)
}

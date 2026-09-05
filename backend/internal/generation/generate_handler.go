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
	// Jobs queued before the language payload existed retain their established Korean
	// behavior. New starts cannot omit the field and EncodeGenerationPayload refuses it.
	if job.TargetLanguage == "" {
		job.TargetLanguage = LanguageKorean
	}
	if !job.TargetLanguage.Valid() {
		return ErrLanguageRequired
	}
	// Language is frozen in the durable payload. Never let the live post target retarget
	// queued work after dequeue.
	post.TargetLanguage = job.TargetLanguage
	// Generation options are frozen when the job is enqueued. A later options edit
	// must not change the prompt of work that is already waiting in the queue.
	post.TargetLength = cloneOptionalInt(job.TargetLength)
	post.WriteNativeEffort = job.WriteNativeEffort
	// Deliberately the payload's brief and never post.TemplateID: editing or deleting the
	// template after the enqueue — including across a restart-resume or an explicit retry —
	// must not change the prompt this run builds.
	post.Template = cloneTemplate(job.Template)
	// Same rule for the 지침: the frozen texts, never a fresh resolution.
	post.Guidelines = cloneTexts(job.Guidelines)
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
		// The payload's frozen decision, never a live snapshot: what this run observes was
		// settled at enqueue and a photo attached since then belongs to the next generation.
		targets, seed := frozenObserveSelection(post.Images, job.ObserveFiles, job.Observations)
		if len(targets) == 0 {
			// Every observation is being reused, so there is nothing to call a provider for
			// and nothing to write: making no SetObservations call at all is what leaves the
			// stored snapshot byte-identical. The stage is complete the moment it starts.
			progress("observe", 0, 0)
			observations = mergeObservations(post.Images, seed, nil)
		} else {
			observations, err = s.observe(ctx, post, targets, seed, observeModel, progress)
			if err != nil {
				return err
			}
		}
		// The write stage is shown ONLY the photos this run has eyesight for. post.Images is
		// read live at dequeue, so a photo confirmed between the enqueue and here is outside
		// the frozen decision: it is not observed, and it must not reach the write prompt
		// either — a filename with no observation is exactly the "write from a photo nothing
		// has looked at" case this change exists to prevent. It belongs to the next run.
		post.Images = observedImages(post.Images, observations)
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
	if err := s.posts.SetGeneratedContent(ctx, post.UserID, post.Slug, content, job.TargetLanguage); err != nil {
		return fmt.Errorf("persist generated content: %w", err)
	}
	s.recordVersionSample(ctx, post.UserID, post.Voice.ID, content)
	progress("write", 1, 1)
	return nil
}

// cloneTemplate deep-copies the slot slice too: a frozen brief must not share backing
// storage with whatever the caller does next to its own slice.
func cloneTemplate(value *TemplateBrief) *TemplateBrief {
	if value == nil {
		return nil
	}
	copied := *value
	if len(value.Slots) > 0 {
		copied.Slots = append([]TemplateSlot(nil), value.Slots...)
	}
	return &copied
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

package generation

import (
	"context"
	"errors"
	"fmt"
)

// Generate handles one durable generate job. Each model call completes before its
// corresponding post write, so no SQLite transaction spans provider latency.
func (s *Service) Generate(ctx context.Context, job GenerateJob, progress Progress) error {
	// The frozen options first, as Revise reads its payload: a payload that cannot be read
	// fails before anything else is read. Decode owns the legacy defaults — a payload queued
	// before the language field existed is Korean, and an unknown language is refused.
	options, err := decodeGenerationPayload(job.Payload)
	if err != nil {
		return err
	}
	post, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("load generation input: %w", err)
	}
	// A backstop no product path reaches, since a paste waits while this job is active: a
	// post published after the enqueue calls no provider.
	if post.Published {
		return ErrPostPublished
	}
	voiceID, err := frozenVoice(post, job.VoiceID)
	if err != nil {
		return err
	}
	// Generation options are frozen when the job is enqueued — language, length, tag count,
	// template, 지침, 기억, rules and the write budget's headroom alike. A later edit must not
	// change the prompt of work that is already waiting in the queue, and none of them is ever
	// resolved afresh here (GEN-5, GEN-30, MEM-19, GEN-51).
	post = options.onto(post)
	if err := s.validateFrozenProfile(ctx, job.UserID, voiceID, options.Profile); err != nil {
		return err
	}
	post, observations, err := s.observeForRun(ctx, post, job.ObserveModel, options.ObserveFiles, options.Observations, progress)
	if err != nil {
		return err
	}
	if len(post.FollowStoryline) > 0 {
		// A run along the storyline is shown only what the storyline holds (GEN-70), so the
		// attachment filter keeps no other (GEN-2).
		post.Images = heldAttachments(post.Images, post.FollowStoryline)
		observations = heldObservations(post.Images, observations)
	}
	writeModel, ok := parseModelRef(job.WriteModel)
	if !ok {
		return ErrWriteModelRequired
	}
	progress("write", 0, 1)
	var answer WriteAnswer
	if options.Profile != nil {
		answer, _, err = s.writeCandidate(ctx, post, *options.Profile, observations, writeModel)
	} else {
		answer, err = s.write(ctx, post, observations, writeModel)
	}
	if err != nil {
		return err
	}
	// The voice is rechecked on a fresh snapshot, as Revise does: a reassignment or deletion
	// that slipped in during the provider calls must not persist output into the wrong
	// profile (GEN-27).
	current, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("reload generation voice: %w", err)
	}
	if _, err := frozenVoice(current, voiceID); err != nil {
		return err
	}
	if err := s.validateFrozenProfile(ctx, job.UserID, voiceID, options.Profile); err != nil {
		return err
	}
	// The answer's annotations replace the post's, a noun-less write's included: its nil nouns
	// clear the ones the last generation stored (GEN-55).
	if err := s.posts.SetGeneratedContent(ctx, post.UserID, post.Slug, answer.Content, options.TargetLanguage, answer.Annotations()); err != nil {
		return fmt.Errorf("persist generated content: %w", err)
	}
	progress("write", 1, 1)
	return nil
}

// observeForRun is the observe step a generation and a storyline job share (GEN-8, GEN-10,
// GEN-11), so reuse, provenance and the non-shrinking snapshot stay one implementation. It
// returns the post with only the attachments the run has eyesight for, and their observations.
//
// observeModel, observeFiles and carried are the run's frozen decision: the row's observe model
// and the payload's selection and snapshot, never a live read.
func (s *Service) observeForRun(ctx context.Context, post PostInput, observeModel string, observeFiles *[]string, carried []Observation, progress Progress) (PostInput, []Observation, error) {
	// An empty observe model records that the start accepted a zero-photo input. Photos
	// attached while the queued job waits belong to the next run; without this snapshot bit
	// the accepted job would fail later for lacking a vision model.
	if observeModel == "" {
		post.Images = nil
	}
	if len(post.Images) == 0 {
		progress("observe", 0, 0)
		if err := s.posts.SetObservations(ctx, post.UserID, post.Slug, nil); err != nil {
			return post, nil, fmt.Errorf("clear observations: %w", err)
		}
		return post, nil, nil
	}
	model, ok := parseModelRef(observeModel)
	if !ok {
		return post, nil, ErrObserveModelRequired
	}
	// The payload's frozen decision, never a live snapshot: what this run observes was settled
	// at enqueue and a photo attached since then belongs to the next run.
	targets, seed := frozenObserveSelection(post.Images, observeFiles, carried)
	var observations []Observation
	if len(targets) == 0 {
		// Every observation is being reused, so there is nothing to call a provider for and
		// nothing to write: making no SetObservations call at all is what leaves the stored
		// snapshot byte-identical. The stage is complete the moment it starts.
		progress("observe", 0, 0)
		observations = mergeObservations(post.Images, seed, nil)
	} else {
		var err error
		observations, err = s.observe(ctx, post, targets, seed, model, progress)
		if err != nil {
			return post, nil, err
		}
	}
	// The next stage is shown ONLY the attachments this run has eyesight for. post.Images is
	// read live at dequeue, so one confirmed between the enqueue and here is outside the frozen
	// decision: it is not observed, and it must not reach the prompt either — a filename with
	// no observation is exactly the "write from a photo nothing has looked at" case. It belongs
	// to the next run.
	post.Images = observedImages(post.Images, observations)
	return post, observations, nil
}

// cloneTemplate deep-copies the fact slice too: a frozen brief must not share backing storage
// with whatever the caller does next to its own slices.
func cloneTemplate(value *TemplateBrief) *TemplateBrief {
	if value == nil {
		return nil
	}
	copied := *value
	if len(value.Facts) > 0 {
		copied.Facts = append([]TemplateFact(nil), value.Facts...)
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

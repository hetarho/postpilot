package generation

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

// startStage is what one start's preconditions differ by. Every start runs the one chain in
// startPreconditions — the published lock, the start's own material, the language, the voice, a
// pending editor comparison, the write model, the observe model — and a stage switches a link off
// or fills one in, never reorders one: the order decides which refusal the owner sees (GEN-23,
// GEN-25).
type startStage struct {
	// readsPublished lets a published post through: a snapshot-only comparison writes nothing onto
	// it (MODEL-31). Every other start refuses one before anything is frozen, held or queued
	// (GEN-56).
	readsPublished bool
	// storyline requires a stored storyline right after the published lock: a run along it and the
	// storyline request both read one (GEN-69, GEN-70).
	storyline bool
	// revision requires generated content right after the published lock and checks the language
	// that content is in, never the target (GEN-38).
	revision bool
	// beforeVoice and withVoice are a comparison snapshot's own work, where it has always done it:
	// its material freezes between the language and the voice, and the voice's projection loads
	// right after the voice check.
	beforeVoice func(*PostInput) error
	withVoice   func(post PostInput, voiceID string) error
	// comparison skips the pending-comparison refusal and the write model: the experiment resolves
	// its own candidates and refuses a second pending comparison itself.
	comparison bool
	writeModel string
	// observe is how the start observes. observeModel is checked only when the post has something
	// to look at, and observeFiles is the re-observation picker's answer (GEN-8, GEN-9).
	observe      observeMode
	observeModel string
	observeFiles *[]string
}

type observeMode int

const (
	// observeNothing: a revision reads no observation and takes no observe model (GEN-38).
	observeNothing observeMode = iota
	// observePicked observes as a generation does, the re-observation picker included.
	observePicked
	// observeHeld observes, along the stored storyline, only what it holds that has no
	// observation, with no picker (GEN-70).
	observeHeld
	// observeReused takes every stored observation as it stands, with no picker and no observe
	// model (GEN-69).
	observeReused
)

// startInput is what a start's preconditions resolved, all from one read of the post.
type startInput struct {
	post PostInput
	// language is the run's language: the target, or a revision's content language.
	language Language
	// voiceID is the voice the run freezes, empty for 말투 없음.
	voiceID string
	// storyline is the stored storyline's paragraphs, copied, for a stage that requires one.
	storyline []StorylineParagraph
	// write is the write model's catalog entry, whose native-effort flag the run freezes; zero for
	// a comparison.
	write   llm.ModelInfo
	observe observePlan
}

// observePlan is the observe decision frozen from the same read. A post with nothing attached
// plans none: it routes no observe model and freezes no selection.
type observePlan struct {
	// model is the observe model the run is routed to.
	model string
	// files and observations are the frozen selection and the reusable snapshot.
	files        *[]string
	observations []Observation
	// calls is how many observation calls the frozen set takes.
	calls int
}

// startPreconditions is the one precondition chain every generation start runs, in this order,
// before anything is frozen, held or queued.
func (s *Service) startPreconditions(ctx context.Context, userID, postSlug string, stage startStage) (startInput, error) {
	post, err := s.posts.AttachedImages(ctx, userID, postSlug)
	if err != nil {
		return startInput{}, err
	}
	// Ahead of everything else: a run that cannot land starts nothing (GEN-56).
	if post.Published && !stage.readsPublished {
		return startInput{}, ErrPostPublished
	}
	var in startInput
	if stage.storyline {
		if post.Storyline == nil || len(post.Storyline.Paragraphs) == 0 {
			return startInput{}, ErrStorylineMissing
		}
		in.storyline = cloneParagraphs(post.Storyline.Paragraphs)
	}
	if stage.revision {
		if post.Content == nil {
			return startInput{}, ErrRevisionContentRequired
		}
		if post.ContentLanguage == nil || !post.ContentLanguage.Valid() {
			return startInput{}, ErrContentLanguageRequired
		}
		in.language = *post.ContentLanguage
	} else {
		if !post.TargetLanguage.Valid() {
			return startInput{}, ErrLanguageRequired
		}
		in.language = post.TargetLanguage
	}
	if stage.beforeVoice != nil {
		if err := stage.beforeVoice(&post); err != nil {
			return startInput{}, err
		}
	}
	if in.voiceID, err = activeVoice(post); err != nil {
		return startInput{}, err
	}
	if stage.withVoice != nil {
		if err := stage.withVoice(post, in.voiceID); err != nil {
			return startInput{}, err
		}
	}
	if !stage.comparison {
		if err := s.refusePendingExperiment(ctx, userID, postSlug); err != nil {
			return startInput{}, err
		}
		if in.write, err = s.requireWriteModel(stage.writeModel); err != nil {
			return startInput{}, err
		}
	}
	if in.observe, err = s.planObserve(post, stage, in.storyline); err != nil {
		return startInput{}, err
	}
	in.post = post
	return in, nil
}

func (s *Service) refusePendingExperiment(ctx context.Context, userID, postSlug string) error {
	if s.experiments == nil {
		return nil
	}
	id, err := s.experiments.BlockingWriteForPost(ctx, userID, postSlug)
	if err != nil {
		return err
	}
	if id != "" {
		return &ExperimentPendingError{ExperimentID: id}
	}
	return nil
}

// requireWriteModel is every start's write-model precondition: an enabled model that serves the
// write stage, which is the stage a storyline call runs on too. It answers the model's catalog
// entry, whose native-effort flag the start freezes.
func (s *Service) requireWriteModel(value string) (llm.ModelInfo, error) {
	write, ok := parseModelRef(value)
	info, found := s.models.Resolve(write)
	if !ok || !found || info.Disabled || !info.ServesStage(llm.StageNameWrite) {
		return llm.ModelInfo{}, ErrWriteModelRequired
	}
	return info, nil
}

// planObserve is the chain's last link. The missing-observe-model refusal runs before the video
// check, so a post with videos and no model chosen hears the simpler fix first (VIDEO-11).
func (s *Service) planObserve(post PostInput, stage startStage, storyline []StorylineParagraph) (observePlan, error) {
	switch stage.observe {
	case observeNothing:
		return observePlan{}, nil
	case observeReused:
		return observePlan{observations: cloneObservations(post.Observations)}, nil
	}
	if len(post.Images) == 0 {
		// A zero-photo post has no reuse decision to make, so nothing about the picker is frozen
		// for it: the run observes nothing and clears the snapshot, as it always has.
		return observePlan{}, nil
	}
	observe, valid := parseModelRef(stage.observeModel)
	if !valid || !modelEnabled(s.models, observe, llm.StageNameObserve) {
		return observePlan{}, ErrObserveModelRequired
	}
	if err := s.refuseVideoBlindObserveModel(post.Images, observe); err != nil {
		return observePlan{}, err
	}
	// Both halves of the reuse decision are resolved HERE, from one read of the post, and frozen
	// into the payload. Attaching a photo, deleting one or switching the observation model
	// afterwards cannot reach the queued run.
	var files []string
	var carried []Observation
	if stage.observe == observeHeld {
		files, carried = freezeStorylineObserveSelection(post.Images, post.Observations, storyline)
	} else {
		files, carried = freezeObserveSelection(post.Images, post.Observations, stage.observeFiles)
	}
	return observePlan{
		model: stage.observeModel, files: &files, observations: carried,
		// Priced over the FROZEN set, never over the attached count: a run that reuses every
		// observation makes no observation call and must not be held for fifteen of them.
		calls: s.observeCalls(observeTargets(post.Images, &files)),
	}, nil
}

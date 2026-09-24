package generation

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

const (
	WriteExperimentPromptVersion   = "write-v3-language-ab"
	ObserveExperimentPromptVersion = "observe-v1-ab"
)

type CandidateUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	CostMicrousd     int64
	CostReported     bool
}

// snapshotOnly is true for a comparison that may not write to the post it reads. It travels
// in the frozen snapshot because preparation happens later, in the worker, from the snapshot
// alone.
func (s *Service) SnapshotWriteInput(ctx context.Context, userID, postSlug string, observeModel llm.ModelRef, targetLength *int, observeFiles *[]string, snapshotOnly bool) ([]byte, error) {
	post, err := s.posts.AttachedImages(ctx, userID, postSlug)
	if err != nil {
		return nil, err
	}
	// The editor's comparison writes its result into the post, so a published one refuses it.
	// A snapshot-only comparison writes nothing there and reads a published post freely
	// (MODEL-31).
	if post.Published && !snapshotOnly {
		return nil, ErrPostPublished
	}
	if !post.TargetLanguage.Valid() {
		return nil, ErrLanguageRequired
	}
	post.TargetLength = cloneOptionalInt(targetLength)
	// Frozen here, once, for the whole comparison, through the very freeze Start makes: brief,
	// 지침, 기억, ticked rules and 분야 phrases (GEN-18, MEM-19, MODEL-30). Both candidates then
	// read one identical set out of this snapshot, so their prompts differ only by model ref —
	// and a different set is a different frozen input, a different hash. It runs before the
	// post's own observations are dropped below, because the memory key reads them (MEM-7).
	material, err := s.freezeWriteMaterial(ctx, post)
	if err != nil {
		return nil, err
	}
	post = material.onto(post)
	voiceID, err := activeVoice(post)
	if err != nil {
		return nil, err
	}
	profile, err := s.profileForTopic(ctx, userID, voiceID, post.TargetLanguage, post.Title+" "+post.Memo, contentTags(post.Content))
	if err != nil {
		return nil, fmt.Errorf("load voice profile: %w", err)
	}
	if len(post.Images) > 0 {
		if !modelEnabled(s.models, observeModel, llm.StageNameObserve) {
			return nil, ErrObserveModelRequired
		}
		// The same per-run video check the ordinary enqueue makes: a comparison that cannot
		// observe the post's clips would compare two writers working from half the material.
		if err := s.refuseVideoBlindObserveModel(post.Images, observeModel); err != nil {
			return nil, err
		}
	}
	// The same freeze the ordinary enqueue performs, for the same reason and through the
	// same helper: the comparison must not re-pay for eyesight it already has, and it must
	// not write from a photo nothing has looked at.
	var frozen *[]string
	var known []Observation
	if len(post.Images) > 0 {
		files, snapshot := freezeObserveSelection(post.Images, post.Observations, observeFiles)
		frozen = &files
		known = snapshot
	}
	// The post's own copy is dropped: the snapshot's Observations field is the ONE the
	// candidates read, and two copies of the same fact in one frozen input could disagree.
	post.Observations = nil
	return encodeWriteSnapshot(writeSnapshot{
		TargetLanguage: post.TargetLanguage,
		ObserveModel:   observeModel.String(), ObserveFiles: frozen,
		Post: post, Profile: profile, Observations: known, SnapshotOnly: snapshotOnly,
	})
}

func (s *Service) SnapshotObserveInput(ctx context.Context, userID, postSlug string) ([]byte, error) {
	post, err := s.posts.AttachedImages(ctx, userID, postSlug)
	if err != nil {
		return nil, err
	}
	if len(post.Images) == 0 {
		return nil, fmt.Errorf("관찰 비교에는 사진이 한 장 이상 필요해요")
	}
	return encodeObserveSnapshot(post.Slug, post.UserID, post.Images)
}

func (s *Service) PrepareWriteInput(ctx context.Context, raw []byte, progress Progress) ([]byte, error) {
	snapshot, err := decodeWriteSnapshot(raw)
	if err != nil {
		return nil, err
	}
	if snapshot.Prepared {
		return append([]byte(nil), raw...), nil
	}
	if len(snapshot.Post.Images) == 0 {
		progress("observe", 0, 0)
		// A post with nothing attached has nothing to observe. Ordinary generation clears
		// what the post held so its contact sheet stops describing attachments that are
		// gone; a comparison that may not write to the post leaves that alone.
		if !snapshot.SnapshotOnly {
			if err := s.posts.SetObservations(ctx, snapshot.Post.UserID, snapshot.Post.Slug, nil); err != nil {
				return nil, err
			}
		}
		snapshot.Observations = nil
	} else {
		model, ok := parseModelRef(snapshot.ObserveModel)
		if !ok {
			return nil, ErrObserveModelRequired
		}
		targets, seed := frozenObserveSelection(snapshot.Post.Images, snapshot.ObserveFiles, snapshot.Observations)
		if len(targets) == 0 {
			progress("observe", 0, 0)
			snapshot.Observations = mergeObservations(snapshot.Post.Images, seed, nil)
		} else {
			observations, _, err := s.observeCandidate(ctx, snapshot.Post, targets, seed, model, progress, !snapshot.SnapshotOnly)
			if err != nil {
				return nil, err
			}
			snapshot.Observations = observations
		}
	}
	snapshot.Prepared = true
	return encodeWriteSnapshot(snapshot)
}

// RunWriteCandidate returns the candidate's whole answer, so the winner, once applied, carries
// its own nouns and replacement candidates into the post.
func (s *Service) RunWriteCandidate(ctx context.Context, raw []byte, model llm.ModelRef) (WriteAnswer, CandidateUsage, error) {
	snapshot, err := decodeWriteSnapshot(raw)
	if err != nil {
		return WriteAnswer{}, CandidateUsage{}, err
	}
	if !snapshot.Prepared {
		return WriteAnswer{}, CandidateUsage{}, fmt.Errorf("write snapshot is not prepared")
	}
	// The two candidates run different models, so the reasoning headroom is this candidate's
	// own, from its catalog entry — the same flag its quote priced (MODEL-39) — and never a
	// value frozen into the snapshot both candidates share. An unresolvable model keeps the
	// bare budget, as before.
	post := snapshot.Post
	if info, ok := s.models.Resolve(model); ok {
		post.WriteNativeEffort = info.ReasoningNativeEffort
	}
	answer, usage, err := s.writeCandidate(ctx, post, snapshot.Profile, snapshot.Observations, model)
	return answer, candidateUsage(usage), err
}

func (s *Service) RunObserveCandidate(ctx context.Context, raw []byte, model llm.ModelRef, progress Progress) ([]Observation, CandidateUsage, error) {
	post, err := decodeObserveSnapshot(raw)
	if err != nil {
		return nil, CandidateUsage{}, err
	}
	// Every photo, no seed: the observe-stage A/B compares observation MODELS, so reusing an
	// observation would be comparing one model against the other's stored work.
	observations, usage, err := s.observeCandidate(ctx, post, post.Images, nil, model, progress, false)
	return observations, candidateUsage(usage), err
}

// ApplyWriteWinner establishes a machine baseline, so it is an AI result landing in a
// voice: the post's current voice must still be alive and match the frozen snapshot's. The
// winner's annotations replace the post's; a candidate recorded before they existed carries
// none and clears them (GEN-4).
func (s *Service) ApplyWriteWinner(ctx context.Context, userID, postSlug string, answer WriteAnswer, raw ...[]byte) error {
	current, err := s.posts.AttachedImages(ctx, userID, postSlug)
	if err != nil {
		return err
	}
	if current.Published {
		return ErrPostPublished
	}
	frozenVoiceID := ""
	var frozenLanguage Language
	if len(raw) > 0 && len(raw[0]) > 0 {
		if snapshot, decodeErr := decodeWriteSnapshot(raw[0]); decodeErr == nil {
			frozenVoiceID = snapshot.Post.Voice.ID
			frozenLanguage = snapshot.TargetLanguage
		}
	}
	if _, err := frozenVoice(current, frozenVoiceID); err != nil {
		return err
	}
	if !frozenLanguage.Valid() {
		return ErrLanguageRequired
	}
	if err := s.posts.SetGeneratedContent(ctx, userID, postSlug, answer.Content, frozenLanguage, answer.Annotations()); err != nil {
		return err
	}
	s.recordVersionSample(ctx, userID, frozenVoiceID, answer.Content)
	return nil
}

// SnapshotVoice reports the voice a frozen write snapshot was taken for, so the experiment
// aggregate can record it without decoding the generation context's private format.
func SnapshotVoice(raw []byte) string {
	snapshot, err := decodeWriteSnapshot(raw)
	if err != nil {
		return ""
	}
	return snapshot.Post.Voice.ID
}

// SnapshotTemplateName reports the template a frozen write snapshot was taken for, by name.
// The name, not the id: the comparison detail has to keep saying which brief both candidates
// were given even after that template is renamed or deleted.
func SnapshotTemplateName(raw []byte) string {
	snapshot, err := decodeWriteSnapshot(raw)
	if err != nil || snapshot.Post.Template == nil {
		return ""
	}
	return snapshot.Post.Template.Name
}

// SnapshotTargetLanguage exposes only the frozen canonical language required by the
// experiment aggregate's detail projection. The snapshot wire shape stays generation-owned.
func SnapshotTargetLanguage(raw []byte) Language {
	snapshot, err := decodeWriteSnapshot(raw)
	if err != nil {
		return ""
	}
	return snapshot.TargetLanguage
}

func (s *Service) ApplyObservationWinner(ctx context.Context, userID, postSlug string, observations []Observation) error {
	return s.posts.SetObservations(ctx, userID, postSlug, observations)
}

func candidateUsage(usage llm.Usage) CandidateUsage {
	return CandidateUsage{PromptTokens: int64(usage.PromptTokens), CompletionTokens: int64(usage.CompletionTokens), CostMicrousd: usage.CostMicrousd, CostReported: usage.CostReported}
}

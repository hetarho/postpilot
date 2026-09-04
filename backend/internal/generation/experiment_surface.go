package generation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

const (
	WriteExperimentPromptVersion   = "write-v3-language-ab"
	ObserveExperimentPromptVersion = "observe-v1-ab"
)

type experimentSnapshot struct {
	Kind           string   `json:"kind"`
	Prepared       bool     `json:"prepared"`
	TargetLanguage Language `json:"target_language,omitempty"`
	ObserveModel   string   `json:"observe_model,omitempty"`
	// ObserveFiles is the frozen re-observation set, with the same presence contract as the
	// generate payload's: absent is a snapshot taken before the picker existed and observes
	// every attached photo, present-and-empty observes nothing.
	ObserveFiles *[]string `json:"observe_files,omitempty"`
	Post         PostInput `json:"post"`
	Profile      Profile   `json:"profile,omitempty"`
	// Observations is pre-seeded at snapshot time with what the run carries over, and
	// PrepareWriteInput merges what it observes into it. Both candidates then read one
	// complete set, which is what makes the frozen input the experiment's identity.
	Observations []Observation `json:"observations,omitempty"`
}

// observeExperimentSnapshot is intentionally narrower than PostInput. Target/content
// language, voice, prose, template and write options cannot affect photo facts, so they
// cannot enter an observation experiment's candidate input or observation-only hash.
type observeExperimentSnapshot struct {
	Kind string                `json:"kind"`
	Post observeExperimentPost `json:"post"`
}

type observeExperimentPost struct {
	Slug   string  `json:"slug"`
	UserID string  `json:"user_id"`
	Images []Image `json:"images"`
}

type CandidateUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	CostMicrousd     int64
	CostReported     bool
}

func (s *Service) SnapshotWriteInput(ctx context.Context, userID, postSlug string, observeModel llm.ModelRef, targetLength *int, observeFiles *[]string) ([]byte, error) {
	post, err := s.posts.AttachedImages(ctx, userID, postSlug)
	if err != nil {
		return nil, err
	}
	if !post.TargetLanguage.Valid() {
		return nil, ErrLanguageRequired
	}
	post.TargetLength = cloneOptionalInt(targetLength)
	// Frozen here, once, for the whole comparison. Both candidates then read the identical
	// brief out of this snapshot, so their system prompts differ only by model ref — and
	// because the brief is part of the snapshot, a different template is a different input.
	brief, err := s.freezeTemplate(ctx, post)
	if err != nil {
		return nil, err
	}
	post.Template = brief
	// The same freeze, for the same reason: both candidates then read one identical set out
	// of this snapshot, and a different applicable set is a different experiment input.
	texts, err := s.freezeGuidelines(ctx, post)
	if err != nil {
		return nil, err
	}
	post.Guidelines = texts
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
	return json.Marshal(experimentSnapshot{
		Kind: "write", TargetLanguage: post.TargetLanguage,
		ObserveModel: observeModel.String(), ObserveFiles: frozen,
		Post: post, Profile: profile, Observations: known,
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
	return json.Marshal(observeExperimentSnapshot{Kind: "observe", Post: observeExperimentPost{
		Slug: post.Slug, UserID: post.UserID, Images: append([]Image(nil), post.Images...),
	}})
}

func (s *Service) PrepareWriteInput(ctx context.Context, raw []byte, progress Progress) ([]byte, error) {
	snapshot, err := decodeExperimentSnapshot(raw, "write")
	if err != nil {
		return nil, err
	}
	if snapshot.Prepared {
		return append([]byte(nil), raw...), nil
	}
	if len(snapshot.Post.Images) == 0 {
		progress("observe", 0, 0)
		if err := s.posts.SetObservations(ctx, snapshot.Post.UserID, snapshot.Post.Slug, nil); err != nil {
			return nil, err
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
			observations, _, err := s.observeCandidate(ctx, snapshot.Post, targets, seed, model, progress, true)
			if err != nil {
				return nil, err
			}
			snapshot.Observations = observations
		}
	}
	snapshot.Prepared = true
	return json.Marshal(snapshot)
}

func (s *Service) RunWriteCandidate(ctx context.Context, raw []byte, model llm.ModelRef) (PostContent, CandidateUsage, error) {
	snapshot, err := decodeExperimentSnapshot(raw, "write")
	if err != nil {
		return PostContent{}, CandidateUsage{}, err
	}
	if !snapshot.Prepared {
		return PostContent{}, CandidateUsage{}, fmt.Errorf("write snapshot is not prepared")
	}
	content, usage, err := s.writeCandidate(ctx, snapshot.Post, snapshot.Profile, snapshot.Observations, model)
	return content, candidateUsage(usage), err
}

func (s *Service) RunObserveCandidate(ctx context.Context, raw []byte, model llm.ModelRef, progress Progress) ([]Observation, CandidateUsage, error) {
	snapshot, err := decodeObserveExperimentSnapshot(raw)
	if err != nil {
		return nil, CandidateUsage{}, err
	}
	post := PostInput{Slug: snapshot.Post.Slug, UserID: snapshot.Post.UserID, Images: append([]Image(nil), snapshot.Post.Images...)}
	// Every photo, no seed: the observe-stage A/B compares observation MODELS, so reusing an
	// observation would be comparing one model against the other's stored work.
	observations, usage, err := s.observeCandidate(ctx, post, post.Images, nil, model, progress, false)
	return observations, candidateUsage(usage), err
}

// ApplyWriteWinner establishes a machine baseline, so it is an AI result landing in a
// voice: the post's current voice must still be alive and match the frozen snapshot's.
func (s *Service) ApplyWriteWinner(ctx context.Context, userID, postSlug string, content PostContent, raw ...[]byte) error {
	current, err := s.posts.AttachedImages(ctx, userID, postSlug)
	if err != nil {
		return err
	}
	frozenVoiceID := ""
	var frozenLanguage Language
	if len(raw) > 0 && len(raw[0]) > 0 {
		if snapshot, decodeErr := decodeExperimentSnapshot(raw[0], "write"); decodeErr == nil {
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
	if err := s.posts.SetGeneratedContent(ctx, userID, postSlug, content, frozenLanguage); err != nil {
		return err
	}
	s.recordVersionSample(ctx, userID, frozenVoiceID, content)
	return nil
}

// SnapshotVoice reports the voice a frozen write snapshot was taken for, so the experiment
// aggregate can record it without decoding the generation context's private format.
func SnapshotVoice(raw []byte) string {
	snapshot, err := decodeExperimentSnapshot(raw, "write")
	if err != nil {
		return ""
	}
	return snapshot.Post.Voice.ID
}

// SnapshotTemplateName reports the template a frozen write snapshot was taken for, by name.
// The name, not the id: the comparison detail has to keep saying which brief both candidates
// were given even after that template is renamed or deleted.
func SnapshotTemplateName(raw []byte) string {
	snapshot, err := decodeExperimentSnapshot(raw, "write")
	if err != nil || snapshot.Post.Template == nil {
		return ""
	}
	return snapshot.Post.Template.Name
}

// SnapshotTargetLanguage exposes only the frozen canonical language required by the
// experiment aggregate's detail projection. The snapshot wire shape stays generation-owned.
func SnapshotTargetLanguage(raw []byte) Language {
	snapshot, err := decodeExperimentSnapshot(raw, "write")
	if err != nil {
		return ""
	}
	return snapshot.TargetLanguage
}

func (s *Service) ApplyObservationWinner(ctx context.Context, userID, postSlug string, observations []Observation) error {
	return s.posts.SetObservations(ctx, userID, postSlug, observations)
}

func decodeExperimentSnapshot(raw []byte, kind string) (experimentSnapshot, error) {
	var snapshot experimentSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil || snapshot.Kind != kind {
		return experimentSnapshot{}, fmt.Errorf("saved %s input is unavailable", kind)
	}
	if kind == "write" {
		// Write snapshots persisted before the language field existed retain Korean on
		// retry. New snapshots cannot omit it because SnapshotWriteInput validates first.
		if snapshot.TargetLanguage == "" {
			snapshot.TargetLanguage = snapshot.Post.TargetLanguage
		}
		if snapshot.TargetLanguage == "" {
			snapshot.TargetLanguage = LanguageKorean
		}
		if !snapshot.TargetLanguage.Valid() {
			return experimentSnapshot{}, ErrLanguageRequired
		}
		if snapshot.Post.TargetLanguage != "" && snapshot.Post.TargetLanguage != snapshot.TargetLanguage {
			return experimentSnapshot{}, ErrLanguageRequired
		}
		snapshot.Post.TargetLanguage = snapshot.TargetLanguage
	}
	return snapshot, nil
}

func decodeObserveExperimentSnapshot(raw []byte) (observeExperimentSnapshot, error) {
	var snapshot observeExperimentSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil || snapshot.Kind != "observe" {
		return observeExperimentSnapshot{}, fmt.Errorf("saved observe input is unavailable")
	}
	return snapshot, nil
}

func candidateUsage(usage llm.Usage) CandidateUsage {
	return CandidateUsage{PromptTokens: int64(usage.PromptTokens), CompletionTokens: int64(usage.CompletionTokens), CostMicrousd: usage.CostMicrousd, CostReported: usage.CostReported}
}

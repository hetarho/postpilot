package generation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

const (
	WriteExperimentPromptVersion   = "write-v2-ab"
	ObserveExperimentPromptVersion = "observe-v1-ab"
)

type experimentSnapshot struct {
	Kind         string        `json:"kind"`
	Prepared     bool          `json:"prepared"`
	ObserveModel string        `json:"observe_model,omitempty"`
	Post         PostInput     `json:"post"`
	Profile      Profile       `json:"profile,omitempty"`
	Observations []Observation `json:"observations,omitempty"`
}

type CandidateUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	CostMicrousd     int64
	CostReported     bool
}

func (s *Service) SnapshotWriteInput(ctx context.Context, userID, postSlug string, observeModel llm.ModelRef) ([]byte, error) {
	post, err := s.posts.AttachedImages(ctx, userID, postSlug)
	if err != nil {
		return nil, err
	}
	profile, err := s.profiles.ProfileForPrompt(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load voice profile: %w", err)
	}
	if len(post.Images) > 0 {
		info, ok := s.models.Resolve(observeModel)
		if !ok || info.Disabled || !info.Vision {
			return nil, ErrObserveModelRequired
		}
	}
	return json.Marshal(experimentSnapshot{
		Kind: "write", ObserveModel: observeModel.String(), Post: post, Profile: profile,
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
	return json.Marshal(experimentSnapshot{Kind: "observe", Post: post})
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
		observations, _, err := s.observeCandidate(ctx, snapshot.Post, model, progress, true)
		if err != nil {
			return nil, err
		}
		snapshot.Observations = observations
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
	snapshot, err := decodeExperimentSnapshot(raw, "observe")
	if err != nil {
		return nil, CandidateUsage{}, err
	}
	observations, usage, err := s.observeCandidate(ctx, snapshot.Post, model, progress, false)
	return observations, candidateUsage(usage), err
}

func (s *Service) ApplyWriteWinner(ctx context.Context, userID, postSlug string, content PostContent) error {
	return s.posts.SetGeneratedContent(ctx, userID, postSlug, content)
}

func (s *Service) ApplyObservationWinner(ctx context.Context, userID, postSlug string, observations []Observation) error {
	return s.posts.SetObservations(ctx, userID, postSlug, observations)
}

func decodeExperimentSnapshot(raw []byte, kind string) (experimentSnapshot, error) {
	var snapshot experimentSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil || snapshot.Kind != kind {
		return experimentSnapshot{}, fmt.Errorf("saved %s input is unavailable", kind)
	}
	return snapshot, nil
}

func candidateUsage(usage llm.Usage) CandidateUsage {
	return CandidateUsage{PromptTokens: int64(usage.PromptTokens), CompletionTokens: int64(usage.CompletionTokens), CostMicrousd: usage.CostMicrousd, CostReported: usage.CostReported}
}

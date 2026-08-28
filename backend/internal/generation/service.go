package generation

import (
	"context"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

type Service struct {
	posts     Posts
	profiles  Profiles
	models    LLM
	images    ImageReader
	jobs      Jobs
	batchSize int
}

func NewService(posts Posts, profiles Profiles, models LLM, images ImageReader, jobs Jobs, batchSize int) *Service {
	if batchSize <= 0 {
		panic("generation: batch size must be positive")
	}
	return &Service{posts: posts, profiles: profiles, models: models, images: images, jobs: jobs, batchSize: batchSize}
}

func (s *Service) Start(ctx context.Context, request StartRequest) (string, error) {
	post, err := s.posts.AttachedImages(ctx, request.UserID, request.PostSlug)
	if err != nil {
		return "", err
	}
	write, ok := parseModelRef(request.WriteModel)
	if !ok || !modelEnabled(s.models, write) {
		return "", ErrWriteModelRequired
	}
	if len(post.Images) == 0 {
		request.ObserveModel = ""
	} else {
		observe, valid := parseModelRef(request.ObserveModel)
		info, found := s.models.Resolve(observe)
		if !valid || !found || info.Disabled || !info.Vision {
			return "", ErrObserveModelRequired
		}
	}
	id, err := s.jobs.EnqueueGeneration(ctx, request)
	if err != nil {
		return "", fmt.Errorf("enqueue generation: %w", err)
	}
	return id, nil
}

func (s *Service) GetJob(ctx context.Context, id, userID string) (*JobSummary, error) {
	return s.jobs.GetGeneration(ctx, id, userID)
}

func modelEnabled(models LLM, ref llm.ModelRef) bool {
	info, ok := models.Resolve(ref)
	return ok && !info.Disabled
}

func parseModelRef(value string) (llm.ModelRef, bool) {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return llm.ModelRef{}, false
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}, true
}

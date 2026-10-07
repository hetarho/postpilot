package provider

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

// RegisteredTestModel returns registry metadata. Execution rights are checked
// for the explicitly selected stage by PrepareTestModel before admission.
func (s *Service) RegisteredTestModel(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return s.catalog.Lookup(ref)
}
func (s *Service) PrepareTestModel(ctx context.Context, user string, stage Stage, ref llm.ModelRef) (llm.ModelInfo, error) {
	if err := s.validateRef(ctx, user, stage, ref); err != nil {
		return llm.ModelInfo{}, err
	}
	info, ok := s.catalog.Lookup(ref)
	if !ok {
		return llm.ModelInfo{}, ErrModelNotRegistered
	}
	return info, nil
}

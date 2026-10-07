package app

import (
	"context"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

type TestPostMaterials interface {
	AttachedImages(context.Context, string, string) (post.Post, error)
	WritingTestAttachments(context.Context, string, []string) ([]post.WritingTestAttachment, error)
	ValidateWritingTestMaterial(post.WritingTestMaterial) error
}
type TestModelDirectory interface {
	RegisteredTestModel(llm.ModelRef) (llm.ModelInfo, bool)
	PrepareTestModel(context.Context, string, provider.Stage, llm.ModelRef) (llm.ModelInfo, error)
}
type TestVoiceProfiles interface {
	FreezeTestProfile(context.Context, string, string, string, voice.Language, string) (voice.FrozenTestProfile, error)
}
type ResolverDependencies struct {
	Posts              TestPostMaterials
	Models             TestModelDirectory
	Voices             TestVoiceProfiles
	Templates          *template.Authoring
	Guidelines         *guideline.Service
	GuidelineAuthoring *guideline.Authoring
	Candidates         authoring.TestCandidates
	Styles             voice.StyleFactory
}
type WritingTestResolvers struct{ dependencies ResolverDependencies }

func NewWritingTestResolvers(d ResolverDependencies) *WritingTestResolvers {
	if d.Posts == nil || d.Models == nil || d.Voices == nil || d.Templates == nil || d.Guidelines == nil || d.GuidelineAuthoring == nil || d.Candidates == nil || d.Styles == nil {
		panic("experiment/app: every writing-test owner resolver is required")
	}
	return &WritingTestResolvers{dependencies: d}
}

// Compile-time ports are supplied by the subsequent owner-backed method files.
var _ generation.WritingTestSources = (*WritingTestResolvers)(nil)
var _ generation.WritingTestModels = (*WritingTestResolvers)(nil)
var _ generation.WritingTestProfiles = (*WritingTestResolvers)(nil)
var _ generation.WritingTestTemplates = (*WritingTestResolvers)(nil)
var _ generation.WritingTestGuidelines = (*WritingTestResolvers)(nil)
var _ generation.WritingTestCandidates = (*WritingTestResolvers)(nil)
var _ experiment.TestVariants = (*WritingTestResolvers)(nil)

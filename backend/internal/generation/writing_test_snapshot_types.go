package generation

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/llm"
)

var (
	ErrWritingTestCount     = errors.New("writing test requires exactly two, four, eight or sixteen entrants")
	ErrWritingTestFactor    = errors.New("writing test factor is invalid")
	ErrWritingTestReference = errors.New("writing test requires an owned versioned reference")
	ErrWritingTestMaterial  = errors.New("writing test material is invalid")
	ErrWritingTestRevision  = errors.New("writing test source revision changed")
	ErrWritingTestDuplicate = errors.New("writing test repeats a semantic entrant")
)

// WritingTestMaterialRequest carries explicit material/options and owned identities.
// It contains no profile, rendered template, observation, rule or private storage key.
type WritingTestMaterialRequest struct {
	Material                             string
	Fictional                            bool
	AttachmentIDs                        []string
	ObserveFiles                         *[]string
	ObserveModel, WriteModel             llm.ModelRef
	VoiceID, TemplateID, GuidelineSlotID string
	TargetLanguage                       Language
	TargetLength                         *int
	TagCount                             int
	TemplateAnswers                      []TemplateAnswer
	UseMemory                            bool
	QualityRules                         []string
}

type WritingTestReference struct {
	SourceKind                               string
	Model                                    llm.ModelRef
	SettingKind, SettingID, SettingRevision  string
	AuthoringSessionID, AuthoringCandidateID string
	AuthoringRevision                        uint32
}

// Stable identities/fingerprints are supplied by the owning material context.
// Image.Key is an authorized object identity, never a signed runtime URL.
type WritingTestAttachment struct {
	ID, Fingerprint string
	Image           Image
}

type WritingTestSource struct {
	Post                           PostInput
	InputRevision, ContentRevision int64
	Revision, AssignmentsHash      string
	Attachments                    []WritingTestAttachment
}

type WritingTestSources interface {
	ResolveWritingTestSource(context.Context, string, string, int64, int64, WritingTestMaterialRequest) (WritingTestSource, error)
}

// Model admission rights are owner-specific and cannot be inferred from Resolve.
// PromptTokens is the same bounded input allowance the quote and execution admit.
type WritingTestModel struct {
	Info         llm.ModelInfo
	PromptTokens int
}

type WritingTestModels interface {
	PrepareWritingTestModel(context.Context, string, string, llm.ModelRef) (WritingTestModel, error)
}

type WritingTestPreparedSetting struct {
	Kind, Revision                     string
	Name, Description, Body, TitleArea string
	Payload                            []byte
	Synthetic                          bool
}

type WritingTestCandidates interface {
	ResolveWritingTestCandidate(context.Context, string, WritingTestReference) (WritingTestPreparedSetting, error)
}

type WritingTestVoice struct {
	Voice                 VoiceRef
	Profile               Profile
	Revision, SemanticKey string
	Synthetic             bool
	Payload               []byte
}

type WritingTestProfiles interface {
	ResolveWritingTestProfile(context.Context, string, WritingTestReference, *WritingTestPreparedSetting, Language, string) (WritingTestVoice, error)
}

type WritingTestTemplateField struct {
	Label, Flavor string
	Required      bool
}

type WritingTestTemplate struct {
	ID, Name, Revision, SemanticKey string
	Fields                          []WritingTestTemplateField
	Payload                         []byte
}

type WritingTestTemplates interface {
	ResolveWritingTestTemplate(context.Context, string, WritingTestReference, *WritingTestPreparedSetting) (WritingTestTemplate, error)
	RenderWritingTestTemplate(context.Context, string, WritingTestTemplate, bool, []TemplateAnswer) (TemplateBrief, error)
}

type WritingTestRule struct {
	ID, Name, Revision, Text, SemanticKey string
	Payload                               []byte
}

type WritingTestRules struct {
	Defaults []string
	Owner    []WritingTestRule
	Stock    []StockGuideline
}

type WritingTestGuidelines interface {
	FreezeWritingTestGuidelines(context.Context, string, string, string, Language, bool) (WritingTestRules, error)
	ResolveWritingTestGuideline(context.Context, string, WritingTestReference, *WritingTestPreparedSetting) (WritingTestRule, error)
}

type WritingTestFactoryDeps struct {
	Sources    WritingTestSources
	Models     WritingTestModels
	Profiles   WritingTestProfiles
	Templates  WritingTestTemplates
	Guidelines WritingTestGuidelines
	Candidates WritingTestCandidates
}

type WritingTestFactory struct {
	service *Service
	deps    WritingTestFactoryDeps
}

func NewWritingTestFactory(service *Service, deps WritingTestFactoryDeps) *WritingTestFactory {
	if service == nil || deps.Sources == nil || deps.Models == nil || deps.Profiles == nil || deps.Templates == nil || deps.Guidelines == nil || deps.Candidates == nil {
		panic("generation: writing test factory requires all owned behavior ports")
	}
	return &WritingTestFactory{service: service, deps: deps}
}

// Private execution DTOs use the existing generation snapshot's edge mappers.
type writingTestCommon struct {
	Version                                                         int
	Factor, ModelStage                                              string
	Post                                                            PostInput
	Profile                                                         Profile
	ObserveModel                                                    llm.ModelRef
	ObserveFiles                                                    *[]string
	Observations                                                    []Observation
	Prepared, Fictional                                             bool
	BatchSize                                                       int
	ObserveCompletionTokens, ObservePromptTokens, WritePromptTokens int
	ObserveStructuredOutput                                         bool
	Reasoning                                                       ReasoningPolicy
	PromptVersion, SchemaVersion                                    string
	SourceRevision, AssignmentsHash                                 string
	VoiceRevision, TemplateRevision                                 string
	InputRevision, ContentRevision                                  int64
	Attachments                                                     []WritingTestAttachment
	Rules                                                           WritingTestRules
	RequiredFields                                                  []WritingTestTemplateField
	QualityRuleIDs                                                  []string
}

type writingTestVariant struct {
	Reference                                      WritingTestReference
	Revision, SemanticKey                          string
	Synthetic                                      bool
	Snapshot                                       writeSnapshot
	ObserveModel, WriteModel                       llm.ModelRef
	ObservePromptTokens, ObserveCompletionTokens   int
	WriteCompletionTokens, WritePromptTokens       int
	ObserveStructuredOutput, WriteStructuredOutput bool
	Payload                                        []byte
}

type WritingTestCall struct {
	Ref                                   llm.ModelRef
	Stage                                 string
	Count, PromptTokens, CompletionTokens int
}

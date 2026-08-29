package generation

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

type ImageReader interface {
	Read(ctx context.Context, key string) ([]byte, error)
}

type Posts interface {
	AttachedImages(ctx context.Context, userID, slug string) (PostInput, error)
	SetObservations(ctx context.Context, userID, slug string, observations []Observation) error
	SetGeneratedContent(ctx context.Context, userID, slug string, content PostContent) error
}

type Profiles interface {
	ProfileForPrompt(ctx context.Context, userID string) (Profile, error)
}

type RuleWriter interface {
	AppendRule(ctx context.Context, userID, line string) error
}

type LLM interface {
	Resolve(ref llm.ModelRef) (llm.ModelInfo, bool)
	Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error)
}

type Jobs interface {
	EnqueueGeneration(ctx context.Context, request StartRequest) (string, error)
	EnqueueRevision(ctx context.Context, request StartRevisionRequest, payload []byte) (string, error)
	GetGeneration(ctx context.Context, id, userID string) (*JobSummary, error)
}

type ExperimentStarter interface {
	StartWrite(ctx context.Context, request StartExperimentRequest) (StartExperimentResult, error)
}

type Progress func(stage string, done, total int)

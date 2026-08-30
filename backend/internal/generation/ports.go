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

// Profiles projects exactly the post's voice; the voice context never falls back to a
// sibling voice, so an empty voice prompts as empty.
type Profiles interface {
	ProfileForPrompt(ctx context.Context, userID, voiceID string) (Profile, error)
}

type TopicProfiles interface {
	ProfileForPromptForTopic(ctx context.Context, userID, voiceID, topic string, tags []string) (Profile, error)
}

type RuleWriter interface {
	AppendRule(ctx context.Context, userID, voiceID, line string) error
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

// PendingExperiments is the experiment context's published post guard. Generation
// asks only whether unresolved write output exists; it never reads experiment rows.
type PendingExperiments interface {
	PendingForPost(ctx context.Context, userID, postSlug string) (string, error)
}

// PurposeBriefs is the purpose context's published brief lookup, consumed only at enqueue
// time. `ok` false means the post has no purpose or it was deleted between the save and the
// start — an ordinary case, not an error, because a prompt without a brief is a valid one.
type PurposeBriefs interface {
	BriefFor(ctx context.Context, userID, purposeID string) (PurposeBrief, bool, error)
}

type Progress func(stage string, done, total int)

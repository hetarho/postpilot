package voice

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Store is persistence behavior owned by the voice context. SQL rows stop in store/.
type Store interface {
	GetProfile(ctx context.Context, userID string) (Profile, error)
	UpsertProfile(ctx context.Context, profile Profile) error
	SetStyleguideIfCorpusVersion(ctx context.Context, userID, styleguide string, version int64, now time.Time) (bool, error)
	SetRules(ctx context.Context, userID, rules string, now time.Time) error
	InsertSample(ctx context.Context, sample Sample) error
	ListSamples(ctx context.Context, userID string) ([]Sample, error)
	ListSampleBodies(ctx context.Context, userID string) ([]Sample, error)
	GetSampleBody(ctx context.Context, userID, sampleID string) (*Sample, error)
	CorpusSnapshot(ctx context.Context, userID string) ([]Sample, int64, error)
	DeleteSample(ctx context.Context, userID, sampleID string, now time.Time) (bool, error)
	CountSamples(ctx context.Context, userID string) (int, error)
}

// Models resolves the acting user's current analyze selection and performs calls
// through the provider-neutral llm boundary.
type Models interface {
	AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error)
	Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error)
}

// Jobs is the shared queue behavior this context consumes. Its types are defined here;
// the composition root adapts the job context without coupling sibling domains.
type Jobs interface {
	Enqueue(ctx context.Context, request AnalysisJobRequest) (string, error)
	ActiveForUserKind(ctx context.Context, userID, kind string) (*ActiveJob, error)
}

type Progress func(stage string, done, total int)

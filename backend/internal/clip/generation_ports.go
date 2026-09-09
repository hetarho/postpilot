package clip

import (
	"context"
	"github.com/postpilot/backend/internal/llm"
	"io"
	"time"
)

type CompletionBudgets struct{ Observe, Plan int }
type Planner interface {
	ValidateModels(llm.ModelRef, llm.ModelRef) error
	Budgets() CompletionBudgets
	ObserveChunk(context.Context, llm.ModelRef, ChunkInput) (ChunkAnalysis, llm.Usage, error)
	Plan(context.Context, llm.ModelRef, PlanningInput) (EditPlan, llm.Usage, error)
}
type GenerationStore interface {
	GetSourceBatch(context.Context, string, string) (SourceBatch, error)
	LinkSourceJob(context.Context, string, string, string, time.Time) error
	BatchForJob(context.Context, string, string) (SourceBatch, error)
	ListConsumingBatches(context.Context) ([]SourceBatch, error)
	AddProxy(context.Context, string, string, string) error
	RemoveProxy(context.Context, string) error
	SaveGeneration(context.Context, string, string, string, string, Result) error
	ResultKeys(context.Context) ([]string, error)
	DeletionKeys(context.Context) ([]string, error)
	RemoveDeletion(context.Context, string) error
}
type GenerationStart struct {
	UserID, ProjectID, Observe, Write string
	Payload                           []byte
}
type ClipJob struct{ ID, Status, Stage string }
type GenerationJobs interface {
	Enqueue(context.Context, GenerationStart) (string, error)
	Activate(context.Context, string, string) error
	FailQueued(context.Context, string, string) (bool, error)
	Reserve(context.Context, string, string, string, string, int, CompletionBudgets) (context.Context, error)
	Active(context.Context, string, string) (*ClipJob, error)
	Get(context.Context, string, string) (*ClipJob, error)
}
type StoredObject struct {
	Key      string
	Modified time.Time
}
type ProcessingObjects interface {
	Download(context.Context, string, io.Writer, int64) (int64, error)
	Upload(context.Context, string, io.ReadSeeker, int64, string) error
	PresignRead(context.Context, string, string, bool, time.Duration) (string, error)
	Delete(context.Context, string) error
	ListResults(context.Context) ([]StoredObject, error)
}
type GenerationConfig struct {
	Media                                 MediaConfig
	Analysis                              AnalysisLimits
	ReadTTL, CleanupTimeout, OrphanMinAge time.Duration
}

const ResultPrefix = "clip-results/"

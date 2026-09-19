package clip

import (
	"context"
	"io"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// JobSubject is what the clip context calls itself when it addresses the job queue: a
// clip job belongs to a project, and the queue matches on the pair without knowing what
// a project is.
const JobSubject = "clip_project"

// The three kinds of clip work. They are the clip context's words, handed to the queue as
// opaque strings: a generation, an owner's revision of a saved plan (CLIP-131), and a
// render that writes nothing and spends nothing (CLIP-19, CLIP-20, CLIP-132).
const (
	JobKindGenerate = "generate_clip"
	JobKindRender   = "render_clip"
	JobKindRevise   = "revise_clip"
)

// SafeJobStage is the stage vocabulary a clip job may have its progress logged under.
// Anything else is an unexpected handler's string and is logged as "unknown".
func SafeJobStage(stage string) string {
	switch stage {
	// `plan` and `plan_retry` are the single writing call this build no longer makes; a
	// job queued before it split keeps a readable stage.
	case "queued", "prepare", "analyze", "analyze_retry", "flow", "flow_retry", "narrate", "narrate_retry", "plan", "plan_retry", "render", "save", "cleanup":
		return stage
	}
	return "unknown"
}

// IsJobKind reports whether a job belongs to the clip surface at all.
func IsJobKind(kind string) bool {
	return kind == JobKindGenerate || kind == JobKindRender || kind == JobKindRevise
}

// ChargedJobKind reports whether a clip job reserves an approved credit ceiling before
// its first model call. A generation and a revision request both do; a render does not.
// Every admission, metering and settlement gate asks this instead of naming the
// generation alone — naming it is what left the revision unable to reserve.
func ChargedJobKind(kind string) bool {
	return kind == JobKindGenerate || kind == JobKindRevise
}

// One budget per call the generation makes: the observation, then the two
// writing calls the assembly contract names (CLIP-135).
type CompletionBudgets struct{ Observe, Flow, Narration int }
type Planner interface {
	ValidateModels(llm.ModelRef, llm.ModelRef) error
	ValidatePreparation(llm.ModelRef, PlanningInput, []AnalysisSource) error
	Budgets() CompletionBudgets
	ObserveChunk(context.Context, llm.ModelRef, ChunkInput) (ChunkAnalysis, llm.Usage, error)
	// The composition writer: the flow call, then the narration over it
	// (CLIP-135). Plan is what a payload without a composition snapshot uses.
	Flow(context.Context, llm.ModelRef, PlanningInput) (EditPlan, llm.Usage, error)
	Narrate(context.Context, llm.ModelRef, NarrationInput) (EditPlan, llm.Usage, error)
	// One owner-written revision of a saved plan (CLIP-131), through the same
	// two contracts.
	Revise(context.Context, llm.ModelRef, RevisionInput) (EditPlan, llm.Usage, error)
	Plan(context.Context, llm.ModelRef, PlanningInput) (EditPlan, llm.Usage, error)
}
type GenerationStore interface {
	GetSourceBatch(context.Context, string, string) (SourceBatch, error)
	LinkRenderSourceJob(context.Context, string, string, string, int, time.Time) error
	SaveCorrection(context.Context, string, string, int, string) (Project, error)
	BatchForJob(context.Context, string, string) (SourceBatch, error)
	ListConsumingBatches(context.Context) ([]SourceBatch, error)
	ResultKeys(context.Context) ([]string, error)
	DeletionKeys(context.Context) ([]string, error)
	RemoveDeletion(context.Context, string) error
}

// A candidate stays separate from the project until its job and result can be
// committed together. The composition root coordinates those owned stores.
type AttemptResult struct {
	JobID, UserID, ProjectID string
	ExpectedRevision         int
	Analysis, EditPlan       string
	Result                   Result
}
type ClipFinisher interface {
	Complete(context.Context, AttemptResult) error
	Recover(context.Context) error
}
type GenerationStart struct {
	UserID, ProjectID, Observe, Write string
	Payload                           []byte
	RenderOnly                        bool
	// One owner-written revision of the saved plan (CLIP-131): charged work
	// with no media in it.
	Revise bool
	Quote  *GenerationQuote
	// What the owner asked the AI for, kept with the project once this start is
	// accepted (CLIP-133). Absent for a re-render, which asks for nothing new.
	Request *ProjectRequest
}
type ClipJob struct {
	FinishedAt              *time.Time
	ID, Status, Stage, Kind string
	Payload                 []byte
	DispatchReady           bool
}
type GenerationJobs interface {
	Enqueue(context.Context, GenerationStart) (string, error)
	Activate(context.Context, string, string) error
	FailQueued(context.Context, string, string) (bool, error)
	ReserveApproved(context.Context, string, string, GenerationApproval, int) (context.Context, error)
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
	Preview                               PreviewConfig
	Render                                RenderConfig
	Media                                 MediaConfig
	Analysis                              AnalysisLimits
	ReadTTL, CleanupTimeout, OrphanMinAge time.Duration
	QuoteTTL                              time.Duration
}

const ResultPrefix = "clip-results/"

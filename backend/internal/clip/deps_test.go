package clip_test

import (
	"context"
	"io"
	"time"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// Neutral collaborators for the clip constructors (ARCH-40): each answers the way the
// context refused before the collaborator existed, so a test that cares about one
// replaces it.
type neutralFinisher struct{}

func (neutralFinisher) Complete(context.Context, clip.AttemptResult) error {
	return clip.ErrCompositionUnavailable
}
func (neutralFinisher) Recover(context.Context) error { return nil }

type neutralPricing struct{}

func (neutralPricing) Freeze(context.Context, llm.ModelRef, llm.ModelRef, int) (clip.GenerationPricing, error) {
	return clip.GenerationPricing{}, clip.ErrPricingUnavailable
}

type neutralAccounting struct{}

func (neutralAccounting) ForJob(context.Context, string, string) (*clip.Accounting, error) {
	return nil, nil
}

type neutralAdmission struct{}

func (neutralAdmission) ObserveModels() []clip.AnalysisCandidate { return nil }
func (neutralAdmission) QualifyObserve(context.Context, llm.ModelRef) error {
	return clip.ErrPricingUnavailable
}

type nullFinalizer struct{}

func (nullFinalizer) Finalize(context.Context, clip.FinalizationRequest) (clip.Project, error) {
	return clip.Project{}, clip.ErrFinalizationInvalid
}

type nullObjects struct{}

func (nullObjects) PresignSource(context.Context, string, string, time.Duration) (clip.SignedSourcePut, error) {
	return clip.SignedSourcePut{}, nil
}
func (nullObjects) PresignSourcePlayback(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (nullObjects) HeadSource(context.Context, string) (clip.SourceObjectInfo, error) {
	return clip.SourceObjectInfo{}, nil
}
func (nullObjects) Delete(context.Context, string) error             { return nil }
func (nullObjects) ListSourceKeys(context.Context) ([]string, error) { return nil, nil }

func neutralGenerationDeps() clipapp.GenerationDeps { return generationDeps(nil, nil, nil) }

// generationDeps fills the collaborators a test leaves nil with the neutral ones.
func generationDeps(f clip.ClipFinisher, p clip.QuotePricing, a clip.AccountingReader) clipapp.GenerationDeps {
	if f == nil {
		f = neutralFinisher{}
	}
	if p == nil {
		p = neutralPricing{}
	}
	if a == nil {
		a = neutralAccounting{}
	}
	return clipapp.GenerationDeps{Finisher: f, Pricing: p, Accounting: a, Admission: neutralAdmission{}}
}

// projectStore is what a project service and its source side both read.
type projectStore interface {
	clip.Store
	clip.SourceStore
}

// testProjects is a project service over a store, with the collaborators no test in this
// package exercises answering neutrally.
func testProjects(store projectStore) *clipapp.Service {
	return clipapp.NewService(store, clip.DefaultLimits(), clipapp.NewSourceService(store, nullObjects{}, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: 6 * time.Hour, PutTTL: 10 * time.Minute})), nullFinalizer{})
}

// neutralJobs is a generation-side job port with no queue behind it: nothing is active,
// nothing can be enqueued.
type neutralJobs struct{}

func (neutralJobs) Enqueue(context.Context, clip.GenerationStart) (string, error) {
	return "", clip.ErrBusy
}
func (neutralJobs) Activate(context.Context, string, string) error           { return nil }
func (neutralJobs) FailQueued(context.Context, string, string) (bool, error) { return false, nil }
func (neutralJobs) ReserveApproved(ctx context.Context, _, _ string, _ clip.GenerationApproval, _ int) (context.Context, error) {
	return ctx, nil
}
func (neutralJobs) Active(context.Context, string, string) (*clip.ClipJob, error) { return nil, nil }
func (neutralJobs) Get(context.Context, string, string) (*clip.ClipJob, error)    { return nil, nil }

// neutralProcessing is a result store that presigns by key alone, so a projection read
// through a bound generation side carries a predictable URL.
type neutralProcessing struct{}

func (neutralProcessing) Download(context.Context, string, io.Writer, int64) (int64, error) {
	return 0, clip.ErrNotFound
}
func (neutralProcessing) Upload(context.Context, string, io.ReadSeeker, int64, string) error {
	return nil
}
func (neutralProcessing) PresignRead(_ context.Context, key, _ string, download bool, _ time.Duration) (string, error) {
	if download {
		return "https://objects.test/" + key + "?download", nil
	}
	return "https://objects.test/" + key, nil
}
func (neutralProcessing) Delete(context.Context, string) error                     { return nil }
func (neutralProcessing) ListResults(context.Context) ([]clip.StoredObject, error) { return nil, nil }

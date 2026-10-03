package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
)

// requestHarness is the real queue over a real database with the real template context on top:
// the credit gate's planned calls, the one-per-account rule and the result on the job row are
// what is under test. Only the provider and the post read are stubbed.
type requestHarness struct {
	d       *db.DB
	queue   *job.Queue
	service *template.Service
	models  *stubRequestModels
	admit   *stubAdmitter
}

type stubRequestModels struct {
	answers []string
	calls   int
	// entered and release hold the first call open, so a test can cancel while it runs.
	entered chan struct{}
	release chan struct{}
}

func (s *stubRequestModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Stages: []string{llm.StageNameWrite}, StructuredOutput: true}, true
}

func (s *stubRequestModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	answer := s.answers[min(s.calls, len(s.answers)-1)]
	s.calls++
	if s.entered != nil && s.calls == 1 {
		close(s.entered)
		<-s.release
	}
	return llm.Response{Text: answer, FinishReason: "stop"}, nil
}

type stubRequestSamples struct{}

func (stubRequestSamples) RequestSample(context.Context, string, string) (template.Sample, error) {
	return template.Sample{Title: "성수 카페", Text: "[사진]"}, nil
}

func newRequestHarness(t *testing.T) *requestHarness {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "request.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(context.Background(), d.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if _, err := d.Writer.Exec("INSERT INTO users(id,password_hash,created_at) VALUES(?,'hash',?)", id, now); err != nil {
			t.Fatal(err)
		}
	}
	queue := job.New(jobstore.New(d.Writer, d.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	admit := &stubAdmitter{}
	queue.Admit(admit)
	queue.AllowCancellation(jobCancellation{})
	models := &stubRequestModels{answers: []string{`{"name":"맛집 리뷰","description":"","title_area":"","body":"<write>방문 이유</write>","wishes":["친근하게"]}`}}
	service := template.NewService(templatestore.New(d.Writer, d.Reader), template.NewLimits(template.Ceilings{
		NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000, TitleAreaMaxChars: 200,
		MaxPerAccount: 50, PhotoRowMax: 4, AskLabelMaxChars: 40, AskMaxPerBody: 10,
	}, postNumberBounds()))
	// The stub replaces the metered registry: this harness tests the queue and the context.
	service.ConfigureRequests(models, stubRequestSamples{}, templateRequestJobs{queue: queue},
		template.RequestLimits{MaxChars: 12000, CorrectionsMax: 3, WishesMax: 5, WishMaxChars: 200})
	return &requestHarness{d: d, queue: queue, service: service, models: models, admit: admit}
}

func (h *requestHarness) start(t *testing.T, userID string) (string, error) {
	t.Helper()
	return h.service.StartRequest(context.Background(), userID, template.StartRequest{
		WriteModel: "p/m", Language: template.LanguageKorean, Text: "맛집 리뷰 템플릿",
	})
}

// QUOTA-67: one admission plans the first call and every correction on the write stage, each at
// the request's own completion cap.
func TestTemplateRequestHoldsEveryCorrectionOnTheWriteStage(t *testing.T) {
	h := newRequestHarness(t)
	if _, err := h.start(t, "alice"); err != nil {
		t.Fatal(err)
	}
	if len(h.admit.holds) != 1 {
		t.Fatalf("holds = %+v", h.admit.holds)
	}
	hold := h.admit.holds[0]
	want := job.PlannedCall{Ref: "p/m", Stage: llm.StageNameWrite, Count: 4, CompletionTokens: template.RequestCompletionBudget}
	if hold.Kind != job.KindTemplateRequest || len(hold.Calls) != 1 || hold.Calls[0] != want {
		t.Fatalf("hold = %+v", hold)
	}
}

// One request per account at a time; another account is not blocked by it.
func TestTemplateRequestIsOnePerAccount(t *testing.T) {
	h := newRequestHarness(t)
	if _, err := h.start(t, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.start(t, "alice"); !errors.Is(err, template.ErrRequestRunning) {
		t.Fatalf("second request = %v, want ErrRequestRunning", err)
	}
	if _, err := h.start(t, "bob"); err != nil {
		t.Fatalf("another account was blocked: %v", err)
	}
}

func TestTemplateRequestRunsAndLeavesItsAnswerOnTheRow(t *testing.T) {
	h := newRequestHarness(t)
	ctx := context.Background()
	id, err := h.start(t, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.RequestResult(ctx, "alice", id); !errors.Is(err, template.ErrRequestNotReady) {
		t.Fatalf("result before the run = %v, want ErrRequestNotReady", err)
	}
	h.queue.Register(job.KindTemplateRequest, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return h.service.RunRequest(ctx, template.RequestRun{
			ID: found.ID, UserID: found.UserID, WriteModel: found.WriteModel, Payload: found.Payload,
		}, progress)
	})
	workerCtx, stop := context.WithCancel(ctx)
	defer stop()
	go h.queue.Run(workerCtx)
	deadline := time.Now().Add(5 * time.Second)
	for {
		summary, err := h.queue.Get(ctx, id, "alice")
		if err != nil {
			t.Fatal(err)
		}
		if job.Terminal(summary.Status) {
			if summary.Status != job.StatusDone {
				t.Fatalf("job finished %s: %+v", summary.Status, summary)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the request did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	result, err := h.service.RequestResult(ctx, "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "맛집 리뷰" || result.Body != "<write>방문 이유</write>" || len(result.Wishes) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if _, err := h.service.RequestResult(ctx, "bob", id); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("another account's read = %v, want ErrNotFound", err)
	}
	if h.models.calls != 1 {
		t.Fatalf("calls = %d", h.models.calls)
	}
}

// A post becomes a sample as its shape: prose as written, photos as positions, and nothing about
// a photo — no file, caption, video, summary or tags (TMPL-64).
func TestSampleTextKeepsTheShapeAndDropsThePhotos(t *testing.T) {
	got := sampleText(&post.PostContent{
		Title: "성수 카페", Summary: "요약은 빠진다", Tags: []string{"태그"},
		Blocks: []post.Block{
			{Type: post.BlockText, Content: "들어가자마자 향이 좋았다"},
			{Type: post.BlockImage, File: "a.jpg", Caption: "캡션은 빠진다"},
			{Type: post.BlockImage, File: "b.jpg"},
			{Type: post.BlockHeading, Content: "메뉴", Level: 2},
			{Type: post.BlockHeading, Content: "디저트", Level: 3},
			{Type: post.BlockList, Items: []string{"라떼", " ", "케이크"}},
			{Type: post.BlockQuote, Content: "또 오고 싶다"},
			{Type: post.BlockVideo, File: "c.mp4", Caption: "영상은 빠진다"},
			{Type: post.BlockGallery, Files: []string{"d.jpg", "e.jpg", "f.jpg"}, Layout: post.GalleryCollage, Caption: "묶음 캡션도 빠진다"},
			// A group larger than the template ceiling reads as a position the parser accepts.
			{Type: post.BlockGallery, Files: []string{"g.jpg", "h.jpg", "i.jpg", "j.jpg", "k.jpg", "l.jpg"}, Layout: post.GallerySlide},
		},
	}, 4)
	want := strings.Join([]string{
		"들어가자마자 향이 좋았다", "[사진]", "[사진]", "## 메뉴", "### 디저트", "- 라떼\n- 케이크", "> 또 오고 싶다",
		"[사진 3장 묶음]", "[사진 4장 묶음]",
	}, "\n\n")
	if got != want {
		t.Fatalf("sampleText =\n%s\nwant\n%s", got, want)
	}
	if sampleText(nil, 4) != "" || sampleText(&post.PostContent{}, 4) != "" {
		t.Fatal("a post with no content must have no sample")
	}
}

type stubRates struct {
	rate plan.RateSnapshot
	err  error
}

func (s stubRates) SelectRate(context.Context) (plan.RateSnapshot, error) { return s.rate, s.err }

// The request box's figure is the post estimate's own pricing of one call: catalog prices at the
// current eligible rate, and no figure without a rate (QUOTA-67).
func TestTemplateEstimatesPriceOneCallAtTheCatalog(t *testing.T) {
	info := llm.ModelInfo{InputUSDPerMillion: "1", OutputUSDPerMillion: "4"}
	rate := plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-29", ReferenceE4: 13_600_000, AppliedE4: 13_600_000}
	credits, ok := templateEstimates{rates: stubRates{rate: rate}}.CallCredits(context.Background(), info, 6_000, 3_000)
	want, wantOK := plan.CallCreditsAt(catalogPricer(info), rate, 6_000, 3_000)
	if !ok || !wantOK || credits != want || credits < 1 {
		t.Fatalf("credits = %d ok=%v, want %d", credits, ok, want)
	}
	if _, ok := (templateEstimates{rates: stubRates{err: errors.New("no rate")}}).CallCredits(context.Background(), info, 6_000, 3_000); ok {
		t.Fatal("a missing rate produced a figure")
	}
}

func (h *requestHarness) work(t *testing.T) {
	t.Helper()
	h.queue.Register(job.KindTemplateRequest, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return h.service.RunRequest(ctx, template.RequestRun{
			ID: found.ID, UserID: found.UserID, WriteModel: found.WriteModel, Payload: found.Payload,
		}, progress)
	})
	workerCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	go h.queue.Run(workerCtx)
}

func (h *requestHarness) waitTerminal(t *testing.T, id string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		summary, err := h.queue.Get(context.Background(), id, "alice")
		if err != nil {
			t.Fatal(err)
		}
		if job.Terminal(summary.Status) {
			return summary.Status
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s did not finish: %+v", id, summary)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TMPL-63: a queued request stops at once, makes no call and leaves nothing to read.
func TestTemplateRequestCancelledWhileQueuedMakesNoCall(t *testing.T) {
	h := newRequestHarness(t)
	ctx := context.Background()
	id, err := h.start(t, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.service.CancelRequest(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	summary, err := h.queue.Get(ctx, id, "alice")
	if err != nil || summary.Status != job.StatusCancelled {
		t.Fatalf("status = %v, %v", summary, err)
	}
	h.work(t)
	time.Sleep(50 * time.Millisecond)
	if h.models.calls != 0 {
		t.Fatalf("a cancelled request called the model %d times", h.models.calls)
	}
	if _, err := h.service.RequestResult(ctx, "alice", id); !errors.Is(err, template.ErrRequestNotReady) {
		t.Fatalf("result of a cancelled request = %v", err)
	}
}

// A running request stops before its next paid call: the answer in flight is the last one.
func TestTemplateRequestCancelledWhileRunningSendsNoCorrection(t *testing.T) {
	h := newRequestHarness(t)
	ctx := context.Background()
	h.models.answers = []string{"not json"}
	h.models.entered, h.models.release = make(chan struct{}), make(chan struct{})
	id, err := h.start(t, "alice")
	if err != nil {
		t.Fatal(err)
	}
	h.work(t)
	<-h.models.entered
	if err := h.service.CancelRequest(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	close(h.models.release)
	if status := h.waitTerminal(t, id); status != job.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", status)
	}
	if h.models.calls != 1 {
		t.Fatalf("calls = %d: a correction followed the cancellation", h.models.calls)
	}
}

// A finished request is left as it is, and another account cannot reach it.
func TestTemplateRequestCancelAfterItFinishedChangesNothing(t *testing.T) {
	h := newRequestHarness(t)
	ctx := context.Background()
	id, err := h.start(t, "alice")
	if err != nil {
		t.Fatal(err)
	}
	h.work(t)
	if status := h.waitTerminal(t, id); status != job.StatusDone {
		t.Fatalf("status = %s", status)
	}
	if err := h.service.CancelRequest(ctx, "alice", id); err != nil {
		t.Fatalf("cancel after done = %v", err)
	}
	if summary, _ := h.queue.Get(ctx, id, "alice"); summary.Status != job.StatusDone {
		t.Fatalf("status after a late cancel = %s", summary.Status)
	}
	if _, err := h.service.RequestResult(ctx, "alice", id); err != nil {
		t.Fatalf("the result was lost: %v", err)
	}
	if err := h.service.CancelRequest(ctx, "bob", id); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("another account's cancel = %v, want ErrNotFound", err)
	}
}

// The template context caps a photo position's suggested group at its own copy of the post's
// group cap, since it may not import post (TMPL-38, GEN-77); the two must stay one number.
func TestTemplateGroupCapIsThePostGroupCap(t *testing.T) {
	if template.PhotoGroupCap != post.PhotoGroupMax {
		t.Fatalf("template cap %d, post cap %d", template.PhotoGroupCap, post.PhotoGroupMax)
	}
}

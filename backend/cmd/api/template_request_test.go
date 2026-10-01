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
}

func (s *stubRequestModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Stages: []string{llm.StageNameWrite}, StructuredOutput: true}, true
}

func (s *stubRequestModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	answer := s.answers[min(s.calls, len(s.answers)-1)]
	s.calls++
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
		},
	})
	want := strings.Join([]string{
		"들어가자마자 향이 좋았다", "[사진]", "[사진]", "## 메뉴", "### 디저트", "- 라떼\n- 케이크", "> 또 오고 싶다",
	}, "\n\n")
	if got != want {
		t.Fatalf("sampleText =\n%s\nwant\n%s", got, want)
	}
	if sampleText(nil) != "" || sampleText(&post.PostContent{}) != "" {
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

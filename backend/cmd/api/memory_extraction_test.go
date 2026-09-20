package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/memory"
	memorystore "github.com/postpilot/backend/internal/memory/store"
	"github.com/postpilot/backend/internal/platform/db"
)

// extractionHarness is the real queue over a real database with the real memory context on
// top: the credit gate, the guards and the job row are the things under test here, so none
// of them is faked. Only the provider and the post read are.
type extractionHarness struct {
	d       *db.DB
	queue   *job.Queue
	service *memory.Service
	models  *stubExtractionModels
	admit   *stubAdmitter
}

type stubExtractionModels struct {
	answer string
	calls  int
}

func (s *stubExtractionModels) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return llm.ModelRef{ProviderID: "p", ModelID: "m"}, true, nil
}

func (s *stubExtractionModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{StructuredOutput: true}, true
}

func (s *stubExtractionModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	s.calls++
	return llm.Response{Text: s.answer}, nil
}

type stubExtractionPosts struct{}

func (stubExtractionPosts) ExtractionSource(_ context.Context, _, slug string) (memory.ExtractionSource, error) {
	return memory.ExtractionSource{PostSlug: slug, Title: "제주", Memo: "성산에서 아침", Body: "바다를 봤다."}, nil
}

// stubAdmitter is the credit gate. `refuse` is what an account without the balance meets,
// and the gate runs BEFORE the row is written — which is the thing this asserts.
type stubAdmitter struct {
	refuse error
	holds  []job.Start
}

func (a *stubAdmitter) Hold(_ context.Context, start job.Start) error {
	if a.refuse != nil {
		return a.refuse
	}
	a.holds = append(a.holds, start)
	return nil
}

func (a *stubAdmitter) Release(context.Context, string)             {}
func (a *stubAdmitter) Settle(context.Context, string, string)      {}
func (a *stubAdmitter) OpenHolds(context.Context) ([]string, error) { return nil, nil }

func newExtractionHarness(t *testing.T) *extractionHarness {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "extract.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if _, err := d.Writer.Exec("INSERT INTO users(id,password_hash,created_at) VALUES(?,'hash',?)", id, now); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Writer.Exec("INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,'기본',1,?,?)", "voice-"+id, id, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Writer.Exec("INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES(?,?,?,?,?)", id+"-post", id, "voice-"+id, now, now); err != nil {
			t.Fatal(err)
		}
	}
	queue := job.New(jobstore.New(d.Writer, d.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	admit := &stubAdmitter{}
	queue.Admit(admit)
	models := &stubExtractionModels{answer: `{"candidates":[{"text":"성산 일출봉을 좋아한다","kind":"place","tags":["제주","성산"]}]}`}
	service := memory.NewService(
		memorystore.New(d.Writer, d.Reader),
		memory.Limits{TextMaxChars: 120, TagsMax: 5, MaxPerAccount: 300, InjectMax: 8},
	)
	service.ConfigureExtraction(models, stubExtractionPosts{}, memoryExtractions{queue: queue})
	return &extractionHarness{d: d, queue: queue, service: service, models: models, admit: admit}
}

func (h *extractionHarness) jobRows(t *testing.T) int {
	t.Helper()
	var rows int
	if err := h.d.Reader.QueryRow(`SELECT count(*) FROM generation_jobs`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

// QUOTA-13: the extraction passes the shared enqueue gate like every other LLM job, and a
// refusal there leaves no job row behind — the user is not shown work that will never run.
func TestMemoryExtractionIsRefusedByTheCreditGateBeforeAnyRow(t *testing.T) {
	h := newExtractionHarness(t)
	h.admit.refuse = errors.New("insufficient credits")

	if _, err := h.service.StartExtraction(context.Background(), "alice", "alice-post"); err == nil {
		t.Fatal("the gate admitted an account it should have refused")
	}
	if rows := h.jobRows(t); rows != 0 {
		t.Fatalf("job rows = %d after a refused start, want none", rows)
	}
	if h.models.calls != 0 {
		t.Fatalf("a refused start still called the provider %d times", h.models.calls)
	}
}

// The post-dimension guard: one job at a time on a post, so an extraction cannot start
// while a generation is running on the same post — and the caller is told which job is.
func TestMemoryExtractionIsGuardedByOtherWorkOnThePost(t *testing.T) {
	h := newExtractionHarness(t)
	ctx := context.Background()
	subjects, guards := postVoiceWork(job.KindGenerate, "alice", "alice-post", "voice-alice")
	running, err := h.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindGenerate, UserID: "alice", Subjects: subjects, Guards: guards,
		WriteModel: "p/m", TargetLanguage: "ko",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = h.service.StartExtraction(ctx, "alice", "alice-post")
	var active *memory.ExtractionInProgressError
	if !errors.As(err, &active) || active.ActiveID != running {
		t.Fatalf("start during other work = %v, want the active job %s", err, running)
	}

	// Another account's post is not blocked by it.
	if _, err := h.service.StartExtraction(ctx, "bob", "bob-post"); err != nil {
		t.Fatalf("a second account was blocked by the first: %v", err)
	}
}

// The whole path, driven through completion: the job runs, the candidates land on its row,
// and the memory tables are still empty (MEM-14, MEM-15).
func TestACompletedExtractionLeavesTheMemoryTablesEmpty(t *testing.T) {
	h := newExtractionHarness(t)
	ctx := context.Background()
	id, err := h.service.StartExtraction(ctx, "alice", "alice-post")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.admit.holds) != 1 || h.admit.holds[0].Kind != job.KindExtractMemory {
		t.Fatalf("holds = %+v, want one for the extraction", h.admit.holds)
	}

	h.queue.Register(job.KindExtractMemory, func(ctx context.Context, found job.Job, progress job.Progress) error {
		source, err := memory.DecodeExtractionSource(found.Payload)
		if err != nil {
			return err
		}
		if source.Body == "" || source.PostSlug != "alice-post" {
			t.Errorf("the job row did not carry the frozen post: %+v", source)
		}
		return h.service.Extract(ctx, memory.ExtractionJob{
			ID: found.ID, UserID: found.UserID, PostSlug: source.PostSlug,
			Model: found.WriteModel, Source: source,
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
			t.Fatal("the extraction did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}

	slug, candidates, err := h.service.Extraction(ctx, "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	if slug != "alice-post" || len(candidates) != 1 || candidates[0].Kind != memory.KindPlace {
		t.Fatalf("candidates = %q %+v", slug, candidates)
	}
	// Nothing was stored. An approval is the user's own CreateMemory, and until then the
	// proposal exists only on the job row.
	for _, table := range []string{"memories", "memory_tags", "memory_sources"} {
		var rows int
		if err := h.d.Reader.QueryRow(`SELECT count(*) FROM ` + table).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != 0 {
			t.Fatalf("%s holds %d rows after an extraction", table, rows)
		}
	}
	// And the post it read is untouched.
	var status string
	var revision int64
	if err := h.d.Reader.QueryRow(`SELECT status, content_revision FROM posts WHERE slug='alice-post'`).Scan(&status, &revision); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || revision != 0 {
		t.Fatalf("the post moved: status %s, revision %d", status, revision)
	}

	// A second account cannot read the result, and an unknown id reads the same way.
	if _, _, err := h.service.Extraction(ctx, "bob", id); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("a foreign read = %v, want ErrNotFound", err)
	}
	if _, _, err := h.service.Extraction(ctx, "alice", "no-such-job"); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("an unknown id = %v, want ErrNotFound", err)
	}
}

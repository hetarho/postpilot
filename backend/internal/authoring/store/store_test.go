package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/authoring"
	authorstore "github.com/postpilot/backend/internal/authoring/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
)

var once sync.Once
var basePath string
var baseError error

func database(t *testing.T) *db.DB {
	t.Helper()
	once.Do(func() {
		dir, e := os.MkdirTemp("", "authoring-sqlite-baseline")
		if e != nil {
			baseError = e
			return
		}
		basePath = filepath.Join(dir, "base.db")
		d, e := db.Open(basePath)
		if e != nil {
			baseError = e
			return
		}
		defer d.Close()
		baseError = db.Migrate(context.Background(), d.Writer)
		if baseError != nil {
			return
		}
		_, baseError = d.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES ('alice','hash','2026-10-07T00:00:00Z'),('bob','hash','2026-10-07T00:00:00Z')`)
	})
	if baseError != nil {
		t.Fatal(baseError)
	}
	data, e := os.ReadFile(basePath)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "test.db")
	if e = os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
	d, e := db.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

type models struct {
	mu    sync.Mutex
	text  string
	calls int
	after func()
}

func (m *models) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Ref: ref, Stages: []string{"write"}, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0", ContextTokens: 131072, StructuredOutput: true}, true
}
func (m *models) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	m.mu.Lock()
	m.calls++
	text, after := m.text, m.after
	m.mu.Unlock()
	if after != nil {
		after()
	}
	return llm.Response{Text: text}, nil
}

type jobRecord struct {
	owner string
	authoring.Job
}
type jobs struct {
	mu       sync.Mutex
	rows     map[string]jobRecord
	enqueues int
}

func (j *jobs) Enqueue(_ context.Context, in authoring.JobRequest) (string, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, r := range j.rows {
		if r.owner == in.UserID && (r.Status == "queued" || r.Status == "running") {
			return "", authoring.ErrBusy
		}
	}
	j.enqueues++
	id := fmt.Sprintf("job-%d", j.enqueues)
	j.rows[id] = jobRecord{in.UserID, authoring.Job{ID: id, Status: "queued", WriteModel: in.WriteModel, Payload: append([]byte(nil), in.Payload...)}}
	return id, nil
}
func (j *jobs) Get(_ context.Context, owner, id string) (authoring.Job, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	r, ok := j.rows[id]
	if !ok || r.owner != owner {
		return authoring.Job{}, authoring.ErrNotFound
	}
	out := r.Job
	out.Payload = append([]byte(nil), out.Payload...)
	return out, nil
}
func (j *jobs) Latest(ctx context.Context, owner string) (*authoring.Job, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	var result *authoring.Job
	for _, r := range j.rows {
		if r.owner == owner && (result == nil || r.ID > result.ID) {
			copy := r.Job
			result = &copy
		}
	}
	return result, nil
}
func (j *jobs) SaveResult(_ context.Context, id string, payload []byte) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	r := j.rows[id]
	if r.Status == "cancelled" {
		return context.Canceled
	}
	r.Payload = append([]byte(nil), payload...)
	j.rows[id] = r
	return nil
}
func (j *jobs) Cancel(_ context.Context, owner, id string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	r, ok := j.rows[id]
	if !ok || r.owner != owner {
		return authoring.ErrNotFound
	}
	if r.Status != "done" && r.Status != "failed" {
		r.Status = "cancelled"
		j.rows[id] = r
	}
	return nil
}
func (j *jobs) status(id, status string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	r := j.rows[id]
	r.Status = status
	j.rows[id] = r
}

type targets struct {
	mu              sync.Mutex
	seeds           int
	creates         int
	failAfterCommit bool
	saved           map[string]authoring.SavedRef
	defaults        map[string]bool
}

func (t *targets) Seed(_ context.Context, owner string, kind authoring.Kind, target string) (authoring.Seed, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seeds++
	if target == "foreign" {
		return authoring.Seed{}, authoring.ErrNotFound
	}
	if target == "owned" {
		body := "old body"
		if kind == authoring.WritingVoice {
			body = ""
		}
		return authoring.Seed{Artifact: &authoring.Artifact{ID: "seed", Name: "원래 설정", Description: "현재 인상", Body: body}, TargetVersion: "version-one", ForkVoice: kind == authoring.WritingVoice}, nil
	}
	return authoring.Seed{}, nil
}
func (t *targets) CanStart(context.Context, string, authoring.Kind, string) error { return nil }
func (t *targets) Validate(kind authoring.Kind, a authoring.Artifact) error {
	if a.Body == "" {
		return authoring.ErrOutput
	}
	if kind == authoring.WritingVoice && (utf8.RuneCountInString(a.Body) < 200 || utf8.RuneCountInString(a.Body) > 700) {
		return authoring.ErrOutput
	}
	return nil
}
func (t *targets) Guide(authoring.Kind) string { return "설정 본문은 문법을 지키세요." }
func (t *targets) Publish(_ context.Context, p authoring.Publication) (authoring.SavedRef, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if r, ok := t.saved[p.Key]; ok {
		return r, nil
	}
	t.creates++
	r := authoring.SavedRef{Kind: p.Kind, ID: fmt.Sprintf("saved-%d", t.creates), Name: p.Artifact.Name}
	t.saved[p.Key] = r
	t.defaults[p.Key] = p.MakeDefault
	if t.failAfterCommit {
		t.failAfterCommit = false
		return authoring.SavedRef{}, errors.New("interrupted after target commit")
	}
	return r, nil
}

type budget struct{}

func (budget) CompletionCap(authoring.Kind, authoring.Mode, int, bool) int { return 8192 }

type estimates struct{}

func (estimates) CallCredits(context.Context, llm.ModelInfo, int64, int64) (int, bool) {
	return 12, true
}

type harness struct {
	svc     *authoring.Service
	store   *authorstore.Store
	db      *db.DB
	dbPath  string
	models  *models
	jobs    *jobs
	targets *targets
}

func fixture(t *testing.T) harness {
	t.Helper()
	d := database(t)
	h := harness{db: d, store: authorstore.New(d.Writer, d.Reader), models: &models{}, jobs: &jobs{rows: map[string]jobRecord{}}, targets: &targets{saved: map[string]authoring.SavedRef{}, defaults: map[string]bool{}}}
	var sequence int
	var name string
	if e := d.Reader.QueryRow("PRAGMA database_list").Scan(&sequence, &name, &h.dbPath); e != nil {
		t.Fatal(e)
	}
	h.svc = authoring.NewService(h.store, h.models, h.jobs, h.targets, budget{}, estimates{})
	return h
}
func batch(kind authoring.Kind) string {
	a := []map[string]string{}
	for n := 0; n < 8; n++ {
		body := fmt.Sprintf("<write>본문 %d</write>", n)
		if kind == authoring.WritingVoice {
			body = strings.Repeat("가", 250) + fmt.Sprint(n)
		}
		a = append(a, map[string]string{"name": fmt.Sprintf("후보%d", n), "description": "친절한 설명", "body": body, "title_area": ""})
	}
	b, _ := json.Marshal(map[string]any{"candidates": a})
	return string(b)
}
func create(t *testing.T, h harness, kind authoring.Kind) authoring.Session {
	t.Helper()
	s, e := h.svc.Create(context.Background(), "alice", kind, "", "create-key")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func recommend(t *testing.T, h harness, s authoring.Session) authoring.Session {
	t.Helper()
	ctx := context.Background()
	id, _, e := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: fmt.Sprintf("recommend-%d", s.Revision), Mode: authoring.Recommend, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if e != nil {
		t.Fatal(e)
	}
	j, _ := h.jobs.Get(ctx, "alice", id)
	h.models.text = batch(s.Kind)
	if e = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: j.WriteModel, Payload: j.Payload}, func(string, int, int) {}); e != nil {
		t.Fatal(e)
	}
	h.jobs.status(id, "done")
	s, e = h.svc.Get(ctx, "alice", s.ID)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func selected(t *testing.T, h harness, kind authoring.Kind) authoring.Session {
	t.Helper()
	s := recommend(t, h, create(t, h, kind))
	s, e := h.svc.Select(context.Background(), "alice", s.ID, s.Revision, s.Candidates[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestOwnerScopedCreationReplayAndLatestKinds(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := create(t, h, authoring.PostTemplate)
	same, e := h.svc.Create(ctx, "alice", authoring.PostTemplate, "", "create-key")
	if e != nil || same.ID != s.ID || h.targets.seeds != 1 {
		t.Fatalf("create replay %+v %v seeds%d", same, e, h.targets.seeds)
	}
	for _, fn := range []func() error{func() error { _, e := h.svc.Get(ctx, "bob", s.ID); return e }, func() error { _, e := h.svc.Select(ctx, "bob", s.ID, 0, "candidate"); return e }, func() error { _, e := h.svc.Save(ctx, "bob", s.ID, 0, false); return e }} {
		if !errors.Is(fn(), authoring.ErrNotFound) {
			t.Fatal("foreign session exposed")
		}
	}
	latest, e := h.svc.Latest(ctx, "alice", authoring.VideoTemplate, "")
	if e != nil || latest != nil {
		t.Fatal("kind was ignored")
	}
	if h.models.calls != 0 || h.jobs.enqueues != 0 || h.targets.creates != 0 {
		t.Fatal("read or seed performed model work/publication")
	}
}
func TestDuplicateAdmissionAndCASSelection(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := create(t, h, authoring.PostTemplate)
	in := authoring.Start{SessionID: s.ID, ExpectedRevision: 0, RequestID: "same-operation", Mode: authoring.Recommend, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}}
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); id, _, e := h.svc.Start(ctx, "alice", in); ids <- id; errs <- e }()
	}
	wg.Wait()
	close(ids)
	for range 8 {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
	}
	want := ""
	for id := range ids {
		if want == "" {
			want = id
		}
		if id != want {
			t.Fatal("replay admitted another job")
		}
	}
	if h.jobs.enqueues != 1 || h.models.calls != 0 {
		t.Fatal("duplicate work")
	}
	if _, e := h.svc.Select(ctx, "alice", s.ID, 1, "x"); !errors.Is(e, authoring.ErrBusy) {
		t.Fatalf("select during operation %v", e)
	}
	in.Prompt = "different"
	if _, _, e := h.svc.Start(ctx, "alice", in); !errors.Is(e, authoring.ErrStale) {
		t.Fatal("key content changed")
	}
}
func TestStagedResultCancellationAndInvalidReplacementPreserveDraft(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := selected(t, h, authoring.PostTemplate)
	prior := *s.Selected
	id, running, e := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "reroll", Mode: authoring.Recommend, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if e != nil {
		t.Fatal(e)
	}
	j, _ := h.jobs.Get(ctx, "alice", id)
	h.models.text = batch(s.Kind)
	if e = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: j.WriteModel, Payload: j.Payload}, func(string, int, int) {}); e != nil {
		t.Fatal(e)
	}
	before, e := h.svc.Get(ctx, "alice", s.ID)
	if e != nil || before.Revision != running.Revision || *before.Selected != prior {
		t.Fatal("staged output was prematurely applied")
	}
	after, e := h.svc.Cancel(ctx, "alice", s.ID, id)
	if e != nil || after.Phase != "failed" || *after.Selected != prior || len(after.Candidates) != 8 {
		t.Fatalf("cancel retention %+v %v", after, e)
	}
	id, _, e = h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: after.Revision, RequestID: "invalid", Mode: authoring.Recommend, Prompt: "失敗", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if e != nil {
		t.Fatal(e)
	}
	h.jobs.status(id, "done")
	after, e = h.svc.Get(ctx, "alice", s.ID)
	if e != nil || after.FailureReason != "AUTHORING_OUTPUT_INVALID" || *after.Selected != prior {
		t.Fatal("invalid result replaced the draft")
	}
}
func TestSaveSealInterruptedPublicationAndCurrentRevisionRetry(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := selected(t, h, authoring.WritingVoice)
	h.targets.failAfterCommit = true
	sealed, e := h.svc.Save(ctx, "alice", s.ID, s.Revision, true)
	if e == nil || sealed.Phase != "saving" || sealed.Revision <= s.Revision {
		t.Fatalf("seal %+v %v", sealed, e)
	}
	if _, e = h.svc.Select(ctx, "alice", s.ID, sealed.Revision, s.Candidates[1].ID); !errors.Is(e, authoring.ErrBusy) {
		t.Fatal("sealed draft changed")
	}
	restarted := authoring.NewService(h.store, h.models, h.jobs, h.targets, budget{}, estimates{})
	saved, e := restarted.Save(ctx, "alice", s.ID, sealed.Revision, false)
	if e != nil || saved.Phase != "saved" || h.targets.creates != 1 {
		t.Fatalf("publication replay %+v %v", saved, e)
	}
	if !h.targets.defaults[sealed.Publication.Key] {
		t.Fatal("retry changed frozen default choice")
	}
	calls := h.models.calls
	_, e = restarted.Save(ctx, "alice", s.ID, 0, false)
	if e != nil || h.targets.creates != 1 || h.models.calls != calls {
		t.Fatal("duplicate save changed settings")
	}
}
func TestPersonalSeedNeedsRefinementBeforeSyntheticPublication(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s, e := h.svc.Create(ctx, "alice", authoring.WritingVoice, "owned", "copy-personal")
	if e != nil || s.Selected == nil || s.Selected.Body != "" {
		t.Fatal("personal source was fabricated")
	}
	if _, e = h.svc.Save(ctx, "alice", s.ID, 0, true); e == nil {
		t.Fatal("empty personal preview was published")
	}
	if h.targets.creates != 0 || h.models.calls != 0 {
		t.Fatal("seed/save invoked provider")
	}
	_, _, e = h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: 0, RequestID: "refine", Mode: authoring.Refine, Prompt: "좀 더 밝게", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if e != nil {
		t.Fatal("personal empty-body draft could not be refined", e)
	}
}
func TestHistoryBoundAndCompletedChatOnlyUpdatesSelected(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := selected(t, h, authoring.PostGuideline)
	original := s.Candidates[0]
	for n := 0; n < 20; n++ {
		id, _, e := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: fmt.Sprintf("chat-%d", n), Mode: authoring.Refine, Prompt: "더 간결하게", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
		if e != nil {
			t.Fatal(e)
		}
		j, _ := h.jobs.Get(ctx, "alice", id)
		h.models.text = fmt.Sprintf(`{"artifact":{"name":"후보0","description":"설명","body":"수정된 방향 %d","title_area":""},"reply":"간결하게 바꿨어요."}`, n)
		if e = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: j.WriteModel, Payload: j.Payload}, func(string, int, int) {}); e != nil {
			t.Fatal(e)
		}
		h.jobs.status(id, "done")
		s, e = h.svc.Get(ctx, "alice", s.ID)
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(s.Turns) != 20 || s.Candidates[0] != original || s.Selected.Body == original.Body {
		t.Fatal("chat history or original options changed")
	}
	before := h.jobs.enqueues
	if _, _, e := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "overflow", Mode: authoring.Refine, Prompt: "추가 수정", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}}); !errors.Is(e, authoring.ErrHistoryFull) || h.jobs.enqueues != before {
		t.Fatal("unbounded chat admitted")
	}
}
func TestRestartAttachesAdmittedJobWithoutProviderReplay(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := create(t, h, authoring.PostTemplate)
	id, _, e := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: 0, RequestID: "once", Mode: authoring.Recommend, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if e != nil {
		t.Fatal(e)
	}
	h.jobs.status(id, "failed")
	restarted := authoring.NewService(h.store, h.models, h.jobs, h.targets, budget{}, estimates{})
	found, e := restarted.Get(ctx, "alice", s.ID)
	if e != nil || found.Phase != "failed" || h.models.calls != 0 || h.jobs.enqueues != 1 {
		t.Fatal("restart reissued uncertain work")
	}
	replayed, _, e := restarted.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: 0, RequestID: "once", Mode: authoring.Recommend, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if e != nil || replayed != id || h.jobs.enqueues != 1 {
		t.Fatal("replay repeated admission")
	}
}

func secondService(t *testing.T, h harness) *authoring.Service {
	t.Helper()
	d, e := db.Open(h.dbPath)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = d.Close() })
	return authoring.NewService(authorstore.New(d.Writer, d.Reader), h.models, h.jobs, h.targets, budget{}, estimates{})
}

func TestIndependentSQLiteWritersCASAndOneActiveAccountAcrossKinds(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := recommend(t, h, create(t, h, authoring.PostTemplate))
	other := secondService(t, h)
	barrier := make(chan struct{})
	errorsOut := make(chan error, 2)
	for i, svc := range []*authoring.Service{h.svc, other} {
		go func(i int, svc *authoring.Service) {
			<-barrier
			_, e := svc.Select(ctx, "alice", s.ID, s.Revision, s.Candidates[i].ID)
			errorsOut <- e
		}(i, svc)
	}
	close(barrier)
	accepted, stale := 0, 0
	for range 2 {
		e := <-errorsOut
		if e == nil {
			accepted++
		} else if errors.Is(e, authoring.ErrStale) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	if accepted != 1 || stale != 1 {
		t.Fatal("concurrent selection was not compare-and-swap")
	}
	a, e := h.svc.Create(ctx, "alice", authoring.PostGuideline, "", "post-guide")
	if e != nil {
		t.Fatal(e)
	}
	b, e := other.Create(ctx, "alice", authoring.VideoGuideline, "", "video-guide")
	if e != nil {
		t.Fatal(e)
	}
	before := h.jobs.enqueues
	barrier = make(chan struct{})
	for i, svc := range []*authoring.Service{h.svc, other} {
		go func(i int, svc *authoring.Service) {
			<-barrier
			state := []authoring.Session{a, b}[i]
			_, _, e := svc.Start(ctx, "alice", authoring.Start{SessionID: state.ID, ExpectedRevision: 0, RequestID: "generate", Mode: authoring.Recommend, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
			errorsOut <- e
		}(i, svc)
	}
	close(barrier)
	accepted, busy := 0, 0
	for range 2 {
		e := <-errorsOut
		if e == nil {
			accepted++
		} else if errors.Is(e, authoring.ErrBusy) {
			busy++
		} else {
			t.Fatal(e)
		}
	}
	if accepted != 1 || busy != 1 || h.jobs.enqueues != before+1 {
		t.Fatal("account active limit did not span all configuration kinds")
	}
}

func TestIndependentSQLiteSelectionAndSendShareRevisionCAS(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := selected(t, h, authoring.PostGuideline)
	other := secondService(t, h)
	barrier := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-barrier
		_, e := h.svc.Select(ctx, "alice", s.ID, s.Revision, s.Candidates[1].ID)
		results <- e
	}()
	go func() {
		<-barrier
		_, _, e := other.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "concurrent-chat", Mode: authoring.Refine, Prompt: "더 친절하게", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
		results <- e
	}()
	close(barrier)
	accepted, refused := 0, 0
	for range 2 {
		e := <-results
		if e == nil {
			accepted++
		} else if errors.Is(e, authoring.ErrStale) || errors.Is(e, authoring.ErrBusy) {
			refused++
		} else {
			t.Fatal(e)
		}
	}
	if accepted != 1 || refused != 1 || h.models.calls != 1 {
		t.Fatal("selection and send bypassed revision protection or invoked the model")
	}
}

type pricedModels struct{ *models }

func (p pricedModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	info, ok := p.models.Resolve(ref)
	info.Levels["write"] = "value"
	info.InputUSDPerMillion, info.OutputUSDPerMillion = "1", "2"
	return info, ok
}

type sizedBudget struct{}

func (sizedBudget) CompletionCap(_ authoring.Kind, _ authoring.Mode, chars int, _ bool) int {
	if chars > 4000 {
		return 32768
	}
	return 8192
}

type capturedEstimates struct{ cap int64 }

func (e *capturedEstimates) CallCredits(_ context.Context, _ llm.ModelInfo, _ int64, completion int64) (int, bool) {
	e.cap = completion
	return 12, true
}

func TestRefineEstimateUsesOwnedCurrentDraftAndMatchesFrozenCallCap(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s, e := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "seed-quote")
	if e != nil {
		t.Fatal(e)
	}
	capture := &capturedEstimates{}
	svc := authoring.NewService(h.store, pricedModels{h.models}, h.jobs, h.targets, sizedBudget{}, capture)
	ref := llm.ModelRef{ProviderID: "p", ModelID: "writer"}
	quote, e := svc.EstimateFor(ctx, "alice", authoring.PostGuideline, authoring.Refine, ref, s.ID)
	if e != nil || quote.Free || !quote.Available || capture.cap != 8192 || h.models.calls != 0 || h.jobs.enqueues != 0 {
		t.Fatal("current compact draft had a mismatched estimate", e, capture.cap)
	}
	if _, e = svc.EstimateFor(ctx, "bob", authoring.PostGuideline, authoring.Refine, ref, s.ID); !errors.Is(e, authoring.ErrNotFound) {
		t.Fatal("quote exposed another owner's draft")
	}
	id, _, e := svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "quoted-chat", Mode: authoring.Refine, Prompt: "더 간결하게", WriteModel: ref})
	if e != nil {
		t.Fatal(e)
	}
	job, _ := h.jobs.Get(ctx, "alice", id)
	var frozen struct {
		CompletionTokens int `json:"completion_tokens"`
	}
	if e = json.Unmarshal(job.Payload, &frozen); e != nil || int64(frozen.CompletionTokens) != capture.cap {
		t.Fatal("admitted cap differed from quote", e)
	}
}

func TestCancellationAfterProviderCannotStageLateResult(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := selected(t, h, authoring.WritingVoice)
	prior := *s.Selected
	id, _, e := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "late-refine", Mode: authoring.Refine, Prompt: "더 밝게", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if e != nil {
		t.Fatal(e)
	}
	j, _ := h.jobs.Get(ctx, "alice", id)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	h.models.text = fmt.Sprintf(`{"artifact":{"name":"새 말투","description":"밝은 느낌","body":%q,"title_area":""},"reply":"더 밝게 바꿨어요."}`, strings.Repeat("밝", 250))
	h.models.after = func() {
		cancel()
		if _, e := h.svc.Cancel(ctx, "alice", s.ID, id); e != nil {
			t.Error(e)
		}
	}
	if e = h.svc.Run(runCtx, authoring.Run{ID: id, UserID: "alice", WriteModel: j.WriteModel, Payload: j.Payload}, func(string, int, int) {}); !errors.Is(e, context.Canceled) {
		t.Fatal("late response ignored cancellation", e)
	}
	after, e := h.svc.Get(ctx, "alice", s.ID)
	if e != nil || *after.Selected != prior || after.Phase != "failed" || after.Turns[len(after.Turns)-1].Status != "cancelled" {
		t.Fatal("cancelled response changed durable draft", e)
	}
	jobAfter, _ := h.jobs.Get(ctx, "alice", id)
	if string(jobAfter.Payload) != string(j.Payload) {
		t.Fatal("cancelled response was staged")
	}
}

func TestUnicodeMessageLimitRefusesBeforeAdmission(t *testing.T) {
	h := fixture(t)
	s, e := h.svc.Create(context.Background(), "alice", authoring.PostGuideline, "owned", "seed")
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = h.svc.Start(context.Background(), "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: 0, RequestID: "too-long", Mode: authoring.Refine, Prompt: strings.Repeat("한", 2001), WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if !errors.Is(e, authoring.ErrInvalid) || h.jobs.enqueues != 0 || h.models.calls != 0 {
		t.Fatal("oversized message was charged or admitted", e)
	}
}

func TestRestartBindsDurableEnqueueReceiptWithoutRepeatingIt(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := create(t, h, authoring.PostTemplate)
	payload, _ := json.Marshal(map[string]any{"version": 1, "session_id": s.ID, "operation_id": "unbound-op", "base_revision": 0, "kind": "post_template", "mode": "recommend", "completion_tokens": 8192})
	op := authoring.Operation{ID: "unbound-op", RequestID: "unbound-request", Fingerprint: "frozen", Mode: authoring.Recommend, Payload: payload, CreatedAt: time.Now().UTC()}
	if _, _, _, e := h.store.Reserve(ctx, "alice", s.ID, 0, op); e != nil {
		t.Fatal(e)
	}
	id, e := h.jobs.Enqueue(ctx, authoring.JobRequest{UserID: "alice", WriteModel: "p/writer", Payload: payload})
	if e != nil {
		t.Fatal(e)
	}
	restarted := secondService(t, h)
	found, e := restarted.Get(ctx, "alice", s.ID)
	if e != nil || found.ActiveJobID != id || h.jobs.enqueues != 1 || h.models.calls != 0 {
		t.Fatal("restart did not bind the admitted receipt", e)
	}
}

func TestOperationOwnerForeignKeyAndAccountDeletionCascade(t *testing.T) {
	h := fixture(t)
	s := create(t, h, authoring.PostTemplate)
	_, e := h.db.Writer.Exec(`INSERT INTO configuration_authoring_operations(id,user_id,session_id,request_id,fingerprint,base_revision,mode,payload,status,created_at) VALUES('foreign','bob',?,'request','hash',0,'recommend',X'00','pending','2026-10-07T00:00:00Z')`, s.ID)
	if e == nil {
		t.Fatal("foreign account operation linked to another owner's session")
	}
	if _, _, e = h.svc.Start(context.Background(), "alice", authoring.Start{SessionID: s.ID, RequestID: "owned", Mode: authoring.Recommend, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}}); e != nil {
		t.Fatal(e)
	}
	if _, e = h.db.Writer.Exec(`DELETE FROM users WHERE id='alice'`); e != nil {
		t.Fatal(e)
	}
	var sessions, operations int
	if e = h.db.Reader.QueryRow(`SELECT count(*) FROM configuration_authoring_sessions`).Scan(&sessions); e != nil {
		t.Fatal(e)
	}
	if e = h.db.Reader.QueryRow(`SELECT count(*) FROM configuration_authoring_operations`).Scan(&operations); e != nil {
		t.Fatal(e)
	}
	if sessions != 0 || operations != 0 {
		t.Fatal("private authoring records survived account deletion")
	}
}

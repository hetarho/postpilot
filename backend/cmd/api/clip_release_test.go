package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/clip"
	clipai "github.com/postpilot/backend/internal/clip/ai"
	clipmedia "github.com/postpilot/backend/internal/clip/media"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/llm/openaicompat"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type releaseAdmission struct {
	jobAdmission
	metrics  *releaseMetrics
	expected int
	holds    int
}

func (a *releaseAdmission) Hold(ctx context.Context, s job.Start) error {
	a.metrics.mu.Lock()
	defer a.metrics.mu.Unlock()
	if len(a.metrics.prepared) != a.expected {
		return errors.New("hold preceded all prepared proxies")
	}
	for _, c := range a.metrics.prepared {
		info, e := os.Stat(c.Path)
		if e != nil || info.Size() != c.Bytes || c.Bytes > 8<<20 || c.DurationMS > 60000 {
			return errors.New("hold preceded verified media")
		}
	}
	files, _ := filepath.Glob(filepath.Join(a.metrics.workspace, "source-*"))
	if len(files) != 0 {
		return errors.New("original retained at hold")
	}
	if len(s.Calls) != 2 || s.Calls[0].Count != a.expected || s.Calls[1].Count != 1 {
		return errors.New("inexact reserved call count")
	}
	err := a.jobAdmission.Hold(ctx, s)
	if err == nil {
		a.holds++
	}
	return err
}

type releaseSaveStore struct {
	*clipstore.Store
	mode string
}

func (s releaseSaveStore) SaveGeneration(ctx context.Context, u, p, a, e string, r clip.Result) error {
	if s.mode == "save failure" {
		return errors.New("private-media-canary result save failed")
	}
	return s.Store.SaveGeneration(ctx, u, p, a, e, r)
}

type releaseHarness struct {
	// The plan the job persisted, so the speech probes below read the OUTPUT
	// timeline the way the renderer laid it out rather than a mapping tuned to
	// one compiler version.
	plan           *v1.ClipEditPlan
	t              *testing.T
	d              *db.DB
	client         postpilotv1connect.ClipServiceClient
	cookie         string
	project, batch string
	service        *clip.GenerationService
	queue          *job.Queue
	jobs           *jobstore.Store
	ledger         *usage.Service
	objects        *releaseObjects
	media          *releaseMedia
	metrics        *releaseMetrics
	provider       *releaseProvider
	admission      *releaseAdmission
	expected       int
	before         int
	master         bool
	sourceKeys     []string
	firstFixture   string
	aborted        *releaseAbortTransport
}

func releaseRequest[T any](h *releaseHarness, msg *T) *connect.Request[T] {
	r := connect.NewRequest(msg)
	r.Header().Set("Cookie", auth.SessionCookieName+"="+h.cookie)
	return r
}

// The environment opt-in deliberately isolates real binaries and databases from
// ordinary unit runs. Docker's release-smoke target has the production runtime,
// zero credentials and no external network, but loopback HTTP remains available.
func TestClipRelease(t *testing.T) {
	if os.Getenv("CLIP_RELEASE_SMOKE") != "1" {
		t.Skip("run the isolated nonroot release-smoke image")
	}
	if os.Getuid() == 0 {
		t.Fatal("release smoke must run nonroot")
	}
	logs := &releaseLog{}
	previousLog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLog)
		logs.mu.Lock()
		defer logs.mu.Unlock()
		if strings.Contains(logs.text.String(), "private-media-canary") || strings.Contains(logs.text.String(), "data:video/") || logs.exceeded {
			t.Error("release logs leaked private payload or exceeded their test bound")
		}
	})
	for _, name := range []string{"memory.max", "cpu.max"} {
		data, e := os.ReadFile("/sys/fs/cgroup/" + name)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("runtime %s=%s", name, strings.TrimSpace(string(data)))
		if name == "memory.max" && strings.TrimSpace(string(data)) != "1073741824" || name == "cpu.max" && strings.Join(strings.Fields(string(data)), " ") != "200000 100000" {
			t.Fatal("release gate requires --memory 1g --cpus 2")
		}
	}
	t.Cleanup(func() {
		if b, e := os.ReadFile("/sys/fs/cgroup/memory.peak"); e == nil {
			t.Logf("runtime peak memory bytes=%s", strings.TrimSpace(string(b)))
		}
	})
	if os.Getenv("CLIP_RELEASE_STRESS") == "1" {
		t.Run("20-sources-30-minutes", func(t *testing.T) { h := newReleaseHarness(t, "success", true); h.exercise("success") })
		return
	}
	for _, mode := range []string{"success", "multi-source", "multi-source-timing", "seeked cut", "delayed audio", "master", "denied", "partial", "overage", "unknown usage", "malformed", "truncated", "oversized response", "save failure", "malformed last source", "oversized proxy", "disk", "unknown prices", "price drift", "legacy client", "expired quote", "changed quote", "aborted client", "restart prepare", "restart hold", "restart partial", "restart save"} {
		t.Run(mode, func(t *testing.T) { h := newReleaseHarness(t, mode, false); h.exercise(mode) })
	}
}

func newReleaseHarness(t *testing.T, mode string, stress bool) *releaseHarness {
	t.Helper()
	ctx := t.Context()
	root := t.TempDir()
	d, err := db.Open(filepath.Join(root, "release.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err = db.Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	as := authstore.New(d.Writer, d.Reader)
	tier := plan.Free
	if mode == "master" {
		tier = plan.Master
	}
	if err = as.CreateUser(ctx, auth.User{ID: "release-user", PasswordHash: "not-a-real-password", Plan: tier, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	cookie, hash, err := auth.NewLinkToken()
	if err != nil {
		t.Fatal(err)
	}
	if err = as.CreateSession(ctx, auth.Session{Token: hash, UserID: "release-user", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(as, time.Hour)
	metrics := &releaseMetrics{hashes: map[string]bool{}, mode: mode}
	provider := &releaseProvider{mode: mode, metrics: metrics}
	server := httptest.NewServer(provider)
	t.Cleanup(server.Close)
	registry, err := llm.Parse([]byte(fmt.Sprintf("providers:\n  - id: fixture\n    adapter: openai\n    base_url: %s/v1\n    reasoning_format: openrouter\n", server.URL)), func(string) string { return "" }, map[string]llm.AdapterFactory{"openai": func(c llm.AdapterConfig) (llm.Provider, error) { return openaicompat.New(c, server.Client()), nil }}, releaseModelSource{}, llm.Options{Timeout: time.Minute, MaxTokens: 8192})
	if err != nil {
		t.Fatal(err)
	}
	ledger := usage.NewService(usagestore.New(d.Writer, d.Reader), registry, 8192)
	ledger.SetAnchors(usageAnchors{auth: authSvc})
	if err = ledger.EnsureMonthlyLot(ctx, "release-user", tier); err != nil {
		t.Fatal(err)
	}
	// A separate purchased lot proves overage cannot drain unrelated available
	// credits. SQL constraints and same-lot refunds are inspected after recovery.
	if _, err = d.Writer.Exec("INSERT INTO credit_lots(id,user_id,kind,granted,remaining,created_at) VALUES ('release-extra','release-user','purchased',5000,5000,?)", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ClipWorkRoot: filepath.Join(root, "work"), ClipFFmpegPath: "/usr/local/bin/ffmpeg", ClipFFprobePath: "/usr/local/bin/ffprobe", ClipResvgPath: "/usr/local/bin/resvg", ClipFontPath: "/usr/share/postpilot-fonts/pretendard/PretendardVariable.ttf", ClipDisplayFontPath: "/usr/share/postpilot-fonts/paperlogy/Paperlogy-8ExtraBold.ttf", ClipWorkStaleAge: time.Hour, ClipMediaTimeout: 15 * time.Minute, ClipSourceBatchTTL: 6 * time.Hour, PresignPutTTL: 10 * time.Minute, PresignGetTTL: time.Minute, ClipQuoteTTL: 5 * time.Minute, OrphanMinAge: time.Hour}
	mcfg := config.ClipMedia(cfg)
	runner := releaseRunner{ExecRunner: clipmedia.ExecRunner{StdoutLimit: mcfg.StdoutLimit, StderrLimit: mcfg.StderrLimit, WaitDelay: mcfg.WaitDelay}, metrics: metrics, root: cfg.ClipWorkRoot}
	adapter, err := clipmedia.New(mcfg, runner)
	if err != nil {
		t.Fatal(err)
	}
	media := &releaseMedia{Adapter: adapter, metrics: metrics, root: cfg.ClipWorkRoot}
	render, err := clipmedia.NewRenderer(adapter, config.ClipRender(cfg))
	if err != nil {
		t.Fatal(err)
	}
	renderer := releaseRenderer{Rendering: render, metrics: metrics}
	planner, err := clipai.New(clipModels{meteredRegistry{Registry: registry, ledger: ledger}}, renderer, config.ClipAI(cfg))
	if err != nil {
		t.Fatal(err)
	}
	st := clipstore.New(d.Writer, d.Reader)
	projects := clip.NewService(st, config.ClipLimits())
	objects := &releaseObjects{root: root, paths: map[string]string{}, downloads: map[string]int{}}
	if strings.HasPrefix(mode, "multi-source") {
		blob := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			objects.mu.Lock()
			path := objects.paths[key]
			objects.mu.Unlock()
			if !strings.HasPrefix(key, clip.ResultPrefix) || path == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "video/mp4")
			http.ServeFile(w, r, path)
		}))
		t.Cleanup(blob.Close)
		objects.readBase = blob.URL
	}
	sources := clip.NewSourceService(st, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
	projects.SetSources(sources)
	js := jobstore.New(d.Writer, d.Reader)
	q := job.New(js, 10*time.Millisecond)
	admission := &releaseAdmission{jobAdmission: jobAdmission{ledger: ledger, registry: registry, plans: authSvc}, metrics: metrics}
	q.Admit(admission)
	g := clip.NewGenerationService(releaseSaveStore{Store: st, mode: mode}, projects, sources, objects, media, planner, renderer, clipJobs{q}, config.ClipGeneration(cfg)).WithCredits(clipQuotePricing{registry: registry, cfg: config.ClipAI(cfg)}, clipAccounting{ledger: ledger})
	projects.SetGeneration(g)
	if !strings.HasPrefix(mode, "restart ") {
		q.Register(job.KindGenerateClip, metered(func(ctx context.Context, j job.Job, p job.Progress) error {
			return g.Run(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, p)
		}))
	}
	q.Register(job.KindRenderClip, metered(func(ctx context.Context, j job.Job, p job.Progress) error {
		return g.RunRender(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, p)
	}))
	for _, kind := range []string{job.KindGenerateClip, job.KindRenderClip} {
		q.OnTerminal(kind, func(ctx context.Context, j job.Job, at time.Time) error {
			return sources.ReleaseAttempt(ctx, j.UserID, j.ID, at)
		})
	}

	mux := http.NewServeMux()
	path, handler := postpilotv1connect.NewClipServiceHandler(cliprpc.NewHandler(projects).WithSources(sources).WithGeneration(g, q), connect.WithInterceptors(authrpc.NewInterceptor(authSvc, nil, "")))
	mux.Handle(path, handler)
	rpcServer := httptest.NewServer(mux)
	t.Cleanup(rpcServer.Close)
	h := &releaseHarness{t: t, d: d, client: postpilotv1connect.NewClipServiceClient(rpcServer.Client(), rpcServer.URL), cookie: cookie, service: g, queue: q, jobs: js, ledger: ledger, objects: objects, media: media, metrics: metrics, provider: provider, admission: admission, master: mode == "master"}
	if mode == "aborted client" {
		h.aborted = &releaseAbortTransport{RoundTripper: rpcServer.Client().Transport}
		client := *rpcServer.Client()
		client.Transport = h.aborted
		h.client = postpilotv1connect.NewClipServiceClient(&client, rpcServer.URL)
	}
	// The reserved labels a clip's own chips and cards read (CDS-30, CDS-28).
	// Two of them plus a campaign type are what the approval gate requires
	// before a credit is reserved (CDS-1, CDS-5).
	recipe := clip.Recipe{Name: "synthetic release", Preset: "restaurant", InformationFields: []clip.InformationField{
		{Label: "상호", Prompt: "가게 이름"}, {Label: "위치", Prompt: "어디"}, {Label: "place", Prompt: "where"},
	}, CopyStyles: []string{"clean"}}
	ratio := "horizontal"
	if strings.HasPrefix(mode, "multi-source") {
		recipe.CopyStyles, recipe.Accent, recipe.CutGuidance, ratio = []string{"clean", "memo"}, "amber", "균등분할", "vertical"
	}
	template, err := projects.CreateTemplate(ctx, "release-user", recipe)
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.CreateProject(ctx, "release-user", clip.ProjectInput{Title: "synthetic release", VideoTemplateID: template.ID, Ratio: ratio, TargetDurationMS: 15000, Disclosure: "ad", Answers: []clip.Answer{
		{Label: "상호", Text: "연남 김밥"}, {Label: "위치", Text: "서울 연남동"}, {Label: "place", Text: "fixture"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	h.project = p.ID
	if _, err = h.client.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: p.ID})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("unauthenticated access", err)
	}
	durations := []int{16000, 16000}
	if strings.HasPrefix(mode, "multi-source") {
		durations = []int{4290, 3744, 1480, 5010, 5108, 4508, 5428, 6702}
	}
	if mode == "overage" {
		durations[0] = 60000
	}
	if stress {
		durations = make([]int, 20)
		for i := range durations {
			durations[i] = 61000
		}
		durations[19] = 641000
	}
	paths := map[int]string{}
	var inputPaths []string
	var manifest []*v1.ClipSourceMetadata
	for i, duration := range durations {
		path := paths[duration]
		if path == "" {
			path = h.fixture(root, duration)
			paths[duration] = path
		}
		variant := filepath.Join(root, fmt.Sprintf("source-fixture-%02d.mp4", i))
		r := clipmedia.ExecRunner{StdoutLimit: 65536, StderrLimit: 8192, WaitDelay: 2 * time.Second}
		if _, e := r.Run(ctx, clipmedia.Command{Binary: cfg.ClipFFmpegPath, Dir: root, Args: []string{"-v", "error", "-i", path, "-map", "0", "-c", "copy", "-metadata", fmt.Sprintf("title=synthetic fixture %d", i), "-movflags", "+faststart", variant}}); e != nil {
			t.Fatal(e)
		}
		inputPaths = append(inputPaths, variant)
		info, e := os.Stat(variant)
		if e != nil {
			t.Fatal(e)
		}
		// Real distinct container bytes and the browser's bounded v1 fingerprint;
		// filenames alone never manufacture distinct source identities.
		fingerprint, e := releaseFingerprint(variant, duration)
		if e != nil {
			t.Fatal(e)
		}
		width, height := int32(1280), int32(720)
		if strings.HasPrefix(mode, "multi-source") {
			width, height = 1440, 1920
			if i == 5 {
				width, height = height, width
			}
		}
		manifest = append(manifest, &v1.ClipSourceMetadata{Filename: fmt.Sprintf("fixture-%02d.mp4", i), ContentType: "video/mp4", Bytes: info.Size(), DurationMs: int32(duration), Width: width, Height: height, Fingerprint: fingerprint})
		h.expected += (duration + 59999) / 60000
	}
	upload, err := h.client.CreateClipSourceBatch(ctx, releaseRequest(h, &v1.CreateClipSourceBatchRequest{ProjectId: p.ID, Sources: manifest}))
	if err != nil {
		t.Fatal(err)
	}
	h.batch = upload.Msg.Batch.Id
	owned, err := st.GetSourceBatch(ctx, "release-user", h.batch)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range owned.Sources {
		objects.paths[s.Key] = inputPaths[i]
		h.sourceKeys = append(h.sourceKeys, s.Key)
		if _, err = h.client.ConfirmClipSource(ctx, releaseRequest(h, &v1.ConfirmClipSourceRequest{BatchId: h.batch, SourceId: s.ID})); err != nil {
			t.Fatal(err)
		}
	}
	h.firstFixture = inputPaths[0]
	if mode == "malformed last source" {
		bad := filepath.Join(root, "corrupt.mp4")
		if err = os.WriteFile(bad, make([]byte, manifest[1].Bytes), 0600); err != nil {
			t.Fatal(err)
		}
		objects.paths[h.sourceKeys[1]] = bad
	}
	admission.expected = h.expected
	provider.beforePost = func(n int) error {
		metrics.mu.Lock()
		defer metrics.mu.Unlock()
		if len(metrics.prepared) != h.expected || admission.holds != 1 {
			return errors.New("provider preceded complete preparation and hold")
		}
		if n == 1 {
			for _, c := range metrics.prepared {
				if f, e := os.Stat(c.Path); e != nil || f.Size() != c.Bytes {
					return errors.New("first provider missing a prepared proxy")
				}
			}
		}
		return nil
	}
	h.before = h.balance()
	// Sample during subprocess execution, not only at operation boundaries.
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				metrics.scan(cfg.ClipWorkRoot)
			}
		}
	}()
	t.Cleanup(func() { close(stop); <-done })
	return h
}

func (h *releaseHarness) fixture(root string, duration int) string {
	h.t.Helper()
	path := filepath.Join(root, fmt.Sprintf("synthetic-%d.mp4", duration))
	// Repetition is only fixture construction, never an analysis shortcut. Each
	// declared source is downloaded, fully decoded and encoded independently.
	args := []string{"-hide_banner", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30", "-stream_loop", "-1", "-i", "/usr/share/postpilot-media/speech.m4a", "-t", fmt.Sprintf("%.3f", float64(duration)/1000), "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", "-c:a", "aac", "-ar", "48000", "-ac", "2", path}
	if strings.HasPrefix(h.metrics.mode, "multi-source") {
		args[7] = "color=c=blue:s=1440x1920:r=30"
		if duration == 4508 {
			args[7] = "color=c=blue:s=1920x1440:r=30"
		}
	}
	if h.metrics.mode == "delayed audio" {
		args = append(append(append([]string{}, args[:8]...), "-itsoffset", "0.250"), args[8:]...)
	}
	runner := clipmedia.ExecRunner{StdoutLimit: 65536, StderrLimit: 8192, WaitDelay: 2 * time.Second}
	if _, err := runner.Run(h.t.Context(), clipmedia.Command{Binary: "/usr/local/bin/ffmpeg", Dir: root, Args: args}); err != nil {
		h.t.Fatal(err)
	}
	return path
}
func (h *releaseHarness) balance() int {
	h.t.Helper()
	var n int
	if err := h.d.Reader.QueryRow("SELECT COALESCE(SUM(remaining),0) FROM credit_lots WHERE user_id='release-user'").Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}
func (h *releaseHarness) exercise(mode string) {
	t := h.t
	ctx := t.Context()
	model := &v1.ModelRef{ProviderId: "fixture", ModelId: releaseModel}
	quote, err := h.client.QuoteClipGeneration(ctx, releaseRequest(h, &v1.QuoteClipGenerationRequest{ProjectId: h.project, BatchId: h.batch, ObserveModel: model, WriteModel: model}))
	if mode == "unknown prices" {
		if err == nil || h.provider.posts.Load() != 0 || h.balance() != h.before {
			t.Fatal("unknown pricing executed", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	maxCredits := quote.Msg.MaxCredits
	req := &v1.StartClipGenerationRequest{ProjectId: h.project, BatchId: h.batch, ObserveModel: model, WriteModel: model, QuoteId: quote.Msg.QuoteId, ApprovedMaxCredits: &maxCredits}
	switch mode {
	case "legacy client":
		req.QuoteId = ""
		req.ApprovedMaxCredits = nil
	case "expired quote":
		if _, err = h.d.Writer.Exec("UPDATE clip_generation_quotes SET expires_at=? WHERE id=?", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), req.QuoteId); err != nil {
			t.Fatal(err)
		}
	case "changed quote":
		req.ApprovedMaxCredits = new(int32)
		*req.ApprovedMaxCredits = maxCredits - 1
	}
	accepted, err := h.client.StartClipGeneration(ctx, releaseRequest(h, req))
	if mode == "aborted client" {
		if err == nil {
			t.Fatal("fixture did not abort the response")
		}
		read, e := h.client.GetClipProject(ctx, releaseRequest(h, &v1.GetClipProjectRequest{Id: h.project}))
		if e != nil || read.Msg.Project.LatestAttempt == nil {
			t.Fatal("ambiguous acceptance was not durably readable", e)
		}
		attempt := read.Msg.Project.LatestAttempt
		if attempt.BatchId != h.batch || attempt.QuoteId != req.QuoteId {
			t.Fatal("ambiguous response attached to another attempt")
		}
		accepted = connect.NewResponse(&v1.StartClipGenerationResponse{JobId: attempt.JobId})
		err = nil
	}
	if mode == "legacy client" || mode == "expired quote" || mode == "changed quote" {
		if err == nil || h.provider.posts.Load() != 0 || h.balance() != h.before {
			t.Fatal("invalid approval executed", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.Msg.JobId
	// Concurrent identical approvals resolve to one durable executable attempt;
	// every accepted response must identify that same job, never another paid run.
	var wg sync.WaitGroup
	duplicates := 4
	if h.aborted != nil {
		duplicates = 0
	}
	for range duplicates {
		wg.Go(func() {
			r, e := h.client.StartClipGeneration(ctx, releaseRequest(h, req))
			if e == nil && r.Msg.JobId != id {
				t.Error("duplicate accepted a second job")
			}
		})
	}
	wg.Wait()
	if h.aborted != nil && h.aborted.starts.Load() != 1 {
		t.Fatal("ambiguous acceptance automatically submitted another start")
	}
	var jobs int
	if err = h.d.Reader.QueryRow("SELECT COUNT(*) FROM generation_jobs WHERE clip_project_id=?", h.project).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatal("duplicate jobs", jobs, err)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	if strings.HasPrefix(mode, "restart ") {
		h.queue.Register(job.KindGenerateClip, metered(func(ctx context.Context, j job.Job, p job.Progress) error {
			return h.service.Run(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, func(stage string, done, total int) {
				p(stage, done, total)
				if mode == "restart prepare" && stage == "prepare" && done == 1 || mode == "restart hold" && stage == "analyze" && done == 0 || mode == "restart partial" && stage == "analyze" && done == 1 || mode == "restart save" && stage == "save" {
					cancel()
				}
			})
		}))
	}
	go func() { defer close(workerDone); h.queue.Run(workerCtx) }()
	defer func() { cancel(); <-workerDone }()
	var restarted <-chan struct{}
	if strings.HasPrefix(mode, "restart ") {
		restarted = workerDone
	}
	start := time.Now()
	var got *v1.ClipProject
	deadline := time.NewTimer(25 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		r, e := h.client.GetClipProject(ctx, releaseRequest(h, &v1.GetClipProjectRequest{Id: h.project}))
		if e != nil {
			t.Fatal(e)
		}
		got = r.Msg.Project
		if got.LatestAttempt == nil || got.LatestAttempt.JobId != id || got.LatestAttempt.BatchId != h.batch || got.LatestAttempt.QuoteId != quote.Msg.QuoteId {
			t.Fatal("lost owned attempt projection")
		}
		if got.LatestJob != nil && (got.LatestJob.Status == "done" || got.LatestJob.Status == "failed") && got.Accounting != nil && got.Accounting.Settled {
			break
		}
		select {
		case <-restarted:
			restarted = nil
			persisted, e := h.jobs.GetByID(ctx, id)
			if e != nil || persisted.Status != job.StatusRunning {
				t.Fatal("interrupted worker did not leave durable running state", e)
			}
			// New worker instance reads only durable state, has no old allowance or
			// media references and never installs a handler capable of AI replay.
			recovery := job.New(jobstore.New(h.d.Writer, h.d.Reader), time.Millisecond)
			recovery.Admit(h.admission)
			if _, e = recovery.SweepRunning(ctx); e != nil {
				t.Fatal(e)
			}
			if _, e = recovery.SweepOpenHolds(ctx); e != nil {
				t.Fatal(e)
			}
			if e = h.service.Sweep(ctx); e != nil {
				t.Fatal(e)
			}
		case <-deadline.C:
			t.Fatal("release attempt timed out")
		case <-ticker.C:
		}
	}
	wantCalls := h.expected + 1
	wantStatus := "done"
	switch mode {
	case "denied", "unknown usage", "malformed", "truncated", "oversized response":
		wantCalls = 1
		wantStatus = "failed"
	case "partial":
		wantCalls = 2
		wantStatus = "failed"
	case "save failure", "restart save":
		wantStatus = "failed"
	case "restart partial":
		wantCalls = 1
		wantStatus = "failed"
	case "malformed last source", "oversized proxy", "disk", "price drift", "restart prepare", "restart hold":
		wantCalls = 0
		wantStatus = "failed"
	}
	if int(h.provider.posts.Load()) != wantCalls || got.LatestJob.Status != wantStatus {
		t.Fatalf("calls=%d want=%d status=%s stage=%s failure=%v", h.provider.posts.Load(), wantCalls, got.LatestJob.Status, got.LatestJob.Stage, got.LatestJob.Failure)
	}
	a := got.Accounting
	if a.FinalChargeCredits == nil || a.RefundCredits == nil {
		t.Fatal("missing authoritative settlement", a)
	}
	charged := int(*a.FinalChargeCredits)
	held := int(a.GetReservedCredits())
	if charged > held || charged > int(maxCredits) || charged < 0 || h.balance() != h.before-charged {
		t.Fatal("credit ceiling/debit violated", a)
	}
	if !h.master && int(a.GetRefundCredits()) != held-charged {
		t.Fatal("refund does not reconcile", a)
	}
	if mode == "unknown usage" {
		var costSource string
		if err := h.d.Reader.QueryRow("SELECT cost_source FROM usage_events WHERE job_id=?", id).Scan(&costSource); err != nil || costSource != "unavailable" {
			t.Fatal("unknown supplier usage was fabricated", costSource, err)
		}
	}
	if mode == "denied" || mode == "unknown usage" || mode == "oversized response" || wantCalls == 0 || h.master {
		if charged != 0 {
			t.Fatal("unused/exempt attempt charged", a)
		}
	} else if mode == "overage" {
		if charged != min(held, int(maxCredits)) {
			t.Fatal("overage not capped", a)
		}
	} else if charged != usageChargeForRelease(wantCalls, mode) {
		t.Fatal("incorrect measured charge", a)
	}
	if wantStatus == "done" {
		if got.Result == nil {
			t.Fatal("no result")
		}
		if mode == "multi-source-timing" {
			persisted := got.GetEditing().GetPlan()
			if persisted.GetDurationMs() != 15000 || len(persisted.GetCuts()) != 4 {
				t.Fatal("compiled plan was not persisted", persisted)
			}
			// The frozen template's guidance and selected source ranges now
			// own timing. Check reconciliation without reinstating a preset's
			// old per-cut rhythm as a required native result.
			for i, start := range []int32{0, 500, 1000, 1000} {
				cut := persisted.Cuts[i]
				if cut.StartMs != start || cut.EndMs <= cut.StartMs || cut.EndMs > 5000 || cut.GetCopy().GetEndMs() > cut.EndMs-cut.StartMs {
					t.Fatal("persisted plan still has model timing errors")
				}
			}
		}
		if strings.HasPrefix(mode, "multi-source") {
			for _, url := range []string{got.Result.ViewUrl, got.Result.DownloadUrl} {
				response, err := http.Get(url)
				if err != nil {
					t.Fatal(err)
				}
				read, err := io.Copy(io.Discard, io.LimitReader(response.Body, got.Result.Bytes+1))
				_ = response.Body.Close()
				if err != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "video/mp4" || read != got.Result.Bytes {
					t.Fatal("persisted preview/download is not readable", err)
				}
			}
		}
		h.plan = got.GetEditing().GetPlan()
		h.inspectResult()
		h.objects.mu.Lock()
		for i, key := range h.sourceKeys {
			want := 1
			if i == 0 || strings.HasPrefix(mode, "multi-source") {
				want = 2
			}
			if mode == "multi-source-timing" && i != 0 && i != 3 && i != 6 && i != 7 {
				want = 1
			}
			if h.objects.downloads[key] != want {
				t.Errorf("source %d downloads=%d want=%d", i, h.objects.downloads[key], want)
			}
		}
		if h.objects.uploads != 1 {
			t.Error("analysis uploaded an extra object")
		}
		h.objects.mu.Unlock()
	} else if got.Result != nil {
		t.Fatal("failed attempt replaced prior empty result")
	}
	for range 2 {
		if _, err = h.queue.SweepRunning(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err = h.queue.SweepOpenHolds(ctx); err != nil {
			t.Fatal(err)
		}
		if err = h.service.Sweep(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if h.balance() != h.before-charged || int(h.provider.posts.Load()) != wantCalls {
		t.Fatal("recovery charged/replayed work")
	}
	if next, e := h.jobs.PickNextQueued(ctx, time.Now()); !errors.Is(e, job.ErrNotFound) || next.ID != "" {
		t.Fatal("restart replayed work", e)
	}
	keys, _ := h.objects.ListSourceKeys(ctx)
	if len(keys) != len(h.sourceKeys) {
		t.Fatal("terminal cleanup lost retained original mappings")
	}
	entries, err := os.ReadDir(h.media.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "postpilot-clip-") {
			t.Fatal("workspace leaked")
		}
	}
	var badLots int
	if err = h.d.Reader.QueryRow("SELECT COUNT(*) FROM credit_lots WHERE remaining<0 OR remaining>granted").Scan(&badLots); err != nil || badLots != 0 {
		t.Fatal("invalid lots", err)
	}
	h.provider.mu.Lock()
	violations := append([]string(nil), h.provider.violations...)
	h.provider.mu.Unlock()
	if len(violations) > 0 {
		t.Fatal(violations)
	}
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()
	m := h.metrics
	// Measurement owns a short-lived scratch workspace inside the single
	// generation job: the captions' plates and, since T107, the ending card that
	// every clip closes on — so every mode measures, empty captions or not. The
	// job workspace is counted separately (maxJobWorkspaces) so this never
	// permits concurrent media jobs.
	workspaceLimit := 2
	if m.maxOriginals > 1 || m.maxJobWorkspaces > 1 || m.maxWorkspaces > workspaceLimit || m.maxProcesses > 1 || len(m.prepared) > 49 || m.maxProxy > 8<<20 || m.disk > 8<<30 || m.proxyBytes > 512<<20 {
		t.Fatalf("resource bound originals=%d job_workspaces=%d workspaces=%d processes=%d proxies=%d max_proxy=%d disk=%d", m.maxOriginals, m.maxJobWorkspaces, m.maxWorkspaces, m.maxProcesses, len(m.prepared), m.maxProxy, m.disk)
	}
	t.Logf("release sources=%d chunks=%d HTTP=%d max_request=%d max_proxy=%d disk_peak=%d prepared_peak=%d originals=%d workspaces=%d processes=%d preparation=%s render=%s elapsed=%s approved=%d held=%d charged=%d refund=%d", len(h.sourceKeys), len(m.prepared), wantCalls, h.provider.maxRequest.Load(), m.maxProxy, m.disk, m.proxyBytes, m.maxOriginals, m.maxWorkspaces, m.maxProcesses, m.prepareTime, m.renderTime, time.Since(start), maxCredits, held, charged, a.GetRefundCredits())
}
func usageChargeForRelease(calls int, mode string) int {
	if mode == "partial" {
		calls = 1
	}
	return plan.Charge(int64(calls) * 1000)
}

// sourceAt maps an output instant to the source millisecond the persisted plan
// shows there, mirroring the renderer's cutOffsets.
func (h *releaseHarness) sourceAt(at int) int {
	elapsed := 0
	for _, cut := range h.plan.GetCuts() {
		start := elapsed - int(cut.GetTransitionMs())
		end := start + int(cut.GetEndMs()-cut.GetStartMs())
		if at >= start && at < end {
			return int(cut.GetStartMs()) + (at - start)
		}
		elapsed = end
	}
	return -1
}

func (h *releaseHarness) inspectResult() {
	h.objects.mu.Lock()
	var path string
	for key, p := range h.objects.paths {
		if strings.HasPrefix(key, clip.ResultPrefix) {
			path = p
		}
	}
	h.objects.mu.Unlock()
	if path == "" {
		h.t.Fatal("result was not stored")
	}
	if err := h.media.Adapter.WithWorkspace(h.t.Context(), "inspect-result", func(ws clip.MediaWorkspace) error {
		p := filepath.Join(ws.Path, "result.mp4")
		if err := releaseCopy(path, p); err != nil {
			return err
		}
		info, err := h.media.Adapter.Probe(h.t.Context(), ws, p)
		if err != nil {
			return err
		}
		width, height := 1920, 1080
		if strings.HasPrefix(h.metrics.mode, "multi-source") {
			width, height = 1080, 1920
		}
		if info.Width != width || info.Height != height || info.DurationMS < 15000 || info.DurationMS > 15034 || !info.HasAudio {
			return fmt.Errorf("invalid original-derived final output: %+v", info)
		}
		for _, at := range []int{700, 3100, 10700} {
			originalAt := at
			if h.metrics.mode == "seeked cut" {
				originalAt += 1000
			}
			if strings.HasPrefix(h.metrics.mode, "multi-source") {
				// Every fixture source carries the same speech loop, so the source
				// position an output instant shows is the persisted plan's cut
				// arithmetic — the same offsets the renderer used (a transition
				// starts a cut TransitionMS earlier on the output timeline).
				originalAt = h.sourceAt(at)
				if originalAt < 0 {
					return fmt.Errorf("output instant %d ms lies outside the persisted plan", at)
				}
			}
			x, e := releaseSpeech(h.t.Context(), h.firstFixture, originalAt)
			if e != nil {
				return e
			}
			y, e := releaseSpeech(h.t.Context(), p, at)
			if e != nil {
				return e
			}
			correlation := releaseCorrelation(x, y)
			if correlation < .85 {
				x, _ := releaseSpeechSpan(h.t.Context(), h.firstFixture, originalAt-100, 400)
				y, _ := releaseSpeechSpan(h.t.Context(), p, at-100, 400)
				lag, best := releaseBestLag(x, y)
				// A cut the compiler reconciled to an arbitrary millisecond is
				// encoded on whole frames, so a later cut's audio may sit up to one
				// frame from the plan's arithmetic. That is quantization, not a
				// misaligned source: the speech must still match exactly within it.
				if best >= .85 && math.Abs(lag) <= 1000.0/30 {
					h.t.Logf("original/output speech at %d ms correlation %.4f at a %.1f ms frame lag (%.4f)", at, correlation, lag, best)
					continue
				}
				x, _ = releaseTimelineSpeech(h.t.Context(), h.firstFixture, originalAt)
				y, _ = releaseTimelineSpeech(h.t.Context(), p, at)
				return fmt.Errorf("original/output speech at %d ms correlation %.4f; best lag %.4f ms correlation %.4f; decoded timeline correlation %.4f", at, correlation, lag, best, releaseCorrelation(x, y))
			}
			h.t.Logf("original/output speech at %d ms correlation %.4f", at, correlation)
		}
		return nil
	}); err != nil {
		h.t.Fatal(err)
	}
	// Read only finite headers, not complete output media into the test process.
	f, err := os.Open(path)
	if err != nil {
		h.t.Fatal(err)
	}
	defer f.Close()
	header := make([]byte, 4096)
	n, _ := io.ReadFull(f, header)
	if !strings.Contains(string(header[:n]), "moov") {
		h.t.Fatal("result lacks fast-start metadata")
	}
}

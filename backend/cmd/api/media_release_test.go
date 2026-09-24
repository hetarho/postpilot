package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// splitRelease drives real public RPC, never a database or an in-memory queue.
// Only the API child owns SQLite. Worker control is a host-side handshake so no
// Docker socket, DB mount or cloud/provider credential reaches the worker.
type splitRelease struct {
	t               *testing.T
	h               *releaseHarness
	child           *exec.Cmd
	action          atomic.Value
	ack             atomic.Int64
	failDelete      atomic.Bool
	deletedFailures atomic.Int64
	duplicate       atomic.Int64
	holdComplete    atomic.Bool
	completed       atomic.Int64
	mu              sync.Mutex
	timings         map[string]time.Duration
	starts          map[string]time.Time
	maxHealth       time.Duration
}

func (s *splitRelease) state() splitState {
	s.t.Helper()
	v, e := splitJSON[splitState](s.t.Context(), "http://127.0.0.1:8082/state")
	if e != nil {
		s.t.Fatal(e)
	}
	return v
}
func (s *splitRelease) boot() {
	s.t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMediaReleaseAPIProcess$", "-test.v", "-test.timeout=30m")
	cmd.Env = append(os.Environ(), "MEDIA_RELEASE_CHILD=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		s.t.Fatal(err)
	}
	s.child = cmd
	s.wait("API health after process start", time.Minute, func() bool {
		r, e := http.Get("http://127.0.0.1:8080/health")
		if e != nil {
			return false
		}
		r.Body.Close()
		return r.StatusCode == 200
	})
}
func (s *splitRelease) kill() {
	if s.child != nil {
		_ = s.child.Process.Kill()
		_ = s.child.Wait()
		s.child = nil
	}
}
func (s *splitRelease) wait(label string, limit time.Duration, ready func() bool) {
	s.t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		select {
		case <-s.t.Context().Done():
			s.t.Fatal(s.t.Context().Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	s.t.Fatal("timeout: " + label)
}
func (s *splitRelease) worker(action string) {
	s.t.Helper()
	before := s.ack.Load()
	s.action.Store(action)
	s.wait("host worker "+action, time.Minute, func() bool { return s.ack.Load() > before })
	s.action.Store("")
}
func (s *splitRelease) project() *pb.ClipProject {
	s.t.Helper()
	start := time.Now()
	v, e := s.h.client.GetClipProject(s.t.Context(), releaseRequest(s.h, &pb.GetClipProjectRequest{Id: s.h.project}))
	if e != nil {
		s.t.Fatal(e)
	}
	s.maxHealth = max(s.maxHealth, time.Since(start))
	return v.Msg.Project
}
func (s *splitRelease) terminal(id, status string) *pb.ClipProject {
	s.t.Helper()
	var p *pb.ClipProject
	s.wait("job "+status, 10*time.Minute, func() bool {
		p = s.project()
		j := p.GetLatestJob()
		if j.GetId() != id {
			return false
		}
		if j.GetStatus() == "failed" {
			s.t.Fatalf("job failed: %v", j.Failure)
		}
		return j.GetStatus() == status
	})
	return p
}
func (s *splitRelease) stage(op, state string, minAttempts int) {
	s.t.Helper()
	s.wait(op+" "+state, time.Minute, func() bool {
		for _, st := range s.state().Stages {
			if st.Operation == op && st.State == state && st.Attempts >= minAttempts {
				return true
			}
		}
		return false
	})
}
func (s *splitRelease) render(p *pb.ClipProject) string {
	s.t.Helper()
	v, e := s.h.client.StartClipRender(s.t.Context(), releaseRequest(s.h, &pb.StartClipRenderRequest{ProjectId: s.h.project, BatchId: s.h.batch, ExpectedRevision: p.EditPlanRevision, RenderKind: pb.ClipRenderKind_CLIP_RENDER_KIND_SERVER}))
	if e != nil {
		s.t.Fatal(e)
	}
	return v.Msg.JobId
}

func TestMediaRelease(t *testing.T) {
	if os.Getenv("MEDIA_RELEASE_SMOKE") != "1" {
		t.Skip("private multi-process CPU release fixture")
	}
	if os.Getuid() == 0 {
		t.Fatal("release API must run nonroot")
	}
	ctx := t.Context()
	root := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(root, "api", "release.db"))
	if e := os.MkdirAll(filepath.Join(root, "api"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("CLIP_WORK_ROOT", filepath.Join(root, "api-work"))
	s := &splitRelease{t: t, timings: map[string]time.Duration{}, starts: map[string]time.Time{}}
	s.action.Store("")
	defer s.kill()
	metrics := &releaseMetrics{hashes: map[string]bool{}, mode: "success"}
	provider := &releaseProvider{mode: "success", metrics: metrics}
	provider.beforePost = func(_ int) error {
		state, e := splitJSON[splitState](ctx, "http://127.0.0.1:8082/state")
		if e != nil {
			return e
		}
		if len(state.Digests) != 1 || state.Holds != 1 {
			return fmt.Errorf("provider preceded accepted artifacts and one hold: %d/%d", len(state.Digests), state.Holds)
		}
		metrics.mu.Lock()
		defer metrics.mu.Unlock()
		for _, hash := range state.Digests {
			metrics.hashes[hash] = true
		}
		return nil
	}
	minio, _ := url.Parse("http://minio:9000")
	storageProxy := httputil.NewSingleHostReverseProxy(minio)
	control := http.NewServeMux()
	control.Handle("/v1/", provider)
	control.HandleFunc("/control", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			s.action.Store("")
			s.ack.Add(1)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"action": s.action.Load()})
	})
	control.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && s.failDelete.Load() {
			s.deletedFailures.Add(1)
			http.Error(w, "fixture delete outage", 503)
			return
		}
		storageProxy.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: ":8081", Handler: control, ReadHeaderTimeout: time.Second}
	go server.ListenAndServe()
	defer server.Close()
	// Test-only transport fault injection in front of the real authenticated private
	// server. The successful completion is replayed verbatim, including credentials.
	privateTarget, _ := url.Parse("http://127.0.0.1:9000")
	private := httputil.NewSingleHostReverseProxy(privateTarget)
	operations := map[string]string{}
	private.ModifyResponse = func(response *http.Response) error {
		if response.StatusCode != 200 || !strings.HasSuffix(response.Request.URL.Path, "/ClaimMediaStage") {
			return nil
		}
		raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if err != nil {
			return err
		}
		response.Body = io.NopCloser(bytes.NewReader(raw))
		var claimed pb.ClaimMediaStageResponse
		if err = splitDecode(response.Header, raw, &claimed); err != nil {
			return err
		}
		if work := claimed.GetWork(); work != nil {
			s.mu.Lock()
			id := work.GetLease().GetAttemptId()
			s.starts[id] = time.Now()
			operations[id] = work.Operation
			s.mu.Unlock()
		}
		return nil
	}
	gateway := &http.Server{Addr: ":9002", ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/ReserveMediaOutputs") {
			raw, e := io.ReadAll(io.LimitReader(r.Body, 4<<20))
			if e != nil {
				http.Error(w, "fixture read", 500)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			var reservation pb.ReserveMediaOutputsRequest
			if splitDecode(r.Header, raw, &reservation) == nil {
				s.mu.Lock()
				id := reservation.GetLease().GetAttemptId()
				s.timings[operations[id]+"_download_compute"] = time.Since(s.starts[id])
				s.starts[id+"/upload"] = time.Now()
				s.mu.Unlock()
			}
		}
		if !strings.HasSuffix(r.URL.Path, "/CompleteMediaStage") {
			private.ServeHTTP(w, r)
			return
		}
		completeRaw, e := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if e != nil {
			http.Error(w, "fixture read", 500)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(completeRaw))
		var complete pb.CompleteMediaStageRequest
		if splitDecode(r.Header, completeRaw, &complete) == nil {
			s.mu.Lock()
			id := complete.GetLease().GetAttemptId()
			s.timings[operations[id]+"_reserve_upload"] = time.Since(s.starts[id+"/upload"])
			s.mu.Unlock()
		}
		s.completed.Add(1)
		for s.holdComplete.Load() {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
		body, e := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if e != nil {
			http.Error(w, "fixture body", 500)
			return
		}
		send := func() (*http.Response, error) {
			q, e := http.NewRequestWithContext(r.Context(), r.Method, privateTarget.String()+r.URL.Path, bytes.NewReader(body))
			if e != nil {
				return nil, e
			}
			q.Header = r.Header.Clone()
			return http.DefaultClient.Do(q)
		}
		response, e := send()
		if e != nil {
			http.Error(w, "API restart", 503)
			return
		}
		defer response.Body.Close()
		if response.StatusCode == 200 {
			again, e := send()
			if e == nil {
				io.Copy(io.Discard, again.Body)
				again.Body.Close()
				if again.StatusCode == 200 {
					s.duplicate.Add(1)
				}
			}
		}
		for k, vs := range response.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
	})}
	go gateway.ListenAndServe()
	defer gateway.Close()
	s.boot()
	seed, e := splitJSON[splitSeed](ctx, "http://127.0.0.1:8082/seed")
	if e != nil {
		t.Fatal(e)
	}
	s.h = &releaseHarness{t: t, cookie: seed.Cookie, project: seed.Project, client: newReleaseClipClient(http.DefaultClient, "http://127.0.0.1:8080"), metrics: metrics}
	h := s.h
	fixture := h.fixture(root, 16000)
	info, e := os.Stat(fixture)
	if e != nil {
		t.Fatal(e)
	}
	fingerprint, e := releaseFingerprint(fixture, 16000)
	if e != nil {
		t.Fatal(e)
	}
	uploads, e := h.client.CreateClipSourceBatch(ctx, releaseRequest(h, &pb.CreateClipSourceBatchRequest{ProjectId: h.project, Sources: []*pb.ClipSourceMetadata{{Filename: "split.mp4", ContentType: "video/mp4", Bytes: info.Size(), DurationMs: 16000, Width: 1280, Height: 720, Fingerprint: fingerprint}}}))
	if e != nil {
		t.Fatal(e)
	}
	h.batch = uploads.Msg.Batch.Id
	upload := uploads.Msg.Uploads[0]
	file, e := os.Open(fixture)
	if e != nil {
		t.Fatal(e)
	}
	put, _ := http.NewRequestWithContext(ctx, "PUT", upload.PutUrl, file)
	put.ContentLength = info.Size()
	for k, v := range upload.Headers {
		put.Header.Set(k, v)
	}
	response, e := http.DefaultClient.Do(put)
	file.Close()
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("source PUT", response.StatusCode)
	}
	if _, e = h.client.ConfirmClipSource(ctx, releaseRequest(h, &pb.ConfirmClipSourceRequest{BatchId: h.batch, SourceId: upload.SourceId})); e != nil {
		t.Fatal(e)
	}
	if _, e = h.client.SetClipSourceOriginalSound(ctx, releaseRequest(h, &pb.SetClipSourceOriginalSoundRequest{ProjectId: h.project, BatchId: h.batch, SourceId: upload.SourceId, ExpectedFingerprint: fingerprint, RetainOriginalAudio: true})); e != nil {
		t.Fatal(e)
	}
	before := s.state().Balance
	model := &pb.ModelRef{ProviderId: "fixture", ModelId: releaseModel}
	quote, e := h.client.QuoteClipGeneration(ctx, releaseRequest(h, &pb.QuoteClipGenerationRequest{ProjectId: h.project, BatchId: h.batch, ObserveModel: model, WriteModel: model}))
	if e != nil {
		t.Fatal(e)
	}
	start, e := h.client.StartClipGeneration(ctx, releaseRequest(h, &pb.StartClipGenerationRequest{ProjectId: h.project, BatchId: h.batch, ObserveModel: model, WriteModel: model, QuoteId: quote.Msg.QuoteId, ApprovedMaxCredits: &quote.Msg.MaxCredits, CancellationPolicyVersion: quote.Msg.GetCancellationPolicy().GetVersion()}))
	if e != nil {
		t.Fatal(e)
	}
	s.stage("prepare", "queued", 0)
	s.kill()
	s.boot() // truly lose the entire API heap, retaining only its private DB.
	s.stage("prepare", "queued", 0)
	reopened := s.project()
	if reopened.GetLatestJob().GetId() != start.Msg.JobId || reopened.GetResult() != nil || provider.posts.Load() != 0 || s.state().Balance != before {
		t.Fatal("waiting/reopen/restart changed paid state")
	}
	t.Log("offline worker + reopen + API process restart: PASS")
	prepareStart := time.Now()
	s.worker("start")
	plan := s.terminal(start.Msg.JobId, "done")
	s.mu.Lock()
	s.timings["prepare_and_planning"] = time.Since(prepareStart)
	s.mu.Unlock()
	if plan.GetResult() != nil || plan.GetEditing().GetPlan() == nil || provider.posts.Load() != 3 || s.state().Holds != 0 {
		t.Fatalf("generation did not stop at paid plan: calls=%d holds=%d", provider.posts.Load(), s.state().Holds)
	}
	balance := s.state().Balance
	renderStart := time.Now()
	renderID := s.render(plan)
	s.stage("render", "running", 1)
	s.worker("kill")
	s.worker("start")
	s.stage("render", "running", 2)
	s.kill()
	s.boot() // preserve the second worker lease across an API crash, too.
	s.stage("render", "running", 2)
	delivered := s.terminal(renderID, "done")
	s.mu.Lock()
	s.timings["render_including_reclaim"] = time.Since(renderStart)
	s.mu.Unlock()
	if delivered.GetResult() == nil || delivered.RenderedPlanRevision != plan.EditPlanRevision || provider.posts.Load() != 3 || s.state().Balance != balance {
		t.Fatal("reclaimed free render changed result/accounting")
	}
	if s.duplicate.Load() < 2 {
		t.Fatal("duplicate accepted completion was not idempotent")
	}
	t.Log("real prepare/artifact/provider/plan + killed worker reclaim + duplicate completion: PASS")
	// Make cancellation deterministic at the upload/completion boundary, after a real
	// rendered upload. The cancellation fences publication while its receipt is held.
	s.holdComplete.Store(true)
	previous := s.completed.Load()
	cancelID := s.render(delivered)
	s.wait("uploaded output pending completion", 10*time.Minute, func() bool { return s.completed.Load() > previous })
	if _, e = h.client.CancelClipJob(ctx, releaseRequest(h, &pb.CancelClipJobRequest{ProjectId: h.project, JobId: cancelID})); e != nil {
		t.Fatal(e)
	}
	s.holdComplete.Store(false)
	cancelled := s.terminal(cancelID, "cancelled")
	if cancelled.GetResult().GetId() != delivered.GetResult().GetId() {
		t.Fatal("cancellation replaced retained result")
	}
	s.wait("cancelled output queued for cleanup", time.Minute, func() bool { return s.state().Deletions > 0 })
	s.failDelete.Store(true)
	cleanup, e := http.Get("http://127.0.0.1:8082/cleanup")
	if e != nil {
		t.Fatal(e)
	}
	cleanup.Body.Close()
	if cleanup.StatusCode != 503 || s.deletedFailures.Load() == 0 || s.state().Deletions == 0 {
		t.Fatal("cleanup failure lost durable retry")
	}
	s.failDelete.Store(false)
	cleanup, e = http.Get("http://127.0.0.1:8082/cleanup")
	if e != nil {
		t.Fatal(e)
	}
	cleanup.Body.Close()
	if cleanup.StatusCode != 204 || s.state().Deletions != 0 {
		t.Fatal("cleanup retry failed")
	}
	retained, e := http.Get(s.project().GetResult().GetViewUrl())
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, retained.Body)
	retained.Body.Close()
	if retained.StatusCode != 200 {
		t.Fatal("cleanup removed retained result")
	}
	t.Log("cancellation/completion race + failed object delete + cleanup retry + retained result: PASS")
	if s.maxHealth > 2*time.Second {
		t.Fatalf("API responsiveness gate exceeded: %s", s.maxHealth)
	}
	provider.mu.Lock()
	violations := append([]string(nil), provider.violations...)
	provider.mu.Unlock()
	if len(violations) > 0 {
		t.Fatal(violations)
	}
	s.mu.Lock()
	milliseconds := map[string]int64{}
	for k, v := range s.timings {
		milliseconds[k] = v.Milliseconds()
	}
	t.Logf("MEDIA_RELEASE_REPORT %s", mustReleaseJSON(map[string]any{"provider": "explicit synthetic fixture; zero paid requests", "layout": os.Getenv("MEDIA_RELEASE_LAYOUT"), "timings_ms": milliseconds, "max_project_rpc_ms": s.maxHealth.Milliseconds(), "api_disk_bytes": s.state().DiskBytes, "duplicate_completions": s.duplicate.Load()}))
	s.mu.Unlock()
	s.worker("stop")
}
func mustReleaseJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func splitDecode(header http.Header, body []byte, message proto.Message) error {
	if header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer reader.Close()
		body, err = io.ReadAll(io.LimitReader(reader, (4<<20)+1))
		if err != nil {
			return err
		}
		if len(body) > 4<<20 {
			return fmt.Errorf("fixture decoded message exceeds limit")
		}
	}
	if strings.Contains(header.Get("Content-Type"), "json") {
		return protojson.Unmarshal(body, message)
	}
	return proto.Unmarshal(body, message)
}

// Connect compresses larger unary claims. Instrumentation must inspect that
// envelope without changing the bytes the real worker receives.
func TestSplitReleaseTimingReadsCompressedClaims(t *testing.T) {
	want := &pb.ClaimMediaStageResponse{Work: &pb.MediaWork{Operation: "prepare", Lease: &pb.MediaLeaseCredentials{AttemptId: "fixture"}}}
	for _, kind := range []string{"application/proto", "application/json"} {
		var raw []byte
		var err error
		if strings.Contains(kind, "json") {
			raw, err = protojson.Marshal(want)
		} else {
			raw, err = proto.Marshal(want)
		}
		if err != nil {
			t.Fatal(err)
		}
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err = writer.Write(raw); err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		header := http.Header{}
		header.Set("Content-Type", kind)
		header.Set("Content-Encoding", "gzip")
		var got pb.ClaimMediaStageResponse
		if err = splitDecode(header, compressed.Bytes(), &got); err != nil || !proto.Equal(want, &got) {
			t.Fatal(kind, err)
		}
	}
}

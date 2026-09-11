package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipmedia "github.com/postpilot/backend/internal/clip/media"
	"github.com/postpilot/backend/internal/llm"
)

// All objects and HTTP responses are local synthetic fixtures. This adapter has
// no cloud credentials, network client or connection to any development stack.
type releaseObjects struct {
	mu        sync.Mutex
	root      string
	paths     map[string]string
	downloads map[string]int
	uploads   int
	readBase  string
}

type releaseLog struct {
	mu       sync.Mutex
	text     strings.Builder
	exceeded bool
}

// Drop a real HTTP response after the server has accepted it. The client sees
// cancellation, not a job id; recovery must only read the owned projection.
type releaseAbortTransport struct {
	http.RoundTripper
	used   atomic.Bool
	starts atomic.Int32
}

func (a *releaseAbortTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := strings.HasSuffix(r.URL.Path, "/StartClipGeneration")
	if start {
		a.starts.Add(1)
	}
	response, err := a.RoundTripper.RoundTrip(r)
	if err == nil && start && a.used.CompareAndSwap(false, true) {
		_ = response.Body.Close()
		return nil, context.Canceled
	}
	return response, err
}

func (l *releaseLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(p)
	remaining := max(0, (1<<20)-l.text.Len())
	if n > remaining {
		l.exceeded = true
	}
	_, _ = l.text.Write(p[:min(n, remaining)])
	return n, nil
}

func (o *releaseObjects) PresignSource(_ context.Context, key, mime string, _ time.Duration) (clip.SignedSourcePut, error) {
	return clip.SignedSourcePut{URL: "http://fixture.invalid/" + key, Headers: map[string]string{"Content-Type": mime}}, nil
}
func (o *releaseObjects) HeadSource(_ context.Context, key string) (clip.SourceObjectInfo, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	f, err := os.Stat(o.paths[key])
	if err != nil {
		return clip.SourceObjectInfo{}, clip.ErrNotFound
	}
	return clip.SourceObjectInfo{Bytes: f.Size(), ContentType: "video/mp4"}, nil
}
func (o *releaseObjects) Delete(_ context.Context, key string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	// Source fixture masters may represent multiple distinct uploads. Removing
	// the object's mapping, not its reusable test fixture, models cloud deletion.
	delete(o.paths, key)
	return nil
}
func (o *releaseObjects) ListSourceKeys(context.Context) ([]string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []string
	for k := range o.paths {
		if !strings.HasPrefix(k, clip.ResultPrefix) {
			out = append(out, k)
		}
	}
	return out, nil
}
func (o *releaseObjects) Download(ctx context.Context, key string, w io.Writer, limit int64) (int64, error) {
	o.mu.Lock()
	p := o.paths[key]
	o.downloads[key]++
	o.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	f, err := os.Open(p)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(w, io.LimitReader(f, limit+1))
}
func (o *releaseObjects) Upload(_ context.Context, key string, r io.ReadSeeker, n int64, _ string) error {
	if !strings.HasPrefix(key, clip.ResultPrefix) {
		return errors.New("analysis proxy must never be uploaded")
	}
	f, err := os.CreateTemp(o.root, "result-")
	if err != nil {
		return err
	}
	defer f.Close()
	got, err := io.Copy(f, r)
	if err != nil {
		return err
	}
	if got != n {
		return io.ErrUnexpectedEOF
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.paths[key] = f.Name()
	o.uploads++
	return nil
}
func (o *releaseObjects) PresignRead(_ context.Context, key, _ string, _ bool, _ time.Duration) (string, error) {
	if !strings.HasPrefix(key, clip.ResultPrefix) {
		return "", errors.New("source/proxy signed for analysis")
	}
	if o.readBase != "" {
		return o.readBase + "/result?key=" + url.QueryEscape(key), nil
	}
	return "http://fixture.invalid/result", nil
}
func (o *releaseObjects) ListResults(context.Context) ([]clip.StoredObject, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []clip.StoredObject
	for k := range o.paths {
		if strings.HasPrefix(k, clip.ResultPrefix) {
			out = append(out, clip.StoredObject{Key: k, Modified: time.Now()})
		}
	}
	return out, nil
}

type releaseMetrics struct {
	mu                                                   sync.Mutex
	prepared                                             []clip.AnalysisChunk
	hashes                                               map[string]bool
	workspace                                            string
	disk, proxyBytes, maxProxy                           int64
	maxOriginals, maxWorkspaces, maxProcesses, processes int
	jobWorkspaces, maxJobWorkspaces                      int
	prepareTime, renderTime                              time.Duration
	probeCount                                           int
	mode                                                 string
}

func (m *releaseMetrics) scan(root string) {
	var disk, proxies int64
	originals, workspaces := 0, 0
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), "postpilot-clip-") {
				workspaces++
			}
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return nil
		}
		disk += info.Size()
		if strings.HasPrefix(d.Name(), "source-") {
			originals++
		}
		if strings.HasPrefix(d.Name(), "proxy-") {
			proxies += info.Size()
		}
		return nil
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disk = max(m.disk, disk)
	m.proxyBytes = max(m.proxyBytes, proxies)
	m.maxOriginals = max(m.maxOriginals, originals)
	m.maxWorkspaces = max(m.maxWorkspaces, workspaces)
}

type releaseRunner struct {
	clipmedia.ExecRunner
	metrics *releaseMetrics
	root    string
}

func (r releaseRunner) Run(ctx context.Context, c clipmedia.Command) ([]byte, error) {
	r.metrics.mu.Lock()
	r.metrics.processes++
	r.metrics.maxProcesses = max(r.metrics.maxProcesses, r.metrics.processes)
	r.metrics.mu.Unlock()
	defer func() { r.metrics.scan(r.root); r.metrics.mu.Lock(); r.metrics.processes--; r.metrics.mu.Unlock() }()
	return r.ExecRunner.Run(ctx, c)
}

type releaseMedia struct {
	*clipmedia.Adapter
	metrics *releaseMetrics
	root    string
}

func (m *releaseMedia) WithWorkspace(ctx context.Context, id string, fn func(clip.MediaWorkspace) error) error {
	return m.Adapter.WithWorkspace(ctx, id, func(ws clip.MediaWorkspace) error {
		m.metrics.mu.Lock()
		m.metrics.workspace = ws.Path
		m.metrics.jobWorkspaces++
		m.metrics.maxJobWorkspaces = max(m.metrics.maxJobWorkspaces, m.metrics.jobWorkspaces)
		m.metrics.mu.Unlock()
		defer func() { m.metrics.mu.Lock(); m.metrics.jobWorkspaces--; m.metrics.mu.Unlock() }()
		if m.metrics.mode == "disk" {
			ws.CheckCapacity = func(int64) error { return clip.ErrWorkspaceLimit }
		}
		return fn(ws)
	})
}
func (m *releaseMedia) Probe(ctx context.Context, ws clip.MediaWorkspace, p string) (clip.MediaInfo, error) {
	start := time.Now()
	info, err := m.Adapter.Probe(ctx, ws, p)
	m.metrics.mu.Lock()
	m.metrics.probeCount++
	m.metrics.prepareTime += time.Since(start)
	m.metrics.mu.Unlock()
	m.metrics.scan(m.root)
	return info, err
}
func (m *releaseMedia) PrepareAnalysisChunks(ctx context.Context, ws clip.MediaWorkspace, s clip.MediaSource, fn func(clip.AnalysisChunk) error) error {
	start := time.Now()
	defer func() { m.metrics.mu.Lock(); m.metrics.prepareTime += time.Since(start); m.metrics.mu.Unlock() }()
	return m.Adapter.PrepareAnalysisChunks(ctx, ws, s, func(c clip.AnalysisChunk) error {
		if m.metrics.mode == "oversized proxy" && m.metrics.probeCount == 2 {
			c.Bytes = 8<<20 + 1
		}
		if err := fn(c); err != nil {
			return err
		}
		f, err := os.Open(c.Path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return err
		}
		m.metrics.mu.Lock()
		m.metrics.prepared = append(m.metrics.prepared, c)
		m.metrics.hashes[hex.EncodeToString(h.Sum(nil))] = true
		m.metrics.maxProxy = max(m.metrics.maxProxy, c.Bytes)
		m.metrics.mu.Unlock()
		m.metrics.scan(m.root)
		return nil
	})
}

type releaseRenderer struct {
	*clipmedia.Rendering
	metrics *releaseMetrics
}

func (r releaseRenderer) Render(ctx context.Context, ws clip.MediaWorkspace, p clip.EditPlan, s []clip.RenderSource, loader clip.RenderSourceLoader) (clip.RenderedVideo, error) {
	start := time.Now()
	defer func() { r.metrics.mu.Lock(); r.metrics.renderTime += time.Since(start); r.metrics.mu.Unlock() }()
	return r.Rendering.Render(ctx, ws, p, s, func(ctx context.Context, id string, consume func(clip.MediaSource) error) error {
		return loader(ctx, id, func(source clip.MediaSource) error {
			matches := false
			for _, expected := range s {
				if expected.ID == id && source.Info.Width == expected.Info.Width && source.Info.Height == expected.Info.Height {
					matches = true
				}
			}
			if !strings.HasPrefix(filepath.Base(source.Path), "source-") || !matches {
				return errors.New("renderer did not receive original geometry")
			}
			return consume(source)
		})
	})
}

type releaseModelSource struct{}

const releaseModel = "google/gemini-2.5-flash"

func (releaseModelSource) Models() []llm.SourceModel {
	return []llm.SourceModel{{ModelID: releaseModel, Vision: true, VideoInput: true, StructuredOutput: true, ContextTokens: 1048576, InputUSDPerMillion: "0.3", OutputUSDPerMillion: "2.5", Stages: []string{"observe", "write"}}}
}
func (s releaseModelSource) Lookup(id string) (llm.SourceModel, bool) {
	return s.Models()[0], id == releaseModel
}

type releaseProvider struct {
	mode        string
	posts, gets atomic.Int32
	maxRequest  atomic.Int64
	beforePost  func(int) error
	metrics     *releaseMetrics
	mu          sync.Mutex
	violations  []string
}

func (p *releaseProvider) reject(w http.ResponseWriter, why string) {
	p.mu.Lock()
	p.violations = append(p.violations, why)
	p.mu.Unlock()
	http.Error(w, "fixture contract violation", 500)
}
func (p *releaseProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		p.gets.Add(1)
		pricing := map[string]any{"prompt": "0.0000003", "completion": "0.0000025", "request": "0", "image": "0.0000003", "audio": "0.000001", "input_audio_cache": "0.0000001", "internal_reasoning": "0.0000025", "input_cache_read": "0.00000003", "input_cache_write": "0.0000000833333333333333", "web_search": "0.014", "discount": 0}
		if p.mode == "unknown prices" {
			// An omitted media rate is a known zero under the OpenRouter contract;
			// a dimension this code does not know is the unknown charge that refuses.
			pricing["video_second"] = "0.001"
		}
		if p.mode == "price drift" {
			p.metrics.mu.Lock()
			prepared := len(p.metrics.prepared)
			p.metrics.mu.Unlock()
			if prepared > 0 {
				pricing["image"] = "0.0001"
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": releaseModel, "endpoints": []any{map[string]any{"tag": "google-ai-studio", "model_id": releaseModel, "pricing": pricing, "supported_parameters": []string{"reasoning", "max_tokens", "response_format", "structured_outputs"}}}}})
		return
	}
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
		p.reject(w, "unexpected HTTP route")
		return
	}
	n := int(p.posts.Add(1))
	if p.beforePost != nil {
		if err := p.beforePost(n); err != nil {
			p.reject(w, err.Error())
			return
		}
	}
	if r.ContentLength <= 0 || r.ContentLength > 12<<20 || len(r.TransferEncoding) != 0 {
		p.reject(w, "unbounded request")
		return
	}
	for old := p.maxRequest.Load(); r.ContentLength > old && !p.maxRequest.CompareAndSwap(old, r.ContentLength); old = p.maxRequest.Load() {
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 12<<20+1))
	if err != nil || int64(len(data)) != r.ContentLength {
		p.reject(w, "short request")
		return
	}
	var wire struct {
		MaxTokens int `json:"max_tokens"`
		Provider  struct {
			AllowFallbacks    bool              `json:"allow_fallbacks"`
			RequireParameters bool              `json:"require_parameters"`
			MaxPrice          map[string]string `json:"max_price"`
		} `json:"provider"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(data, &wire) != nil || wire.Provider.AllowFallbacks || !wire.Provider.RequireParameters || len(wire.Provider.MaxPrice) != 5 {
		p.reject(w, "missing frozen routing")
		return
	}
	if strings.Contains(string(data), "fixture.invalid") || strings.Contains(string(data), "cache_control") || strings.Contains(string(data), `"tools"`) {
		p.reject(w, "unexpected URL or optional feature")
		return
	}
	var metadata map[string]any
	videoCount := 0
	for _, msg := range wire.Messages {
		if msg.Role != "user" {
			continue
		}
		var text string
		if json.Unmarshal(msg.Content, &text) == nil {
			_ = json.Unmarshal([]byte(text), &metadata)
			continue
		}
		var parts []struct {
			Type  string                           `json:"type"`
			Text  string                           `json:"text"`
			Video struct{ URL, Processing string } `json:"video_url"`
		}
		if json.Unmarshal(msg.Content, &parts) != nil {
			p.reject(w, "bad message")
			return
		}
		for _, part := range parts {
			if part.Type == "text" {
				_ = json.Unmarshal([]byte(part.Text), &metadata)
			}
			if part.Type == "video_url" {
				videoCount++
				if part.Video.Processing != "static" || !strings.HasPrefix(part.Video.URL, "data:video/mp4;base64,") {
					p.reject(w, "not bounded static inline")
					return
				}
				raw, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(part.Video.URL, "data:video/mp4;base64,"))
				if e != nil || len(raw) > 8<<20 {
					p.reject(w, "bad inline bytes")
					return
				}
				h := sha256.Sum256(raw)
				p.metrics.mu.Lock()
				ok := p.metrics.hashes[hex.EncodeToString(h[:])]
				p.metrics.mu.Unlock()
				if !ok {
					p.reject(w, "not a verified analysis proxy")
					return
				}
			}
		}
	}
	if p.mode == "denied" || p.mode == "partial" && n == 2 {
		w.WriteHeader(403)
		_, _ = io.WriteString(w, `{"error":{"message":"private-media-canary","code":403}}`)
		return
	}
	if p.mode == "oversized response" {
		_, _ = io.WriteString(w, strings.Repeat("x", (4<<20)+1))
		return
	}
	var content any
	if videoCount == 1 && wire.MaxTokens == 8192 {
		// The observation contract of CDS-39: the scene type, whether the footage
		// already carries readable text and the principal subject's own box are
		// what the design system reads to place a caption (T105).
		content = map[string]any{"source_id": metadata["source_id"], "chunk_index": metadata["chunk_index"], "segments": []any{map[string]any{
			"start_ms": 0, "end_ms": metadata["chunk_duration_ms"], "event": "synthetic scene",
			"subjects": []string{"test pattern"}, "speech": "synthetic speech", "quality": "usable",
			"focal": map[string]float64{"x": .5, "y": .5}, "scene": "scenery", "readable_text": false,
			"subject": map[string]float64{"x": .3, "y": .3, "width": .4, "height": .4},
		}}}
	} else if videoCount == 0 && wire.MaxTokens == 32768 {
		analyses, ok := metadata["analyses"].([]any)
		if !ok || len(analyses) == 0 {
			p.reject(w, "missing structured analyses")
			return
		}
		source := analyses[0].(map[string]any)["source_id"]
		// The writing contract of T105: the model writes WORDS — the sentence, a
		// shorter alternative, the one word to accent and the cut's own chips —
		// and the design system chooses the style, the anchor and the accent.
		// Three contiguous cuts of one source, not one long take: CDS-37 holds
		// every cut to 6.0 s, so fifteen seconds is three cuts at the least. They
		// are adjacent and show one scene, so the output timeline still maps
		// one-to-one onto the source and the speech comparison below holds.
		offset := 0
		if p.mode == "seeked cut" {
			offset = 1000
		}
		var fixtureCuts []any
		for i := 0; i < 3; i++ {
			fixtureCuts = append(fixtureCuts, map[string]any{
				"id": fmt.Sprintf("fixture-cut-%d", i), "source_id": source,
				"start_ms": offset + i*5000, "end_ms": offset + (i+1)*5000, "volume": 1,
				"focal": map[string]float64{"x": .5, "y": .5}, "chips": []string{},
				"caption": map[string]any{"text": "", "start_ms": 0, "end_ms": 5000, "short_text": "", "keyword": ""},
			})
		}
		content = map[string]any{"ratio": metadata["ratio"], "duration_ms": 15000, "hook": "", "cuts": fixtureCuts}
		if strings.HasPrefix(p.mode, "multi-source") {
			// The recorded lengths, 1000 ms third cut included: CDS-37 r3 aims at
			// 1.2 s (2.5 s under the 음식점 preset) and never refuses for it.
			lengths := []int{2000, 2000, 1000, 2400, 2300, 2300, 2200, 2200}
			if len(analyses) != len(lengths) {
				p.reject(w, "multi-source fixture requires eight analyses")
				return
			}
			var cuts []any
			for i, analysis := range analyses {
				cuts = append(cuts, map[string]any{
					"id": fmt.Sprintf("cut-%d", i), "source_id": analysis.(map[string]any)["source_id"],
					"start_ms": 0, "end_ms": lengths[i], "volume": 1,
					"focal": map[string]float64{"x": .5, "y": .5}, "chips": []string{"위치"},
					"caption": map[string]any{"text": fmt.Sprintf("한글 장면 %d", i+1), "start_ms": 0, "end_ms": lengths[i], "short_text": fmt.Sprintf("장면 %d", i+1), "keyword": ""},
				})
			}
			content.(map[string]any)["cuts"] = cuts
			content.(map[string]any)["hook"] = "연남 김밥 한 줄"
			if p.mode == "multi-source-timing" {
				// The real failing response's timing shape, with synthetic copy
				// and source IDs. Exercise compilation through the actual queue.
				selected := []int{0, 3, 6, 7}
				starts, ends := []int{0, 500, 1000, 1000}, []int{3800, 4000, 4000, 4000}
				var timingCuts []any
				for i, index := range selected {
					cut := cuts[index].(map[string]any)
					cut["start_ms"], cut["end_ms"] = starts[i], ends[i]
					caption := cut["caption"].(map[string]any)
					caption["start_ms"], caption["end_ms"] = 400, 3600
					if i == 0 {
						caption["start_ms"], caption["end_ms"] = 500, 3500
					}
					timingCuts = append(timingCuts, cut)
				}
				content.(map[string]any)["cuts"] = timingCuts
				content.(map[string]any)["duration_ms"] = 15200
			}
		}
	} else {
		p.reject(w, "wrong observation/plan budget or modality")
		return
	}
	encoded, _ := json.Marshal(content)
	finish := "stop"
	if p.mode == "malformed" {
		encoded = []byte(`{"unknown":true}`)
	}
	if p.mode == "truncated" {
		finish = "length"
	}
	cost := 0.001
	if p.mode == "overage" {
		cost = 1000
	}
	usage := map[string]any{"prompt_tokens": 120, "completion_tokens": 40, "completion_tokens_details": map[string]int{"reasoning_tokens": 10}, "cost": cost}
	if p.mode == "unknown usage" {
		delete(usage, "cost")
		encoded = []byte(`{"unknown":true}`)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(encoded)}, "finish_reason": finish}}, "usage": usage})
}

func releaseCopy(source, dest string) error {
	f, e := os.Open(source)
	if e != nil {
		return e
	}
	defer f.Close()
	w, e := os.Create(dest)
	if e != nil {
		return e
	}
	_, e = io.Copy(w, f)
	return errors.Join(e, w.Close())
}
func releaseFingerprint(path string, duration int) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return "", e
	}
	mime := []byte("video/mp4")
	prefix := make([]byte, 1+8+4+len(mime)+8)
	prefix[0] = 1
	binary.BigEndian.PutUint64(prefix[1:], uint64(info.Size()))
	binary.BigEndian.PutUint32(prefix[9:], uint32(len(mime)))
	copy(prefix[13:], mime)
	binary.BigEndian.PutUint64(prefix[13+len(mime):], uint64(duration))
	h := sha256.New()
	_, _ = h.Write(prefix)
	for _, at := range []int64{0, max(0, info.Size()-65536)} {
		data := make([]byte, min(info.Size(), 65536))
		if _, e = f.ReadAt(data, at); e != nil {
			return "", e
		}
		_, _ = h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func releaseSpeech(ctx context.Context, path string, at int) ([]byte, error) {
	return releaseSpeechSpan(ctx, path, at, 200)
}
func releaseSpeechSpan(ctx context.Context, path string, at, span int) ([]byte, error) {
	r := clipmedia.ExecRunner{StdoutLimit: 65536, StderrLimit: 8192, WaitDelay: 2 * time.Second}
	return r.Run(ctx, clipmedia.Command{Binary: "/usr/local/bin/ffmpeg", Dir: filepath.Dir(path), Args: []string{"-v", "error", "-threads", "1", "-ss", fmt.Sprintf("%.3f", float64(at)/1000), "-i", path, "-t", fmt.Sprintf("%.3f", float64(span)/1000), "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1"}})
}

func releaseBestLag(a, b []byte) (float64, float64) {
	if min(len(a), len(b)) < 12800 {
		return 0, 0
	}
	reference := a[3200:9600]
	best, lag := -1.0, 0
	for shift := -1600; shift <= 1600; shift++ {
		c := releaseCorrelation(reference, b[3200+shift*2:9600+shift*2])
		if c > best {
			best, lag = c, shift
		}
	}
	return float64(lag) / 16, best
}

func releaseTimelineSpeech(ctx context.Context, path string, at int) ([]byte, error) {
	r := clipmedia.ExecRunner{StdoutLimit: 65536, StderrLimit: 8192, WaitDelay: 2 * time.Second}
	return r.Run(ctx, clipmedia.Command{Binary: "/usr/local/bin/ffmpeg", Dir: filepath.Dir(path), Args: []string{"-v", "error", "-threads", "1", "-i", path, "-ss", fmt.Sprintf("%.3f", float64(at)/1000), "-t", "0.200", "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1"}})
}
func releaseCorrelation(a, b []byte) float64 {
	n := min(len(a), len(b)) / 2
	if n < 2000 {
		return 0
	}
	var aa, bb, ab float64
	for i := 0; i < n; i++ {
		x, y := float64(int16(binary.LittleEndian.Uint16(a[2*i:]))), float64(int16(binary.LittleEndian.Uint16(b[2*i:])))
		aa += x * x
		bb += y * y
		ab += x * y
	}
	if aa < 1 || bb < 1 {
		return 0
	}
	return ab / math.Sqrt(aa*bb)
}

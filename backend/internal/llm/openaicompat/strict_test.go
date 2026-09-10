package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

func strictRequest() llm.Request {
	call := llm.CallPolicy{Ref: llm.ModelRef{ProviderID: "test", ModelID: "google/gemini-2.5-flash"}, Stage: "observe", CompletionTokens: 8192, Reasoning: llm.ReasoningLow, InputUSDPerMillion: "0.125", OutputUSDPerMillion: "0.75"}
	call, _ = strictEndpointFixture(call.Ref.ModelID).freeze(call, llm.ExecutionInlineStatic)
	return llm.Request{Model: call.Ref.ModelID, System: "Analyze this synthetic fixture.", Stage: call.Stage, MaxTokens: call.CompletionTokens, Reasoning: call.Reasoning,
		JSONSchema: []byte(`{"type":"object"}`),
		Execution:  &llm.ExecutionPolicy{Call: call, Delivery: llm.ExecutionInlineStatic, NoFallback: true, RequireParameters: true},
		Messages:   []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("Describe the clip."), llm.InlineVideoPart(llm.InlineVideo{MIME: "video/mp4", Size: 3, DurationMS: 1000, Sampling: llm.VideoSamplingFixed, Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("abc")), nil }})}}},
	}
}

func strictEndpointFixture(model string) pricedEndpoint {
	return pricedEndpoint{Tag: "google-ai-studio", ModelID: model, Pricing: map[string]json.RawMessage{"prompt": json.RawMessage(`"0.000000125"`), "completion": json.RawMessage(`"0.00000075"`), "request": json.RawMessage(`"0"`), "image": json.RawMessage(`"0"`), "audio": json.RawMessage(`"0"`)}, SupportedParameters: []string{"max_tokens", "response_format", "structured_outputs", "reasoning"}}
}

func endpointResponse(w http.ResponseWriter, model string, endpoints ...pricedEndpoint) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": model, "endpoints": endpoints}})
}

func strictTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(llm.AdapterConfig{ProviderID: "test", BaseURL: srv.URL + "/v1", ReasoningFormat: "openrouter"}, srv.Client())
}

func TestStrictInlineWireMatchesDocumentedFixture(t *testing.T) {
	req := strictRequest()
	var gets, posts atomic.Int32
	client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			gets.Add(1)
			endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
			return
		}
		posts.Add(1)
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if int64(len(raw)) != r.ContentLength || len(r.TransferEncoding) != 0 {
			t.Error("missing exact content length")
		}
		want, err := os.ReadFile("testdata/strict_inline_request.json")
		if err != nil {
			t.Error(err)
			return
		}
		var gotJSON, wantJSON any
		if json.Unmarshal(raw, &gotJSON) != nil || json.Unmarshal(want, &wantJSON) != nil || !reflect.DeepEqual(gotJSON, wantJSON) {
			t.Errorf("unexpected wire: %s", raw)
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":30,"completion_tokens":2,"cost":0}}`)
	})
	out, err := client.Complete(context.Background(), req)
	if err != nil || out.Text != "{}" || !out.Usage.CostReported || out.Usage.CostMicrousd != 0 || gets.Load() != 1 || posts.Load() != 1 {
		t.Fatalf("out=%+v err=%v GET=%d POST=%d", out, err, gets.Load(), posts.Load())
	}
}

func TestStrictRejectsInvalidPolicyAndMediaBeforeAnyHTTP(t *testing.T) {
	var calls atomic.Int32
	client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) })
	for name, mutate := range map[string]func(*llm.Request){
		"oversized":                      func(r *llm.Request) { r.Messages[0].Parts[1].InlineVideo.Size = clipLimits.InlineBytes + 1 },
		"too long":                       func(r *llm.Request) { r.Messages[0].Parts[1].InlineVideo.DurationMS = 60_001 },
		"unknown duration":               func(r *llm.Request) { r.Messages[0].Parts[1].InlineVideo.DurationMS = 0 },
		"metadata":                       func(r *llm.Request) { r.System = strings.Repeat("a", int(clipLimits.MetadataBytes)+1) },
		"escaped metadata":               func(r *llm.Request) { r.System = strings.Repeat("\x00", int(clipLimits.MetadataBytes)/2) },
		"no price":                       func(r *llm.Request) { r.Execution.Call.InputUSDPerMillion = "" },
		"bad price":                      func(r *llm.Request) { r.Execution.Call.OutputUSDPerMillion = "NaN" },
		"fallback":                       func(r *llm.Request) { r.Execution.NoFallback = false },
		"unsupported parameters allowed": func(r *llm.Request) { r.Execution.RequireParameters = false },
		"wrong MIME":                     func(r *llm.Request) { r.Messages[0].Parts[1].InlineVideo.MIME = "video/webm" },
		"adaptive":                       func(r *llm.Request) { r.Messages[0].Parts[1].InlineVideo.Sampling = "adaptive" },
		"no policy":                      func(r *llm.Request) { r.Execution = nil },
		"different budget":               func(r *llm.Request) { r.MaxTokens++ },
		"two inline parts":               func(r *llm.Request) { r.Messages[0].Parts = append(r.Messages[0].Parts, r.Messages[0].Parts[1]) },
		"signed url": func(r *llm.Request) {
			r.Messages[0].Parts[1] = llm.VideoPart("https://private.test/secret", "video/mp4")
		},
		"unknown family": func(r *llm.Request) { r.Model = "unknown/video"; r.Execution.Call.Ref.ModelID = r.Model },
		"online variant": func(r *llm.Request) { r.Model += ":online"; r.Execution.Call.Ref.ModelID = r.Model },
	} {
		t.Run(name, func(t *testing.T) {
			r := strictRequest()
			mutate(&r)
			if _, err := client.Complete(context.Background(), r); !errors.Is(err, llm.ErrUnsupported) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("preflight dispatched %d requests", calls.Load())
	}
}

func TestStrictPriceDimensionsAndUnitsFailClosed(t *testing.T) {
	for name, mutate := range map[string]func(*pricedEndpoint){
		"unknown prompt":              func(e *pricedEndpoint) { delete(e.Pricing, "prompt") },
		"null output":                 func(e *pricedEndpoint) { e.Pricing["completion"] = json.RawMessage(`null`) },
		"price drift":                 func(e *pricedEndpoint) { e.Pricing["prompt"] = json.RawMessage(`"0.0000001250000000001"`) },
		"request surcharge":           func(e *pricedEndpoint) { e.Pricing["request"] = json.RawMessage(`"0.01"`) },
		"image surcharge":             func(e *pricedEndpoint) { e.Pricing["image"] = json.RawMessage(`"0.000001"`) },
		"audio":                       func(e *pricedEndpoint) { e.Pricing["audio"] = json.RawMessage(`"0.000001"`) },
		"unknown dimension even zero": func(e *pricedEndpoint) { e.Pricing["new_dimension"] = json.RawMessage(`0`) },
		"null extra":                  func(e *pricedEndpoint) { e.Pricing["audio"] = json.RawMessage(`null`) },
		"negative":                    func(e *pricedEndpoint) { e.Pricing["prompt"] = json.RawMessage(`"-1"`) },
		"fraction":                    func(e *pricedEndpoint) { e.Pricing["prompt"] = json.RawMessage(`"1/1000000"`) },
		"parameters":                  func(e *pricedEndpoint) { e.SupportedParameters = []string{"max_tokens"} },
		"different model":             func(e *pricedEndpoint) { e.ModelID = "google/different" },
		"unknown endpoint":            func(e *pricedEndpoint) { e.Tag = "undocumented" },
	} {
		t.Run(name, func(t *testing.T) {
			req := strictRequest()
			endpoint := strictEndpointFixture(req.Model)
			mutate(&endpoint)
			if endpoint.eligible(req) {
				t.Fatal("unenforceable endpoint admitted")
			}
		})
	}
	r := strictRequest()
	e := strictEndpointFixture(r.Model)
	if !e.eligible(r) {
		t.Fatal("equal exact decimal caps refused")
	}
	r.Execution.Call.InputUSDPerMillion = "0"
	r.Execution.Call.OutputUSDPerMillion = "0"
	e.Pricing["prompt"] = json.RawMessage(`"0"`)
	e.Pricing["completion"] = json.RawMessage(`0`)
	r.Execution.Call, _ = e.freeze(r.Execution.Call, r.Execution.Delivery)
	if !e.eligible(r) {
		t.Fatal("known zero refused")
	}
	for _, value := range []string{"0", ".5", "01.25", "1e-20", "0.12345678901234567890"} {
		if !json.Valid([]byte(wirePrice(value))) || !priceWithin(json.RawMessage(`"0"`), value) {
			t.Fatalf("invalid decimal conversion: %s", value)
		}
	}
}

func TestStrictPlannerUsesTheSameFrozenRoutingWithoutVideo(t *testing.T) {
	req := strictRequest()
	req.Messages[0].Parts = req.Messages[0].Parts[:1]
	req.Stage = "write"
	req.Execution.Call.Stage = "write"
	req.Execution.Delivery = llm.ExecutionTextOnly
	req.Execution.Call.InputUSDPerMillion = "0"
	req.Execution.Call.OutputUSDPerMillion = "0"
	endpoint := strictEndpointFixture(req.Model)
	endpoint.Pricing["prompt"] = json.RawMessage(`"0"`)
	endpoint.Pricing["completion"] = json.RawMessage(`"0"`)
	req.Execution.Call, _ = endpoint.freeze(req.Execution.Call, req.Execution.Delivery)
	var posts atomic.Int32
	client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			e := strictEndpointFixture(req.Model)
			e.Pricing["prompt"] = json.RawMessage(`"0"`)
			e.Pricing["completion"] = json.RawMessage(`"0"`)
			endpointResponse(w, req.Model, e)
			return
		}
		posts.Add(1)
		raw, _ := io.ReadAll(r.Body)
		if bytes.Contains(raw, []byte("video_url")) || !bytes.Contains(raw, []byte(`"max_price":{"prompt":"0","completion":"0","request":"0","image":"0","audio":"0"}`)) || !bytes.Contains(raw, []byte(`"allow_fallbacks":false`)) {
			t.Errorf("planner policy missing: %s", raw)
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`)
	})
	if _, err := client.Complete(context.Background(), req); err != nil || posts.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, posts.Load())
	}
}

func TestStrictRedirectsFailuresAndTruncationNeverReplayOrExposeMedia(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308, 400, 403, 429, 500, 200} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			req := strictRequest()
			var posts, redirects atomic.Int32
			client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/redirect") {
					redirects.Add(1)
					return
				}
				if r.Method == "GET" {
					endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
					return
				}
				posts.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Location", "/redirect")
				w.WriteHeader(status)
				if status == 200 {
					_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"private-canary"},"finish_reason":"length"}],"usage":{"prompt_tokens":10,"completion_tokens":8192,"cost":0.01}}`)
					return
				}
				_, _ = io.WriteString(w, `{"error":{"message":"private-canary data:video/mp4;base64,YWJj https://private.test/?sig=canary"}}`)
			})
			out, err := client.Complete(context.Background(), req)
			if err == nil || posts.Load() != 1 || redirects.Load() != 0 || out.Text != "" || strings.Contains(err.Error(), "canary") || strings.Contains(llm.NormalizeFailure(err).TechnicalDetail, "canary") {
				t.Fatalf("out=%+v err=%v posts=%d redirects=%d", out, err, posts.Load(), redirects.Load())
			}
			if status == 200 && (!errors.Is(err, llm.ErrOutputTruncated) || out.Usage.CostMicrousd != 10000) {
				t.Fatalf("truncation lost usage: %+v %v", out, err)
			}
		})
	}
}

type countedVideoReader struct {
	remaining int64
	largest   int
	reads     int64
	closes    atomic.Int32
}

func (r *countedVideoReader) Read(p []byte) (int, error) {
	if len(p) > r.largest {
		r.largest = len(p)
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), r.remaining)
	clear(p[:n])
	r.remaining -= n
	r.reads += n
	return int(n), nil
}
func (r *countedVideoReader) Close() error { r.closes.Add(1); return nil }

func TestStrictBodyBoundsReadsAndChecksExactSize(t *testing.T) {
	client := New(llm.AdapterConfig{ProviderID: "test", ReasoningFormat: "openrouter"}, nil)
	for _, extra := range []int64{-1, 0, 1} {
		req := strictRequest()
		video := req.Messages[0].Parts[1].InlineVideo
		video.Size = clipLimits.InlineBytes
		reader := &countedVideoReader{remaining: video.Size + extra}
		video.Open = func(context.Context) (io.ReadCloser, error) { return reader, nil }
		envelope, _, err := client.strictEnvelope(req, "google-ai-studio", clipLimits)
		if err != nil {
			t.Fatal(err)
		}
		body, size, err := streamStrictBody(context.Background(), envelope, video)
		if err != nil {
			t.Fatal(err)
		}
		n, err := io.Copy(io.Discard, body)
		_ = body.Close()
		_ = body.Close()
		if (extra == 0 && (err != nil || n != size)) || (extra != 0 && err == nil) {
			t.Fatalf("extra=%d n=%d size=%d err=%v", extra, n, size, err)
		}
		if reader.largest > 32<<10 || reader.reads > video.Size+1 || reader.closes.Load() != 1 {
			t.Fatalf("unbounded reader: %+v", reader)
		}
		limits := clipLimits
		limits.RequestBytes = size - 1
		if _, _, err := client.strictEnvelope(req, "google-ai-studio", limits); !errors.Is(err, llm.ErrUnsupported) {
			t.Fatal("encoded request limit not enforced")
		}
	}
}

type blockingVideo struct {
	started chan struct{}
	closed  chan struct{}
	once    atomic.Bool
}

func (r *blockingVideo) Read([]byte) (int, error) {
	close(r.started)
	<-r.closed
	return 0, errors.New("private-canary")
}
func (r *blockingVideo) Close() error {
	if r.once.CompareAndSwap(false, true) {
		close(r.closed)
	}
	return nil
}

func TestStrictCancellationClosesBlockedSourceAndPipe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := strictRequest()
	reader := &blockingVideo{started: make(chan struct{}), closed: make(chan struct{})}
	video := req.Messages[0].Parts[1].InlineVideo
	video.Open = func(context.Context) (io.ReadCloser, error) { return reader, nil }
	client := New(llm.AdapterConfig{ProviderID: "test", ReasoningFormat: "openrouter"}, nil)
	envelope, _, err := client.strictEnvelope(req, "google-ai-studio", clipLimits)
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := streamStrictBody(ctx, envelope, video)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, body); _ = body.Close(); close(done) }()
	select {
	case <-reader.started:
	case <-time.After(time.Second):
		t.Fatal("reader did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation leaked a source/pipe")
	}
}

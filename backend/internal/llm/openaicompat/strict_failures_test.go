package openaicompat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStrictFailureResponsesPreserveUsageWithoutReplaying(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		stream bool
		body   string
		cost   int64
	}{
		"http error usage":   {403, false, `{"error":{"message":"private-canary"},"usage":{"cost":0.004,"prompt_tokens":10}}`, 4000},
		"http reported zero": {403, false, `{"error":{"message":"private-canary"},"usage":{"cost":0}}`, 0},
		"stream error":       {200, true, "data: {\"usage\":{\"cost\":0.004}}\n\ndata: {\"error\":{\"message\":\"private-canary\"}}\n\n", 4000},
		"stream truncation":  {200, true, "data: {\"usage\":{\"cost\":0.004},\"choices\":[{\"delta\":{\"content\":\"private-canary\"},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n", 4000},
		"unfinished stream":  {200, true, "data: {\"usage\":{\"cost\":0.004},\"choices\":[{\"delta\":{\"content\":\"private-canary\"}}]}\n\n", 4000},
	} {
		t.Run(name, func(t *testing.T) {
			req := strictRequest()
			var posts atomic.Int32
			c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
					return
				}
				posts.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if tc.stream {
					w.Header().Set("Content-Type", "text/event-stream")
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			out, err := c.Complete(context.Background(), req)
			if err == nil || posts.Load() != 1 || out.Text != "" || !out.Usage.CostReported || out.Usage.CostMicrousd != tc.cost || strings.Contains(err.Error(), "canary") {
				t.Fatalf("out=%+v err=%v posts=%d", out, err, posts.Load())
			}
		})
	}
}

func TestStrictMetadataFailuresNeverPostOrFollowRedirects(t *testing.T) {
	for _, status := range []int{200, 302, 403, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "GET" {
					t.Error("metadata failure sent completion")
				}
				w.Header().Set("Location", "/secret-canary")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, strings.Repeat("x", int(clipLimits.EndpointBytes)+1))
			})
			if _, err := c.Complete(context.Background(), strictRequest()); err == nil || calls.Load() != 1 {
				t.Fatal(err, calls.Load())
			}
		})
	}
}

func TestStrictCompleteCancellationClosesActualHTTPBodyAndSource(t *testing.T) {
	req := strictRequest()
	source := &blockingVideo{started: make(chan struct{}), closed: make(chan struct{})}
	req.Messages[0].Parts[1].InlineVideo.Open = func(context.Context) (io.ReadCloser, error) { return source, nil }
	var posts atomic.Int32
	c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
			return
		}
		posts.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := c.Complete(ctx, req); result <- err }()
	select {
	case <-source.started:
	case <-time.After(2 * time.Second):
		t.Fatal("source never opened")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil || strings.Contains(err.Error(), "canary") {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("completion leaked on cancellation")
	}
	select {
	case <-source.closed:
	default:
		t.Fatal("source leaked")
	}
	if posts.Load() > 1 {
		t.Fatal("completion replayed")
	}
}

func TestStrictOpenFailureAndOversizedResponseAreSanitized(t *testing.T) {
	for _, openFailure := range []bool{true, false} {
		req := strictRequest()
		if openFailure {
			req.Messages[0].Parts[1].InlineVideo.Open = func(context.Context) (io.ReadCloser, error) { return nil, errors.New("private-canary") }
		}
		var posts atomic.Int32
		c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
				return
			}
			posts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = io.WriteString(w, strings.Repeat("x", int(clipLimits.ResponseBytes)+1))
		})
		out, err := c.Complete(context.Background(), req)
		want := int32(1)
		if openFailure {
			want = 0
		}
		if err == nil || strings.Contains(err.Error(), "canary") || out.Text != "" || posts.Load() != want {
			t.Fatal(out, err, posts.Load())
		}
	}
}

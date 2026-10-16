package storage

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const probeWebOrigin = "https://web.example"

type corsProbeFixture struct {
	mu         sync.Mutex
	objects    map[string][]byte
	deleted    []string
	status     int
	allow      string
	expose     string
	rangeValue string
	length     string
	deleteFail bool
	putFail    bool
}

func newCORSProbeFixture(t *testing.T) (*Bucket, *corsProbeFixture) {
	t.Helper()
	f := &corsProbeFixture{objects: map[string][]byte{}, status: 206, allow: probeWebOrigin,
		expose: "ETag, Content-Range", rangeValue: "bytes 0-0/2", length: "1"}
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		key := strings.TrimPrefix(r.URL.Path, "/private/")
		if !strings.HasPrefix(key, "cors-probe/") || !strings.HasSuffix(key, ".bin") {
			t.Errorf("probe escaped its namespace: %s", key)
		}
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil || string(body) != "ok" || r.Header.Get("Content-Type") != "application/octet-stream" {
				t.Errorf("invalid probe object: bytes=%d err=%v", len(body), err)
			}
			f.objects[key] = body
			if f.putFail {
				w.WriteHeader(http.StatusForbidden)
			}
		case http.MethodDelete:
			f.deleted = append(f.deleted, key)
			if f.deleteFail {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			delete(f.objects, key)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("browser GET used the internal endpoint: %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method != http.MethodGet || r.Header.Get("Origin") != probeWebOrigin || r.Header.Get("Range") != "bytes=0-0" || r.URL.Query().Get("X-Amz-Signature") == "" {
			t.Errorf("probe did not use a presigned ranged browser GET")
		}
		key := strings.TrimPrefix(r.URL.Path, "/private/")
		if len(f.objects[key]) != 2 {
			t.Errorf("probe object was not written first")
		}
		w.Header().Set("Access-Control-Allow-Origin", f.allow)
		w.Header().Set("Access-Control-Expose-Headers", f.expose)
		w.Header().Set("Content-Range", f.rangeValue)
		w.Header().Set("Content-Length", f.length)
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte("o"))
	}))
	t.Cleanup(internal.Close)
	t.Cleanup(public.Close)
	b, err := New(t.Context(), Config{Endpoint: internal.URL, PublicEndpoint: public.URL,
		AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "private", MaxReadBytes: 2})
	if err != nil {
		t.Fatal(err)
	}
	return b, f
}

func TestCORSProbePasses(t *testing.T) {
	for _, expose := range []string{"ETag, cOnTeNt-RaNgE", "*"} {
		for _, allow := range []string{probeWebOrigin, "*"} {
			t.Run(expose+allow, func(t *testing.T) {
				b, f := newCORSProbeFixture(t)
				f.allow, f.expose = allow, expose
				if err := b.CheckCORS(t.Context(), probeWebOrigin); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCORSProbeHiddenContentRange(t *testing.T) {
	for _, exposed := range []string{"ETag", "Content-Range-Other", ""} {
		t.Run(exposed, func(t *testing.T) {
			b, f := newCORSProbeFixture(t)
			f.expose = exposed
			assertCORSProbeRefused(t, b)
		})
	}
}

func TestCORSProbeForeignOrigin(t *testing.T) {
	b, f := newCORSProbeFixture(t)
	f.allow = "https://other.example"
	assertCORSProbeRefused(t, b)
}

func TestCORSProbeRejectsInvalidOriginBeforeStorage(t *testing.T) {
	for _, origin := range []string{"", "https://web.example/path", "https://web.example?query=1", "https://user@web.example", "file:///tmp/probe"} {
		// No SDK client: malformed input must be refused before any object operation.
		b := &Bucket{}
		if err := b.CheckCORS(t.Context(), origin); err == nil || !strings.Contains(err.Error(), "deploy/r2-cors.json") {
			t.Fatalf("invalid origin accepted: %s", origin)
		}
	}
}

func TestCORSProbeNotPartial(t *testing.T) {
	b, f := newCORSProbeFixture(t)
	f.status = http.StatusOK
	assertCORSProbeRefused(t, b)
}

func TestCORSProbeInvalidRangeMetadata(t *testing.T) {
	for _, change := range []func(*corsProbeFixture){
		func(f *corsProbeFixture) { f.rangeValue = "bytes 0-1/2" },
		func(f *corsProbeFixture) { f.length = "2" },
		func(f *corsProbeFixture) { f.rangeValue = "bytes 0-0/3" },
	} {
		b, f := newCORSProbeFixture(t)
		change(f)
		assertCORSProbeRefused(t, b)
	}
}

func TestCORSProbeDeletesProbeObject(t *testing.T) {
	for _, failure := range []string{"", "get", "put"} {
		t.Run(failure, func(t *testing.T) {
			b, f := newCORSProbeFixture(t)
			if failure == "get" {
				f.expose = "ETag"
			}
			f.putFail = failure == "put"
			err := b.CheckCORS(t.Context(), probeWebOrigin)
			if (err != nil) != (failure != "") {
				t.Fatalf("unexpected result: %v", err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.objects) != 0 || len(f.deleted) != 1 {
				t.Fatalf("probe was not cleaned: objects=%d deletes=%d", len(f.objects), len(f.deleted))
			}
		})
	}
}

func TestCORSProbeFailedDeleteOnlyWarns(t *testing.T) {
	b, f := newCORSProbeFixture(t)
	f.deleteFail = true
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	if err := b.CheckCORS(t.Context(), probeWebOrigin); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `"level":"WARN"`) || !strings.Contains(logs.String(), "probe cleanup failed") {
		t.Fatalf("cleanup failure was not warned: %s", logs.String())
	}
}

func assertCORSProbeRefused(t *testing.T, b *Bucket) {
	t.Helper()
	if err := b.CheckCORS(context.Background(), probeWebOrigin); err == nil || !strings.Contains(err.Error(), "deploy/r2-cors.json") {
		t.Fatalf("expected actionable CORS refusal, got %v", err)
	}
}

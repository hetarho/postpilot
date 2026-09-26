package main

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/storage"
)

// retiredPhraseSettings are the three settings the removed 분야 phrase batch read, each set to a
// value that boot used to act on or refuse: both search keys, which started a refresh loop, and
// an interval below that loop's one-hour floor, which failed the boot.
var retiredPhraseSettings = map[string]string{
	"NAVER_SEARCH_CLIENT_ID":          "client-id",
	"NAVER_SEARCH_CLIENT_SECRET":      "client-secret",
	"QUALITY_PHRASE_REFRESH_INTERVAL": "1m",
}

// lockedBuffer is a log sink the server's goroutines may write to while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// emptyBucket is an S3 endpoint holding nothing: every listing is empty and every object is
// missing. The server's sweeps list the bucket as soon as they start, so a nil bucket would not do.
func emptyBucket(t *testing.T) *storage.Bucket {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` +
				`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>boot</Name>` +
				`<KeyCount>0</KeyCount><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated></ListBucketResult>`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code></Error>`))
	}))
	t.Cleanup(server.Close)
	bucket, err := storage.New(t.Context(), storage.Config{
		Endpoint: server.URL, AccessKeyID: "boot", SecretAccessKey: "boot", Bucket: "boot", MaxReadBytes: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	return bucket
}

// QUAL-47: the API boots and serves with no phrase refresh. The retired settings change nothing:
// the configuration equals the one loaded without them, the graph builds, /health answers, no
// goroutine of the running server is in the quality context, which has no loop of its own, and
// nothing is logged about phrases.
func TestTheAPIBootsAndServesWithoutThePhraseRefresh(t *testing.T) {
	t.Setenv("MAIL_DRIVER", "log")
	for name := range retiredPhraseSettings {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	want, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range retiredPhraseSettings {
		t.Setenv(name, value)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("the retired settings failed the config: %v", err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("the retired settings changed the config:\n got %+v\nwant %+v", cfg, want)
	}

	// A free loopback port: the server listens on every interface at cfg.Port.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(probe.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Port = port

	p := wiringPlatform(t, cfg)
	p.bucket = emptyBucket(t)

	// From the contexts on: the platform's own migration log names the migration that drops the
	// phrase lists, which is history, not a refresh.
	logs := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	app, err := buildContexts(ctx, p)
	if err != nil {
		t.Fatalf("the boot was refused: %v", err)
	}
	registerJobs(app)
	served := make(chan error, 1)
	go func() { served <- serve(ctx, app) }()

	healthy := false
	for deadline := time.Now().Add(30 * time.Second); !healthy && time.Now().Before(deadline); {
		select {
		case err := <-served:
			t.Fatalf("the server stopped before answering /health: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		response, err := http.Get("http://127.0.0.1:" + port + "/health")
		if err != nil {
			continue
		}
		response.Body.Close()
		healthy = response.StatusCode == http.StatusOK
	}
	if !healthy {
		t.Fatal("/health never answered 200")
	}

	stacks := make([]byte, 1<<24)
	stacks = stacks[:runtime.Stack(stacks, true)]
	for _, goroutine := range strings.Split(string(stacks), "\n\n") {
		if strings.Contains(goroutine, "github.com/postpilot/backend/internal/quality") {
			t.Errorf("a goroutine of the running server is in the quality context:\n%s", goroutine)
		}
	}

	cancel()
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serve = %v, want a clean stop", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the server did not stop after its context ended")
	}

	logged := logs.String()
	if !strings.Contains(logged, "server starting") {
		t.Fatalf("the boot's own log line was not captured:\n%s", logged)
	}
	if strings.Contains(strings.ToLower(logged), "phrase") {
		t.Fatalf("the boot logged about phrases:\n%s", logged)
	}
}

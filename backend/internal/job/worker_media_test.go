package job

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type testMediaFailure struct {
	error
	operation, class string
	ms               int64
}

func (e testMediaFailure) Unwrap() error             { return e.error }
func (e testMediaFailure) MediaOperation() string    { return e.operation }
func (e testMediaFailure) MediaFailureClass() string { return e.class }
func (e testMediaFailure) MediaElapsedMS() int64     { return e.ms }

func TestClipFailureLogsSafeMediaCause(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, unsafe := range []bool{false, true} {
		logs.Reset()
		cause := testMediaFailure{error: errors.New("private-canary /video.mp4"), operation: "render_cut", class: "timeout", ms: 900000}
		if unsafe {
			cause.operation, cause.class, cause.ms = "private-canary", "private-canary", -1
		}
		err := diagnosticStageError{error: cause, stage: "render"}
		logJobFailure(Job{ID: "owned-job", Kind: KindGenerateClip}, failureFromError(err), err)
		var got map[string]any
		if json.Unmarshal(logs.Bytes(), &got) != nil || strings.Contains(logs.String(), "private-canary") || strings.Contains(logs.String(), "video.mp4") {
			t.Fatal("private diagnostic escaped", logs.String())
		}
		if unsafe {
			if got["media_operation"] != nil || got["media_error_class"] != nil || got["media_elapsed_ms"] != nil {
				t.Fatal(got)
			}
		} else if got["media_operation"] != "render_cut" || got["media_error_class"] != "timeout" || got["media_elapsed_ms"] != float64(900000) {
			t.Fatal(got)
		}
	}
}

type progressLogStore struct{ terminalStore }

func (*progressLogStore) UpdateProgress(context.Context, string, string, int, int, time.Time) error {
	return nil
}

func TestClipWorkerLogsStageChangesAndTotalDuration(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	store := &progressLogStore{terminalStore: terminalStore{commit: true}}
	queue := New(store, time.Second)
	queue.Register(KindGenerateClip, func(_ context.Context, _ Job, progress Progress) error {
		progress("prepare", 0, 2)
		progress("prepare", 1, 2)
		progress("render", 0, 1)
		progress("private-canary", 0, 1)
		return context.DeadlineExceeded
	})
	queue.run(t.Context(), Job{ID: "owned-job", Kind: KindGenerateClip})
	if strings.Contains(logs.String(), "private-canary") || strings.Count(logs.String(), "clip stage changed") != 3 || strings.Count(logs.String(), "clip job stopped") != 1 {
		t.Fatal(logs.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var got map[string]any
		if json.Unmarshal([]byte(line), &got) != nil || got["job"] != "owned-job" {
			t.Fatal(line)
		}
		if got["msg"] == "clip stage changed" && (got["previous_elapsed_ms"] == nil || got["elapsed_ms"] == nil) || got["msg"] == "clip job stopped" && (got["stage_elapsed_ms"] == nil || got["elapsed_ms"] == nil) {
			t.Fatal("missing elapsed time", line)
		}
	}
}

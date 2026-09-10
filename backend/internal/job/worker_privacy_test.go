package job

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestClipWorkerLogsExcludeRawFailuresAndPanics(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	const secret = "/private/source-secret.mp4: private provider body"
	for _, kind := range []string{KindGenerateClip, KindRenderClip} {
		logs.Reset()
		found := Job{ID: "owned-job", Kind: kind}
		logJobFailure(found, failureFromError(errors.New(secret)), errors.New(secret))
		err := callHandler(context.Background(), func(context.Context, Job, Progress) error {
			panic(secret)
		}, found, nil)
		if !errors.Is(err, errHandlerPanicked) || strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), "source-secret") {
			t.Fatalf("kind=%s err=%v logs=%s", kind, err, logs.String())
		}
		if !strings.Contains(logs.String(), "job failed") || !strings.Contains(logs.String(), "job handler panicked") || !strings.Contains(logs.String(), "owned-job") {
			t.Fatal(logs.String())
		}
	}
}

type diagnosticStageError struct {
	error
	stage string
}

func (e diagnosticStageError) Unwrap() error        { return e.error }
func (e diagnosticStageError) FailureStage() string { return e.stage }

func TestClipWorkerLogsOnlyAllowlistedDiagnostics(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, unsafe := range []bool{false, true} {
		logs.Reset()
		info := llm.CallDiagnostic{Operation: "response", Class: "http_error", HTTPStatus: 403, UpstreamCode: 400, RequestID: "req-0123456789abcdef"}
		stage := "analyze"
		if unsafe {
			info = llm.CallDiagnostic{Operation: "private-canary", Class: "private-canary", HTTPStatus: 999, UpstreamCode: 999, RequestID: "https://private-canary"}
			stage = "private-canary"
		}
		cause := llm.WithCallDiagnostic(errors.New("private-canary /source.mp4 data:video/mp4;base64,private"), info)
		err := diagnosticStageError{error: fmt.Errorf("private-canary: %w", cause), stage: stage}
		logJobFailure(Job{ID: "owned-job", Kind: KindGenerateClip}, Failure{Reason: "CLIP_PROCESSING_FAILED", TechnicalDetail: "private-canary"}, err)
		if strings.Contains(logs.String(), "private-canary") || strings.Contains(logs.String(), "data:video") || strings.Contains(logs.String(), "source.mp4") {
			t.Fatal("private metadata escaped", logs.String())
		}
		var got map[string]any
		if json.Unmarshal(logs.Bytes(), &got) != nil {
			t.Fatal(logs.String())
		}
		if unsafe {
			if got["operation"] != "unknown" || got["error_class"] != "unknown" || got["http_status"] != nil || got["upstream_code"] != nil || got["request_id"] != nil || got["stage"] != nil {
				t.Fatal(got)
			}
		} else if got["stage"] != "analyze" || got["operation"] != "response" || got["error_class"] != "http_error" || got["http_status"] != float64(403) || got["upstream_code"] != float64(400) || got["request_id"] != info.RequestID {
			t.Fatal(got)
		}
	}
}

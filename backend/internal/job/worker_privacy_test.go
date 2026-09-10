package job

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
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

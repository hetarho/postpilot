package clip

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestAttemptDiagnosticsLogOnlyOwnedBoundedMetadata(t *testing.T) {
	var out bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	logAttemptDiagnostic("job", "plan", AttemptDiagnostic{Check: "private-response", Phase: "private-filename", Values: map[string]int{"target_ms": 30000, "cut": 2, "private-signed-url": 1, "after_ms": -1}})
	text := out.String()
	if strings.Contains(text, "private") || strings.Contains(text, "after_ms") || !strings.Contains(text, `"target_ms":30000`) || !strings.Contains(text, `"cut":2`) {
		t.Fatal(text)
	}
}

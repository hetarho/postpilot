package publishing

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type fixedRetirementSource struct{ snapshot RetirementSnapshot }

func (f fixedRetirementSource) RetirementSnapshot(context.Context) (RetirementSnapshot, error) {
	return f.snapshot, nil
}

func TestRetirementReportIsDeterministicMinimalAndNoOverwrite(t *testing.T) {
	source := fixedRetirementSource{snapshot: RetirementSnapshot{
		CutoffAt: "2026-09-22 00:00:00",
		Agents:   []RetirementAgent{{ID: "agent", AccountID: "alice", Label: "Mac", PlatformAccountID: "blog", RevokedAt: "2026-09-22T00:00:00Z"}},
		Jobs:     []RetirementJob{{ID: "job", AccountID: "alice", PostSlug: "post", PostCreatedAt: "2026-09-01T00:00:00Z", Status: "outcome_unknown", Stage: "verifying", CommittedAt: "2026-09-22T00:00:00Z", FailureReason: "PUBLISH_OUTCOME_UNKNOWN"}},
		Pairings: 1, Reservations: 1, Assets: 2,
	}}
	one, err := BuildRetirementReport(context.Background(), source, "production", "sha256:db")
	if err != nil {
		t.Fatal(err)
	}
	two, err := BuildRetirementReport(context.Background(), source, "production", "sha256:db")
	if err != nil {
		t.Fatal(err)
	}
	first, _ := json.Marshal(one)
	second, _ := json.Marshal(two)
	if string(first) != string(second) || one.Digest == "" {
		t.Fatalf("report is not deterministic: %s / %s", first, second)
	}
	for _, forbidden := range []string{"token", "manifest", "browser_path", "signed_url", "content"} {
		if json.Valid(first) && containsJSONKey(first, forbidden) {
			t.Fatalf("private field %q entered report: %s", forbidden, first)
		}
	}
	path := filepath.Join(t.TempDir(), "retirement.json")
	if err := WriteRetirementReport(path, one); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteRetirementReport(path, RetirementReport{Digest: "replacement"}); err == nil {
		t.Fatal("existing report was overwritten")
	}
	after, _ := os.ReadFile(path)
	if string(original) != string(after) {
		t.Fatal("existing report bytes changed")
	}
}

func containsJSONKey(payload []byte, key string) bool {
	var value any
	if json.Unmarshal(payload, &value) != nil {
		return false
	}
	var visit func(any) bool
	visit = func(v any) bool {
		switch typed := v.(type) {
		case map[string]any:
			for name, child := range typed {
				if name == key || visit(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	return visit(value)
}

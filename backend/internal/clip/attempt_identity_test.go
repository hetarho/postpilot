package clip

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAttemptIdentityIsOwnedAndContainsNoSourceMaterial(t *testing.T) {
	batch := SourceBatch{ID: "batch", ProjectID: "project", UserID: "alice"}
	for _, kind := range []string{"generate_clip", "render_clip"} {
		var data []byte
		if kind == "generate_clip" {
			data, _ = json.Marshal(generationPayload{Version: generationPayloadVersion, ProjectID: "project", Batch: batch, Approval: &GenerationApproval{QuoteID: "quote"}})
		} else {
			data, _ = json.Marshal(renderPayload{Version: 1, ProjectID: "project", Batch: batch, PlanJSON: "private content"})
		}
		a := IdentifyAttempt("alice", "project", "job", kind, data)
		if a == nil || a.JobID != "job" || a.BatchID != "batch" || (kind == "generate_clip" && a.QuoteID != "quote") {
			t.Fatal(a)
		}
		encoded, _ := json.Marshal(a)
		if strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "Sources") {
			t.Fatal(string(encoded))
		}
		if IdentifyAttempt("bob", "project", "job", kind, data) != nil || IdentifyAttempt("alice", "foreign", "job", kind, data) != nil || IdentifyAttempt("alice", "project", "", kind, data) != nil || IdentifyAttempt("alice", "project", "job", "generate", data) != nil {
			t.Fatal("unowned identity")
		}
	}
	for _, raw := range []string{"invalid", "{}", `{"Version":1,"ProjectID":"project","Batch":{"ID":"batch","ProjectID":"project","UserID":"alice"}}`} {
		if IdentifyAttempt("alice", "project", "job", "generate_clip", []byte(raw)) != nil {
			t.Fatal(raw)
		}
	}
}

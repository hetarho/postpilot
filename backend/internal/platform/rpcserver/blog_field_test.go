package rpcserver

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestBlogFieldMappingCoversGeneratedEnum(t *testing.T) {
	// 없음 plus the nine of QUAL-23. A tenth 분야 has to reach both switches before this passes.
	if got := len(postpilotv1.BlogField_name); got != 10 {
		t.Fatalf("generated 분야 = %d, want 10; update the closed mapping", got)
	}
	seen := map[string]postpilotv1.BlogField{}
	for number, name := range postpilotv1.BlogField_name {
		wire := postpilotv1.BlogField(number)
		t.Run(name, func(t *testing.T) {
			id, ok := BlogFieldFromProto(wire)
			if !ok {
				t.Fatalf("generated enum %s has no domain mapping", name)
			}
			if wire == postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED {
				if id != "" {
					t.Fatalf("없음 mapped to %q, want the empty id", id)
				}
			} else if id == "" {
				t.Fatalf("%s mapped to 없음", name)
			}
			if other, taken := seen[id]; taken {
				t.Fatalf("%s and %s both map to %q", other, wire, id)
			}
			seen[id] = wire
			if back, ok := BlogFieldToProto(id); !ok || back != wire {
				t.Fatalf("round trip = %s, %v; want %s, true", back, ok, wire)
			}
		})
	}
}

func TestBlogFieldMappingRejectsUnknownValues(t *testing.T) {
	for _, number := range []int32{10, 999, -1} {
		if id, ok := BlogFieldFromProto(postpilotv1.BlogField(number)); ok || id != "" {
			t.Errorf("unknown wire value %d mapped to %q, %v", number, id, ok)
		}
	}
	// Names, queries and near misses of an id are not ids.
	for _, id := range []string{"맛집", "Restaurant", "fashion beauty", " cafe", "unknown"} {
		if got, ok := BlogFieldToProto(id); ok || got != postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED {
			t.Errorf("id %q mapped to %s, %v", id, got, ok)
		}
	}
}

package generation

import (
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// The stage reasoning defaults moved here from platform/config with T269: they
// are a property of the work, not of the deployment.
func TestDefaultReasoningPolicy(t *testing.T) {
	p := DefaultReasoningPolicy()
	if p.Observe != llm.ReasoningLow || p.Write != llm.ReasoningLow {
		t.Fatalf("reasoning policy = %+v", p)
	}
}

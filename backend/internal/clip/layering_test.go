package clip_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestClipLayering pins ARCH-5/ARCH-7 for the clip context: the root is the pure domain
// (no store, rpc, application layer or database/sql behind it) and the adapters under it
// read the root only — none of them may reach the application layer.
func TestClipLayering(t *testing.T) {
	deps := func(pkg string) string {
		out, err := exec.Command("go", "list", "-deps", pkg).Output()
		if err != nil {
			t.Fatalf("go list -deps %s: %v", pkg, err)
		}
		return string(out)
	}
	root := deps("github.com/postpilot/backend/internal/clip")
	for _, forbidden := range []string{"internal/clip/store", "internal/clip/rpc", "internal/clip/app", "database/sql", "internal/gen/"} {
		if strings.Contains(root, forbidden) {
			t.Errorf("the clip root depends on %s", forbidden)
		}
	}
	for _, sub := range []string{"store", "ai", "media", "composition", "design", "overlay"} {
		if strings.Contains(deps("github.com/postpilot/backend/internal/clip/"+sub), "internal/clip/app") {
			t.Errorf("clip/%s depends on the application layer", sub)
		}
	}
}

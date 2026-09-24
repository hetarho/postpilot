package media

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestMediaSmokeAPIManifest(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("API command is inspected in the real image")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/api", "media-manifest")
	command.Env = []string{"DB_PATH=/unwritable/not-a-database", "PROVIDERS_CONFIG=/no-providers", "CLIP_WORK_ROOT=" + filepath.Join(t.TempDir(), "probe")}
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("offline manifest loaded API configuration: %v %s", err, raw)
	}
	var profile clip.MediaWorkerProfile
	if err = json.Unmarshal(raw, &profile); err != nil || !profile.Compatible() || profile.WorkerID != "" || profile.RuntimeManifest == "" {
		t.Fatal("invalid API profile", err)
	}
	bad := exec.CommandContext(ctx, "/api", "media-manifest")
	bad.Env = append(command.Env, "CLIP_FONT_PATH=/old/pretendard/PretendardVariable.ttf")
	if raw, err = bad.CombinedOutput(); err == nil || !strings.Contains(string(raw), "font") {
		t.Fatalf("obsolete font override was accepted: %v %s", err, raw)
	}
}

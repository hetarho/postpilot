package clip

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

// The rollout refuses an API whose executable reports another contract than its image's labels
// (deploy/media_rollout.py), so a changed identifier here that the Dockerfile does not follow
// stops every deploy. Every image the Dockerfile labels must name the identifiers this build
// speaks.
func TestTheImageLabelsNameTheMediaContractThisBuildSpeaks(t *testing.T) {
	raw, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"protocol": strconv.Itoa(MediaContractVersion),
		"renderer": MediaRendererVersion,
		"assets":   MediaAssetVersion,
	}
	for key, value := range want {
		labels := regexp.MustCompile(`org\.postpilot\.media\.`+key+`="([^"]*)"`).FindAllStringSubmatch(string(raw), -1)
		if len(labels) == 0 {
			t.Fatalf("the Dockerfile labels no media %s", key)
		}
		for _, label := range labels {
			if label[1] != value {
				t.Errorf("the Dockerfile labels media %s %q, the build speaks %q", key, label[1], value)
			}
		}
	}
}

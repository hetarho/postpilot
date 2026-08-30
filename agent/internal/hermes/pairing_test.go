package hermes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareNaverEditorUsesPairingSkillAndBoundCDP(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "pairing.log")
	t.Setenv("HERMES_PAIRING_TEST_LOG", logPath)
	binary := filepath.Join(root, "hermes")
	script := "#!/bin/sh\nprintf 'argv=%s\\nmode=%s\\ncdp=%s\\n' \"$*\" \"$POSTPILOT_MODE\" \"$BROWSER_CDP_URL\" > \"$HERMES_PAIRING_TEST_LOG\"\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := PrepareNaverEditor(context.Background(), binary, "postpilot-test", "ws://127.0.0.1:9222/devtools/browser/test"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	for _, expected := range []string{
		"--skills postpilot-publisher:postpilot-naver-pairing",
		"--toolsets postpilot-publisher,browser",
		"mode=pairing",
		"cdp=ws://127.0.0.1:9222/devtools/browser/test",
	} {
		if !strings.Contains(log, expected) {
			t.Fatalf("pairing command %q does not contain %q", log, expected)
		}
	}
}

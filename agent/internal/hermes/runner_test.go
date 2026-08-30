package hermes

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerInjectsDedicatedBrowserCDP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(writer, `{"webSocketDebuggerUrl":%q}`, "ws://"+request.Host+"/devtools/browser/test")
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, "DevToolsActivePort"), []byte(port+"\n/devtools/browser/test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	jobDir := t.TempDir()
	hermesBinary := filepath.Join(t.TempDir(), "hermes")
	script := "#!/bin/sh\nprintf '%s' \"$BROWSER_CDP_URL\" > \"$POSTPILOT_JOB_DIR/cdp.txt\"\nprintf '%s\\n' 'diagnostic' >&2\nprintf '%s\\n' '{\"status\":\"published\",\"published_url\":\"https://blog.naver.com/test/1\"}'\n"
	if err := os.WriteFile(hermesBinary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	result, err := (Runner{Binary: hermesBinary, Profile: "test", BrowserBinary: hermesBinary, BrowserProfile: profile}).Run(context.Background(), "handle", jobDir, "http://127.0.0.1:1234", "token")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "" {
		t.Fatalf("model stdout was trusted as terminal evidence: %+v", result)
	}
	cdpURL, err := os.ReadFile(filepath.Join(jobDir, "cdp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cdpURL) != "ws://127.0.0.1:"+port+"/devtools/browser/test" {
		t.Fatalf("unexpected injected CDP URL %q", cdpURL)
	}
}

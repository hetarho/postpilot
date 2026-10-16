package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/postpilot/backend/internal/platform/config"
)

func TestStorageCORSCommand(t *testing.T) {
	for _, exposed := range []bool{true, false} {
		t.Run(map[bool]string{true: "exposed", false: "hidden"}[exposed], func(t *testing.T) {
			var deleted atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPut:
					body, _ := io.ReadAll(r.Body)
					if string(body) != "ok" {
						t.Error("command did not write its two-byte object")
					}
				case http.MethodGet:
					w.Header().Set("Access-Control-Allow-Origin", "https://web.example")
					if exposed {
						w.Header().Set("Access-Control-Expose-Headers", "Content-Range")
					}
					w.Header().Set("Content-Range", "bytes 0-0/2")
					w.Header().Set("Content-Length", "1")
					w.WriteHeader(http.StatusPartialContent)
					_, _ = w.Write([]byte("o"))
				case http.MethodDelete:
					deleted.Store(true)
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer server.Close()
			cfg := &config.Config{R2Endpoint: server.URL, R2PublicEndpoint: server.URL,
				R2AccessKeyID: "fixture", R2SecretAccessKey: "fixture", R2Bucket: "private", MaxImageBytes: 2}
			var out bytes.Buffer
			err := runStorageCORSCheck(t.Context(), cfg, []string{"https://web.example"}, &out)
			if (err == nil) != exposed || !deleted.Load() {
				t.Fatalf("exposed=%t deleted=%t result=%v", exposed, deleted.Load(), err)
			}
			if exposed && !strings.Contains(out.String(), "storage CORS:") {
				t.Fatal("successful command did not report its check")
			}
			if !exposed && (out.Len() != 0 || !strings.Contains(err.Error(), "deploy/r2-cors.json")) {
				t.Fatal("failed command did not propagate its actionable error")
			}
		})
	}
}

func TestStorageCORSCommandRequiresOneOrigin(t *testing.T) {
	for _, args := range [][]string{nil, {"https://web.example", "extra"}} {
		if err := runStorageCORSCheck(t.Context(), nil, args, io.Discard); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("invalid command arguments were accepted: %v", args)
		}
	}
}

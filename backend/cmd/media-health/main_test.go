package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func unixServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	// macOS's named test temp directory can exceed Unix's socket-path limit.
	dir, err := os.MkdirTemp("/tmp", "pp-health-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "h.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return path
}

func TestHealthUsesLocalSocketAndCurrentCredentialConfiguration(t *testing.T) {
	path := unixServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/health" || r.Header.Get("X-Media-Worker-ID") != "cpu-one" ||
			r.Header.Get("Authorization") != "Bearer private-token" || r.Header.Get("X-Media-Worker-Config") != "stamp" {
			t.Error("readiness did not retain current identity/configuration/authentication")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte(`{"Ready":true,"Profile":"cpu-v4","Waiting":2,"Active":1,"OwnActive":1}`))
	}))
	status, err := check(t.Context(), path, "cpu-one", "private-token", "stamp")
	if err != nil || !status.Ready || status.Waiting != 2 || status.OwnActive != 1 {
		t.Fatalf("valid private readiness was lost: %+v, %v", status, err)
	}
}

func TestHealthRejectsBadResponsesAndUnavailableOrTimedOutSocket(t *testing.T) {
	for _, tc := range []struct {
		code int
		body string
	}{
		{503, `{"Ready":true,"Profile":"cpu-v4"}`},
		{200, `{"Ready":false,"Profile":"cpu-v4"}`},
		{200, `{"Ready":true}`},
		{200, `{"Ready":true,"Profile":"cpu-v4","Active":-1}`},
		{200, `{"Ready":true,"Profile":"cpu-v4","Active":1,"OwnActive":2}`},
		{200, `not json`},
		{200, strings.Repeat("x", 4097)},
		{302, ""},
	} {
		path := unixServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "http://unexpected.invalid/")
			w.WriteHeader(tc.code)
			w.Write([]byte(tc.body))
		}))
		if _, err := check(t.Context(), path, "cpu", "token", "stamp"); err == nil {
			t.Fatalf("invalid readiness accepted: %d %q", tc.code, tc.body)
		}
	}
	if _, err := check(t.Context(), filepath.Join(t.TempDir(), "missing.sock"), "cpu", "token", "stamp"); err == nil {
		t.Fatal("unavailable worker accepted")
	}
	path := unixServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := check(ctx, path, "cpu", "token", "stamp"); err == nil {
		t.Fatal("hung readiness escaped its context deadline")
	}
}

func TestSocketNamespaceAndConfigurationBindAllRuntimeInputsWithoutSecrets(t *testing.T) {
	base := t.TempDir()
	path, err := socketPath(base, "cpu-one")
	if err != nil || len(path) >= 108 {
		t.Fatalf("invalid short Unix socket path: %q %v", path, err)
	}
	digest := sha256.Sum256([]byte("cpu-one"))
	root, _ := filepath.Abs(filepath.Join(base, "worker-"+hex.EncodeToString(digest[:])))
	expected := sha256.Sum256([]byte(root))
	if path != filepath.Join("/tmp", "postpilot-health-"+hex.EncodeToString(expected[:]), "h.sock") {
		t.Fatal("socket no longer names the worker's absolute identity root")
	}
	other, _ := socketPath(base, "cpu-two")
	if other == path {
		t.Fatal("worker identities share readiness")
	}
	env := map[string]string{"MEDIA_WORKER_ID": "cpu-one", "MEDIA_WORKER_TOKEN": "secret"}
	getenv := func(key string) string { return env[key] }
	before := configuration(getenv)
	env["MEDIA_WORKER_TOKEN"] = "replacement"
	if configuration(getenv) != before {
		t.Fatal("credential material leaked into configuration stamp")
	}
	for _, key := range configKeys {
		original := env[key]
		env[key] = original + "changed"
		if configuration(getenv) == before {
			t.Fatalf("runtime input %s not bound", key)
		}
		env[key] = original
	}
}

func TestLocalHTTPFramingIsBoundedAndCannotAcceptTruncatedOrAmbiguousBodies(t *testing.T) {
	valid := `{"Ready":true,"Profile":"cpu","Waiting":0,"Active":0,"OwnActive":0}`
	for _, response := range []string{
		"HTTP/1.1 200 OK\r\n\r\n{}",
		"HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\n{}",
		fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\nContent-Length: %d\r\n\r\n%s", len(valid), len(valid), valid),
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n",
		"HTTP/1.1 200 OK\r\nContent-Length: -1\r\n\r\n",
		fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: +%d\r\n\r\n%s", len(valid), valid),
		fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\nTransfer-Encoding : chunked\r\n\r\n%s", len(valid), valid),
		"HTTP/1.1 200 OK\r\nX-Large: " + strings.Repeat("x", 4096) + "\r\n\r\n",
	} {
		path := unixServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			defer connection.Close()
			connection.Write([]byte(response))
		}))
		if _, err := check(t.Context(), path, "cpu", "token", "stamp"); err == nil {
			t.Fatal("invalid local HTTP framing accepted")
		}
	}
}

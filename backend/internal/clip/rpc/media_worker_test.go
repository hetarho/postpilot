package rpc

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type unreadMediaBody struct{ read bool }

func (b *unreadMediaBody) Read([]byte) (int, error) { b.read = true; return 0, io.EOF }
func (*unreadMediaBody) Close() error               { return nil }

func TestMediaAuthenticationPrecedesBodyAndRejectsOtherAuthorities(t *testing.T) {
	for name, headers := range map[string]map[string]string{
		"missing": {}, "cookie": {"Cookie": "session=user-session"},
		"wrong token":       {MediaWorkerIdentityHeader: "prod-1", "Authorization": "Bearer other-env"},
		"other environment": {MediaWorkerIdentityHeader: "staging-1", "Authorization": "Bearer secret"},
		"disabled identity": {MediaWorkerIdentityHeader: "old-1", "Authorization": "Bearer old-secret"},
	} {
		t.Run(name, func(t *testing.T) {
			server := NewMediaWorkerServer("", map[string]string{"prod-1": "secret"}, nil)
			body := &unreadMediaBody{}
			r := httptest.NewRequest(http.MethodPost, "/postpilot.v1.ClipMediaWorkerService/ClaimMediaStage", body)
			for key, value := range headers {
				r.Header.Set(key, value)
			}
			w := httptest.NewRecorder()
			server.Handler.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized || body.read {
				t.Fatalf("status=%d body read=%v", w.Code, body.read)
			}
		})
	}
}

type mediaDeadlineProbe struct {
	MediaWorkerService
	checked bool
}

type roleClaimProbe struct {
	MediaWorkerService
	claims int
}

func (p *roleClaimProbe) Claim(context.Context, clip.MediaWorkerProfile) (*clip.MediaWork, error) {
	p.claims++
	return nil, nil
}
func TestMediaVerificationRoleCannotClaimNativeWorkOrChangeItsRole(t *testing.T) {
	for _, test := range []struct {
		id, role, operation string
		allowed             bool
	}{{"native", clip.NativeWorkerRole, "verify_analysis", false}, {"verify", clip.AnalysisVerificationRole, "render", false}, {"verify", clip.AnalysisVerificationRole, "prepare", false}, {"verify", clip.AnalysisVerificationRole, "verify_analysis", true}, {"native", clip.NativeWorkerRole, "render", true}} {
		p := &roleClaimProbe{}
		body := `{"profile":{"operation":"` + test.operation + `"}}`
		r := httptest.NewRequest(http.MethodPost, "/postpilot.v1.ClipMediaWorkerService/ClaimMediaStage", strings.NewReader(body))
		r.Header.Set(MediaWorkerIdentityHeader, test.id)
		r.Header.Set("Authorization", "Bearer secret")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		NewMediaWorkerServerWithRoles("", map[string]string{test.id: "secret"}, map[string]string{test.id: test.role}, p).Handler.ServeHTTP(w, r)
		if (p.claims == 1) != test.allowed {
			t.Fatalf("%s/%s delegated claims=%d status=%d", test.role, test.operation, p.claims, w.Code)
		}
	}
}

func (p *mediaDeadlineProbe) Claim(ctx context.Context, profile clip.MediaWorkerProfile) (*clip.MediaWork, error) {
	deadline, ok := ctx.Deadline()
	p.checked = ok && time.Until(deadline) > 0 && time.Until(deadline) <= clip.MediaUnaryTimeout && profile.WorkerID == "prod-1"
	return nil, errors.New("subprocess stderr https://signed.example?secret=private")
}
func TestMediaDeadlineAndClosedErrors(t *testing.T) {
	p := &mediaDeadlineProbe{}
	r := httptest.NewRequest(http.MethodPost, "/postpilot.v1.ClipMediaWorkerService/ClaimMediaStage", strings.NewReader(`{"profile":{"operation":"render"}}`))
	r.Header.Set(MediaWorkerIdentityHeader, "prod-1")
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	NewMediaWorkerServer("", map[string]string{"prod-1": "secret"}, p).Handler.ServeHTTP(w, r)
	if !p.checked || w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "stderr") {
		t.Fatal("deadline/principal/error boundary", w.Code, w.Body.String())
	}
}
func TestMediaRequestCapsCompressedAndPlainBodies(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		payload := []byte(`{"profile":{"runtimeManifest":"` + strings.Repeat("a", clip.MediaRequestMaxBytes) + `"}}`)
		if compressed {
			var b bytes.Buffer
			z := gzip.NewWriter(&b)
			_, _ = z.Write(payload)
			_ = z.Close()
			payload = b.Bytes()
		}
		r := httptest.NewRequest(http.MethodPost, "/postpilot.v1.ClipMediaWorkerService/ClaimMediaStage", bytes.NewReader(payload))
		r.Header.Set(MediaWorkerIdentityHeader, "prod-1")
		r.Header.Set("Authorization", "Bearer secret")
		r.Header.Set("Content-Type", "application/json")
		if compressed {
			r.Header.Set("Content-Encoding", "gzip")
		}
		w := httptest.NewRecorder()
		NewMediaWorkerServer("", map[string]string{"prod-1": "secret"}, nil).Handler.ServeHTTP(w, r)
		if w.Code != http.StatusTooManyRequests && w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("compressed=%v status=%d body=%s", compressed, w.Code, w.Body.String())
		}
	}
}

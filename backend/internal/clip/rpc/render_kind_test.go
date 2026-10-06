package rpc

import (
	"errors"
	"testing"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestNativeExportPlanRefusalIsAProductFailure(t *testing.T) {
	err := toConnectError(clip.ErrServerExportPlan)
	var rpc *connect.Error
	if !errors.As(err, &rpc) || rpc.Code() != connect.CodePermissionDenied || len(rpc.Details()) != 1 {
		t.Fatal(err)
	}
	value, e := rpc.Details()[0].Value()
	detail, ok := value.(*v1.AppErrorDetail)
	if e != nil || !ok || detail.Reason != "CLIP_SERVER_EXPORT_PLAN_REQUIRED" || len(detail.Params) != 0 {
		t.Fatal(value, e)
	}
}

func TestRenderKindRefusedBeforeWork(t *testing.T) {
	h := NewHandler(nil)
	h.generation = &clipapp.GenerationService{}
	for _, tc := range []struct {
		kind v1.ClipRenderKind
		code connect.Code
	}{
		{v1.ClipRenderKind_CLIP_RENDER_KIND_UNSPECIFIED, connect.CodeInvalidArgument},
		{v1.ClipRenderKind(99), connect.CodeInvalidArgument},
	} {
		_, err := h.StartClipRender(auth.WithUser(t.Context(), "alice"), connect.NewRequest(&v1.StartClipRenderRequest{RenderKind: tc.kind}))
		if connect.CodeOf(err) != tc.code {
			t.Fatalf("kind %v: %v", tc.kind, err)
		}
	}
}

func TestBrowserIncompatibleVersionRefusesExplicitly(t *testing.T) {
	err := toConnectError(clip.ErrBrowserCompositionVersion)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !errors.Is(err, clip.ErrBrowserCompositionVersion) {
		t.Fatal(err)
	}
}

func TestProjectLastRenderKindComesOnlyFromItsResult(t *testing.T) {
	if p := projectProto(clip.Project{}); p.LastRenderKind != nil || p.Result != nil {
		t.Fatal("unrendered project acquired a kind", p)
	}
	for _, tc := range []struct {
		kind clip.RenderKind
		want v1.ClipRenderKind
	}{
		{"", v1.ClipRenderKind_CLIP_RENDER_KIND_SERVER},
		{clip.RenderServer, v1.ClipRenderKind_CLIP_RENDER_KIND_SERVER},
		{clip.RenderBrowser, v1.ClipRenderKind_CLIP_RENDER_KIND_BROWSER},
	} {
		p := projectProto(clip.Project{Result: &clip.Result{Kind: tc.kind}, EditPlanRevision: 3, RenderedPlanRevision: 2})
		if p.LastRenderKind == nil || *p.LastRenderKind != tc.want || p.Result.RenderKind != tc.want {
			t.Fatalf("kind %q: %v", tc.kind, p)
		}
	}
}

func TestNativeCapacityRefusalsAreDistinctProductFailures(t *testing.T) {
	for _, tc := range []struct {
		cause  error
		reason string
	}{
		{clip.ErrRenderOverloaded, "CLIP_SERVER_RENDER_OVERLOADED"},
		{clip.ErrRenderAccountBusy, "CLIP_SERVER_RENDER_ACCOUNT_BUSY"},
	} {
		err := toConnectError(tc.cause)
		var rpc *connect.Error
		if !errors.As(err, &rpc) || rpc.Code() != connect.CodeResourceExhausted || len(rpc.Details()) != 1 {
			t.Fatal(err)
		}
		value, e := rpc.Details()[0].Value()
		detail, ok := value.(*v1.AppErrorDetail)
		if e != nil || !ok || detail.Reason != tc.reason {
			t.Fatal(value, e)
		}
	}
}

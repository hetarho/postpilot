package rpc

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestRenderKindRefusedBeforeWork(t *testing.T) {
	h := NewHandler(nil)
	h.generation = &clip.GenerationService{}
	for _, tc := range []struct {
		kind v1.ClipRenderKind
		code connect.Code
	}{
		{v1.ClipRenderKind_CLIP_RENDER_KIND_UNSPECIFIED, connect.CodeInvalidArgument},
		{v1.ClipRenderKind(99), connect.CodeInvalidArgument},
		{v1.ClipRenderKind_CLIP_RENDER_KIND_BROWSER, connect.CodeUnimplemented},
	} {
		_, err := h.StartClipRender(auth.WithUser(t.Context(), "alice"), connect.NewRequest(&v1.StartClipRenderRequest{RenderKind: tc.kind}))
		if connect.CodeOf(err) != tc.code {
			t.Fatalf("kind %v: %v", tc.kind, err)
		}
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

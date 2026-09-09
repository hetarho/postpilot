package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/config"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type rpcStore struct {
	clip.Store
	user          string
	patch         clip.ProjectPatch
	templatePatch clip.TemplatePatch
}

func (s *rpcStore) GetProject(_ context.Context, user, id string) (clip.Project, error) {
	s.user = user
	if id != "owned" {
		return clip.Project{}, clip.ErrNotFound
	}
	return clip.Project{ID: id, UserID: user, Ratio: "vertical", TargetDurationMS: 30000}, nil
}
func (s *rpcStore) UpdateProject(_ context.Context, user, id string, p clip.ProjectPatch, _ time.Time) (clip.Project, error) {
	s.user = user
	s.patch = p
	return clip.Project{ID: id, Ratio: "vertical"}, nil
}
func (s *rpcStore) GetTemplate(_ context.Context, user, id string) (clip.VideoTemplate, error) {
	s.user = user
	if id != "owned" {
		return clip.VideoTemplate{}, clip.ErrNotFound
	}
	return clip.VideoTemplate{ID: id}, nil
}
func (s *rpcStore) UpdateTemplate(_ context.Context, user, id string, p clip.TemplatePatch, _ time.Time) (clip.VideoTemplate, error) {
	s.user = user
	s.templatePatch = p
	return clip.VideoTemplate{ID: id}, nil
}
func TestPresenceActorAndNotFound(t *testing.T) {
	s := &rpcStore{}
	h := NewHandler(clip.NewService(s, config.ClipLimits()))
	ctx := auth.WithUser(context.Background(), "alice")
	title := "new title"
	response, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", Title: &title, Answers: []*v1.ClipAnswer{{Label: "場所", Text: ""}}}))
	if err != nil {
		t.Fatal(err)
	}
	if s.user != "alice" || s.patch.Title == nil || s.patch.TargetDurationMS != nil || s.patch.VideoTemplateID != nil || len(s.patch.Answers) != 1 || response.Msg.Project.Ratio != "vertical" {
		t.Fatalf("presence/actor lost: %+v", s)
	}
	neutral := ""
	_, err = h.UpdateVideoTemplate(ctx, connect.NewRequest(&v1.UpdateVideoTemplateRequest{Id: "owned", InformationFields: &v1.ClipInformationFields{}, Accent: &neutral}))
	if err != nil {
		t.Fatal(err)
	}
	if s.templatePatch.InformationFields == nil || len(*s.templatePatch.InformationFields) != 0 || s.templatePatch.CopyStyles != nil || s.templatePatch.Name != nil || s.templatePatch.Accent == nil {
		t.Fatal("wrapper presence lost")
	}
	for _, id := range []string{"foreign", "unknown"} {
		_, err := h.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: id}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatal(err)
		}
	}
}
func TestEveryProcedureRequiresActor(t *testing.T) {
	h := NewHandler(nil)
	ctx := context.Background()
	calls := []func() error{
		func() error { _, e := h.ListVideoTemplates(ctx, nil); return e }, func() error { _, e := h.CreateVideoTemplate(ctx, nil); return e }, func() error { _, e := h.UpdateVideoTemplate(ctx, nil); return e }, func() error { _, e := h.DeleteVideoTemplate(ctx, nil); return e },
		func() error { _, e := h.ListClipProjects(ctx, nil); return e }, func() error { _, e := h.CreateClipProject(ctx, nil); return e }, func() error { _, e := h.GetClipProject(ctx, nil); return e }, func() error { _, e := h.UpdateClipProject(ctx, nil); return e }, func() error { _, e := h.DeleteClipProject(ctx, nil); return e },
	}
	for _, call := range calls {
		if err := call(); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatal(err)
		}
	}
}
func TestStableFailureDetails(t *testing.T) {
	for _, tc := range []struct {
		err    error
		code   connect.Code
		reason string
	}{{clip.ErrInvalid, connect.CodeInvalidArgument, "CLIP_INVALID_INPUT"}, {clip.ErrNotFound, connect.CodeNotFound, "CLIP_NOT_FOUND"}, {clip.ErrDuplicateName, connect.CodeAlreadyExists, "CLIP_TEMPLATE_NAME_TAKEN"}} {
		err := toConnectError(tc.err)
		var ce *connect.Error
		if !errors.As(err, &ce) || ce.Code() != tc.code || len(ce.Details()) != 1 {
			t.Fatal(err)
		}
		detail, e := ce.Details()[0].Value()
		if e != nil || detail.(*v1.AppErrorDetail).Reason != tc.reason {
			t.Fatalf("detail: %v %v", detail, e)
		}
	}
}
func TestWireHasNoOwnerClaimOrRatioUpdate(t *testing.T) {
	methods := v1.File_postpilot_v1_clip_proto.Services().ByName("ClipService").Methods()
	for i := 0; i < methods.Len(); i++ {
		if methods.Get(i).Input().Fields().ByName("user_id") != nil {
			t.Fatal("request claims an owner")
		}
	}
	fields := (&v1.UpdateClipProjectRequest{}).ProtoReflect().Descriptor().Fields()
	if fields.ByName(protoreflect.Name("ratio")) != nil {
		t.Fatal("ratio is mutable")
	}
}

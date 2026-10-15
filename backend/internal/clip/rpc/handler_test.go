package rpc

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type rpcStore struct {
	clip.Store
	clip.SourceStore
	user          string
	patch         clip.ProjectPatch
	templatePatch clip.TemplatePatch
	project       *clip.Project
	inserted      []clip.Project
}

func (s *rpcStore) GetProject(_ context.Context, user, id string) (clip.Project, error) {
	s.user = user
	if id != "owned" {
		return clip.Project{}, clip.ErrNotFound
	}
	if s.project != nil {
		return *s.project, nil
	}
	return clip.Project{ID: id, UserID: user, Ratio: "vertical", TargetDurationMS: 30000}, nil
}
func (s *rpcStore) UpdateProject(_ context.Context, user, id string, p clip.ProjectPatch, _ time.Time) (clip.Project, error) {
	s.user = user
	s.patch = p
	if s.project != nil {
		value := *s.project
		if p.Channel != nil {
			value.Channel = *p.Channel
		}
		s.project = &value
		return value, nil
	}
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
	h := NewHandler(testProjects(s))
	ctx := auth.WithUser(context.Background(), "alice")
	title := "new title"
	response, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", Title: &title}))
	if err != nil {
		t.Fatal(err)
	}
	if s.user != "alice" || s.patch.Title == nil || s.patch.TargetDurationMS != nil || s.patch.VideoTemplateID != nil || response.Msg.Project.Ratio != "vertical" {
		t.Fatalf("presence/actor lost: %+v", s)
	}
	name := "새 이름"
	if _, err = h.UpdateVideoTemplate(ctx, connect.NewRequest(&v1.UpdateVideoTemplateRequest{Id: "owned", Name: &name})); err != nil {
		t.Fatal(err)
	}
	if s.templatePatch.Name == nil || *s.templatePatch.Name != name || s.templatePatch.CompositionBody != nil {
		t.Fatalf("template presence lost: %+v", s.templatePatch)
	}
	// The disclosure carries the same presence semantics: absent is not a
	// change, and an explicit value reaches the service as one.
	disclosure := "ad"
	if _, err = h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", Disclosure: &disclosure})); err != nil {
		t.Fatal(err)
	}
	if s.patch.Disclosure == nil || *s.patch.Disclosure != "ad" {
		t.Fatalf("disclosure presence: %+v", s.patch)
	}
	// A template is its outline: a create without a body is refused before it
	// reaches the store, rather than minted as a template of another kind
	// (CLIP-4, CLIP-14).
	for _, body := range []string{"", "  "} {
		if _, err := h.CreateVideoTemplate(ctx, connect.NewRequest(&v1.CreateVideoTemplateRequest{Name: "카페", CompositionBody: body})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("a body-less template was not refused: %q %v", body, err)
		}
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
		func() error { _, e := h.SaveClipEditPlan(ctx, nil); return e },
		func() error { _, e := h.StartClipRender(ctx, nil); return e },
		func() error { _, e := h.ReportClipRenderVerdict(ctx, nil); return e },
		func() error { _, e := h.PrepareClipRenderUpload(ctx, nil); return e },
		func() error { _, e := h.CompleteClipRenderUpload(ctx, nil); return e },
		func() error { _, e := h.CancelClipBrowserRender(ctx, nil); return e },
		func() error { _, e := h.StartClipGeneration(ctx, nil); return e },
		func() error { _, e := h.QuoteClipGeneration(ctx, nil); return e },
		func() error { _, e := h.CreateClipSourceBatch(ctx, nil); return e }, func() error { _, e := h.ConfirmClipSource(ctx, nil); return e }, func() error { _, e := h.DiscardClipSourceBatch(ctx, nil); return e },
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
	}{{clip.ErrInvalid, connect.CodeInvalidArgument, "CLIP_INVALID_INPUT"}, {clip.ErrCopyTooLong, connect.CodeInvalidArgument, "CLIP_COPY_TOO_LONG"}, {clip.ErrNotFound, connect.CodeNotFound, "CLIP_NOT_FOUND"}, {clip.ErrDuplicateName, connect.CodeAlreadyExists, "CLIP_TEMPLATE_NAME_TAKEN"}} {
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
	// Every clip service, since the split (T281): a request may never claim its own owner.
	for _, file := range []protoreflect.FileDescriptor{
		v1.File_postpilot_v1_clip_template_proto,
		v1.File_postpilot_v1_clip_source_proto,
		v1.File_postpilot_v1_clip_generation_proto,
		v1.File_postpilot_v1_clip_plan_proto,
		v1.File_postpilot_v1_clip_render_proto,
	} {
		services := file.Services()
		for s := 0; s < services.Len(); s++ {
			methods := services.Get(s).Methods()
			for i := 0; i < methods.Len(); i++ {
				if methods.Get(i).Input().Fields().ByName("user_id") != nil {
					t.Fatalf("%s claims an owner", methods.Get(i).FullName())
				}
			}
		}
	}
	fields := (&v1.UpdateClipProjectRequest{}).ProtoReflect().Descriptor().Fields()
	if fields.ByName(protoreflect.Name("ratio")) != nil {
		t.Fatal("ratio is mutable")
	}
}

func TestResultWireExposesURLsButNoPrivateObjectKey(t *testing.T) {
	p := projectProto(clip.Project{ID: "clip", UserID: "alice", Analysis: "private analysis", EditPlan: "private plan", Result: &clip.Result{Key: "clip-results/private", ViewURL: "https://signed/inline", DownloadURL: "https://signed/download", Bytes: 5, DurationMS: 15000}})
	if p.Result.GetViewUrl() != "https://signed/inline" || p.Result.GetDownloadUrl() != "https://signed/download" {
		t.Fatal(p)
	}
	for _, name := range []protoreflect.Name{"key", "object_key", "user_id"} {
		if p.Result.ProtoReflect().Descriptor().Fields().ByName(name) != nil {
			t.Fatal("private result field on wire", name)
		}
	}
	fields := (&v1.StartClipGenerationRequest{}).ProtoReflect().Descriptor().Fields()
	for _, name := range []protoreflect.Name{"project_id", "batch_id", "observe_model", "write_model"} {
		if fields.ByName(name) == nil {
			t.Fatal("missing generation input", name)
		}
	}
}

func TestCreateProjectRefusesAbsentAndUnknownWireLanguages(t *testing.T) {
	h := NewHandler(testProjects(&rpcStore{}))
	ctx := auth.WithUser(t.Context(), "alice")
	for _, language := range []v1.ContentLanguage{v1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED, v1.ContentLanguage(99)} {
		_, err := h.CreateClipProject(ctx, connect.NewRequest(&v1.CreateClipProjectRequest{Language: language}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("accepted absent/unknown language: %v", err)
		}
	}
}

func (s *rpcStore) InsertProject(_ context.Context, p clip.Project) error {
	s.inserted = append(s.inserted, p)
	return nil
}

func TestCreateIsVerticalOnlyAndDefaultsTheChannel(t *testing.T) {
	s := &rpcStore{}
	h := NewHandler(testProjects(s))
	ctx := auth.WithUser(t.Context(), "alice")
	for _, channel := range []string{"", clip.ChannelNaver, clip.ChannelInstagram} {
		got, err := h.CreateClipProject(ctx, connect.NewRequest(&v1.CreateClipProjectRequest{Title: "new", Ratio: "vertical", Channel: channel, Language: v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN}))
		if err != nil {
			t.Fatal(err)
		}
		if got.Msg.Project.Channel != clip.DefaultChannel(channel) || got.Msg.Project.Ratio != "vertical" || s.inserted[len(s.inserted)-1].UserID != "alice" {
			t.Fatal(got.Msg.Project)
		}
	}
	accepted := len(s.inserted)
	for _, ratio := range []string{"", "horizontal", "square", "9:16"} {
		_, err := h.CreateClipProject(ctx, connect.NewRequest(&v1.CreateClipProjectRequest{Title: "new", Ratio: ratio, Language: v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN}))
		var ce *connect.Error
		if !errors.As(err, &ce) || ce.Code() != connect.CodeInvalidArgument || len(ce.Details()) != 1 {
			t.Fatal(ratio, err)
		}
		detail, _ := ce.Details()[0].Value()
		if detail.(*v1.AppErrorDetail).Reason != "CLIP_RATIO_UNSUPPORTED" {
			t.Fatal(detail)
		}
	}
	if len(s.inserted) != accepted {
		t.Fatal("invalid ratio was written")
	}
	_, err := h.CreateClipProject(ctx, connect.NewRequest(&v1.CreateClipProjectRequest{Title: "new", Ratio: "vertical", Channel: "unknown", Language: v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || len(s.inserted) != accepted {
		t.Fatal(err)
	}
}

func TestUpdateMovesTheChannelAlone(t *testing.T) {
	original := clip.Project{ID: "owned", UserID: "alice", Title: "keep", Ratio: "horizontal", Channel: clip.ChannelNaver, Language: "ko", Instruction: "keep instruction", TargetDurationMS: 30000, EditPlanRevision: 3, RenderedPlanRevision: 3, Analysis: "saved analysis", Result: &clip.Result{ID: "saved"}}
	s := &rpcStore{project: &original}
	h := NewHandler(testProjects(s))
	ctx := auth.WithUser(t.Context(), "alice")
	for _, channel := range []string{clip.ChannelInstagram, clip.ChannelNaver} {
		response, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", Channel: &channel}))
		if err != nil {
			t.Fatal(err)
		}
		want := original
		want.Channel = channel
		if !reflect.DeepEqual(*s.project, want) || response.Msg.Project.Channel != channel || !reflect.DeepEqual(s.patch, clip.ProjectPatch{Channel: &channel}) {
			t.Fatalf("move changed another field: %+v %+v", s.project, s.patch)
		}
	}
	for _, invalid := range []string{"", "unknown"} {
		if _, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", Channel: &invalid})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatal(err)
		}
	}
	_, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "foreign", Channel: ptrChannel(clip.ChannelInstagram)}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal(err)
	}
}
func ptrChannel(value string) *string { return &value }

func TestFinalizedChannelMoveKeepsEveryCanonicalFieldLocked(t *testing.T) {
	original := clip.Project{ID: "owned", UserID: "alice", Title: "confirmed", Ratio: "square", Channel: clip.ChannelNaver, Finalized: &clip.Finalization{At: time.Now(), PlanRevision: 1, ResultID: "result"}}
	s := &rpcStore{project: &original}
	h := NewHandler(testProjects(s))
	ctx := auth.WithUser(t.Context(), "alice")
	channel, title := clip.ChannelInstagram, "changed"
	if _, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", Channel: &channel, Title: &title})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatal("mixed edit accepted", err)
	}
	response, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", Channel: &channel}))
	if err != nil {
		t.Fatal(err)
	}
	want := original
	want.Channel = channel
	if !reflect.DeepEqual(*s.project, want) || response.Msg.Project.FinalizedResultId != "result" {
		t.Fatal(s.project, response.Msg.Project)
	}
}

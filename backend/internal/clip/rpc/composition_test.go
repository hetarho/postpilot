package rpc

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/config"
)

type nativeRPCStore struct{ rpcStore }

func (s *nativeRPCStore) GetProject(_ context.Context, user, id string) (clip.Project, error) {
	if id != "owned" || user != "alice" {
		return clip.Project{}, clip.ErrNotFound
	}
	return clip.Project{ID: id, UserID: user, Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: 1, Body: `<clip version="1"><field id="a" label="同名"/><field id="b" label="同名"/></clip>`}, Inputs: clip.CompositionInputs{Values: map[string]string{"a": "old"}}}}, nil
}

func TestCompositionRPCPresenceAndCapabilities(t *testing.T) {
	s := &nativeRPCStore{}
	h := NewHandler(clip.NewService(s, config.ClipLimits()))
	ctx := auth.WithUser(context.Background(), "alice")
	if _, err := h.GetClipCapabilities(context.Background(), connect.NewRequest(&v1.GetClipCapabilitiesRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
	caps, err := h.GetClipCapabilities(ctx, connect.NewRequest(&v1.GetClipCapabilitiesRequest{}))
	if err != nil || caps.Msg.CompositionVersion != 1 || caps.Msg.CompositionPlanVersion != 0 {
		t.Fatal(caps, err)
	}
	_, err = h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", CompositionInputs: &v1.ClipCompositionInputs{Values: map[string]string{"a": "  <한글>\n", "b": "exact"}}}))
	if err != nil {
		t.Fatal(err)
	}
	if s.patch.Composition == nil || s.patch.Composition.Inputs.Values["a"] != "  <한글>\n" {
		t.Fatal("RPC lost exact ID values", s.patch)
	}
	_, err = h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", CompositionInputs: &v1.ClipCompositionInputs{}}))
	if err != nil || s.patch.Composition == nil || len(s.patch.Composition.Inputs.Values) != 0 {
		t.Fatal("explicit clear lost", s.patch, err)
	}
	_, err = h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned"}))
	if err != nil || s.patch.Composition != nil {
		t.Fatal("absent became clear", err)
	}
	_, err = h.UpdateClipProject(auth.WithUser(ctx, "bob"), connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", CompositionInputs: &v1.ClipCompositionInputs{}}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("foreign mutation", err)
	}
}

func TestCompositionRPCMapsExactGroupedInputsAndStableErrors(t *testing.T) {
	c := &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: 1, Body: " <clip version=\"1\"/> ", TemplateID: "template"}, Inputs: clip.CompositionInputs{Values: map[string]string{"a": "same label"}, Items: map[string][]composition.Item{"menu": {{ID: "first", Values: map[string]string{"price": "10,000원"}}, {ID: "second", Values: map[string]string{"price": "20,000원"}}}}, Associations: []clip.SourceAssociation{{GroupID: "menu", ItemID: "second", SourceID: "source", Fingerprint: "sha", StartMS: 100, EndMS: 200}}}}
	proto := compositionProto(c)
	if proto.Snapshot.Body != c.Snapshot.Body || !reflect.DeepEqual(compositionInputs(proto.Inputs), &c.Inputs) {
		t.Fatal("composition mapping changed values")
	}
	err := toConnectError(&composition.Problem{ElementID: "price", Line: 7, Reason: "required_binding"})
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeInvalidArgument || len(ce.Details()) != 1 {
		t.Fatal(err)
	}
	if connect.CodeOf(toConnectError(clip.ErrCompositionUnavailable)) != connect.CodeFailedPrecondition {
		t.Fatal("unavailable execution is not input rejection")
	}
}

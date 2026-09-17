package rpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/config"
)

// The design selection crosses the wire the way the pace and the accent do:
// absent means leave it, present means change it, and a present-and-empty style
// list is a selection of none (CLIP-139, CLIP-142).
func TestDesignSelectionKeepsItsPresenceAcrossTheWire(t *testing.T) {
	s := &rpcStore{}
	h := NewHandler(clip.NewService(s, config.ClipLimits()))
	ctx := auth.WithUser(context.Background(), "alice")
	if _, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned"})); err != nil {
		t.Fatal(err)
	}
	if s.patch.IntroPreset != nil || s.patch.OutroPreset != nil || s.patch.CaptionStyles != nil {
		t.Fatalf("an untouched selection was sent as a change: %+v", s.patch)
	}
	intro, outro := "a", "b"
	request := &v1.UpdateClipProjectRequest{Id: "owned", IntroPreset: &intro, OutroPreset: &outro,
		AllowedCaptionStyles: &v1.ClipCaptionStyles{Values: []string{"bold"}}}
	if _, err := h.UpdateClipProject(ctx, connect.NewRequest(request)); err != nil {
		t.Fatal(err)
	}
	if s.patch.IntroPreset == nil || *s.patch.IntroPreset != "a" || s.patch.OutroPreset == nil || *s.patch.OutroPreset != "b" {
		t.Fatalf("the presets did not cross the wire: %+v", s.patch)
	}
	if s.patch.CaptionStyles == nil || len(*s.patch.CaptionStyles) != 1 || (*s.patch.CaptionStyles)[0] != "bold" {
		t.Fatalf("the styles did not cross the wire: %+v", s.patch.CaptionStyles)
	}
	empty := &v1.UpdateClipProjectRequest{Id: "owned", AllowedCaptionStyles: &v1.ClipCaptionStyles{}}
	if _, err := h.UpdateClipProject(ctx, connect.NewRequest(empty)); err != nil {
		t.Fatal(err)
	}
	if s.patch.CaptionStyles == nil || len(*s.patch.CaptionStyles) != 0 {
		t.Fatalf("selecting none read as selecting nothing: %+v", s.patch.CaptionStyles)
	}
}

// createStore is the one seam minting touches: with no template there is no
// template to read, so only the insert is recorded.
type createStore struct {
	clip.Store
	created clip.Project
}

func (s *createStore) InsertProject(_ context.Context, p clip.Project) error {
	s.created = p
	return nil
}

// A project minted with no template reaches the service as one (CLIP-5).
func TestCreatingAClipWithNoTemplateCrossesTheWireAsNone(t *testing.T) {
	s := &createStore{}
	h := NewHandler(clip.NewService(s, config.ClipLimits()))
	ctx := auth.WithUser(context.Background(), "alice")
	_, err := h.CreateClipProject(ctx, connect.NewRequest(&v1.CreateClipProjectRequest{
		Title: "템플릿 없이", Ratio: "vertical", Language: v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN}))
	if err != nil {
		t.Fatal("minting without a template was refused at the boundary", err)
	}
	if s.created.VideoTemplateID != "" || s.created.Composition == nil || s.created.Composition.Snapshot.TemplateID != "" {
		t.Fatal("the boundary invented a template", s.created.VideoTemplateID, s.created.Composition)
	}
	// An unset selection is what the shared default looks like on the wire.
	if s.created.IntroPreset != "" || s.created.OutroPreset != "" {
		t.Fatal("the boundary invented a selection", s.created.IntroPreset, s.created.OutroPreset)
	}
	if presets := s.created.DesignSelection().RegionPresets(); presets.Intro != "b" || presets.Outro != "e" {
		t.Fatal("the unset selection did not resolve to the defaults", presets)
	}
}

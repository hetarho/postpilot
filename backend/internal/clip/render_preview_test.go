package clip_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
)

// renderGroundsStore holds one browser render and what its sampling kept.
type renderGroundsStore struct {
	clip.GenerationStore
	render clip.BrowserRender
}

func (s *renderGroundsStore) GetBrowserRender(_ context.Context, user, id string) (clip.BrowserRender, error) {
	if user != s.render.UserID || id != s.render.ID {
		return clip.BrowserRender{}, clip.ErrNotFound
	}
	return s.render, nil
}
func (s *renderGroundsStore) BeginBrowserRender(context.Context, clip.BrowserRender) error {
	return clip.ErrInvalid
}
func (s *renderGroundsStore) SaveBrowserRenderVerdict(context.Context, string, string, clip.RenderVerdict, time.Time) error {
	return clip.ErrInvalid
}

// groundedRenderer records which drawing a preparation asked for, and the
// layout each grounded read named.
type groundedRenderer struct {
	previewRenderer
	grounds       []clip.SampledGround
	grounded      int
	groundedCells int
	layouts       []clip.PreviewLayoutKey
}

func (r *groundedRenderer) named(ctx context.Context) {
	if key, ok := clip.PreviewLayoutFrom(ctx); ok {
		r.layouts = append(r.layouts, key)
	}
}

func (r *groundedRenderer) PrepareGroundedPreview(ctx context.Context, _ clip.EditPlan, _ []clip.RenderSource, grounds []clip.SampledGround, _ []string, _ int, _ clip.PreviewConfig) (clip.PreparedPreview, error) {
	r.grounded++
	r.grounds = grounds
	r.named(ctx)
	return clip.PreparedPreview{NextOffset: -1}, nil
}
func (r *groundedRenderer) PrepareCaptionFrames(context.Context, clip.EditPlan, []clip.RenderSource, string, int, clip.PreviewConfig) (clip.CaptionFrames, error) {
	return clip.CaptionFrames{NextOffset: -1}, nil
}
func (r *groundedRenderer) PrepareGroundedCaptionFrames(ctx context.Context, _ clip.EditPlan, _ []clip.RenderSource, grounds []clip.SampledGround, _ string, _ int, _ clip.PreviewConfig) (clip.CaptionFrames, error) {
	r.groundedCells++
	r.grounds = grounds
	r.named(ctx)
	return clip.CaptionFrames{NextOffset: -1}, nil
}

// A browser render's asset and caption-frame reads name the layout they draw:
// the render, the revision and the digest of the draft they carry, so one
// export is laid out once. Another draft names another layout.
func TestARenderBoundReadNamesItsLayout(t *testing.T) {
	_, store, _, draft := previewSetup(t)
	at := time.Now()
	revision := store.project.EditPlanRevision
	renders := &renderGroundsStore{render: clip.BrowserRender{ID: "render", UserID: "alice", ProjectID: "owned", Revision: revision, SampleJobID: "sampling", Grounds: []clip.SampledGround{}, SampledAt: &at}}
	renderer := &groundedRenderer{}
	cfg := clip.DefaultGenerationConfig(clip.Environment{GetTTL: time.Minute, OrphanMinAge: time.Hour})
	service := clipapp.NewGenerationService(renders, testProjects(store), nil, neutralProcessing{}, nil, nil, renderer, neutralJobs{}, cfg, neutralGenerationDeps())
	edited := draft
	edited.Cuts = slices.Clone(draft.Cuts)
	edited.Cuts[0].Focal = &clip.Point{X: .3, Y: .5}
	for _, d := range []clip.CorrectionPlan{draft, edited} {
		if _, err := service.PrepareRenderPreview(t.Context(), "alice", "render", "owned", revision, "hash", d, nil, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := service.PrepareRenderCaptionFrames(t.Context(), "alice", "render", "owned", revision, "hash", d, "caption", 0); err != nil {
			t.Fatal(err)
		}
	}
	digest, err := clip.DraftDigest(draft)
	if err != nil {
		t.Fatal(err)
	}
	want := clip.PreviewLayoutKey{Render: "render", Revision: revision, Draft: digest}
	if len(renderer.layouts) != 4 || renderer.layouts[0] != want || renderer.layouts[1] != want {
		t.Fatalf("the reads named %+v, want %+v", renderer.layouts, want)
	}
	if renderer.layouts[2] == want || renderer.layouts[2] != renderer.layouts[3] {
		t.Fatalf("another draft named %+v", renderer.layouts[2:])
	}
}

// CLIP-192: a browser render's assets and frames are drawn with the grounds its
// own sampling job kept, and only for the owner's live render of that project
// at the revision asked for; the editing preview never reads a ground.
func TestARenderBoundPreviewDrawsTheRendersSampledGrounds(t *testing.T) {
	_, store, _, draft := previewSetup(t)
	at := time.Now()
	grounds := []clip.SampledGround{{InstanceID: "project-outro", Mean: .8, Sigma: .01, R: .9, G: .9, B: .9, Frames: []float64{.8, .8, .8}}}
	sampled := clip.BrowserRender{ID: "render", UserID: "alice", ProjectID: "owned", Revision: store.project.EditPlanRevision, SampleJobID: "sampling", Grounds: grounds, SampledAt: &at}
	cfg := clip.DefaultGenerationConfig(clip.Environment{GetTTL: time.Minute, OrphanMinAge: time.Hour})
	for _, tc := range []struct {
		name   string
		render func(clip.BrowserRender) clip.BrowserRender
		id     string
		want   error
	}{
		{"sampled", func(r clip.BrowserRender) clip.BrowserRender { return r }, "render", nil},
		{"an unknown render", func(r clip.BrowserRender) clip.BrowserRender { return r }, "other", clip.ErrNotFound},
		{"another project's render", func(r clip.BrowserRender) clip.BrowserRender { r.ProjectID = "elsewhere"; return r }, "render", clip.ErrNotFound},
		{"a cancelled render", func(r clip.BrowserRender) clip.BrowserRender { r.CancelledAt = &at; return r }, "render", clip.ErrNotFound},
		{"a render of an older revision", func(r clip.BrowserRender) clip.BrowserRender { r.Revision--; return r }, "render", clip.ErrPlanConflict},
		{"a render still sampling", func(r clip.BrowserRender) clip.BrowserRender { r.SampledAt, r.Grounds = nil, nil; return r }, "render", clip.ErrRenderNotSampled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			renderer := &groundedRenderer{}
			renders := &renderGroundsStore{render: tc.render(sampled)}
			service := clipapp.NewGenerationService(renders, testProjects(store), nil, neutralProcessing{}, nil, nil, renderer, neutralJobs{}, cfg, neutralGenerationDeps())
			revision := store.project.EditPlanRevision
			_, err := service.PrepareRenderPreview(t.Context(), "alice", tc.id, "owned", revision, "hash", draft, nil, 0)
			_, framesErr := service.PrepareRenderCaptionFrames(t.Context(), "alice", tc.id, "owned", revision, "hash", draft, "caption", 0)
			if !errors.Is(err, tc.want) || !errors.Is(framesErr, tc.want) {
				t.Fatalf("assets %v, frames %v, want %v", err, framesErr, tc.want)
			}
			if tc.want != nil {
				if renderer.grounded != 0 || renderer.groundedCells != 0 || renderer.called != 0 {
					t.Fatal("a refused render was drawn")
				}
				return
			}
			if renderer.grounded != 1 || renderer.groundedCells != 1 || renderer.called != 0 || !reflect.DeepEqual(renderer.grounds, grounds) {
				t.Fatalf("the render was not drawn on its grounds: %+v", renderer)
			}
			// The editing preview of the same project reads no ground.
			if _, err := service.PreparePreview(t.Context(), "alice", "owned", revision, "hash", draft, nil, 0); err != nil || renderer.called != 1 || renderer.grounded != 1 {
				t.Fatalf("the editing preview took a render's grounds: %v %+v", err, renderer)
			}
		})
	}
}

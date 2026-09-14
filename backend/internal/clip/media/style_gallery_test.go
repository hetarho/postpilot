package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// Opt-in design review: every approved copy style, both caption paces and the
// furniture, over one flat green fixture so the owner can compare them and say
// which stay. Synthetic footage only — no network, model or stored project.
func TestStyleGalleryExample(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" || os.Getenv("CLIP_STYLE_GALLERY") != "1" {
		t.Skip("opt-in style gallery runs inside Docker")
	}
	cfg := mediaConfig(t)
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	type shot struct {
		style, anchor, align, text, keyword string
		ms                                  int
		rapid                               bool
		chips                               []string
	}
	shots := []shot{
		{ms: 2500},
		{style: "clean", anchor: "bottom", align: "center", text: "오늘은 철판 요리를 먹었어요", ms: 3000, chips: []string{"메뉴"}},
		{style: "memo", anchor: "top", align: "left", text: "토요일 오후 1시 방문", ms: 3000, chips: []string{"메뉴"}},
		{style: "bold", anchor: "upper_mid", align: "center", text: "겉은 바삭했어요", keyword: "바삭", ms: 3000},
		{style: "mark", anchor: "bottom", align: "center", text: "1인분 18,000원", keyword: "18,000원", ms: 3000},
		{style: "simple", anchor: "bottom", align: "center", text: "국물이 진하고 깔끔해요", ms: 3000},
		{style: "simple", anchor: "bottom", align: "center", text: "지글지글 익어가면 한 입 더", ms: 3000, rapid: true},
		{ms: 2500},
	}
	total := 0
	for _, s := range shots {
		total += s.ms
	}
	err = a.WithWorkspace(t.Context(), "style-gallery", func(ws clip.MediaWorkspace) error {
		// One flat green original per cut: the background the owner asked to
		// review the styles against, and the only footage this test uses.
		green := filepath.Join(ws.Path, "green.mp4")
		if _, err := a.run(t.Context(), ws, cfg.FFmpegPath,
			"-hide_banner", "-nostdin", "-v", "error",
			"-f", "lavfi", "-i", "color=c=0x1F7A3D:s=1080x1920:r=30",
			"-t", "6", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "2", "-pix_fmt", "yuv420p",
			green); err != nil {
			return err
		}
		info, err := a.Probe(t.Context(), ws, green)
		if err != nil {
			return err
		}
		plan := clip.EditPlan{
			Ratio: "vertical", DurationMS: total,
			Disclosure: "ad", Preset: "restaurant", Accent: "coral", CTA: "save",
			Hook:   "스타일 비교",
			Styles: []string{"clean", "memo", "bold", "mark", "simple"},
			Facts:  []clip.Answer{{Label: "상호", Text: "스타일 갤러리"}, {Label: "메뉴", Text: "철판 요리"}},
		}
		sources := []clip.RenderSource{}
		paths := map[string]string{}
		for i, s := range shots {
			id := fmt.Sprintf("green-%02d", i)
			sources = append(sources, clip.RenderSource{ID: id, Fingerprint: id, Info: info})
			paths[id] = green
			cut := clip.EditCut{ID: id, SourceID: id, Fingerprint: id, EndMS: s.ms, Focal: clip.Point{X: .5, Y: .5}, Chips: s.chips}
			if s.text != "" {
				copy := clip.Copy{Text: s.text, Style: s.style, Anchor: s.anchor, Align: s.align, Accent: plan.Accent, Keyword: s.keyword}
				if s.rapid {
					phrases, ok := clip.SplitRapid(copy, 120, s.ms-120)
					if !ok {
						return fmt.Errorf("rapid phrases did not fit cut %d", i)
					}
					cut.Copies = phrases
				} else {
					cut.Copies = []clip.Copy{copy}
				}
			}
			plan.Cuts = append(plan.Cuts, cut)
		}
		laidOut, _, err := r.Layout(t.Context(), plan, sources)
		if err != nil {
			return err
		}
		result, err := r.Render(t.Context(), ws, laidOut, sources, func(_ context.Context, id string, consume func(clip.MediaSource) error) error {
			path, ok := paths[id]
			if !ok {
				return clip.ErrNotFound
			}
			return consume(clip.MediaSource{SourceID: id, Fingerprint: id, Info: info, Path: path})
		})
		if err != nil {
			return err
		}
		t.Logf("gallery duration=%dms bytes=%d", result.Info.DurationMS, result.Bytes)
		return exportRenderSmoke("style-gallery.mp4", result.Path)
	})
	if err != nil {
		t.Fatal(err)
	}
}

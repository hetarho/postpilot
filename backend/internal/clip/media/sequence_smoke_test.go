package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// Production binaries and fonts: a sequence-rendered caption draws one PNG per
// output frame, the overlay chain reads them back with `image2` beside the
// static plates the same clip carries, and the whole clip is delivered twice
// byte for byte (CDS-80, CDS-81, CLIP-125).
func TestRenderSmokeSequenceCaption(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	t.Parallel()
	// Jua sets one of these captions and not the other, so one render carries a
	// sequence-rendered caption and a static one at once: 똠얌꿍's first syllable
	// is outside the 2,367 Jua covers, and that caption falls back to the
	// default style (CDS-84).
	style, ok := design.LookupCaptionStyle("pop")
	if !ok || style.Static() || style.Face != "jua" {
		t.Fatal("the fixture no longer names a sequence-rendered Jua style")
	}
	fallback := design.DefaultCaption()
	if !fallback.Static() {
		t.Fatal("the default style is no longer the static one a fallback lands on")
	}
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 15 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	// The intro block, the disclosure badge and the caption share one overlay
	// chain: the first two are single plates the chain loops, the third is the
	// frame sequence. A change that broke either input would show here.
	body := `<clip version="1" intro="a" caption="bold" outro="e">` +
		`<text id="disclosure" kind="fixed" role="badge" position="header" basis="output-start" start="1" end="14">제작비 일부 지원</text>` +
		`<text id="hello" kind="fixed" role="hook" basis="output-start"><row>오늘의 장면</row><row>남긴 기록</row></text>` +
		`<text id="moving" kind="fixed" role="caption" position="upper_mid" basis="output-start" start="2" end="6">여기 진짜 좋아요</text>` +
		`<text id="settled" kind="fixed" role="caption" position="lower_mid" basis="output-start" start="8" end="12">똠얌꿍이 최고</text>` +
		`<text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`

	deliver := func(name string) string {
		digest := ""
		if err := a.WithWorkspace(t.Context(), name, func(ws clip.MediaWorkspace) error {
			path := filepath.Join(ws.Path, "source.mp4")
			if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30", "-t", "16", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", path); err != nil {
				return err
			}
			info, err := a.Probe(t.Context(), ws, path)
			if err != nil {
				return err
			}
			plan := declaredPlan(t, body, "vertical")
			plan.Disclosure, plan.IntroPreset, plan.OutroPreset = "ad", "a", "e"
			plan.CaptionStyles = []string{style.ID}
			plan.Cuts = []clip.Cut{
				{ID: "a", SourceID: "source", Fingerprint: "fp", EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}},
				{ID: "b", SourceID: "source", Fingerprint: "fp", StartMS: 7600, EndMS: 15200, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}},
			}
			plan.Portable.Cuts = []composition.Cut{{ID: "a", SourceID: "source", EndMS: 7600}, {ID: "b", SourceID: "source", StartMS: 7600, EndMS: 15200, TransitionMS: 200}}
			result, err := r.Render(t.Context(), ws, plan, []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: info}}, func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
				return consume(clip.MediaSource{SourceID: "source", Fingerprint: "fp", Info: info, Path: path})
			})
			if err != nil {
				return err
			}
			sequences, statics := 0, 0
			for _, e := range result.Elements {
				switch {
				case e.Role != "caption":
				case e.Style == style.ID:
					sequences++
				case e.Style == fallback.ID:
					statics++
				default:
					return fmt.Errorf("a caption was drawn in %q", e.Style)
				}
			}
			if sequences != 1 || statics != 1 {
				return fmt.Errorf("%d sequence and %d static captions: %+v", sequences, statics, result.Elements)
			}
			// Nothing a sequence wrote may outlive the render that wrote it.
			entries, err := os.ReadDir(ws.Path)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.IsDir() {
					return fmt.Errorf("a caption's frames outlived the render: %s", entry.Name())
				}
			}
			data, err := os.ReadFile(result.Path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			digest = hex.EncodeToString(sum[:])
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return digest
	}
	if first, second := deliver("sequence-once"), deliver("sequence-twice"); first == "" || first != second {
		t.Fatalf("one plan with a sequence caption delivered two different clips: %s against %s", first, second)
	}
}

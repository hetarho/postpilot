package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

// The CDS-53 owner review needs whole clips, not fixtures: three of them, one per
// selectable intro/outro pairing, each carrying the disclosure badge, an
// information pair and captions, and between them every assembly case the
// decision names — a same-source split, all six CLIP-98 rates, a transition
// overlap, a caption boundary inside a rate-transformed cut and an omitted
// optional slot. Synthetic footage only: no network, model or stored project.
// The renders are the evidence for docs/design/clip-release-qa-<date>.md.
func TestReleaseQAViewingClips(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" || os.Getenv("CLIP_RELEASE_QA") != "1" {
		t.Skip("opt-in release QA render runs inside Docker")
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
	for _, clipCase := range qaClipCases() {
		t.Run(clipCase.name, func(t *testing.T) {
			if err := a.WithWorkspace(t.Context(), "release-qa", func(ws clip.MediaWorkspace) error {
				sources, paths, info, err := qaFootage(t, a, ws, cfg)
				if err != nil {
					return err
				}
				plan, err := qaPlan(clipCase, sources)
				if err != nil {
					return err
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
				t.Logf("%s duration=%dms bytes=%d", clipCase.name, result.Info.DurationMS, result.Bytes)
				return exportRenderSmoke(clipCase.name+".mp4", result.Path)
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type qaClip struct {
	name, intro, outro string
	// The rates this clip's cuts play at, in order. Together the three clips
	// cover every CLIP-98 preset.
	rates []int
}

func qaClipCases() []qaClip {
	return []qaClip{
		{name: "qa-clip-1-intro-a-outro-b", intro: "a", outro: "b", rates: []int{1000, 1000, 500}},
		{name: "qa-clip-2-intro-b-outro-e", intro: "b", outro: "e", rates: []int{750, 1250, 1000}},
		{name: "qa-clip-3-intro-a-outro-e", intro: "a", outro: "e", rates: []int{1500, 2000, 1000}},
	}
}

// qaFootage builds three distinguishable originals: a flat hue with one square
// crossing it, so a cut boundary and a changed rate are both visible, and a
// distinct sine tone per source, so the audio pass can hear which original is
// playing and whether any of it arrives unasked. Only the allowlisted lavfi
// sources and filters exist in the runtime image.
func qaFootage(t *testing.T, a *Adapter, ws clip.MediaWorkspace, cfg clip.MediaConfig) ([]clip.RenderSource, map[string]string, clip.MediaInfo, error) {
	t.Helper()
	hues := []string{"0x1F7A3D", "0x2E4C8A", "0x8A3B2E"}
	tones := []int{330, 440, 550}
	var sources []clip.RenderSource
	paths := map[string]string{}
	var info clip.MediaInfo
	for i, hue := range hues {
		id := fmt.Sprintf("qa-source-%d", i+1)
		path := filepath.Join(ws.Path, id+".mp4")
		if _, err := a.run(t.Context(), ws, cfg.FFmpegPath,
			"-hide_banner", "-nostdin", "-v", "error",
			"-f", "lavfi", "-i", "color=c="+hue+":s=1080x1920:r=60",
			"-f", "lavfi", "-i", "color=c=white:s=160x160:r=60",
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:sample_rate=48000", tones[i]),
			"-filter_complex", "[0:v][1:v]overlay=x='(W-w)*mod(t,4)/4':y='H/2-h/2'[v]",
			"-map", "[v]", "-map", "2:a",
			"-t", "20", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "2", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-ar", "48000", "-ac", "2",
			path); err != nil {
			return nil, nil, info, err
		}
		probed, err := a.Probe(t.Context(), ws, path)
		if err != nil {
			return nil, nil, info, err
		}
		info = probed
		sources = append(sources, clip.RenderSource{ID: id, Fingerprint: id, Info: probed})
		paths[id] = path
	}
	return sources, paths, info, nil
}

// qaBody is the authored composition: a disclosure badge for the whole clip, an
// information pair, two captions and the intro/outro rows the selected presets
// paint. The second information slot is deliberately empty in clip 3 — CDS-73
// omits that line and keeps the rest of the block in place.
func qaBody(c qaClip) string {
	// Outro B paints two slots, outro E three; a region may never carry more rows
	// than its preset has slots.
	closing := `<text id="closing" kind="fixed" role="ending" basis="output-end" start="-3" end="0"><row>다시 만나요</row><row>기록을 남겨요</row></text>`
	if c.outro == "e" {
		closing = `<text id="closing" kind="fixed" role="ending" basis="output-end" start="-3" end="0"><row>오늘의 점수</row><row>4.5</row><row>다시 보고 싶은 장면</row></text>`
	}
	place := `<text id="place" kind="fixed" role="info" position="bottom" basis="output-start" start="0" end="4"><row role="label">위치</row><row role="caption">서울 성수동</row></text>`
	if c.intro == "a" && c.outro == "e" {
		// The absent optional field: an information element with no value row.
		place = `<text id="place" kind="fixed" role="info" position="bottom" basis="output-start" start="0" end="4"><row role="label">위치</row></text>`
	}
	return `<clip version="1" intro="` + c.intro + `" caption="bold" outro="` + c.outro + `">` +
		`<text id="ad" kind="fixed" role="badge" position="header" basis="whole">광고</text>` +
		place +
		`<text id="opening" kind="fixed" role="hook" basis="output-start" start="0" end="3"><row>오늘의 기록</row><row>직접 남긴 장면</row></text>` +
		`<text id="first" kind="fixed" role="caption" basis="output-start" start="3" end="7">첫 장면을 그대로 담았어요</text>` +
		`<text id="second" kind="fixed" role="caption" basis="output-start" start="7" end="11">속도가 바뀌는 구간입니다</text>` +
		closing +
		`<repeat for="scenes"><scene id="footage" scope="scene"/></repeat></clip>`
}

// qaPlan binds the authored body to cuts that carry the assembly cases: cuts 1
// and 2 are one source split in two at the same point in its own timeline, cut 2
// fades into cut 1 rather than cutting hard, and each cut plays at its own rate
// so the caption windows above cross a transformed boundary.
func qaPlan(c qaClip, sources []clip.RenderSource) (clip.EditPlan, error) {
	body := qaBody(c)
	limits := config.ClipCompositionLimits()
	doc, problem := composition.Parse(body, limits)
	if problem != nil {
		return clip.EditPlan{}, problem
	}
	section := ""
	if len(doc.Sections) > 0 {
		section = doc.Sections[0].ID
	}
	// Cuts 1 and 2 are one source split at a single point in its own timeline;
	// cut 2 fades in rather than cutting hard. Each cut holds the same OUTPUT
	// duration whatever its rate, so the clip length does not move with the
	// rates under review and stays inside the rendered duration range.
	outputs := []int{5000, 5000, 6000}
	transitions := []int{0, 200, 0}
	var bindings []composition.Cut
	var cuts []clip.Cut
	audio := &clip.SourceAudioSettings{}
	used := map[string]bool{}
	sourceAt := 0
	for i, out := range outputs {
		id := fmt.Sprintf("cut-%d", i+1)
		rate := c.rates[i]
		span := out * rate / composition.RateUnitPermille
		source, start := sources[0].ID, sourceAt
		if i == 2 {
			source, start = sources[1].ID, 0
		} else {
			sourceAt += span
		}
		used[source] = true
		bindings = append(bindings, composition.Cut{ID: id, SectionID: section, SourceID: source, StartMS: start, EndMS: start + span, TransitionMS: transitions[i], PlaybackRatePermille: rate})
		cuts = append(cuts, clip.Cut{ID: id, SourceID: source, Fingerprint: source, StartMS: start, EndMS: start + span, TransitionMS: transitions[i], Focal: clip.Point{X: .5, Y: .5}, PlaybackRatePermille: rate})
	}
	for _, source := range sources {
		// The snapshot names exactly the sources these cuts draw on. Source sound
		// is ON for this review, so the audio pass has something to judge
		// (CLIP-100 keeps it off by default).
		if used[source.ID] {
			audio.Values = append(audio.Values, clip.SourceAudioSetting{SourceID: source.ID, Fingerprint: source.Fingerprint, RetainOriginal: true})
		}
	}
	total := 0
	for _, cut := range cuts {
		span, ok := composition.TransformedDurationMS(cut.EndMS-cut.StartMS, cut.Rate())
		if !ok {
			return clip.EditPlan{}, fmt.Errorf("cut %s does not transform", cut.ID)
		}
		total += span - cut.TransitionMS
	}
	resolved, problem := composition.Resolve(doc, composition.Inputs{Cuts: bindings}, limits, total)
	if problem != nil {
		return clip.EditPlan{}, problem
	}
	plan := clip.EditPlan{
		Ratio: "vertical", DurationMS: total, Cuts: cuts, SourceAudio: audio,
		Portable: &clip.PortablePlan{Snapshot: clip.CompositionSnapshot{Version: 1, Body: body}, Cuts: bindings, TargetDurationMS: total},
	}
	for _, element := range resolved.Elements {
		plan.Portable.Elements = append(plan.Portable.Elements, clip.PortableText{Resolved: element, Pace: doc.Pace, Accent: doc.Accent})
	}
	return plan, nil
}

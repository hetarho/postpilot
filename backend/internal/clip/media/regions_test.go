package media

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

func regionVisual(kind string, rows ...string) declaredVisual {
	role := "hook"
	if kind == "outro" {
		role = "ending"
	}
	text := clip.PortableText{Accent: "coral", Resolved: composition.ResolvedElement{Element: composition.Element{ID: "owned-region", Role: role, Kind: "fixed", Span: composition.Span{Line: 7}, Align: "center", Position: "auto", Style: "auto"}, StartMS: 0, EndMS: 2500}}
	for _, row := range rows {
		text.Resolved.Rows = append(text.Resolved.Rows, composition.ResolvedRow{Text: row})
	}
	return declaredVisual{text: text, manifest: declaredManifest(text)}
}

// regionPlacement is what the plan gives this one entry: the slot its first line
// takes and how many of them the preset draws (CLIP-147).
func regionPlacement(v declaredVisual, kind, id string) clip.RegionPlacement {
	presets := composition.DesignSelection{Intro: "b", Outro: "e"}
	if kind == "intro" {
		presets.Intro = id
	} else {
		presets.Outro = id
	}
	return clip.RegionPlacements([]composition.ResolvedElement{v.text.Resolved}, presets)[v.text.Resolved.InstanceID]
}

// regionSamples is one realistic filling of every preset's slots.
var regionSamples = []struct {
	kind, id string
	rows     []string
}{
	{"intro", "a", []string{"오늘의 장면", "직접 남긴 기록"}},
	{"intro", "b", []string{"오늘의 장면", "직접 남긴 기록"}},
	{"intro", "cover", []string{"성수 로컬 가이드", "성수동 골목 곱창집", "서울 성동구 · 저녁 영업"}},
	{"intro", "serif", []string{"이번 주의 식탁", "연남동 숯불 한우", "서울 마포구 연남동"}},
	{"intro", "frame", []string{"해미 한우", "서울 연남동 · 숯불 구이"}},
	{"intro", "outline", []string{"한우", "연남동 숯불 구이", "서울 마포구"}},
	{"intro", "lower", []string{"오늘의 기록", "연남동 골목 한우집", "서울 마포구 연남동"}},
	{"intro", "sticker", []string{"여기 진짜 맛집", "연남동 숯불 한우"}},
	{"outro", "b", []string{"다시 만나자", "오늘의 기록을 남겨요"}},
	{"outro", "e", []string{"오늘의 점수", "9.5", "다시 보고 싶은 장면"}},
	{"outro", "credits", []string{"오늘의 한 끼", "해미 한우", "서울 마포구 연남동", "매일 11시부터 22시까지"}},
	{"outro", "sidebar", []string{"다시 가고 싶은 집", "메뉴와 가격은 블로그에", "해미 한우 연남점"}},
	{"outro", "chips", []string{"이런 분께 추천해요", "데이트", "회식", "혼밥", "주차 가능", "예약 필수"}},
	{"outro", "list", []string{"오늘의 정리", "숯불 향이 진한 한우", "두 명이면 5만 원대", "주말 저녁은 예약 필수"}},
	{"outro", "stamp", []string{"해미 한우 · 연남동", "다시 올 집", "2026 오늘의 기록", "자세한 후기는 블로그에"}},
}

// roundedCorner is any rectangle drawn with a non-zero corner radius.
var roundedCorner = regexp.MustCompile(`rx="([0-9.]+)"`)

func checkRegionPresets(t *testing.T, a *Adapter, r *Rendering, raster bool) {
	t.Helper()
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, choice := range regionSamples {
			t.Run(ratio+"/"+choice.kind+"-"+choice.id, func(t *testing.T) {
				canvas, _ := clip.ClipCanvas(ratio)
				if err := a.WithWorkspace(t.Context(), "region", func(ws clip.MediaWorkspace) error {
					visual := regionVisual(choice.kind, choice.rows...)
					v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, ratio, visual, choice.kind, choice.id, regionPlacement(visual, choice.kind, choice.id), choice.rows)
					if err != nil {
						return err
					}
					if err := design.VerifyRegion(choice.kind, choice.id, ratio, choice.rows, 0, len(choice.rows), true, v.manifest.Parts); err != nil {
						return err
					}
					block, _ := design.LayoutRegion(choice.kind, choice.id, ratio, choice.rows)
					want, drawn := 0, len(v.region.Lines)+len(v.region.Arcs)
					for _, slot := range block.Slots {
						want += len(slot.Lines)
					}
					if v.region.Turn != nil {
						drawn += len(v.region.Turn.Lines) + len(v.region.Turn.Arcs)
					}
					if drawn != want || want == 0 {
						t.Fatal("the drawn lines left the block layout", drawn, want)
					}
					for _, part := range v.manifest.Parts {
						if !inside(clip.Region(part.Region), canvas.Safe) || part.FontSize > 0 && part.FontSize < design.MinTypeSize() {
							t.Fatal("unsafe part", part)
						}
						if part.Kind == "copy" && part.Fill != design.Color["text_white"].Hex {
							t.Fatal("a region line is not white", part)
						}
					}
					body, err := r.declaredSVG(canvas, v)
					if err != nil {
						return err
					}
					// White type only, and a rounded corner only on a pill, a
					// plate or a list square (CDS-88).
					if strings.Contains(body, design.Accent["coral"]) {
						t.Fatal("region acquired an accent", body)
					}
					rounded := slices.ContainsFunc(block.Shapes, func(s design.RegionShape) bool { return s.Radius > 0 && !s.Circle })
					for _, m := range roundedCorner.FindAllStringSubmatch(body, -1) {
						if m[1] != "0.000" && !rounded {
							t.Fatal("region acquired a rounded plate", body)
						}
					}
					if raster {
						if dir := os.Getenv("CLIP_REGION_DUMP"); dir != "" {
							_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%s-%s.svg", choice.kind, choice.id, ratio)), []byte(body), 0o644)
						}
						_, err = r.rasterize(t.Context(), ws, canvas, body, "region-proof")
						return err
					}
					golden(t, fmt.Sprintf("region-%s-%s-%s.svg", choice.kind, choice.id, ratio), body)
					for m, mutate := range []func(*design.Element){func(p *design.Element) { p.Region.Y++ }, func(p *design.Element) { p.Fill = "#FF6B57" }, func(p *design.Element) { p.Rotate += 3 }, func(p *design.Element) { p.BaselineY++ }} {
						for i := range v.manifest.Parts {
							if m == 3 && v.manifest.Parts[i].Kind != "copy" {
								continue
							}
							parts := slices.Clone(v.manifest.Parts)
							mutate(&parts[i])
							if err := design.VerifyRegion(choice.kind, choice.id, ratio, choice.rows, 0, len(choice.rows), true, parts); !errors.Is(err, design.ViolationRegion) {
								t.Fatal("V20 accepted a changed preset", i, parts[i], err)
							}
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// regionMeasured measures each text at the metrics table's own advance width
// (CDS-86) with Hangul ink from 80 % of the size above the baseline to 2 %
// below it, so every preset — edge-anchored ones included — lands where the
// fit put it.
func regionMeasured(t *testing.T) (*Adapter, *Rendering) {
	t.Helper()
	faces := map[string]string{}
	for key, family := range design.Faces {
		faces[family] = key
	}
	attr := func(tag, name string) string {
		_, rest, _ := strings.Cut(tag, name+`="`)
		value, _, _ := strings.Cut(rest, `"`)
		return value
	}
	a := newAdapter(t, &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		if !slices.Contains(c.Args, "--query-all") {
			svg, err := os.ReadFile(c.Args[len(c.Args)-2])
			if err != nil {
				return nil, err
			}
			return nil, os.WriteFile(c.Args[len(c.Args)-1], svg, 0600)
		}
		data, err := os.ReadFile(c.Args[len(c.Args)-1])
		if err != nil {
			return nil, err
		}
		out := ""
		for i, part := range strings.Split(string(data), `id="m`)[1:] {
			tag, rest, _ := strings.Cut(part, ">")
			text, _, _ := strings.Cut(rest, "</text>")
			weight, _ := strconv.Atoi(attr(tag, "font-weight"))
			tracking, _ := strconv.ParseFloat(attr(tag, "letter-spacing"), 64)
			width := design.TextWidth(faces[attr(tag, "font-family")], weight, tracking/100, 100, html.UnescapeString(text))
			out += fmt.Sprintf("m%d,1,120,%.3f,82\n", i, width)
		}
		return []byte(out), nil
	}})
	return a, testRenderer(t, a)
}

func TestRegionPresetsOnEveryRatio(t *testing.T) {
	a, r := regionMeasured(t)
	checkRegionPresets(t, a, r, false)
}

// The presets are the project's since CLIP-139, so the plan carries them and the
// frozen document's own attributes no longer choose anything.
func TestDeclaredRegionsUseTheProjectSelection(t *testing.T) {
	plan := declaredPlan(t, `<clip version="1" intro="a" caption="bold" outro="b"><text id="intro" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row>오늘의 장면</row><row>남긴 기록</row></text><text id="outro" kind="fixed" role="ending" basis="output-end" start="-3" end="0"><row>다시 만나자</row><row>오늘을 남겨요</row></text></clip>`, "vertical")
	plan.IntroPreset, plan.OutroPreset = "a", "b"
	layout := measuredDeclared(t, plan)
	intro, _ := design.LayoutRegion("intro", "a", "vertical", []string{"오늘의 장면", "남긴 기록"})
	outro, _ := design.LayoutRegion("outro", "b", "vertical", []string{"다시 만나자", "오늘을 남겨요"})
	if len(layout.visuals) != 2 || layout.visuals[0].region.Lines[0].Y != intro.Slots[0].Lines[0].Baseline || layout.visuals[1].region.Lines[0].Y != outro.Slots[0].Lines[0].Baseline {
		t.Fatal("document design selection was ignored", layout.elements())
	}
}

// A slot is bounded by its width, not a character count (CDS-86): nine
// syllables shrink into intro A's headline, and only text too wide at the floor
// even on two lines is refused before generation (CDS-77).
func TestAuthoredAdmissionRefusesOnlyWhatCannotFit(t *testing.T) {
	_, r := measured(t)
	for _, intro := range []string{"a", "b"} {
		for _, c := range []struct {
			text string
			fits bool
		}{{"하나둘셋넷다섯여섯", true}, {strings.Repeat("하나둘셋넷", 5), false}} {
			body := `<clip version="1" caption="bold"><text id="intro" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row>` + c.text + `</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
			err := r.ValidateAuthoredInput(t.Context(), clip.PlanningInput{Ratio: "vertical", TargetDurationMS: 15000, Design: clip.ProjectDesign{IntroPreset: intro, OutroPreset: "e"}, Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Body: body}}})
			if c.fits {
				if err != nil {
					t.Fatal(intro, c.text, err)
				}
				continue
			}
			var problem *composition.Problem
			if !errors.As(err, &problem) || problem.ElementID != "intro" || problem.Reason != "copy_limit" {
				t.Fatal("an unbreakable line wider than the floor was admitted", intro, err)
			}
		}
	}
}

// CDS-73 and CDS-87: an empty slot closes up with the gap before it while the
// block keeps its rules and its centre; a line no slot holds is drawn by nobody;
// a newline or a line too wide at the floor is refused.
func TestRegionSlotsCloseUpAndRefuseOverflow(t *testing.T) {
	a, r := measured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	if err := a.WithWorkspace(t.Context(), "slots", func(ws clip.MediaWorkspace) error {
		rows := []string{"점수", "", "오늘의 기록"}
		filled := regionVisual("outro", rows...)
		v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", filled, "outro", "e", regionPlacement(filled, "outro", "e"), rows)
		if err != nil {
			return err
		}
		block, _ := design.LayoutRegion("outro", "e", "vertical", rows)
		if len(v.region.Lines) != 2 || v.region.Lines[0].Y != block.Slots[0].Lines[0].Baseline || v.region.Lines[1].Y != block.Slots[1].Lines[0].Baseline || len(v.region.Rules) != 1 {
			t.Fatal("the empty slot did not close up around the block", v.region)
		}
		empty := regionVisual("outro", "", "", "")
		v, err = r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", empty, "outro", "e", regionPlacement(empty, "outro", "e"), []string{"", "", ""})
		if err != nil {
			return err
		}
		if len(v.manifest.Parts) != 0 || len(v.region.Rules) != 0 || len(v.region.Lines) != 0 {
			t.Fatal("empty region drew parts")
		}
		// A line past the preset's last slot is drawn by nobody and refuses
		// nothing; the notice for it is the plan's (CLIP-147).
		surplus := regionVisual("intro", "첫째", "둘째", "셋째")
		drawn, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", surplus, "intro", "b", regionPlacement(surplus, "intro", "b"), []string{"첫째", "둘째"})
		if err != nil {
			return err
		}
		// Two drawn lines and preset B's own two rules; the third line is nowhere.
		if len(drawn.region.Lines) != 2 || len(drawn.manifest.Parts) != 4 {
			t.Fatal("the surplus line was drawn or the drawn ones were not", drawn.region.Lines, drawn.manifest.Parts)
		}
		for _, rows := range [][]string{{"한 줄\n두 줄"}, {strings.Repeat("하나둘셋넷", 5)}} {
			visual := regionVisual("intro", rows...)
			_, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", visual, "intro", "b", regionPlacement(visual, "intro", "b"), rows)
			var problem *composition.Problem
			if !errors.As(err, &problem) || problem.Reason != "copy_limit" || problem.ElementID != "owned-region" || problem.Line != 7 {
				t.Fatalf("%v: %v", rows, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// CLIP-147: two entries of one region fill its slots in order, so the second
// entry's line is drawn on the second slot rather than over the first.
func TestTwoRegionEntriesFillConsecutiveSlots(t *testing.T) {
	a, r := measured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	presets := composition.DesignSelection{Intro: "b", Outro: "e"}
	first, second := regionVisual("intro", "성수 곱창"), regionVisual("intro", "성수동")
	second.text.Resolved.InstanceID, second.text.Resolved.Element.ID = "second", "second"
	placements := clip.RegionPlacements(clip.ResolvedElements([]clip.PortableText{first.text, second.text}), presets)
	if err := a.WithWorkspace(t.Context(), "slots", func(ws clip.MediaWorkspace) error {
		rows := clip.RegionRows(clip.ResolvedElements([]clip.PortableText{first.text, second.text}), presets, "intro")
		block, _ := design.LayoutRegion("intro", "b", "vertical", rows)
		for i, entry := range []declaredVisual{first, second} {
			v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", entry, "intro", "b", placements[entry.text.Resolved.InstanceID], rows)
			if err != nil {
				return err
			}
			if len(v.region.Lines) != 1 || v.region.Lines[0].Y != block.Slots[i].Lines[0].Baseline {
				t.Fatal("the entry did not land on its own slot", i, v.region.Lines)
			}
			if v.manifest.Parts[0].Slot != i+1 {
				t.Fatal("the manifest named another slot", v.manifest.Parts[0].Slot)
			}
			// Only the first entry paints the preset's rules, so nothing is
			// drawn twice (CDS-73).
			if rules := len(v.region.Rules); (i == 0) != (rules > 0) {
				t.Fatal("the rules were painted by the wrong entry", i, rules)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRegionPresetsRealFontSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real bundled-font renderer gate")
	}
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	checkRegionPresets(t, a, r, true)
}

func TestLegacyHookOverflowIsIdentifiedBeforeRerender(t *testing.T) {
	a, r := measured(t)
	for _, hook := range []string{"한 줄\n두 줄", "하나둘셋넷다섯여섯일"} {
		plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Disclosure: "ad", Hook: hook,
			Cuts: []clip.Cut{{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}}}
		if err := a.WithWorkspace(t.Context(), "legacy-region", func(ws clip.MediaWorkspace) error {
			_, err := r.Render(t.Context(), ws, plan, []clip.RenderSource{{ID: "s", Fingerprint: "s", Info: clip.MediaInfo{DurationMS: 15000}}}, func(_ context.Context, _ string, _ func(clip.MediaSource) error) error {
				t.Fatal("overflow fetched source media")
				return nil
			})
			var problem *composition.Problem
			if !errors.As(err, &problem) || problem.Reason != "copy_limit" || problem.ElementID != "legacy-hook" {
				t.Fatalf("%q: %v", hook, err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAuthoredRegionAdmissionChecksOnlyFixedRows(t *testing.T) {
	_, r := measured(t)
	for _, kind := range []string{"fixed", "ai"} {
		body := `<clip version="1" intro="b" caption="bold" outro="e"><text id="intro" kind="` + kind + `" role="hook" basis="output-start"><row kind="fixed">직접 입력</row><row kind="ai">Write a grounded phrase that is much longer than the generated slot limit.</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
		in := clip.PlanningInput{Ratio: "vertical", TargetDurationMS: 15000, Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Body: body}}}
		if err := r.ValidateAuthoredInput(t.Context(), in); err != nil {
			t.Fatal(err)
		}
		in.Composition.Snapshot.Body = strings.Replace(body, "직접 입력", strings.Repeat("하나둘셋넷", 5), 1)
		var problem *composition.Problem
		if err := r.ValidateAuthoredInput(t.Context(), in); !errors.As(err, &problem) || problem.ElementID != "intro" || problem.Reason != "copy_limit" {
			t.Fatal(err)
		}
	}
}

// CDS-32 and CDS-44: a block on a bright ground takes its own scrim — an
// ellipse 1360 px wide and 560 px taller than a centred block, or its edge's
// band reaching 150 px past an edge-anchored one — drawn once, by the entry
// that paints the block's decoration, and a line's background reads through the
// scrim and any fill under it.
func TestRegionBlocksTakeTheirOwnScrim(t *testing.T) {
	a, r := regionMeasured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	bright := Luminance{Mean: .95, R: 1, G: 1, B: 1, Frames: []float64{.95, .95, .95}}
	if err := a.WithWorkspace(t.Context(), "scrim", func(ws clip.MediaWorkspace) error {
		for _, c := range []struct {
			kind, id string
			rows     []string
		}{
			{"intro", "a", []string{"오늘의 장면", "직접 남긴 기록"}},
			{"intro", "cover", []string{"성수 가이드", "성수동 곱창집", "서울 성동구"}},
			{"outro", "sidebar", []string{"다시 갈 집", "메뉴는 블로그에", "해미 한우"}},
			{"intro", "sticker", []string{"여기 진짜 맛집", "연남동 한우"}},
		} {
			visual := regionVisual(c.kind, c.rows...)
			v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", visual, c.kind, c.id, regionPlacement(visual, c.kind, c.id), c.rows)
			if err != nil {
				return err
			}
			block, _ := design.LayoutRegion(c.kind, c.id, "vertical", c.rows)
			v.ground = bright
			applyDeclaredGround(canvas, &v)
			var scrims []design.Element
			for _, p := range v.manifest.Parts {
				if p.Kind == "scrim" {
					scrims = append(scrims, p)
				}
			}
			if len(scrims) != 1 || scrims[0].StartMS != v.manifest.StartMS || scrims[0].EndMS != v.manifest.EndMS {
				t.Fatal(c.id, "the block's scrim is not one part on its interval", scrims)
			}
			b := block.Bounds
			switch block.Scrim {
			case "radial":
				e := v.region.Radial
				if e == nil || v.region.Scrim != nil || e.RX != 680 || e.RY != (b.Height+560)/2 || e.CX != b.X+b.Width/2 || e.CY != b.Y+b.Height/2 {
					t.Fatal(c.id, "centred block took no ellipse", e)
				}
			case "top":
				if s := v.region.Scrim; s == nil || v.region.Radial != nil || s.Y != 0 || s.Height != b.Y+b.Height+150 {
					t.Fatal(c.id, "top block took no top band", s)
				}
			case "bottom":
				if s := v.region.Scrim; s == nil || s.Y != b.Y-150 || s.Y+s.Height != float64(canvas.Height) {
					t.Fatal(c.id, "bottom block took no bottom band", s)
				}
			}
			if p := scrims[0].Region; p.X < 0 || p.Y < 0 || p.X+p.Width > float64(canvas.Width) || p.Y+p.Height > float64(canvas.Height) {
				t.Fatal(c.id, "the scrim part leaves the canvas", p)
			}
			if c.id == "sticker" {
				// The label stands on the dark plate, so it reads against the
				// plate over the scrim, not against the bright frame.
				label := v.manifest.Parts[slices.IndexFunc(v.manifest.Parts, func(p design.Element) bool { return p.Kind == "copy" && p.Slot == 2 })]
				if label.ContrastNotice || label.Background == bright.Hex() {
					t.Fatal("the plate was not under the label", label)
				}
			}
			// The same ground on a dark frame takes no scrim at all.
			v.ground = Luminance{Mean: .05, R: .05, G: .05, B: .05, Frames: []float64{.05, .05, .05}}
			applyDeclaredGround(canvas, &v)
			if v.region.Scrim != nil || v.region.Radial != nil || slices.ContainsFunc(v.manifest.Parts, func(p design.Element) bool { return p.Kind == "scrim" }) {
				t.Fatal(c.id, "a dark ground kept the scrim")
			}
		}
		// Two entries of one block: only the owner draws the scrim, and the
		// other's lines still read through it.
		presets := composition.DesignSelection{Intro: "a", Outro: "e"}
		first, second := regionVisual("intro", "성수 곱창"), regionVisual("intro", "성수동")
		second.text.Resolved.InstanceID, second.text.Resolved.Element.ID = "second", "second"
		elements := clip.ResolvedElements([]clip.PortableText{first.text, second.text})
		placements := clip.RegionPlacements(elements, presets)
		rows := clip.RegionRows(elements, presets, "intro")
		for i, entry := range []declaredVisual{first, second} {
			v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", entry, "intro", "a", placements[entry.text.Resolved.InstanceID], rows)
			if err != nil {
				return err
			}
			if sampledBounds(v) != v.block.text {
				t.Fatal("an entry samples its own lines rather than its block", i)
			}
			v.ground = bright
			applyDeclaredGround(canvas, &v)
			if (v.region.Radial != nil) != (i == 0) {
				t.Fatal("the scrim was drawn by the wrong entry", i)
			}
			for _, p := range v.manifest.Parts {
				if p.Kind == "copy" && p.Background == bright.Hex() {
					t.Fatal("a line read the bright frame through no scrim", i, p)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// CDS-84 and CDS-77: an authored slot holding a character its preset's face
// lacks is identified by entry before generation, not substituted.
func TestAuthoredRegionMissingGlyphIsRefusedByEntry(t *testing.T) {
	_, r := regionMeasured(t)
	missing := ""
	for c := rune(0xAC00); c <= 0xD7A3; c++ {
		if !design.Covers("jua", 400, string(c)) && design.Covers("paperlogy", 800, string(c)) {
			missing = string(c)
			break
		}
	}
	body := `<clip version="1" caption="bold"><text id="intro" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row>맛집 ` + missing + `</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
	in := clip.PlanningInput{Ratio: "vertical", TargetDurationMS: 15000, Design: clip.ProjectDesign{IntroPreset: "sticker", OutroPreset: "b"}, Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Body: body}}}
	var problem *composition.Problem
	if err := r.ValidateAuthoredInput(t.Context(), in); !errors.As(err, &problem) || problem.ElementID != "intro" || problem.Reason != "unsupported_glyph" {
		t.Fatal(err)
	}
	in.Design.IntroPreset = "a"
	if err := r.ValidateAuthoredInput(t.Context(), in); err != nil {
		t.Fatal("Paperlogy draws the syllable, so intro A admits it", err)
	}
}

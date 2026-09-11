// Package design holds the clip output design system (CDS): the single
// configuration file the renderer and the render-time verifier both read, so
// identical input renders identical output. It is pure by construction — no
// proto, SQL, transport or media imports — and every number in it is a CDS
// decision, never a literal spelled again at a call site.
//
// `chars` is 0 where CDS-20 states no per-line count for that type role; a
// style's own limit (CDS-23..26) lives in Styles instead.
package design

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed design.json
var files embed.FS

// JSON tags exist because this package parses the shared configuration file.
// They stop at this boundary: the clip domain's own types stay tag-free.
type Region struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}
type Anchor struct {
	Top      float64 `json:"top"`
	UpperMid float64 `json:"upper_mid"`
	LowerMid float64 `json:"lower_mid"`
	Bottom   float64 `json:"bottom"`
	Left     float64 `json:"left"`
	Center   float64 `json:"center"`
	Right    float64 `json:"right"`
}
type CardBox struct {
	Width   float64 `json:"width"`
	CenterY float64 `json:"center_y"`
}
type ChipStack struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	MaxWidth float64 `json:"max_width"`
	Columns  int     `json:"columns"`
}
type BadgeBox struct {
	Right float64 `json:"right"`
	Top   float64 `json:"top"`
}

// One ratio's whole geometry. CDS-8 states every dimension on 9:16; CDS-46
// through CDS-48 restate only the positions and widths the other two change.
type RatioLayout struct {
	Canvas       Size      `json:"canvas"`
	Safe         Region    `json:"safe"`
	Anchor       Anchor    `json:"anchor"`
	CopyMaxWidth float64   `json:"copy_max_width"`
	HookSize     float64   `json:"hook_size"`
	HookCard     CardBox   `json:"hook_card"`
	EndCard      CardBox   `json:"end_card"`
	Chip         ChipStack `json:"chip"`
	Badge        BadgeBox  `json:"badge"`
	ScrimTop     Region    `json:"scrim_top"`
	ScrimBottom  Region    `json:"scrim_bottom"`
}

// The estimated Naver overlay geometry behind SA-N (CDS-10). It is an estimate
// until CDS-11 is measured, which is why it is a constant and never a literal.
type OverlayEstimate struct {
	Top        float64 `json:"top"`
	Bottom     float64 `json:"bottom"`
	Right      float64 `json:"right"`
	RightFromY float64 `json:"right_from_y"`
	Left       float64 `json:"left"`
}
type TypeRole struct {
	Size       float64 `json:"size"`
	Min        float64 `json:"min"`
	Face       string  `json:"face"`
	Weight     int     `json:"weight"`
	Tracking   float64 `json:"tracking"`
	LineHeight float64 `json:"line_height"`
	Chars      int     `json:"chars"`
}
type Paint struct {
	Hex   string  `json:"hex"`
	Alpha float64 `json:"alpha"`
}
type ShadowPaint struct {
	Hex   string  `json:"hex"`
	Alpha float64 `json:"alpha"`
	Blur  float64 `json:"blur"`
	DX    float64 `json:"dx"`
	DY    float64 `json:"dy"`
}
type ScrimPaint struct {
	Hex  string  `json:"hex"`
	From float64 `json:"from"`
	To   float64 `json:"to"`
}
type Pad struct {
	V float64 `json:"v"`
	H float64 `json:"h"`
}
type Underline struct {
	HeightEM float64 `json:"height_em"`
	RaiseEM  float64 `json:"raise_em"`
	Extend   float64 `json:"extend"`
}
type SpacingTokens struct {
	PadBox        Pad       `json:"pad_box"`
	PadChip       Pad       `json:"pad_chip"`
	GapStack      float64   `json:"gap_stack"`
	GapChip       float64   `json:"gap_chip"`
	RadiusBox     float64   `json:"radius_box"`
	RadiusChip    float64   `json:"radius_chip"`
	RadiusCard    float64   `json:"radius_card"`
	BarAccent     float64   `json:"bar_accent"`
	StrokeText    float64   `json:"stroke_text"`
	StrokeMark    float64   `json:"stroke_mark"`
	DotAccent     float64   `json:"dot_accent"`
	UnderlineMark Underline `json:"underline_mark"`
}

// One copy style (CDS-22..26). Plate is empty for an unplated style, AnchorAlt is
// empty where CDS states no alternative anchor for it, and Stroke and Shadow name
// a spacing/shadow token rather than repeating its number.
type StyleRule struct {
	Type      string  `json:"type"`
	Plate     string  `json:"plate"`
	Lines     int     `json:"lines"`
	Chars     int     `json:"chars"`
	Anchor    string  `json:"anchor"`
	AnchorAlt string  `json:"anchor_alt"`
	Align     string  `json:"align"`
	Padding   Pad     `json:"padding"`
	PadLeft   float64 `json:"pad_left"`
	Bar       bool    `json:"bar"`
	Dot       bool    `json:"dot"`
	Stroke    string  `json:"stroke"`
	Shadow    string  `json:"shadow"`
	Highlight bool    `json:"highlight"`
}

// StrokeWidth is the round-joined stroke painted under an unplated style's fill:
// 6 px for 크게 강조 and 4 px for 형광펜 (CDS-21, CDS-25, CDS-26).
func (s StyleRule) StrokeWidth() float64 {
	switch s.Stroke {
	case "text":
		return Spacing.StrokeText
	case "mark":
		return Spacing.StrokeMark
	}
	return 0
}

// Role is the style's type scale entry (CDS-19).
func (s StyleRule) Role() TypeRole { return Type[s.Type] }

type MotionTokens struct {
	InMS  int        `json:"in_ms"`
	InDY  float64    `json:"in_dy"`
	OutMS int        `json:"out_ms"`
	Ease  [4]float64 `json:"ease"`
}
type TimingTokens struct {
	SubMinBaseMS    int     `json:"sub_min_base_ms"`
	SubMinPerCharMS int     `json:"sub_min_per_char_ms"`
	CutMinS         float64 `json:"cut_min_s"`
	CutMaxS         float64 `json:"cut_max_s"`
	HookCardS       float64 `json:"hook_card_s"`
	EndCardS        float64 `json:"end_card_s"`
	BadgeMinHeadS   float64 `json:"badge_min_head_s"`
	BadgeMinTailS   float64 `json:"badge_min_tail_s"`
}
type TransitionTokens struct {
	Default      string  `json:"default"`
	FadeMS       int     `json:"fade_ms"`
	FadeRatioMax float64 `json:"fade_ratio_max"`
}
type Loudnorm struct {
	I   float64 `json:"i"`
	TP  float64 `json:"tp"`
	LRA float64 `json:"lra"`
}
type AudioTokens struct {
	Loudnorm    Loudnorm `json:"loudnorm"`
	CrossfadeMS int      `json:"crossfade_ms"`
	HookDipDB   float64  `json:"hook_dip_db"`
}
type LumaTokens struct {
	ScrimThreshold float64 `json:"scrim_threshold"`
	SigmaThreshold float64 `json:"sigma_threshold"`
}
type system struct {
	Ratios            map[string]RatioLayout `json:"ratios"`
	SafeNaverEstimate Region                 `json:"safe_naver_estimate"`
	OverlayEstimate   OverlayEstimate        `json:"overlay_estimate"`
	Type              map[string]TypeRole    `json:"type"`
	Color             map[string]Paint       `json:"color"`
	Shadow            map[string]ShadowPaint `json:"shadow"`
	Scrim             map[string]ScrimPaint  `json:"scrim"`
	Accent            map[string]string      `json:"accent"`
	Spacing           SpacingTokens          `json:"spacing"`
	Styles            map[string]StyleRule   `json:"styles"`
	Motion            MotionTokens           `json:"motion"`
	Timing            TimingTokens           `json:"timing"`
	Transition        TransitionTokens       `json:"transition"`
	Audio             AudioTokens            `json:"audio"`
	Luma              LumaTokens             `json:"luma"`
}

var loaded = parse()

// The embedded file is code-owned and committed, so a malformed one is a build
// mistake the process must not start with, exactly like a failed migration.
func parse() system {
	data, err := files.ReadFile("design.json")
	if err != nil {
		panic(err)
	}
	var s system
	if err := json.Unmarshal(data, &s); err != nil {
		panic(fmt.Errorf("clip design system: %w", err))
	}
	return s
}

// JSON returns the embedded configuration bytes, for the mirror test that keeps
// the frontend copy from drifting.
func JSON() []byte {
	data, _ := files.ReadFile("design.json")
	return data
}

var (
	Ratios            = loaded.Ratios
	SafeNaverEstimate = loaded.SafeNaverEstimate
	Overlay           = loaded.OverlayEstimate
	Type              = loaded.Type
	Color             = loaded.Color
	Shadow            = loaded.Shadow
	Scrim             = loaded.Scrim
	Accent            = loaded.Accent
	Spacing           = loaded.Spacing
	Styles            = loaded.Styles
	Motion            = loaded.Motion
	Timing            = loaded.Timing
	Transition        = loaded.Transition
	Audio             = loaded.Audio
	Luma              = loaded.Luma
)

func Layout(ratio string) (RatioLayout, bool) {
	l, ok := Ratios[ratio]
	return l, ok
}
func Canvas(ratio string) (Size, bool) {
	l, ok := Ratios[ratio]
	return l.Canvas, ok
}
func Safe(ratio string) (Region, bool) {
	l, ok := Ratios[ratio]
	return l.Safe, ok
}
func Anchors(ratio string) (Anchor, bool) {
	l, ok := Ratios[ratio]
	return l.Anchor, ok
}

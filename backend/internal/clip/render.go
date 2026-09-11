package clip

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip/design"
)

var ErrCopyTooLong = errors.New("CLIP_COPY_TOO_LONG")

// All persisted time authority is integer milliseconds. Positions and volume are
// normalized values, never arbitrary filter expressions or pixel coordinates.
type Point struct{ X, Y float64 }
type Region struct{ X, Y, Width, Height float64 }
type Caption struct {
	Text, Anchor, Align, Style, Accent string
	// The one word 크게 강조 colours and 형광펜 highlights (CDS-25, CDS-26). A
	// substring of Text, chosen by the planner or the owner; empty means none.
	// It is a field rather than a marker inside Text because Text must stay
	// exactly what the owner approved, down to the byte.
	Keyword string
	// Relative to the trimmed cut. Both zero takes the CDS-27 default window.
	StartMS, EndMS int
}
type Copy = Caption
type Cut struct {
	ID, SourceID, Fingerprint string
	StartMS, EndMS            int
	Focal                     Point
	Copy                      Copy
	// Reserved fact labels whose chips belong on this cut (CDS-30), at most two
	// at a time. Part of the approved composition, so it is stored with it.
	Chips  []string
	Volume *float64 // nil keeps original audio; explicit zero mutes it
}
type EditCut = Cut

// CDS-27: copy enters at cut start + 120 ms and leaves at cut end - 120 ms, so
// no text straddles a transition. Both zero takes that default; an explicit
// window is exactly what the plan or the owner asked for.
func (c Cut) CaptionWindow() (int, int) {
	if c.Copy.StartMS == 0 && c.Copy.EndMS == 0 {
		return CopyLeadMS, c.EndMS - c.StartMS - CopyLeadMS
	}
	return c.Copy.StartMS, c.Copy.EndMS
}

func (c EditCut) OriginalVolume() float64 {
	if c.Volume == nil {
		return 1
	}
	return *c.Volume
}

type EditPlan struct {
	Ratio      string
	DurationMS int
	Cuts       []EditCut
	// Render inputs, not part of the approved composition and never stored with
	// it: the disclosure the badge shows and the facts a chip reads. They are
	// filled from the PROJECT at render time, so the badge is always the owner's
	// current campaign type and a plan stored before presets existed still
	// renders (CDS-31, CDS-30).
	Disclosure string
	Facts      []Answer
	// The template's category preset, which fixes the chip priority (CDS-50).
	Preset string
	// The opening card's title, written by the model under CDS-42 and rendered
	// by the hook card; empty when it could not be grounded.
	Hook string
	// What the model wrote per cut, parallel to Cuts, before the compiler placed
	// it. It is the compiler's input and is never stored with the plan.
	Written []Written
	// What the compiler decided per cut, in the same order: the class it read,
	// the scene it read it in and the fallback it had to use, if any.
	Decisions []Composition
}
type RenderSource struct {
	ID, Fingerprint string
	Info            MediaInfo
}

// The consumer downloads only the requested source, then removes it after fn.
type RenderSourceLoader func(context.Context, string, func(MediaSource) error) error
type RenderedVideo struct {
	Path     string
	Info     MediaInfo
	Bytes    int64
	Manifest Manifest
}
type Renderer interface {
	Render(context.Context, MediaWorkspace, EditPlan, []RenderSource, RenderSourceLoader) (RenderedVideo, error)
}
type RenderConfig struct {
	ResvgPath, FontPath                                              string
	MaxCuts, MaxCopyRunes, FadeMS, FPS, CRF, AudioRate, AudioBitrate int
	MinDurationMS, MaxDurationMS                                     int
}
type Canvas struct {
	Width, Height int
	Safe          Region
	Anchor        design.Anchor
}

// Every geometry here is a CDS constant, never a literal: the 9:16 safe area is
// the cross-platform intersection SA-C (CDS-9) and the other two are broadcast
// title-safe practice plus a player control bar (CDS-13).
func ClipCanvas(ratio string) (Canvas, error) {
	l, ok := design.Layout(ratio)
	if !ok {
		return Canvas{}, ErrInvalid
	}
	return Canvas{l.Canvas.Width, l.Canvas.Height, Region(l.Safe), l.Anchor}, nil
}

// The copy vocabulary. An anchor is the vertical placement CDS-12 names and an
// alignment the horizontal one; the anchor-step rule (CDS-38) reasons over the
// vertical anchor alone, which is why the two are separate fields.
var CopyAnchors = []string{"top", "upper_mid", "lower_mid", "bottom"}
var CopyAligns = []string{"center", "left", "right"}

// Bottom-leaning, because three of the four styles default to BOTTOM or
// LOWER_MID and a caption the owner saw low should not jump to the top.
var copyAnchorFallback = []string{"bottom", "lower_mid", "upper_mid", "top"}

// CDS's constraints count Korean syllables and exclude spaces and punctuation;
// the design system owns the counter so the renderer, the validator and the
// verifier can never disagree about how long a copy is.
func CopyChars(text string) int { return design.Chars(text) }

// CDS-27's inset at both ends of a cut's own copy window.
const CopyLeadMS = 120

// Manifest is every element the renderer places, in the design system's own
// shape so the verifier can read it without knowing the clip domain.
type Manifest = design.Manifest
type ManifestElement = design.Element

// CDS-41's minimum exposure for one copy, in milliseconds.
func MinExposureMS(text string) int {
	return design.Timing.SubMinBaseMS + design.Timing.SubMinPerCharMS*CopyChars(text)
}
func normalized(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }

// Normalized reports whether a value is a usable 0..1 fraction — a focal
// coordinate or an original-audio gain.
func Normalized(v float64) bool { return normalized(v) }
func ValidCopy(c Copy, maxRunes int) bool {
	if !utf8.ValidString(c.Text) || utf8.RuneCountInString(c.Text) > maxRunes || !ValidAccent(c.Accent) {
		return false
	}
	// A cut with NO copy carries no placement at all: the design system has
	// nothing to place there, so it names no anchor, alignment or style. That is
	// what a dropped copy looks like (CDS-41's last fallback).
	if strings.TrimSpace(c.Text) == "" && c.Anchor == "" && c.Align == "" && c.Style == "" {
		return c.Keyword == ""
	}
	_, known := design.Styles[c.Style]
	return slices.Contains(CopyAnchors, c.Anchor) && slices.Contains(CopyAligns, c.Align) && known
}

// planViolation preserves the invalid-plan identity and a content-free cause.
type planViolation string

func (e planViolation) Error() string                { return "invalid clip plan: " + string(e) }
func (e planViolation) Unwrap() error                { return ErrInvalid }
func (e planViolation) OutputValidationCode() string { return string(e) }

// One stable reason per verifier check, so the correction step can point at the
// field instead of saying "invalid plan" (LANG-21). The table is the allowlist:
// a code without an entry has no layout reason at all.
// Each is its own named constant so the public-reason scan can read it
// (internal/platform/rpcserver/failure_reasons_test.go).
const (
	reasonDisclosureRequired = "CLIP_DISCLOSURE_REQUIRED"
	reasonFactsRequired      = "CLIP_FACTS_REQUIRED"
)

const (
	reasonLayoutSafeArea   = "CLIP_LAYOUT_SAFE_AREA"
	reasonLayoutSize       = "CLIP_LAYOUT_SIZE"
	reasonLayoutOverlap    = "CLIP_LAYOUT_OVERLAP"
	reasonLayoutMotion     = "CLIP_LAYOUT_MOTION"
	reasonLayoutAnchorStep = "CLIP_LAYOUT_ANCHOR_STEP"
	reasonLayoutFrequency  = "CLIP_LAYOUT_FREQUENCY"
	reasonLayoutDisclosure = "CLIP_LAYOUT_DISCLOSURE"
	reasonLayoutKind       = "CLIP_LAYOUT_KIND"
)

var layoutReasons = map[string]string{
	string(design.ViolationSafeArea):   reasonLayoutSafeArea,
	string(design.ViolationSize):       reasonLayoutSize,
	string(design.ViolationOverlap):    reasonLayoutOverlap,
	string(design.ViolationMotion):     reasonLayoutMotion,
	string(design.ViolationAnchorStep): reasonLayoutAnchorStep,
	string(design.ViolationFrequency):  reasonLayoutFrequency,
	string(design.ViolationDisclosure): reasonLayoutDisclosure,
	string(design.ViolationKind):       reasonLayoutKind,
}

func (e planViolation) LayoutReason() string { return layoutReasons[string(e)] }

// VerifyLayout gates a render on the design system (CDS-52) and restates the
// failure as an invalid plan, which is what every caller already understands.
func VerifyLayout(ratio string, m Manifest) error {
	err := design.Verify(m, ratio)
	var v design.Violation
	if errors.As(err, &v) {
		return planViolation(v)
	}
	return err
}

// WithProject fills the render inputs the badge and the chips need. It is
// called at render time rather than at approval time so a stored plan never
// carries a stale disclosure.
func (p EditPlan) WithFacts(disclosure string, facts []Answer, preset string) EditPlan {
	p.Disclosure, p.Facts, p.Preset = disclosure, facts, preset
	return p
}

// ChipLabels is the subset of a cut's chips that names a reserved fact and has
// an answer, in the template preset's own priority (CDS-30, CDS-50).
func (p EditPlan) ChipLabels(c Cut) []string {
	answers := map[string]string{}
	for _, a := range p.Facts {
		answers[a.Label] = a.Text
	}
	out := []string{}
	// A chip is shown for the whole cut it belongs to and for at least 2.0 s, so
	// a shorter cut carries none rather than flashing one (CDS-30).
	if c.EndMS-c.StartMS < int(design.Timing.ChipMinS*1000) {
		return out
	}
	for _, label := range design.ChipPriority(p.Preset) {
		if !slices.Contains(c.Chips, label) || strings.TrimSpace(answers[label]) == "" {
			continue
		}
		out = append(out, label)
	}
	return out
}

func ValidateEditPlan(cfg RenderConfig, plan EditPlan, sources []RenderSource) error {
	if _, err := ClipCanvas(plan.Ratio); err != nil {
		return planViolation("plan_ratio")
	}
	if len(plan.Cuts) == 0 || len(plan.Cuts) > cfg.MaxCuts {
		return planViolation("plan_cut_count")
	}
	if plan.DurationMS < cfg.MinDurationMS || plan.DurationMS > cfg.MaxDurationMS {
		return planViolation("plan_duration_range")
	}
	byID := map[string]RenderSource{}
	for _, s := range sources {
		if s.ID == "" || s.Fingerprint == "" || s.Info.DurationMS <= 0 || s.Info.Width <= 0 || s.Info.Height <= 0 || byID[s.ID].ID != "" {
			return planViolation("plan_source_metadata")
		}
		byID[s.ID] = s
	}
	total := 0
	seen := map[string]bool{}
	for _, c := range plan.Cuts {
		if utf8.RuneCountInString(c.Copy.Text) > cfg.MaxCopyRunes {
			return ErrCopyTooLong
		}
		s, ok := byID[c.SourceID]
		if !ok || c.Fingerprint != s.Fingerprint {
			return planViolation("plan_source")
		}
		if strings.TrimSpace(c.ID) == "" || seen[c.ID] {
			return planViolation("plan_cut_identity")
		}
		if c.StartMS < 0 || c.StartMS >= c.EndMS || c.EndMS > s.Info.DurationMS {
			return planViolation("plan_cut_range")
		}
		if c.EndMS-c.StartMS <= 2*cfg.FadeMS {
			return planViolation("plan_cut_fade")
		}
		if !normalized(c.Focal.X) || !normalized(c.Focal.Y) {
			return planViolation("plan_focal")
		}
		if !normalized(c.OriginalVolume()) {
			return planViolation("plan_volume")
		}
		if !ValidCopy(c.Copy, cfg.MaxCopyRunes) {
			return planViolation("plan_copy_format")
		}
		// A chip names one of the five reserved facts, and at most two show at
		// once (CDS-30).
		if len(c.Chips) > 2 {
			return planViolation("plan_chip_count")
		}
		for _, label := range c.Chips {
			if !slices.Contains(design.Fact.Chips, label) {
				return planViolation("plan_chip_label")
			}
		}
		seen[c.ID] = true
		captionStart, captionEnd := c.CaptionWindow()
		if captionStart < 0 || captionEnd <= captionStart || captionEnd > c.EndMS-c.StartMS {
			return planViolation("plan_caption_time")
		}
		// The style's own line and character limits (CDS-20, CDS-23..26) and the
		// exposure its length earns (CDS-41). An empty copy is a cut with no text,
		// not a copy that breaks them.
		if style := design.Styles[c.Copy.Style]; strings.TrimSpace(c.Copy.Text) != "" {
			lines := strings.Split(c.Copy.Text, "\n")
			if len(lines) > style.Lines {
				return planViolation("plan_copy_lines")
			}
			for _, line := range lines {
				if CopyChars(line) > style.Chars {
					return planViolation("plan_copy_chars")
				}
			}
			if captionEnd-captionStart < MinExposureMS(c.Copy.Text) {
				return planViolation("plan_copy_exposure")
			}
			// The accent word must be in the text it accents (CDS-25, CDS-26).
			if c.Copy.Keyword != "" && !strings.Contains(c.Copy.Text, c.Copy.Keyword) {
				return planViolation("plan_copy_keyword")
			}
		}
		if c.EndMS-c.StartMS > cfg.MaxDurationMS+cfg.FadeMS*(len(plan.Cuts)-1)-total {
			return planViolation("plan_duration_limit")
		}
		total += c.EndMS - c.StartMS
	}
	// Overlap is part of the approved timeline, not extra trimming after approval.
	if total-cfg.FadeMS*(len(plan.Cuts)-1) != plan.DurationMS {
		return planViolation("plan_timeline")
	}
	return nil
}

// PlaceCopy resolves one anchor and alignment to the plate's region: TOP is the
// plate's top edge, BOTTOM its bottom edge and the MIDs its centre, while LEFT
// starts at the anchor's x, RIGHT ends at it and CENTER centres on it (CDS-12
// for 9:16, CDS-47 and CDS-48 for the other two). A plate that would leave the
// safe area is refused, never nudged: CDS-2 admits no exception, and 9:16's
// safe area is deliberately off-centre so a wide centred plate can miss it.
func PlaceCopy(canvas Canvas, anchor, align string, width, height float64) (Region, error) {
	a, s := canvas.Anchor, canvas.Safe
	if !slices.Contains(CopyAnchors, anchor) || !slices.Contains(CopyAligns, align) || math.IsNaN(width) || math.IsNaN(height) || width <= 0 || height <= 0 {
		return Region{}, ErrInvalid
	}
	y := a.Top
	switch anchor {
	case "upper_mid":
		y = a.UpperMid - height/2
	case "lower_mid":
		y = a.LowerMid - height/2
	case "bottom":
		y = a.Bottom - height
	}
	x := a.Left
	switch align {
	case "center":
		x = a.Center - width/2
	case "right":
		x = a.Right - width
	}
	if x < s.X || y < s.Y || x+width > s.X+s.Width || y+height > s.Y+s.Height {
		return Region{}, ErrInvalid
	}
	return Region{x, y, width, height}, nil
}

// PickCopyAnchor maps a normalized output-space avoid region only to the four
// approved anchors, keeping the alignment it was given. Manual placement remains
// exactly what the owner selected. CDS-38's full selection — subject rank, placed
// chips and badge, readable footage text and the one-step walk — is T105's.
func PickCopyAnchor(canvas Canvas, preferred, align string, width, height float64, avoid Region) (string, error) {
	if !normalized(avoid.X) || !normalized(avoid.Y) || !normalized(avoid.Width) || !normalized(avoid.Height) || avoid.X+avoid.Width > 1 || avoid.Y+avoid.Height > 1 {
		return "", ErrInvalid
	}
	if _, err := PlaceCopy(canvas, preferred, align, width, height); err != nil {
		return "", err
	}
	avoid = Region{avoid.X * float64(canvas.Width), avoid.Y * float64(canvas.Height), avoid.Width * float64(canvas.Width), avoid.Height * float64(canvas.Height)}
	best, area := preferred, math.Inf(1)
	for _, p := range append([]string{preferred}, copyAnchorFallback...) {
		r, err := PlaceCopy(canvas, p, align, width, height)
		if err != nil {
			continue
		}
		overlap := math.Max(0, math.Min(r.X+r.Width, avoid.X+avoid.Width)-math.Max(r.X, avoid.X)) * math.Max(0, math.Min(r.Y+r.Height, avoid.Y+avoid.Height)-math.Max(r.Y, avoid.Y))
		if overlap < area {
			best, area = p, overlap
		}
	}
	return best, nil
}

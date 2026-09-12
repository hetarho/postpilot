package clip

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

var ErrCopyTooLong = errors.New("CLIP_COPY_TOO_LONG")

// All persisted time authority is integer milliseconds. Positions and volume are
// normalized values, never arbitrary filter expressions or pixel coordinates.
type Point struct{ X, Y float64 }
type Region struct{ X, Y, Width, Height float64 }
type Caption struct {
	// Empty preserves legacy sentence timing; rapid uses explicit phrase windows.
	Pace                               string
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
	// The transition INTO this cut (CDS-36): 0 is a hard cut, 200 the fade a
	// scene change earns and 300 the fade-through-black nothing selects on its
	// own. It belongs to the cut it leads into, so reordering step ②'s cuts
	// keeps each cut's own entry and the first cut is always a hard cut.
	TransitionMS int
	Focal        Point
	// One copy, or the two CDS-43 lets a cut of 4 s or more carry in sequence:
	// a description and then the number it leads to, never both on screen at
	// once. A cut whose copy the composer dropped carries none.
	Copies []Copy
	// Reserved fact labels whose chips belong on this cut (CDS-30), at most two
	// at a time. Part of the approved composition, so it is stored with it.
	Chips  []string
	Volume *float64 // nil keeps original audio; explicit zero mutes it
}
type EditCut = Cut

// FirstCopy is the one a caller that knows nothing of CDS-43 means: the copy a
// cut has always had. A cut with no copy at all answers with an empty one.
func (c Cut) FirstCopy() Copy {
	if len(c.Copies) == 0 {
		return Copy{}
	}
	return c.Copies[0]
}

// Placed is the copies that actually show: a dropped one carries no text and no
// placement, and the cut simply shows its footage.
func (c Cut) Placed() []Copy {
	out := make([]Copy, 0, len(c.Copies))
	for _, copy := range c.Copies {
		if strings.TrimSpace(copy.Text) != "" {
			out = append(out, copy)
		}
	}
	return out
}

// CDS-27: copy enters at cut start + 120 ms and leaves at cut end - 120 ms, so
// no text straddles a transition. Both zero takes that default; an explicit
// window is exactly what the plan or the owner asked for — which is what a
// SECOND copy always carries, since two default windows would overlap.
func (c Cut) CaptionWindow(index int) (int, int) {
	copy := Copy{}
	if index >= 0 && index < len(c.Copies) {
		copy = c.Copies[index]
	}
	if copy.StartMS == 0 && copy.EndMS == 0 {
		return CopyLeadMS, c.EndMS - c.StartMS - CopyLeadMS
	}
	return copy.StartMS, copy.EndMS
}

// TransitionTotal is what the transitions take off the timeline: a cut overlaps
// the one before it by its own transition (CDS-36), so the clip is shorter than
// the sum of its cuts by exactly this.
func (p EditPlan) TransitionTotal() int {
	total := 0
	for _, c := range p.Cuts {
		total += c.TransitionMS
	}
	return total
}

// The three transitions CDS-36 admits and nothing else. 300 is accepted so a
// manual plan may choose the fade-through-black; no control offers it.
func ValidTransition(ms int) bool {
	return ms == 0 || ms == design.Transition.FadeMS || ms == design.Transition.BlackMS
}

func (c EditCut) OriginalVolume() float64 {
	if c.Volume == nil {
		return 1
	}
	return *c.Volume
}

type EditPlan struct {
	Portable       *PortablePlan
	HideDisclosure bool
	Ratio          string
	DurationMS     int
	Cuts           []EditCut
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
	// The closing call to action, already resolved against the preset, and the
	// project accent both cards paint with (CDS-29, CLIP-14).
	CTA, Accent string
	// Styles is the template's approved copy styles, set at render time like
	// the facts above: the verifier reads CDS-40's run rule against it.
	Styles []string
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
	Plan     *EditPlan
	Path     string
	Info     MediaInfo
	Bytes    int64
	Manifest Manifest
	Elements []CompositionElement
}
type Renderer interface {
	Render(context.Context, MediaWorkspace, EditPlan, []RenderSource, RenderSourceLoader) (RenderedVideo, error)
}
type RenderConfig struct {
	OverlayBatchSize    int
	Composition         composition.Limits
	ResvgPath, FontPath string
	// Empty uses the embedded preset catalog; a directory is snapshotted at boot.
	OverlayDir string
	// The secondary face (CDS-17): the hook title and 크게 강조 are set in it.
	DisplayFontPath                                                  string
	MaxCuts, MaxCopyRunes, FadeMS, FPS, CRF, AudioRate, AudioBitrate int
	MinDurationMS, MaxDurationMS                                     int
}
type Canvas struct {
	Width, Height int
	Safe          Region
	Anchor        design.Anchor
}

// Every geometry here is a CDS constant, never a literal: the 9:16 safe area is
// the explicit design bounds (CDS-9) and the other two are broadcast
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
	if !utf8.ValidString(c.Text) || utf8.RuneCountInString(c.Text) > maxRunes || !ValidAccent(c.Accent) || !ValidCaptionPace(c.Pace) {
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
	reasonLayoutContrast   = "CLIP_LAYOUT_CONTRAST"
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
	string(design.ViolationContrast):   reasonLayoutContrast,
}

func (e planViolation) LayoutReason() string { return layoutReasons[string(e)] }

// FurnitureSlot marks a layout failure the design system's own furniture caused.
const FurnitureSlot = design.FurnitureSlot

// LayoutError is one verifier failure and the caption it names: the (cut, copy)
// whose style, anchor or presence the repair ladder may change (CDS-55), or the
// furniture slot when the badge, a chip or a card failed — a renderer defect no
// caption repair can reach. It keeps the invalid-plan identity and the
// content-free reason every caller already understands.
type LayoutError struct {
	planViolation
	Cut, Copy int
}

func (e *LayoutError) Unwrap() error   { return ErrInvalid }
func (e *LayoutError) Furniture() bool { return e.Cut < 0 }

// LayoutViolation restates one design-system violation the renderer found on its
// own — V3's contrast, which only exists once the footage under a copy has been
// sampled (CDS-44) — naming the caption it was measured on.
func LayoutViolation(v design.Violation, cut, copy int) error {
	return &LayoutError{planViolation(v), cut, copy}
}

// VerifyLayout enforces delivery checks (CDS-52), excluding advisory overlaps
// (CDS-56), and names the caption a blocking failure belongs to (CDS-55).
func VerifyLayout(ratio string, approved []string, m Manifest, hideDisclosure ...bool) error {
	err := design.VerifyRenderable(m, ratio, approved, hideDisclosure...)
	var f *design.Failure
	if errors.As(err, &f) {
		return &LayoutError{planViolation(f.Check), f.Cut, f.Copy}
	}
	var v design.Violation
	if errors.As(err, &v) {
		return &LayoutError{planViolation(v), FurnitureSlot, FurnitureSlot}
	}
	return err
}

// Compiled reports whether this plan came straight from the compiler, which is
// the one thing that decides who owns a style choice. The per-cut decisions are
// the compiler's own output and are never stored with a plan (they are not part
// of the approved composition), so a plan that still carries one per cut has not
// been through a person. It is what lets a contrast fallback stay silent on a
// machine's choice and be reported on a person's (CDS-44, CDS-52).
func (p EditPlan) Compiled() bool {
	return len(p.Cuts) > 0 && len(p.Decisions) == len(p.Cuts)
}

// WithProject fills the render inputs the badge and the chips need. It is
// called at render time rather than at approval time so a stored plan never
// carries a stale disclosure.
func (p EditPlan) WithFacts(disclosure string, facts []Answer, preset, cta, accent string, hideDisclosure ...bool) EditPlan {
	p.HideDisclosure = len(hideDisclosure) > 0 && hideDisclosure[0]
	p.Disclosure, p.Facts, p.Preset = disclosure, facts, preset
	p.CTA, p.Accent = cta, accent
	return p
}

// WithStyles records the template's approved copy styles for the verifier.
func (p EditPlan) WithStyles(styles []string) EditPlan {
	p.Styles = styles
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
	// Two lines of nine, which is what the hook card sets (CDS-28).
	if design.Chars(plan.Hook) > 2*design.Type["hook"].Chars || strings.Count(plan.Hook, "\n") > 1 {
		return planViolation("plan_hook")
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
	for i, c := range plan.Cuts {
		for _, copy := range c.Copies {
			if utf8.RuneCountInString(copy.Text) > cfg.MaxCopyRunes {
				return ErrCopyTooLong
			}
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
		// A transition belongs to the cut it leads into and the first cut has
		// none: a clip does not fade in from nothing (CDS-36).
		if !ValidTransition(c.TransitionMS) || (i == 0 && c.TransitionMS != 0) {
			return planViolation("plan_cut_transition")
		}
		// A cut has to outlast both boundaries that eat into it — its own
		// transition and the one the next cut leads in with.
		overlap := c.TransitionMS
		if i+1 < len(plan.Cuts) {
			overlap += plan.Cuts[i+1].TransitionMS
		}
		if c.EndMS-c.StartMS <= max(2*design.Transition.FadeMS, overlap) {
			return planViolation("plan_cut_fade")
		}
		if !normalized(c.Focal.X) || !normalized(c.Focal.Y) {
			return planViolation("plan_focal")
		}
		if !normalized(c.OriginalVolume()) {
			return planViolation("plan_volume")
		}
		// CDS-43: one copy, or two only on a cut of 4 s or more.
		rapid := c.Rapid()
		maxCopies := design.Copy.MaxPerCut
		if rapid {
			maxCopies = design.Rapid.MaxPerCut
		}
		if len(c.Copies) > maxCopies {
			return planViolation("plan_copy_count")
		}
		if !rapid && len(c.Copies) > 1 && c.EndMS-c.StartMS < design.Copy.SecondMinCutMS() {
			return planViolation("plan_copy_second_cut")
		}
		for _, copy := range c.Copies {
			if !ValidCopy(copy, cfg.MaxCopyRunes) || (copy.Pace == "rapid" && !rapid) || (rapid && strings.TrimSpace(copy.Text) == "") {
				return planViolation("plan_copy_format")
			}
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
		previousEnd := 0
		for j, copy := range c.Copies {
			captionStart, captionEnd := c.CaptionWindow(j)
			if captionStart < 0 || captionEnd <= captionStart || captionEnd > c.EndMS-c.StartMS {
				return planViolation("plan_caption_time")
			}
			// Never both at once: the second copy starts a clear 120 ms after
			// the first has left (CDS-43).
			gap := design.Timing.CopyLeadMS
			if rapid {
				gap = 0
			}
			if j > 0 && captionStart-previousEnd < gap {
				return planViolation("plan_copy_sequence")
			}
			previousEnd = captionEnd
			// The style's own line and character limits (CDS-20, CDS-23..26) and
			// the exposure its length earns (CDS-41). An empty copy is a cut with
			// no text, not a copy that breaks them.
			style := design.Styles[copy.Style]
			if strings.TrimSpace(copy.Text) == "" {
				continue
			}
			lines := strings.Split(copy.Text, "\n")
			if len(lines) > style.Lines {
				return planViolation("plan_copy_lines")
			}
			for _, line := range lines {
				if CopyChars(line) > style.Chars {
					return planViolation("plan_copy_chars")
				}
			}
			minimum := MinExposureMS(copy.Text)
			if rapid {
				minimum = design.Rapid.MinMS
				if (copy.StartMS == 0 && copy.EndMS == 0) || captionEnd-captionStart > design.Rapid.MaxMS || strings.Contains(copy.Text, "\n") || CopyChars(copy.Text) > min(style.Chars, design.Rapid.MaxChars) {
					return planViolation("plan_copy_exposure")
				}
			}
			if captionEnd-captionStart < minimum {
				return planViolation("plan_copy_exposure")
			}
			// The accent word must be in the text it accents (CDS-25, CDS-26).
			if copy.Keyword != "" && !strings.Contains(copy.Text, copy.Keyword) {
				return planViolation("plan_copy_keyword")
			}
		}
		// A second copy is a description followed by the number it leads to, in
		// that order and no other (CDS-43).
		if placed := c.Placed(); !rapid && len(placed) > 1 &&
			(design.Classify(placed[0].Text) != design.ClassDesc || design.Classify(placed[1].Text) != design.ClassNum) {
			return planViolation("plan_copy_classes")
		}
		if c.EndMS-c.StartMS > cfg.MaxDurationMS+plan.TransitionTotal()-total {
			return planViolation("plan_duration_limit")
		}
		total += c.EndMS - c.StartMS
	}
	// Overlap is part of the approved timeline, not extra trimming after approval.
	if total-plan.TransitionTotal() != plan.DurationMS {
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

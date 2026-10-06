package clip

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"

	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/llm"
)

// These records contain facts in original, display-oriented source coordinates.
// No source bytes or signed URL survives in an analysis or an edit plan.
type AnalysisSource struct {
	RenderSource
	Filename string
	// Browser measurements remain explicit in recovery. Absence is the
	// existing native-verified cache and preserves its serialized shape.
	OriginalMeasurementProvenance string `json:",omitempty"`
}
type Segment struct {
	StartMS, EndMS         int
	Event, Speech, Quality string
	// What the subject does and how the frame itself moves, recorded apart from
	// Event so a still subject under a panning camera stays distinguishable from
	// a moving subject under a locked one (CLIP-10). Either may be empty.
	Action, Motion string
	Subjects       []string
	Focal          Point
	// What the segment shows, whether legible footage text fills
	// the frame (CDS-38), and the ONE principal subject box in normalized source
	// coordinates — zero-size when the frame has none. The box replaced a
	// keep-out region because CDS-38 needs the subject's own AREA for its 15 %
	// rule, not a box to dodge.
	Scene        string
	ReadableText bool
	Subject      Region
	// Factual empty source-space regions, never an editorial placement choice.
	// Optional in v2 observations so existing recorded work remains reusable.
	CaptionSafe []Region
	// How far the record can be trusted and whether the footage can be used at
	// all. A black, obscured, static or unreadable span is RECORDED with these
	// two fields rather than omitted, so the writer sees the whole timeline
	// (CLIP-10, CLIP-99). Both are empty in a record written under
	// clip-observation-v1; nothing invents a status for it.
	Certainty, Usability string
}

const (
	CertaintyCertain   = "certain"
	CertaintyUncertain = "uncertain"
	CertaintyUnknown   = "unknown"
	UsabilityUsable    = "usable"
	UsabilityUnusable  = "unusable"
)

func validCertainty(v string) bool {
	return v == CertaintyCertain || v == CertaintyUncertain || v == CertaintyUnknown
}
func validUsability(v string) bool { return v == UsabilityUsable || v == UsabilityUnusable }

// Observed reports whether the record carries the v2 status pair at all. A
// legacy record has neither and is never given an invented one.
func (s Segment) Observed() bool { return s.Certainty != "" && s.Usability != "" }

// Confident is the only recorded state that may authorize a derived fact.
func (s Segment) Confident() bool {
	return s.Certainty == CertaintyCertain && s.Usability == UsabilityUsable
}

// Unknowable footage owes a quality reason instead of a factual description:
// there is nothing truthful to say about a black or obscured span.
func (s Segment) Unknowable() bool {
	return s.Certainty == CertaintyUnknown || s.Usability == UsabilityUnusable
}

type SourceAnalysis struct {
	Source   AnalysisSource
	Segments []Segment
}
type ChunkInput struct {
	Language                    string
	Source                      AnalysisSource
	Index, OffsetMS, DurationMS int
	Video                       llm.InlineVideo
	Policy                      llm.CallPolicy
}
type ChunkAnalysis struct {
	SourceID, Fingerprint       string
	Index, OffsetMS, DurationMS int
	Segments                    []Segment
}
type AnalysisLimits struct {
	ChunkMS, MaxSources, MaxSourceDurationMS, MaxSegments, MaxTextRunes, MaxSubjects int
}
type PlanningInput struct {
	MeasuredNarration *NarrationPlan `json:",omitempty"`
	Language          string
	Composition       *ProjectComposition
	Template          Recipe
	Ratio             string
	TargetDurationMS  int
	Analyses          []SourceAnalysis
	Policy            llm.CallPolicy
	// The campaign type the badge shows (CDS-31).
	Disclosure     string
	HideDisclosure bool
	// The project's own instruction (CLIP-121), empty when none was written.
	// On content it outranks the template's authored guidance; every declared
	// structure stays the template's.
	Instruction string
	// The project's design selection (CLIP-139): the presets the intro and the
	// outro will render in and the styles a caption may take. The admission
	// layout checks the SAME presets the render will use, so a region that
	// cannot be placed is refused before any paid work rather than at render.
	Design ProjectDesign
	// The owner's per-source original-sound choice, as a FACT the writer reads:
	// a speech span of a source that keeps its sound cannot be transformed
	// (CLIP-129). The setting itself stays the owner's and is applied by the
	// server after the response (CLIP-100).
	SourceAudio []SourceAudioSetting
	// The clip's frozen 영상 지침 (GUIDE-15, GUIDE-17): read into the flow, narration and
	// revision system prompts, and absent from them when both groups are empty.
	Guidelines VideoGuidelines
	// The storyline 이 스토리로 만들기 builds along (CLIP-178, CDS-37): the flow is given only
	// the scenes it holds and keeps its order and pace, and writes no storyline of its own.
	// Nil is 바로 만들기.
	FollowStoryline *Storyline
	// The project's intro/outro slots as the approval froze them (CLIP-69, CLIP-186): the
	// storyline call and 바로 만들기's flow call draft the generated ones, and every call reads
	// the owner's and the answers' words as written. Nil is a job frozen before slots existed,
	// which drafts none.
	Regions *ProjectRegions
}

// VideoGuidelines are a clip's frozen 영상 지침: the enabled clip 기본 지침 in the project's
// language, then the owner's that apply to its video template, each in injection order
// (GUIDE-14). Frozen with an approval, so a guideline edited mid-flight changes nothing in
// flight.
type VideoGuidelines struct {
	Defaults []string `json:",omitempty"`
	Owner    []string `json:",omitempty"`
}

func (g VideoGuidelines) Empty() bool { return len(g.Defaults) == 0 && len(g.Owner) == 0 }

// IsZero lets a payload omit an empty value whole, so a job frozen with none reads byte for byte
// as it did before 영상 지침 existed.
func (g VideoGuidelines) IsZero() bool { return g.Empty() }

// Digest binds a quote to the 영상 지침 it was taken under (QUOTA-45): a guideline changed
// between the quote and the start invalidates the approval. Empty is the empty digest, so a
// project with none quotes exactly as it did before 영상 지침 existed.
func (g VideoGuidelines) Digest() string {
	if g.Empty() {
		return ""
	}
	raw, _ := json.Marshal(g)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// StorylineInput is the storyline call's input (CLIP-177): the planning input over the
// observations, and for a storyline request (CLIP-181) the current storyline and the owner's
// words. Current is nil and Request empty for 스토리라인 먼저 and 다시 만들기.
type StorylineInput struct {
	PlanningInput
	Current *Storyline
	Request string
}

// RevisionInput is one owner-written revision request (CLIP-131): the plan as
// their own edits left it, what they asked for, and which document the request
// is about. The observations are the ones already stored, and the template's
// declared structure is untouched.
type RevisionInput struct {
	PlanningInput
	Current EditPlan
	Request string
	// RevisionFlow, RevisionNarration or RevisionBoth.
	Target string
}

// The three documents a request may name. A flow target is followed by a
// narration over the rewritten flow, because captions timed against cuts that
// moved are timed against nothing (CLIP-135).
const (
	RevisionFlow      = "flow"
	RevisionNarration = "narration"
	RevisionBoth      = "both"
)

func ValidRevisionTarget(target string) bool {
	return target == RevisionFlow || target == RevisionNarration || target == RevisionBoth
}

// NarrationInput is the second writing call's input: the same planning input,
// and the flow the server has already resolved into exact output intervals. The
// narration is written over that flow and may not change it (CLIP-135).
type NarrationInput struct {
	PlanningInput
	Flow EditPlan
}

// CutScene is the scene of the segment a cut starts in, and ReadableText is
// true when ANY segment the cut spans carries legible footage text: the
// restriction exists to keep typeset copy away from photographed text, so one
// segment is enough to trigger it (CDS-38).
func CutScene(cut Cut, analysis SourceAnalysis) (string, bool) {
	scene, readable := "", false
	for _, s := range analysis.Segments {
		if s.EndMS <= cut.StartMS || s.StartMS >= cut.EndMS {
			continue
		}
		if scene == "" {
			scene = s.Scene
		}
		readable = readable || s.ReadableText
	}
	return design.Scene(scene), readable
}

func ValidRegion(r Region) bool {
	return normalized(r.X) && normalized(r.Y) && normalized(r.Width) && normalized(r.Height) && r.X+r.Width <= 1 && r.Y+r.Height <= 1
}
func ValidateAnalysisSources(l AnalysisLimits, sources []AnalysisSource) error {
	if len(sources) == 0 || len(sources) > l.MaxSources || l.ChunkMS <= 0 {
		return ErrInvalid
	}
	seen, fingerprints, total := map[string]bool{}, map[string]bool{}, 0
	for _, source := range sources {
		if source.OriginalMeasurementProvenance != "" && source.OriginalMeasurementProvenance != BrowserOriginalProvenance {
			return ErrInvalid
		}
		if strings.TrimSpace(source.ID) == "" || source.Fingerprint == "" || strings.TrimSpace(source.Filename) == "" || seen[source.ID] || fingerprints[source.Fingerprint] || source.Info.DurationMS <= 0 || source.Info.DurationMS > l.MaxSourceDurationMS-total || source.Info.Width <= 0 || source.Info.Height <= 0 {
			return ErrInvalid
		}
		seen[source.ID], fingerprints[source.Fingerprint] = true, true
		total += source.Info.DurationMS
	}
	return nil
}
func ValidateChunkInput(l AnalysisLimits, in ChunkInput) error {
	if err := ValidateAnalysisSources(l, []AnalysisSource{in.Source}); err != nil {
		return err
	}
	// Validate bounds before multiplication so a forged index cannot overflow.
	if in.Index < 0 || in.Index > (in.Source.Info.DurationMS-1)/l.ChunkMS || in.OffsetMS != in.Index*l.ChunkMS || in.DurationMS != min(l.ChunkMS, in.Source.Info.DurationMS-in.OffsetMS) {
		return ErrInvalid
	}
	return nil
}

// ValidateSegments holds a clip-observation-v2 record to the complete-coverage
// contract: the scenes open at start, each one continues exactly where the last
// ended, the final one closes at end, and every scene states how far it can be
// trusted and whether its footage is usable (CLIP-10).
func ValidateSegments(l AnalysisLimits, segments []Segment, start, end int) error {
	return validateSegments(l, segments, start, end, true)
}

// ValidateLegacySegments reads a record written under clip-observation-v1: its
// scenes are ordered and non-overlapping but may leave an unobserved gap and
// carry no status at all. It exists so stored v1 work stays readable; no v1
// record may enter a v2 writer input (CLIP-93).
func ValidateLegacySegments(l AnalysisLimits, segments []Segment, start, end int) error {
	return validateSegments(l, segments, start, end, false)
}

func validateSegments(l AnalysisLimits, segments []Segment, start, end int, complete bool) error {
	if len(segments) == 0 || len(segments) > l.MaxSegments {
		return observationViolation("observe_segment_count", 0, map[string]int{"segment_count": len(segments)})
	}
	previous := start
	for i, s := range segments {
		timing := map[string]int{"start_ms": s.StartMS, "end_ms": s.EndMS, "previous_end_ms": previous, "duration_ms": end - start}
		if s.StartMS < start || s.EndMS <= s.StartMS || s.EndMS > end {
			return observationViolation("observe_segment_time", i+1, timing)
		}
		if s.StartMS < previous {
			return observationViolation("observe_segment_overlap", i+1, timing)
		}
		// A missing opening scene and an unobserved span between two scenes are
		// separate facts about the response, so correction feedback can say
		// which one to fix rather than asking for the whole chunk again.
		if complete && s.StartMS != previous {
			check := "observe_coverage_gap"
			if i == 0 {
				check = "observe_coverage_start"
			}
			return observationViolation(check, i+1, timing)
		}
		if complete || s.Certainty != "" || s.Usability != "" {
			if !validCertainty(s.Certainty) || !validUsability(s.Usability) {
				return observationViolation("observe_status", i+1, nil)
			}
		}
		if !normalized(s.Focal.X) || !normalized(s.Focal.Y) {
			return observationViolation("observe_focal", i+1, observationGeometry(s.Focal, s.Subject))
		}
		if !ValidRegion(s.Subject) {
			return observationViolation("observe_subject_bounds", i+1, observationGeometry(s.Focal, s.Subject))
		}
		if len(s.CaptionSafe) > 4 {
			return observationViolation("observe_subject_bounds", i+1, nil)
		}
		for _, box := range s.CaptionSafe {
			if !ValidRegion(box) || box.Width <= 0 || box.Height <= 0 {
				return observationViolation("observe_subject_bounds", i+1, observationGeometry(s.Focal, box))
			}
		}
		if !BoundedText(s.Event, 0, l.MaxTextRunes) || !BoundedText(s.Speech, 0, l.MaxTextRunes) || !BoundedText(s.Quality, 1, l.MaxTextRunes) || !BoundedText(s.Action, 0, l.MaxTextRunes) || !BoundedText(s.Motion, 0, l.MaxTextRunes) {
			return observationViolation("observe_text_length", i+1, map[string]int{"event_runes": len([]rune(s.Event)), "speech_runes": len([]rune(s.Speech)), "quality_runes": len([]rune(s.Quality)), "action_runes": len([]rune(s.Action)), "motion_runes": len([]rune(s.Motion))})
		}
		if strings.TrimSpace(s.Quality) == "" {
			return observationViolation("observe_quality", i+1, nil)
		}
		if len(s.Subjects) > l.MaxSubjects {
			return observationViolation("observe_subject_count", i+1, map[string]int{"subject_count": len(s.Subjects)})
		}
		description := strings.TrimSpace(s.Event) != "" || strings.TrimSpace(s.Speech) != "" || strings.TrimSpace(s.Action) != "" || strings.TrimSpace(s.Motion) != ""
		for _, subject := range s.Subjects {
			if !BoundedText(subject, 1, l.MaxTextRunes) || strings.TrimSpace(subject) == "" {
				return observationViolation("observe_subject_text", i+1, map[string]int{"subject_runes": len([]rune(subject))})
			}
			description = true
		}
		// Black, obscured or unreadable footage has nothing truthful to
		// describe, so its BoundedText quality reason IS the record. Everything
		// else still owes a fact.
		if !description && !s.Unknowable() {
			return observationViolation("observe_description", i+1, nil)
		}
		previous = s.EndMS
	}
	if complete && previous != end {
		return observationViolation("observe_coverage_end", len(segments), map[string]int{"previous_end_ms": previous, "duration_ms": end - start})
	}
	return nil
}

// MergeAnalyses accepts exactly one complete, ordered chunk sequence per frozen
// source. It never sorts or silently drops a hallucinated, duplicated or missing id.
func MergeAnalyses(l AnalysisLimits, sources []AnalysisSource, chunks []ChunkAnalysis) ([]SourceAnalysis, error) {
	if err := ValidateAnalysisSources(l, sources); err != nil {
		return nil, err
	}
	out, cursor := make([]SourceAnalysis, 0, len(sources)), 0
	for _, source := range sources {
		analysis := SourceAnalysis{Source: source}
		for offset, index := 0, 0; offset < source.Info.DurationMS; offset, index = offset+l.ChunkMS, index+1 {
			if cursor >= len(chunks) {
				return nil, ErrInvalid
			}
			c := chunks[cursor]
			duration := min(l.ChunkMS, source.Info.DurationMS-offset)
			if c.SourceID != source.ID || c.Fingerprint != source.Fingerprint || c.Index != index || c.OffsetMS != offset || c.DurationMS != duration {
				return nil, ErrInvalid
			}
			if err := ValidateSegments(l, c.Segments, offset, offset+duration); err != nil {
				return nil, err
			}
			analysis.Segments = append(analysis.Segments, c.Segments...)
			cursor++
		}
		out = append(out, analysis)
	}
	if cursor != len(chunks) {
		return nil, ErrInvalid
	}
	return out, nil
}

// CutSubject projects the principal-subject boxes a cut spans through the same
// cover crop as the renderer, conservatively joining the visible intersections,
// and returns the result in CANVAS pixels — which is what CDS-38's 15 % rule
// measures against. A cut with no box gets a zero region.
func CutSubject(canvas Canvas, cut Cut, analysis SourceAnalysis) Region {
	n := CaptionAvoid(canvas, cut, analysis)
	if n.Width <= 0 || n.Height <= 0 {
		return Region{}
	}
	return Region{X: n.X * float64(canvas.Width), Y: n.Y * float64(canvas.Height), Width: n.Width * float64(canvas.Width), Height: n.Height * float64(canvas.Height)}
}

// CaptionAvoid projects every relevant source-space subject box through the same
// cover crop as the renderer, conservatively joining the visible intersections.
// The result is normalized to the output frame.
func CaptionAvoid(canvas Canvas, cut Cut, analysis SourceAnalysis) Region {
	w, h := float64(analysis.Source.Info.Width), float64(analysis.Source.Info.Height)
	scale := math.Max(float64(canvas.Width)/w, float64(canvas.Height)/h)
	w, h = w*scale, h*scale
	x := math.Max(0, math.Min(w-float64(canvas.Width), w*cut.Focal.X-float64(canvas.Width)/2))
	y := math.Max(0, math.Min(h-float64(canvas.Height), h*cut.Focal.Y-float64(canvas.Height)/2))
	left, top, right, bottom := 1.0, 1.0, 0.0, 0.0
	for _, s := range analysis.Segments {
		if s.EndMS <= cut.StartMS || s.StartMS >= cut.EndMS || s.Subject.Width == 0 || s.Subject.Height == 0 {
			continue
		}
		l := math.Max(0, math.Min(1, (s.Subject.X*w-x)/float64(canvas.Width)))
		r := math.Max(0, math.Min(1, ((s.Subject.X+s.Subject.Width)*w-x)/float64(canvas.Width)))
		t := math.Max(0, math.Min(1, (s.Subject.Y*h-y)/float64(canvas.Height)))
		b := math.Max(0, math.Min(1, ((s.Subject.Y+s.Subject.Height)*h-y)/float64(canvas.Height)))
		if l >= r || t >= b {
			continue
		}
		left, top, right, bottom = math.Min(left, l), math.Min(top, t), math.Max(right, r), math.Max(bottom, b)
	}
	if left >= right || top >= bottom {
		return Region{}
	}
	return Region{X: left, Y: top, Width: right - left, Height: bottom - top}
}

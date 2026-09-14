package clip

import (
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// RateUnitPermille is 1x. Rates ride the whole contract as integer permille so
// that preview, validation and export share one exact arithmetic (CDS-62).
const RateUnitPermille = composition.RateUnitPermille

// PlaybackRates is CLIP-98's closed set, in ascending order. Nothing else is a
// rate: an unsupported value is identified, never rounded to a supported one
// and never simulated (CLIP-99).
func PlaybackRates() []int { return slices.Clone(design.Playback.RatesPermille) }

// ValidPlaybackRate admits only an explicit member of that set. Zero is NOT a
// rate — it is the absence of one, which only a legacy compatibility path may
// read as 1x (CLIP-101).
func ValidPlaybackRate(permille int) bool {
	return slices.Contains(design.Playback.RatesPermille, permille)
}

// TransformedDuration is the shared pre-transition output length of a source
// span at a fixed rate. It is the composition language's own checked helper, so
// the domain cannot drift from the timeline the resolver builds.
func TransformedDuration(spanMS, ratePermille int) (int, bool) {
	if !ValidPlaybackRate(ratePermille) {
		return 0, false
	}
	return composition.TransformedDurationMS(spanMS, ratePermille)
}

// Rate is this cut's fixed playback rate. A zero field is a cut built before
// rates existed — the legacy reading of 1x (CLIP-101). Every stored plan is
// normalized on decode, so a zero survives only in memory.
func (c Cut) Rate() int {
	if c.PlaybackRatePermille == 0 {
		return RateUnitPermille
	}
	return c.PlaybackRatePermille
}

// SourceSpanMS is the original footage this cut selects, in source milliseconds.
// Evidence and observations are always stated in these coordinates.
func (c Cut) SourceSpanMS() int { return c.EndMS - c.StartMS }

// OutputDurationMS is how long the cut occupies the edited output timeline
// before its transition overlap is taken off (CDS-62).
func (c Cut) OutputDurationMS() int {
	out, ok := TransformedDuration(c.SourceSpanMS(), c.Rate())
	if !ok {
		return 0
	}
	return out
}

// Rate is the same reading for a cut being corrected.
func (c CorrectionCut) Rate() int {
	if c.PlaybackRatePermille == 0 {
		return RateUnitPermille
	}
	return c.PlaybackRatePermille
}
func (c CorrectionCut) OutputDurationMS() int {
	out, ok := TransformedDuration(c.EndMS-c.StartMS, c.Rate())
	if !ok {
		return 0
	}
	return out
}

// SourceCadence is a source's VERIFIED original cadence, taken from the full
// decode of the original file: the frame count and the decoded duration the
// probe actually measured, cross-checked against the declared average rate.
// The 15 fps analysis proxy is never cadence evidence — it is a re-encode.
type SourceCadence struct {
	Numerator, Denominator int
}

// Verified reports a usable constant cadence. An unmeasured source (an analysis
// recorded before the decode counted frames), a source whose decoded cadence
// disagrees with its declared one, or one with no declared rate at all is
// treated conservatively as unverified (CDS-68).
func (c SourceCadence) Verified() bool { return c.Numerator > 0 && c.Denominator > 0 }

// VerifiedCadence reads the cadence a probe recorded for an ORIGINAL source.
// The decoded frame count is the authority; the declared average rate must
// agree with it within one frame over the decoded span, which is what separates
// constant-rate footage from variable-rate footage the container merely averages.
func VerifiedCadence(info MediaInfo) SourceCadence {
	if info.FrameRateNumerator <= 0 || info.FrameRateDenominator <= 0 {
		return SourceCadence{}
	}
	if info.DecodedFrames <= 0 || info.DecodedDurationMS <= 0 {
		return SourceCadence{}
	}
	// declared frames over the decoded span, as an exact integer comparison:
	// numerator × ms ÷ (denominator × 1000).
	declared := info.FrameRateNumerator * info.DecodedDurationMS
	measured := info.DecodedFrames * info.FrameRateDenominator * 1000
	tolerance := info.FrameRateNumerator * 1000 / info.FrameRateDenominator
	if declared-measured > tolerance || measured-declared > tolerance {
		return SourceCadence{}
	}
	return SourceCadence{info.FrameRateNumerator, info.FrameRateDenominator}
}

// AllowedPlaybackRates is the rate set this source may actually be cut at.
// 1x and every faster rate need no evidence: speeding footage up never asks for
// a frame that was not recorded. A slow rate does, so it is admitted only when
// verified cadence still reaches the 30 fps output — `source_fps × rate ≥ 30`
// — and refused otherwise rather than quietly replaced by 1x (CDS-68, CLIP-99).
func AllowedPlaybackRates(info MediaInfo) []int {
	cadence := VerifiedCadence(info)
	out := make([]int, 0, len(design.Playback.RatesPermille))
	for _, rate := range design.Playback.RatesPermille {
		if rate >= RateUnitPermille {
			out = append(out, rate)
			continue
		}
		if !cadence.Verified() {
			continue
		}
		// num/den × rate/1000 ≥ output_fps, without leaving integers.
		if cadence.Numerator*rate >= design.Playback.OutputFPS*RateUnitPermille*cadence.Denominator {
			out = append(out, rate)
		}
	}
	return out
}

// SourceAudioSetting is one source's owner-controlled original-sound retention
// (CLIP-18). It names the exact source identity AND fingerprint it was decided
// for, so a replaced file never inherits a decision made about another one.
type SourceAudioSetting struct {
	SourceID, Fingerprint string
	RetainOriginal        bool
}

// SourceAudioSettings is the complete, server-constructed snapshot of every
// source the plan's cuts draw on. A nil pointer on a plan is a LEGACY absence —
// a plan written before the setting existed, whose audio meaning still lives in
// per-cut volume — and is a different thing from a present snapshot that says
// every source is off. Only that distinction lets migration preserve an existing
// result instead of silencing it (CLIP-101).
type SourceAudioSettings struct{ Values []SourceAudioSetting }

// sourceKey identifies footage the way every audio and overlap rule does: the
// source id together with the fingerprint of the exact file behind it.
type sourceKey struct{ id, fingerprint string }

// cutSourceKeys is in canonical identity order, not cut order: reordering cuts
// changes the timeline, never which sources the owner decided about, so the
// snapshot and the bytes it is stored as stay stable across a reorder.
func cutSourceKeys(cuts []Cut) []sourceKey {
	out := []sourceKey{}
	for _, c := range cuts {
		key := sourceKey{c.SourceID, c.Fingerprint}
		if !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	slices.SortFunc(out, func(a, b sourceKey) int {
		if a.id != b.id {
			return strings.Compare(a.id, b.id)
		}
		return strings.Compare(a.fingerprint, b.fingerprint)
	})
	return out
}

// LegacySourceAudio upgrades a plan written before CLIP-18 to the explicit
// snapshot. A source is on when ANY of its saved cuts carried nonzero original
// volume — including the version-0 cut whose `Volume` was nil, which rendered at
// full original sound. Per-cut volume itself is left exactly as it was: it is an
// independent gain, never a permission (CLIP-101).
func LegacySourceAudio(cuts []Cut) *SourceAudioSettings {
	out := &SourceAudioSettings{}
	for _, key := range cutSourceKeys(cuts) {
		retain := false
		for _, c := range cuts {
			if c.SourceID == key.id && c.Fingerprint == key.fingerprint && c.OriginalVolume() != 0 {
				retain = true
			}
		}
		out.Values = append(out.Values, SourceAudioSetting{key.id, key.fingerprint, retain})
	}
	return out
}

// ReconcileSourceAudio rebuilds the complete snapshot for a plan whose cut set
// changed. Each source keeps the choice the owner already made for it; a source
// that was not in the saved snapshot starts OFF, which is the default CLIP-18
// gives every new source. The server always owns this value — no correction,
// template or model reaches it.
func ReconcileSourceAudio(saved *SourceAudioSettings, cuts []Cut) *SourceAudioSettings {
	out := &SourceAudioSettings{}
	if saved == nil {
		// Nothing has authorized any audio, which is CLIP-18's default. Reading
		// a legacy plan's per-cut volume as consent happens on decode alone.
		saved = &SourceAudioSettings{}
	}
	for _, key := range cutSourceKeys(cuts) {
		retain := false
		for _, v := range saved.Values {
			if v.SourceID == key.id && v.Fingerprint == key.fingerprint {
				retain = v.RetainOriginal
			}
		}
		out.Values = append(out.Values, SourceAudioSetting{key.id, key.fingerprint, retain})
	}
	return out
}

// RetainsOriginalAudio answers the one question the renderer asks per cut: may
// this cut contribute source audio at all? A legacy plan with no snapshot keeps
// its original meaning — nonzero volume was audible — and an explicit snapshot
// is the authority. Per-cut volume is applied AFTER this, never instead of it.
func (p EditPlan) RetainsOriginalAudio(c Cut) bool {
	if p.SourceAudio == nil {
		return c.OriginalVolume() != 0
	}
	for _, v := range p.SourceAudio.Values {
		if v.SourceID == c.SourceID && v.Fingerprint == c.Fingerprint {
			return v.RetainOriginal
		}
	}
	return false
}

// ValidateSourceAudioSettings rejects a snapshot that is not exactly the plan's
// own retained sources: a duplicate entry, a source identity the cuts never use
// (a foreign fingerprint), or a used source the snapshot forgot. An absent
// snapshot is the legacy value and is accepted as such.
func ValidateSourceAudioSettings(plan EditPlan) error {
	if plan.SourceAudio == nil {
		return nil
	}
	seen := map[sourceKey]bool{}
	for _, v := range plan.SourceAudio.Values {
		key := sourceKey{v.SourceID, v.Fingerprint}
		if v.SourceID == "" || v.Fingerprint == "" || seen[key] {
			return planViolation("plan_source_audio")
		}
		seen[key] = true
	}
	required := cutSourceKeys(plan.Cuts)
	if len(seen) != len(required) {
		return planViolation("plan_source_audio")
	}
	for _, key := range required {
		if !seen[key] {
			return planViolation("plan_source_audio")
		}
	}
	return nil
}

// overlapKey names an unordered pair of cuts, so a reorder cannot disguise an
// overlap that was already there as a new one.
type overlapKey struct{ a, b string }

func pairKey(a, b string) overlapKey {
	if a > b {
		a, b = b, a
	}
	return overlapKey{a, b}
}

// SourceOverlaps measures, for every pair of cuts taken from the SAME source
// identity and fingerprint, how many milliseconds of footage they share. Source
// ranges are half-open, so touching endpoints are adjacent rather than
// overlapping and only a positive intersection counts (CLIP-98).
func SourceOverlaps(cuts []Cut) map[overlapKey]int {
	out := map[overlapKey]int{}
	for i, a := range cuts {
		for _, b := range cuts[i+1:] {
			if a.SourceID != b.SourceID || a.Fingerprint != b.Fingerprint {
				continue
			}
			shared := min(a.EndMS, b.EndMS) - max(a.StartMS, b.StartMS)
			if shared > 0 {
				out[pairKey(a.ID, b.ID)] = shared
			}
		}
	}
	return out
}

// ValidateSourceRanges refuses overlapping selections of the same footage.
//
// `grandfathered` is what the SAVED plan already overlapped: a plan written
// before the rule stays readable and renderable exactly as it is, but a
// correction may neither introduce a new overlap nor enlarge an existing one.
// Pass nil for a plan being created, which has nothing to inherit.
func ValidateSourceRanges(plan EditPlan, grandfathered map[overlapKey]int) error {
	for pair, shared := range SourceOverlaps(plan.Cuts) {
		if allowed, ok := grandfathered[pair]; !ok || shared > allowed {
			return planViolation("plan_source_overlap")
		}
	}
	return nil
}

// RefuseUnrenderableRates names the first cut whose fixed rate this footage
// cannot actually be played at. It is CLIP-99's boundary, checked against the
// ORIGINAL's verified cadence — not the analysis copy's re-encoded frame rate
// and not an unverified container declaration — immediately before encoding:
// an unsuitable transform is identified, never simulated and never silently
// changed back to 1x. A cut whose source is absent is refused too, because
// nothing has proved its cadence (CDS-68).
func RefuseUnrenderableRates(plan EditPlan, sources []RenderSource) error {
	byID := map[string]RenderSource{}
	for _, s := range sources {
		byID[s.ID] = s
	}
	for _, c := range plan.Cuts {
		source, ok := byID[c.SourceID]
		if !ok || source.Fingerprint != c.Fingerprint {
			return planViolation("plan_source")
		}
		if !slices.Contains(AllowedPlaybackRates(source.Info), c.Rate()) {
			return planViolation("plan_cut_rate")
		}
	}
	return nil
}

package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxSpokenSegments     = 32
	MaxSpokenSegmentRunes = 500
	MaxSpokenScriptRunes  = 2000
)

// SpokenVoiceBinding is a consumer projection of a confirmed private identity.
// The digest covers the immutable sound, profile and synthesis settings, never
// the editable display name. Supplier handles remain behind the voice port.
type SpokenVoiceBinding struct{ ID, Digest string }
type SpokenVoiceResolver interface {
	ResolveClipVoice(context.Context, string, string) (SpokenVoiceBinding, error)
}

// UnavailableSpokenVoices is an explicit capability denial for deployments and
// test fixtures without a spoken library; ordinary clip editing remains usable.
type UnavailableSpokenVoices struct{}

func (UnavailableSpokenVoices) ResolveClipVoice(context.Context, string, string) (SpokenVoiceBinding, error) {
	return SpokenVoiceBinding{}, ErrCompositionUnavailable
}

type DerivedCaption struct {
	SegmentID                string
	TextRevision             int
	TextEdited, TimingEdited bool
}
type SpeechTiming struct {
	Text           string
	StartMS, EndMS int
}
type SpeechRef struct {
	AssetID, VoiceID, BindingDigest, InputHash, SettingsHash, AudioHash string
	ProfileID                                                           string
	ProfileRevision                                                     int64
	Samples                                                             int64
	SampleRate, Channels                                                int
	Timing                                                              []SpeechTiming
}

func (s SpeechRef) DurationMS() int {
	if s.SampleRate <= 0 {
		return 0
	}
	return int((s.Samples*1000 + int64(s.SampleRate) - 1) / int64(s.SampleRate))
}

type SpokenSegment struct {
	ID, Text, InputHash string
	TextRevision        int
	StartMS, EndMS      int
	Speech              *SpeechRef
	// Creation is request-only. The server mints identities, revisions and hashes.
	Creation bool
}
type NarrationPlan struct {
	Enabled                bool
	VoiceID, BindingDigest string
	VolumePermille         int
	Segments               []SpokenSegment
	RetiredSegments        []SpokenSegment
}

func SpokenInputHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
func cloneNarration(n *NarrationPlan) *NarrationPlan {
	if n == nil {
		return nil
	}
	out := *n
	out.Segments = slices.Clone(n.Segments)
	out.RetiredSegments = slices.Clone(n.RetiredSegments)
	for i := range out.Segments {
		if s := out.Segments[i].Speech; s != nil {
			cp := *s
			cp.Timing = slices.Clone(s.Timing)
			out.Segments[i].Speech = &cp
		}
	}
	return &out
}
func (p EditPlan) SourceGainPermille() int {
	if p.SourceVolumePermille == nil {
		return 1000
	}
	return *p.SourceVolumePermille
}
func ValidateNarration(p EditPlan) error {
	if p.SourceGainPermille() < 0 || p.SourceGainPermille() > 1000 {
		return planViolation("plan_source_gain")
	}
	n := p.Narration
	if n == nil {
		return nil
	}
	if n.VolumePermille < 0 || n.VolumePermille > 1000 || len(n.Segments) > MaxSpokenSegments {
		return planViolation("plan_spoken_limits")
	}
	seen, total := map[string]bool{}, 0
	for _, s := range n.Segments {
		count := utf8.RuneCountInString(s.Text)
		total += count
		if !validSpokenID(s.ID) || seen[s.ID] || s.Creation || s.TextRevision <= 0 || s.InputHash != SpokenInputHash(s.Text) || strings.TrimSpace(s.Text) == "" || count > MaxSpokenSegmentRunes || total > MaxSpokenScriptRunes || s.StartMS < 0 || s.EndMS <= s.StartMS {
			return spokenRefusal(s.ID, "plan_spoken_segment")
		}
		seen[s.ID] = true
		if a := s.Speech; a != nil {
			if a.AssetID == "" || a.VoiceID == "" || !digest(a.BindingDigest) || !digest(a.InputHash) || !digest(a.SettingsHash) || !digest(a.AudioHash) || a.ProfileID == "" || a.ProfileRevision <= 0 || a.SampleRate != 44100 || a.Channels != 2 || a.Samples <= 0 || a.Samples > 13230000 {
				return spokenRefusal(s.ID, "plan_speech_provenance")
			}
			last := 0
			for _, t := range a.Timing {
				if t.Text == "" || t.StartMS < last || t.EndMS <= t.StartMS || t.EndMS > a.DurationMS() {
					return spokenRefusal(s.ID, "plan_speech_timing")
				}
				last = t.EndMS
			}
		}
	}
	return nil
}
func digest(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == sha256.Size }
func validSpokenID(s string) bool {
	rest, ok := strings.CutPrefix(s, "spoken-")
	n, e := strconv.Atoi(rest)
	return ok && e == nil && n > 0 && s == fmt.Sprintf("spoken-%d", n)
}

type SpokenError struct{ SegmentID, Reason string }

func (e *SpokenError) Error() string        { return fmt.Sprintf("%s: %s", e.Reason, e.SegmentID) }
func (e *SpokenError) Unwrap() error        { return ErrInvalid }
func spokenRefusal(id, reason string) error { return &SpokenError{id, reason} }
func CompatibleSpeech(n *NarrationPlan, s SpokenSegment) bool {
	return n != nil && s.Speech != nil && s.Speech.VoiceID == n.VoiceID && s.Speech.BindingDigest == n.BindingDigest && s.Speech.InputHash == s.InputHash
}

// NarrationReadiness is separate from editing validity: a saved draft may carry
// stale audio, but an export must contain every requested sentence in full.
func NarrationReadiness(p EditPlan) error {
	if err := ValidateNarration(p); err != nil {
		return err
	}
	n := p.Narration
	if n == nil || !n.Enabled {
		return nil
	}
	if n.VoiceID == "" || !digest(n.BindingDigest) {
		return spokenRefusal("", "spoken_voice_required")
	}
	if len(n.Segments) == 0 {
		return spokenRefusal("", "spoken_script_required")
	}
	last := 0
	for _, s := range n.Segments {
		if !CompatibleSpeech(n, s) {
			return spokenRefusal(s.ID, "spoken_regeneration_required")
		}
		if s.StartMS < last || s.EndMS > p.DurationMS || s.StartMS+s.Speech.DurationMS() > s.EndMS {
			return spokenRefusal(s.ID, "spoken_timing_conflict")
		}
		last = s.EndMS
	}
	return nil
}

// CorrectNarration accepts editable script/placement fields only. A carried
// audio projection must name exactly the existing reference; it cannot create
// provider evidence or bless a stale asset. The store may later attach a
// compatible retained asset through its own publication transaction.
func CorrectNarration(old EditPlan, in CorrectionPlan, next *EditPlan) error {
	next.Narration = cloneNarration(old.Narration)
	next.SourceVolumePermille = old.SourceVolumePermille
	if in.SourceVolumePermille != nil {
		gain := *in.SourceVolumePermille
		next.SourceVolumePermille = &gain
	}
	if in.Narration == nil {
		return ValidateNarration(*next)
	}
	input := in.Narration
	result := cloneNarration(input)
	result.RetiredSegments = nil
	result.BindingDigest = ""
	known := map[string]SpokenSegment{}
	highest := 0
	if old.Narration != nil {
		for _, s := range append(slices.Clone(old.Narration.Segments), old.Narration.RetiredSegments...) {
			known[s.ID] = s
			num, _ := strconv.Atoi(strings.TrimPrefix(s.ID, "spoken-"))
			highest = max(highest, num)
		}
		result.RetiredSegments = slices.Clone(old.Narration.RetiredSegments)
		if result.VoiceID == old.Narration.VoiceID {
			result.BindingDigest = old.Narration.BindingDigest
		}
	}
	seen := map[string]bool{}
	for i, submitted := range input.Segments {
		prior, exists := known[submitted.ID]
		if submitted.Creation {
			if submitted.ID != "" || submitted.Speech != nil {
				return spokenRefusal(submitted.ID, "spoken_identity")
			}
			highest++
			prior = SpokenSegment{ID: fmt.Sprintf("spoken-%d", highest), TextRevision: 1}
		} else if !exists || seen[submitted.ID] {
			return spokenRefusal(submitted.ID, "spoken_identity")
		}
		seen[prior.ID] = true
		if submitted.Speech != nil && !reflect.DeepEqual(submitted.Speech, prior.Speech) {
			return spokenRefusal(prior.ID, "spoken_asset_identity")
		}
		if exists && prior.Text != submitted.Text {
			prior.TextRevision++
		}
		prior.Text, prior.StartMS, prior.EndMS = submitted.Text, submitted.StartMS, submitted.EndMS
		prior.InputHash, prior.Creation = SpokenInputHash(prior.Text), false
		result.Segments[i] = prior
	}
	if old.Narration != nil {
		for _, s := range old.Narration.Segments {
			if !seen[s.ID] {
				result.RetiredSegments = append(result.RetiredSegments, s)
			}
		}
	}
	result.RetiredSegments = slices.DeleteFunc(result.RetiredSegments, func(s SpokenSegment) bool { return seen[s.ID] })
	next.Narration = result
	return ValidateNarration(*next)
}

// PublishSpeech rejects a late revision even when other segments still match.
func PublishSpeech(p EditPlan, expectedRevision, currentRevision int, id, hash, binding string, audio SpeechRef) (EditPlan, error) {
	if expectedRevision <= 0 || expectedRevision != currentRevision || p.Narration == nil || p.Narration.BindingDigest != binding {
		return EditPlan{}, ErrPlanConflict
	}
	p.Narration = cloneNarration(p.Narration)
	for i, s := range p.Narration.Segments {
		if s.ID == id {
			if s.InputHash != hash || audio.InputHash != hash || audio.BindingDigest != binding || audio.VoiceID != p.Narration.VoiceID {
				return EditPlan{}, ErrPlanConflict
			}
			p.Narration.Segments[i].Speech = &audio
			return p, ValidateNarration(p)
		}
	}
	return EditPlan{}, ErrPlanConflict
}

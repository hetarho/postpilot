package clip

import (
	"fmt"
	"github.com/postpilot/backend/internal/clip/design"
	"strings"
	"unicode/utf8"
)

type SpokenDraft struct {
	Version   int
	Narration NarrationPlan
	Storyline *Storyline
}

func NewSpokenDraft(lines []string, storyline *Storyline) (SpokenDraft, error) {
	d := SpokenDraft{Version: 1, Storyline: storyline, Narration: NarrationPlan{Enabled: true, VolumePermille: 1000}}
	for i, text := range lines {
		d.Narration.Segments = append(d.Narration.Segments, SpokenSegment{ID: fmt.Sprintf("spoken-%d", i+1), Text: text, InputHash: SpokenInputHash(text), TextRevision: 1, StartMS: i, EndMS: i + 1})
	}
	if len(lines) == 0 {
		return d, &SpokenError{Reason: "spoken_script_required"}
	}

	return d, ValidateNarration(EditPlan{Narration: &d.Narration})
}

// MeasuredSpokenDraft places every complete recording at natural speed. Padding is silent.
func MeasuredSpokenDraft(d *SpokenDraft, intro, outro, target int) (int, error) {
	cursor := intro
	for i := range d.Narration.Segments {
		s := &d.Narration.Segments[i]
		if !CompatibleSpeech(&d.Narration, *s) {
			return 0, spokenRefusal(s.ID, "spoken_regeneration_required")
		}
		s.StartMS = cursor
		s.EndMS = cursor + s.Speech.DurationMS()
		cursor = s.EndMS
	}
	duration := max(15000, cursor+outro)
	if target < 15000 || target > 60000 || duration > target {
		return 0, spokenRefusal("", "spoken_timing_conflict")
	}
	return duration, nil
}

// InitialSpokenCaptions are independently editable display text. Short recordings may
// omit unreadable captions; the full spoken input and asset remain untouched.
func InitialSpokenCaptions(p *EditPlan) {
	if p.Portable == nil || p.Narration == nil {
		return
	}
	seen := map[string]bool{}
	for _, t := range p.Portable.Elements {
		seen[t.Resolved.InstanceID] = true
	}
	next := 1
	for _, s := range p.Narration.Segments {
		size := max(1, design.Caption().Lines*design.Caption().Chars)
		count := (utf8.RuneCountInString(s.Text) + size - 1) / size
		words := CaptionWords(s.Text, count)
		exact := false
		if s.Speech != nil && len(s.Speech.Timing) == utf8.RuneCountInString(s.Text) {
			var text strings.Builder
			for _, t := range s.Speech.Timing {
				text.WriteString(t.Text)
			}
			exact = text.String() == s.Text
		}
		cursor := 0
		for i, text := range words {
			start := s.StartMS + (s.EndMS-s.StartMS)*i/count
			end := s.StartMS + (s.EndMS-s.StartMS)*(i+1)/count
			if exact {
				runes := []rune(s.Text)
				for cursor < len(runes) && strings.TrimSpace(string(runes[cursor])) == "" {
					cursor++
				}
				n := utf8.RuneCountInString(text)
				if n > 0 && cursor+n <= len(s.Speech.Timing) {
					start = s.StartMS + s.Speech.Timing[cursor].StartMS
					end = s.StartMS + s.Speech.Timing[cursor+n-1].EndMS
				}
				cursor += n
			}
			for seen[NarrationID(next)] {
				next++
			}
			id := NarrationID(next)
			next++
			if text == "" || end-start < MinExposureMS(text) || len(p.Portable.Elements) >= 100 {
				AddPlanNotice(p, NoticeCaptionFloor, "", id, "omitted")
				continue
			}
			t := NarrationCaption(id, text, start, end)
			t.Derived = &DerivedCaption{SegmentID: s.ID, TextRevision: s.TextRevision}
			p.Portable.Elements = append(p.Portable.Elements, t)
			seen[id] = true
		}
	}
}

package clip

import (
	"strings"

	"github.com/postpilot/backend/internal/clip/design"
	"github.com/rivo/uniseg"
)

func ValidCaptionPace(pace string) bool { return design.ValidPace(pace) }

func (c Cut) Rapid() bool {
	if len(c.Copies) == 0 {
		return false
	}
	for _, copy := range c.Copies {
		if copy.Pace != "rapid" {
			return false
		}
	}
	return true
}

// RapidPhrases keeps word boundaries where possible and only breaks an oversized
// word at extended grapheme boundaries. No wording or provider call is added.
func RapidPhrases(text string) []string {
	var words []string
	for _, word := range strings.Fields(text) {
		part := ""
		g := uniseg.NewGraphemes(word)
		for g.Next() {
			if design.Chars(part+g.Str()) > design.Rapid.MaxChars {
				words = append(words, part)
				part = ""
			}
			part += g.Str()
		}
		if part != "" {
			words = append(words, part)
		}
	}
	var phrases []string
	for _, word := range words {
		n := len(phrases)
		if n > 0 && !(n == 1 && design.Chars(phrases[0]) <= design.Rapid.ShortChars) && design.Chars(phrases[n-1]+word) <= design.Rapid.TargetChars &&
			!strings.ContainsAny(phrases[n-1][len(phrases[n-1])-1:], ".!?,;:") {
			phrases[n-1] += " " + word
		} else {
			phrases = append(phrases, word)
		}
	}
	return phrases
}

// SplitRapid returns explicit, touching cue windows. It never truncates the
// sentence to fit; the caller may try the approved short text or sentence mode.
func SplitRapid(seed Caption, start, end int) ([]Caption, bool) {
	phrases := RapidPhrases(seed.Text)
	if len(phrases) == 0 || len(phrases) > design.Rapid.MaxPerCut || start < 0 || end-start < len(phrases)*design.Rapid.MinMS {
		return nil, false
	}
	durations, total := make([]int, len(phrases)), 0
	for i, phrase := range phrases {
		durations[i] = design.PhraseDuration(design.Chars(phrase))
		total += durations[i]
	}
	room, minimum := end-start, len(phrases)*design.Rapid.MinMS
	var out []Caption
	for i, phrase := range phrases {
		duration := durations[i]
		if total > room {
			duration = design.Rapid.MinMS + (duration-design.Rapid.MinMS)*(room-minimum)/(total-minimum)
		}
		copy := seed
		copy.Text, copy.Pace = phrase, "rapid"
		copy.Keyword = keywordIn(phrase, seed.Keyword)
		copy.StartMS, copy.EndMS = start, start+duration
		out = append(out, copy)
		start += duration
	}
	return out, true
}

func composeRapid(cut Cut, written Written, text, accent string, allowed []string, placed Manifest, subject Region, readable bool, previous string, measure func(Caption) (Region, bool, error)) (Cut, bool, error) {
	style := design.SelectStyle("", design.ClassDesc, allowed, nil, 0)
	// A rapid run keeps one drawing/anchor for all of its phrases.
	if style != "simple" {
		style = "clean"
	}
	rule := design.Styles[style]
	for _, value := range []string{text, strings.TrimSpace(written.ShortText)} {
		if value == "" || !Grounded(value, written.Answers) {
			continue
		}
		copies, ok := SplitRapid(Caption{Text: value, Anchor: rule.Anchor, Align: rule.Align, Style: style, Accent: accent},
			design.Timing.CopyLeadMS, cut.EndMS-cut.StartMS-design.Timing.CopyLeadMS)
		if !ok {
			continue
		}
		fits := true
		for _, copy := range copies {
			bounds, ok, err := measure(copy)
			if err != nil {
				return cut, false, err
			}
			candidate := design.Candidate{Anchor: copy.Anchor, Align: copy.Align, Plate: design.Region(bounds), Fits: ok}
			if design.SelectAnchor([]design.Candidate{candidate}, design.Region(subject), placed, readable, previous) < 0 {
				fits = false
				break
			}
		}
		if fits {
			cut.Copies = copies
			return cut, true, nil
		}
	}
	return cut, false, nil
}

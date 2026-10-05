package clip

import (
	"slices"
	"sort"
	"strings"
	"unicode"
)

// CaptionWords partitions display wording without modifying spoken input or timing.
// Rune boundaries keep the browser and server consistent for Korean and emoji.
func CaptionWords(text string, count int) []string {
	if count <= 0 {
		return nil
	}
	runes := []rune(text)
	out := make([]string, count)
	start := 0
	for i := range count {
		end := len(runes) * (i + 1) / count
		if i < count-1 {
			for j := end; j < len(runes); j++ {
				if unicode.IsSpace(runes[j]) {
					end = j
					break
				}
			}
		}
		if end < start {
			end = start
		}
		out[i] = strings.TrimSpace(string(runes[start:end]))
		start = end
	}
	return out
}

// RefreshCaptionWords returns only eligible wording. Missing origins stay archived;
// user-authored wording and every interval/appearance property remain unchanged.
func RefreshCaptionWords(plan EditPlan) map[string]string {
	out := map[string]string{}
	if plan.Narration == nil || plan.Portable == nil {
		return out
	}
	for _, segment := range plan.Narration.Segments {
		var captions []PortableText
		for _, text := range plan.Portable.Elements {
			if text.Derived != nil && text.Derived.SegmentID == segment.ID {
				captions = append(captions, text)
			}
		}
		sort.SliceStable(captions, func(i, j int) bool { return captions[i].Resolved.StartMS < captions[j].Resolved.StartMS })
		words := CaptionWords(segment.Text, len(captions))
		for i, text := range captions {
			phraseWords := CaptionWords(words[i], len(text.Phrases))
			if slices.Contains(phraseWords, "") {
				continue
			}
			if !text.Derived.TextEdited && text.Derived.TextRevision != segment.TextRevision && words[i] != "" {
				out[text.Resolved.InstanceID] = words[i]
			}
		}
	}
	return out
}

func phraseTimesEqual(a, b []EditablePhrase) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].StartMS != b[i].StartMS || a[i].EndMS != b[i].EndMS {
			return false
		}
	}
	return true
}
func phraseWordsEqual(a, b []EditablePhrase) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Text != b[i].Text {
			return false
		}
	}
	return true
}
func refreshPhrasesEqual(expected string, old, edited []EditablePhrase) bool {
	if !phraseTimesEqual(old, edited) {
		return false
	}
	words := CaptionWords(expected, len(old))
	for i := range edited {
		if edited[i].Text != words[i] {
			return false
		}
	}
	return true
}

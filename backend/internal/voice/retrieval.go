package voice

import (
	"fmt"
	"strings"
)

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func excerptAroundTarget(body string, target, limit int) string {
	body = strings.TrimSpace(body)
	if target <= 0 || limit < target {
		return firstRunes(body, limit)
	}
	runes := []rune(body)
	if len(runes) <= target {
		return body
	}
	end := min(limit, len(runes))
	for i := target - 1; i < end; i++ {
		if strings.ContainsRune(".!?。\n", runes[i]) {
			return strings.TrimSpace(string(runes[:i+1]))
		}
	}
	return strings.TrimSpace(string(runes[:end]))
}

// A measurement only a corpus can produce, rendered the way an unmeasured axis is. Zero is
// never a real average sentence length or paragraph size, and a seeded profile (written from a
// description, with nothing measured) would otherwise state "0.00 chars" as a fact.
func renderChars(value float64) string {
	if value <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("%.2f chars", value)
}

func renderParagraphSentences(min, max int) string {
	if min <= 0 && max <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d-%d", min, max)
}

func renderValue(value VoiceValue) string {
	if value.Unknown || strings.TrimSpace(value.Value) == "" {
		return "unknown"
	}
	return value.Value + " (" + string(value.Source) + ")"
}

// An unmeasured axis renders as "unknown" rather than a fabricated 0 — printing a neutral the
// model never claimed into the generation prompt would be the same bug one layer down.
func renderAxes(a AxesProfile) string {
	parts := make([]string, 0, 6)
	for _, axis := range a.AxisValues() {
		if axis.Value == nil {
			parts = append(parts, axis.Key+"=unknown")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%d", axis.Key, *axis.Value))
	}
	return strings.Join(parts, " ")
}
func renderStructuredProfile(p StructuredProfile) string {
	if p.Empty || p.Version == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Structured voice profile v%d]\n[Lexical]\n%s", p.Version, renderValue(p.Lexical.Description))
	renderPreferredWords(&b, p.Lexical.PreferredWords)
	fmt.Fprintf(&b, "\n[Primary endings]\nregister: %s\ndistribution:", renderValue(p.Endings.BaseRegister))
	for _, ratio := range p.Endings.Distribution {
		fmt.Fprintf(&b, " %s=%.2f", ratio.Ending, ratio.Ratio)
	}
	renderEndingLists(&b, p.Endings)
	fmt.Fprintf(&b, "\n[Syntax]\naverage sentence: %s\nsentence length: %s\nconnectives: %s", renderChars(p.Syntax.AverageSentenceChars), renderValue(p.Syntax.SentenceLength), renderValue(p.Syntax.ConnectiveStyle))
	if len(p.Syntax.PreferredConnectives) > 0 {
		b.WriteString("\npreferred connectives: " + strings.Join(p.Syntax.PreferredConnectives, ", "))
	}
	fmt.Fprintf(&b, "\nnominalization: %s\npassive: %s", renderValue(p.Syntax.Nominalization), renderValue(p.Syntax.PassiveTendency))
	fmt.Fprintf(&b, "\n[Structure]\nintro: %s\nclosing: %s\nparagraph sentences: %s\nheadings: %s\nlists: %s\nemojis: %s\n[Axes]\n%s", renderValue(p.Structure.IntroPattern), renderValue(p.Structure.ClosingPattern), renderParagraphSentences(p.Structure.ParagraphSentencesMin, p.Structure.ParagraphSentencesMax), renderValue(p.Structure.HeadingHabit), renderValue(p.Structure.ListHabit), renderValue(p.Structure.EmojiUse), renderAxes(p.Axes))
	if len(p.Lexical.BannedWords) > 0 {
		b.WriteString("\n[Banned words]")
		for _, item := range p.Lexical.BannedWords {
			fmt.Fprintf(&b, "\n- %s: %s", item.Value, item.Reason)
		}
	}
	if len(p.Lexical.BannedPatterns) > 0 {
		b.WriteString("\n[Banned patterns]")
		for _, item := range p.Lexical.BannedPatterns {
			fmt.Fprintf(&b, "\n- %s: %s", item.Value, item.Reason)
		}
	}
	if len(p.Endings.BannedEndings) > 0 {
		b.WriteString("\n[Banned endings]\n" + strings.Join(p.Endings.BannedEndings, ", "))
	}
	return b.String()
}

// renderPreferredWords lists the words the voice reaches for, each with the alternatives it
// uses in their place, when the profile holds any (VOICE-46).
func renderPreferredWords(b *strings.Builder, words []WeightedWord) {
	if len(words) == 0 {
		return
	}
	parts := make([]string, 0, len(words))
	for _, word := range words {
		if len(word.Alternatives) > 0 {
			parts = append(parts, word.Word+" (→"+strings.Join(word.Alternatives, "/")+")")
			continue
		}
		parts = append(parts, word.Word)
	}
	b.WriteString("\npreferred words: " + strings.Join(parts, ", "))
}

// renderEndingLists adds the ending signatures and constraints the profile holds.
func renderEndingLists(b *strings.Builder, endings EndingsProfile) {
	if len(endings.SignatureEndings) > 0 {
		b.WriteString("\nsignatures: " + strings.Join(endings.SignatureEndings, ", "))
	}
	if len(endings.Constraints) > 0 {
		b.WriteString("\nconstraints: " + strings.Join(endings.Constraints, "; "))
	}
}

func renderStructuredProfileForLanguage(p StructuredProfile, language Language) string {
	if language != LanguageEnglish {
		return renderStructuredProfile(p)
	}
	if p.Empty || p.Version == 0 {
		return ""
	}
	averageWords := "unknown"
	if p.Syntax.AverageSentenceWords != nil && *p.Syntax.AverageSentenceWords > 0 {
		averageWords = fmt.Sprintf("%.2f", *p.Syntax.AverageSentenceWords)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Structured English voice profile v%d]\n[Lexical]\n%s", p.Version, renderValue(p.Lexical.Description))
	renderPreferredWords(&b, p.Lexical.PreferredWords)
	fmt.Fprintf(&b, "\n[Register and cadence]\nregister: %s\ncadence:", renderValue(p.Endings.BaseRegister))
	for _, ratio := range p.Endings.Distribution {
		fmt.Fprintf(&b, " %s=%.2f", ratio.Ending, ratio.Ratio)
	}
	fmt.Fprintf(&b, "\n[Syntax]\naverage sentence: %s / %s words\nconnectives: %s\npreferred connectives: %s\npassive: %s\nnominalization: %s", renderChars(p.Syntax.AverageSentenceChars), averageWords, renderValue(p.Syntax.ConnectiveStyle), strings.Join(p.Syntax.PreferredConnectives, ", "), renderValue(p.Syntax.PassiveTendency), renderValue(p.Syntax.Nominalization))
	fmt.Fprintf(&b, "\n[Structure]\nintro: %s\nclosing: %s\nparagraph sentences: %s\nheadings: %s\nlists: %s\nemojis: %s\n[Axes]\n%s", renderValue(p.Structure.IntroPattern), renderValue(p.Structure.ClosingPattern), renderParagraphSentences(p.Structure.ParagraphSentencesMin, p.Structure.ParagraphSentencesMax), renderValue(p.Structure.HeadingHabit), renderValue(p.Structure.ListHabit), renderValue(p.Structure.EmojiUse), renderAxes(p.Axes))
	return b.String()
}

// renderPortableProfile is intentionally a separate allowlist, not a redaction pass over
// the full rendering. Adding a new source-language field to the full profile therefore
// cannot make it cross languages by accident.
func renderPortableProfile(p StructuredProfile) string {
	if p.Empty || p.Version == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Portable voice structure v%d]\n", p.Version)
	fmt.Fprintf(&b, "intro: %s\nclosing: %s\n", renderValue(p.Structure.IntroPattern), renderValue(p.Structure.ClosingPattern))
	fmt.Fprintf(&b, "paragraph sentences: %s\n", renderParagraphSentences(p.Structure.ParagraphSentencesMin, p.Structure.ParagraphSentencesMax))
	fmt.Fprintf(&b, "headings: %s\nlists: %s\nemojis: %s\n", renderValue(p.Structure.HeadingHabit), renderValue(p.Structure.ListHabit), renderValue(p.Structure.EmojiUse))
	fmt.Fprintf(&b, "[Portable axes]\n%s", renderAxes(p.Axes))
	return b.String()
}

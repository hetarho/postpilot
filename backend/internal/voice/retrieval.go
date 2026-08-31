package voice

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func topicTokens(topic string, tags []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range append(strings.FieldsFunc(strings.ToLower(topic), func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) }), tags...) {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			out[part] = struct{}{}
		}
	}
	return out
}
func rankExcerpts(sources []AuthoredSource, topic string, tags []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	tokens := topicTokens(topic, tags)
	type scored struct {
		s     AuthoredSource
		score int
	}
	rows := make([]scored, 0, len(sources))
	for _, source := range sources {
		score := 0
		for _, tag := range source.Tags {
			if _, ok := tokens[strings.ToLower(tag)]; ok {
				score += 3
			}
		}
		for token := range tokens {
			if strings.Contains(strings.ToLower(source.Title), token) {
				score++
			}
		}
		rows = append(rows, scored{s: source, score: score})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		if !rows[i].s.CreatedAt.Equal(rows[j].s.CreatedAt) {
			return rows[i].s.CreatedAt.After(rows[j].s.CreatedAt)
		}
		return rows[i].s.ID < rows[j].s.ID
	})
	out := make([]string, 0, min(limit, len(rows)))
	for _, row := range rows {
		if row.s.Excerpt != "" && !containsString(out, row.s.Excerpt) {
			out = append(out, row.s.Excerpt)
		}
		if len(out) == limit {
			break
		}
	}
	return out
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
	fmt.Fprintf(&b, "[Structured voice profile v%d]\n[Lexical]\n%s\n[Endings — primary]\nregister: %s\ndistribution:", p.Version, renderValue(p.Lexical.Description), renderValue(p.Endings.BaseRegister))
	for _, ratio := range p.Endings.Distribution {
		fmt.Fprintf(&b, " %s=%.2f", ratio.Ending, ratio.Ratio)
	}
	fmt.Fprintf(&b, "\n[Syntax]\naverage sentence: %.2f chars\nconnectives: %s\n[Structure]\nintro: %s\nclosing: %s\n[Axes]\n%s", p.Syntax.AverageSentenceChars, renderValue(p.Syntax.ConnectiveStyle), renderValue(p.Structure.IntroPattern), renderValue(p.Structure.ClosingPattern), renderAxes(p.Axes))
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

func renderStructuredProfileForLanguage(p StructuredProfile, language Language) string {
	if language != LanguageEnglish {
		return renderStructuredProfile(p)
	}
	if p.Empty || p.Version == 0 {
		return ""
	}
	averageWords := "unknown"
	if p.Syntax.AverageSentenceWords != nil {
		averageWords = fmt.Sprintf("%.2f", *p.Syntax.AverageSentenceWords)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Structured English voice profile v%d]\n[Lexical]\n%s\n[Register and cadence]\nregister: %s\ncadence:", p.Version, renderValue(p.Lexical.Description), renderValue(p.Endings.BaseRegister))
	for _, ratio := range p.Endings.Distribution {
		fmt.Fprintf(&b, " %s=%.2f", ratio.Ending, ratio.Ratio)
	}
	fmt.Fprintf(&b, "\n[Syntax]\naverage sentence: %.2f chars / %s words\nconnectives: %s\npreferred connectives: %s\npassive: %s\nnominalization: %s", p.Syntax.AverageSentenceChars, averageWords, renderValue(p.Syntax.ConnectiveStyle), strings.Join(p.Syntax.PreferredConnectives, ", "), renderValue(p.Syntax.PassiveTendency), renderValue(p.Syntax.Nominalization))
	fmt.Fprintf(&b, "\n[Structure]\nintro: %s\nclosing: %s\nparagraph sentences: %d-%d\nheadings: %s\nlists: %s\nemojis: %s\n[Axes]\n%s", renderValue(p.Structure.IntroPattern), renderValue(p.Structure.ClosingPattern), p.Structure.ParagraphSentencesMin, p.Structure.ParagraphSentencesMax, renderValue(p.Structure.HeadingHabit), renderValue(p.Structure.ListHabit), renderValue(p.Structure.EmojiUse), renderAxes(p.Axes))
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
	fmt.Fprintf(&b, "paragraph sentences: %d-%d\n", p.Structure.ParagraphSentencesMin, p.Structure.ParagraphSentencesMax)
	fmt.Fprintf(&b, "headings: %s\nlists: %s\nemojis: %s\n", renderValue(p.Structure.HeadingHabit), renderValue(p.Structure.ListHabit), renderValue(p.Structure.EmojiUse))
	fmt.Fprintf(&b, "[Portable axes]\n%s", renderAxes(p.Axes))
	return b.String()
}

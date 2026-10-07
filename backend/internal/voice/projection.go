package voice

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// PromptProfile is a voice projected for one target (VOICE-46): the section text as the writer
// receives it, and the excerpts from the 학습 글 a Korean target also gets.
type PromptProfile struct {
	Text            string
	Excerpts        []string
	Portable        bool
	AcceptedSources []AcceptedSource
}

// PromptProfileForTopic projects one made voice's current analysis (VOICE-46, VOICE-47). A Korean
// target receives every known counted item as a plain Korean sentence, the AI part and up to
// FewShotMax excerpts — topic matches first, then newest — never from excludeMaterialID; any
// other target receives the portable habits alone. It never falls back to another voice.
func (s *Service) PromptProfileForTopic(ctx context.Context, userID, voiceID, retrievalText string, target Language, excludeMaterialID string) (PromptProfile, error) {
	if !target.Valid() {
		return PromptProfile{}, ErrLanguageRequired
	}
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return PromptProfile{}, err
	}
	analysis, err := s.analyses.CurrentAnalysis(ctx, userID, voiceID)
	if err != nil {
		return PromptProfile{}, fmt.Errorf("current analysis: %w", err)
	}
	if analysis == nil {
		return PromptProfile{}, ErrVoiceNotMade
	}
	if target != LanguageKorean {
		text := portableSection(analysis.Counted)
		if voiceOrigin := NormalizedOrigin(analysis.Origin); voiceOrigin == OriginSynthetic {
			text = strings.Replace(text, "These habits were counted from the writer's own Korean posts; apply them to this English post.", "These habits were counted from an AI-created fictional illustration, not this writer's personal writing; apply the style to this English post without copying any facts or phrases.", 1)
		}
		return PromptProfile{Text: text, Portable: true}, nil
	}
	if NormalizedOrigin(analysis.Origin) == OriginSynthetic {
		return PromptProfile{Text: koreanSection(*analysis) + "\n[AI가 만든 가상의 말투 예시]\n" + analysis.SyntheticSample, Excerpts: []string{}}, nil
	}
	samples, err := s.samples.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return PromptProfile{}, fmt.Errorf("list excerpts: %w", err)
	}
	visible := withoutDeletedExamples(*analysis, samplePresence(samples))
	excerpts, sources := excerptsWithSources(acceptedSamples(*analysis, samples), retrievalText, excludeMaterialID)
	// Examples are part of the accepted profile even when the excerpt window
	// chooses another material. Keep their withdrawal fence in the frozen input.
	needed := make(map[string]bool)
	for _, example := range []Example{visible.Counted.Endings.Example, visible.Counted.Marks.Example, visible.Counted.Emoji.Example, visible.Counted.Shape.Example, visible.Counted.OpenClose.Example, visible.Counted.Adverbs.Example, visible.Counted.Person.Example, visible.Counted.Headings.Example} {
		if example.MaterialID != "" {
			needed[example.MaterialID] = true
		}
	}
	for _, example := range visible.AI.Examples {
		needed[example.MaterialID] = true
	}
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		seen[source.SampleID] = true
	}
	if analysis.SourceVersionsKnown {
		for _, source := range analysis.AcceptedSources {
			if needed[source.SampleID] && !seen[source.SampleID] {
				sources = append(sources, source)
				seen[source.SampleID] = true
			}
		}
	}
	return PromptProfile{Text: koreanSection(visible), Excerpts: excerpts, AcceptedSources: sources}, nil
}

// koreanSection is the `[말투]` section: every known item as a sentence with its value, then the
// AI part. An unknown item has no line; a real "none" says so.
func koreanSection(analysis Analysis) string {
	var b strings.Builder
	if NormalizedOrigin(analysis.Origin) == OriginSynthetic {
		b.WriteString("[말투]\n아래는 사용자가 고른 AI 생성 스타일의 가상 예시에서 센 습관입니다. 사용자가 직접 쓴 글이나 실제 경험이 아닙니다. 말투만 따르고 예시의 표현이나 사실은 베끼지 마세요.")
	} else {
		b.WriteString("[말투]\n아래는 이 글쓴이가 직접 쓴 글에서 센 습관입니다. 이 습관대로 쓰고, 발췌한 문장의 표현이나 사실은 베끼지 마세요.")
	}
	for _, line := range countedLines(analysis.Counted) {
		b.WriteString("\n" + line)
	}
	ai := analysis.AI
	if ai.Impression != "" {
		b.WriteString("\n- 인상: " + ai.Impression)
	}
	for _, tic := range ai.Tics {
		if tic.When != "" {
			fmt.Fprintf(&b, "\n- 말버릇: '%s' — %s", tic.Phrase, tic.When)
		} else {
			fmt.Fprintf(&b, "\n- 말버릇: '%s'", tic.Phrase)
		}
	}
	if len(ai.SignaturePhrases) > 0 {
		b.WriteString("\n- 이 사람만의 표현: " + strings.Join(ai.SignaturePhrases, ", "))
	}
	return b.String()
}

// countedLines is each known counted item as one plain Korean line — the projection's lines and
// the analysis call's counted summary alike.
func countedLines(f Fingerprint) []string {
	var lines []string
	if e := f.Endings; !e.Unknown {
		line := fmt.Sprintf("- 문장 끝: '~다' %s, '~해요' %s, '~습니다' %s, 그 밖의 끝맺음 %s. 이 비율을 따르세요.", pct(e.Da), pct(e.Haeyo), pct(e.Seumnida), pct(e.Other))
		if len(e.Suffixes) > 0 {
			parts := make([]string, 0, len(e.Suffixes))
			for _, suffix := range e.Suffixes {
				parts = append(parts, "~"+suffix.Text)
			}
			line += " 자주 쓰는 끝맺음: " + strings.Join(parts, ", ") + "."
		}
		lines = append(lines, line)
	}
	if m := f.Marks; !m.Unknown {
		lines = append(lines, fmt.Sprintf("- 문장 끝 부호: 느낌표 %s, 물음표 %s, 물결표 %s, 말줄임표 %s, 마침표 %s, 부호 없음 %s. 같은 부호를 두 번 이상 겹쳐 쓰는 문장 %s.",
			pct(m.Exclaim), pct(m.Question), pct(m.Tilde), pct(m.Ellipsis), pct(m.Period), pct(m.None), pct(m.Repeat)))
	}
	if e := f.Emoji; !e.Unknown {
		lines = append(lines, fmt.Sprintf("- 이모지와 자모: 100문장마다 이모지 %s개, ㅎㅎ %s번, ㅋㅋ %s번, ㅠㅠ %s번.", count(e.Emoji), count(e.Hh), count(e.Kk), count(e.Tears)))
	}
	if sh := f.Shape; !sh.Unknown {
		line := fmt.Sprintf("- 문장과 문단: 문장은 평균 %s자, 문단마다 %s(평균 %s문장)", count(sh.AverageChars), paragraphRange(sh), decimal(sh.ParagraphAverage))
		if sh.OwnLine {
			line += "이고 문장마다 줄을 바꿉니다."
		} else {
			line += "입니다. 문장을 여러 개 이어 한 줄에 씁니다."
		}
		lines = append(lines, line)
	}
	if o := f.OpenClose; !o.Unknown {
		var parts []string
		if len(o.Openings) > 0 {
			parts = append(parts, "여는 줄: "+quoted(o.Openings))
		}
		if len(o.Closings) > 0 {
			parts = append(parts, "닫는 줄: "+quoted(o.Closings))
		}
		lines = append(lines, "- "+strings.Join(parts, " / "))
	}
	if a := f.Adverbs; !a.Unknown {
		if a.None {
			lines = append(lines, "- 자주 쓰는 말: 눈에 띄게 반복하는 부사는 없습니다.")
		} else {
			parts := make([]string, 0, len(a.Words))
			for i, word := range a.Words {
				if i == 0 {
					parts = append(parts, fmt.Sprintf("%s(100문장마다 %s번)", word.Word, count(word.PerHundred)))
				} else {
					parts = append(parts, fmt.Sprintf("%s(%s번)", word.Word, count(word.PerHundred)))
				}
			}
			lines = append(lines, "- 자주 쓰는 말: "+strings.Join(parts, ", ")+".")
		}
	}
	if p := f.Person; !p.Unknown {
		if p.Dominant == "" {
			lines = append(lines, "- 1인칭: 1인칭을 거의 쓰지 않습니다.")
		} else {
			lines = append(lines, fmt.Sprintf("- 1인칭: '%s'(100문장마다 %s번).", p.Dominant, count(personRate(p))))
		}
	}
	if h := f.Headings; !h.Unknown {
		var parts []string
		if h.Count > 0 {
			parts = append(parts, fmt.Sprintf("소제목의 %s가 이모지로 시작하고 %s가 질문형입니다.", pct(h.EmojiShare), pct(h.QuestionShare)))
		}
		if h.Marker != "" {
			parts = append(parts, fmt.Sprintf("목록은 '%s'로 씁니다.", h.Marker))
		}
		lines = append(lines, "- 소제목과 목록: "+strings.Join(parts, " "))
	}
	return lines
}

// portableSection is what crosses into another language (LANG-15): the marks, the emoji rate,
// the paragraph shape and the heading habits — no Korean ending, word or excerpt.
func portableSection(f Fingerprint) string {
	var b strings.Builder
	b.WriteString("[Portable voice habits]\nThese habits were counted from the writer's own Korean posts; apply them to this English post.")
	if m := f.Marks; !m.Unknown {
		fmt.Fprintf(&b, "\n- Sentence-final marks: exclamation marks %s, question marks %s, tildes %s, ellipses %s, periods %s, none %s; %s of sentences double a mark.",
			pct(m.Exclaim), pct(m.Question), pct(m.Tilde), pct(m.Ellipsis), pct(m.Period), pct(m.None), pct(m.Repeat))
	}
	if e := f.Emoji; !e.Unknown {
		fmt.Fprintf(&b, "\n- Emoji: %s per 100 sentences.", count(e.Emoji))
	}
	if sh := f.Shape; !sh.Unknown {
		lines := "sentences run together on one line."
		if sh.OwnLine {
			lines = "every sentence on its own line."
		}
		fmt.Fprintf(&b, "\n- Paragraphs: %s sentences each (%s on average); %s", paragraphSpan(sh), decimal(sh.ParagraphAverage), lines)
	}
	if h := f.Headings; !h.Unknown {
		var parts []string
		if h.Count > 0 {
			parts = append(parts, fmt.Sprintf("%s of headings start with an emoji and %s are questions", pct(h.EmojiShare), pct(h.QuestionShare)))
		}
		if h.Marker != "" {
			parts = append(parts, fmt.Sprintf("lists use '%s'", h.Marker))
		}
		fmt.Fprintf(&b, "\n- Headings and lists: %s.", strings.Join(parts, "; "))
	}
	return b.String()
}

// excerptsFor picks up to FewShotMax excerpts from the 학습 글's prose: those whose text, label or
// prompt holds a topic token first, then the newest (VOICE-46).
func excerptsFor(newestFirst []Sample, retrievalText, excludeMaterialID string) []string {
	excerpts, _ := excerptsWithSources(newestFirst, retrievalText, excludeMaterialID)
	return excerpts
}

func excerptsWithSources(newestFirst []Sample, retrievalText, excludeMaterialID string) ([]string, []AcceptedSource) {
	tokens := topicTokens(retrievalText)
	type candidate struct {
		sample  Sample
		matches bool
		order   int
	}
	candidates := make([]candidate, 0, len(newestFirst))
	for i, sample := range newestFirst {
		if sample.ID == excludeMaterialID {
			continue
		}
		haystack := sample.Body + "\n" + sample.Title()
		matches := false
		for _, token := range tokens {
			if strings.Contains(haystack, token) {
				matches = true
				break
			}
		}
		candidates = append(candidates, candidate{sample: sample, matches: matches, order: i})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].matches != candidates[j].matches {
			return candidates[i].matches
		}
		return candidates[i].order < candidates[j].order
	})
	excerpts := make([]string, 0, FewShotMax)
	sources := make([]AcceptedSource, 0, FewShotMax)
	for _, found := range candidates {
		if len(excerpts) == FewShotMax {
			break
		}
		prose := ProseText(found.sample.Body)
		if prose == "" {
			continue
		}
		excerpt := excerptAroundTarget(prose, FewShotExcerptTargetChars, FewShotExcerptMaxChars)
		if !containsString(excerpts, excerpt) {
			excerpts = append(excerpts, excerpt)
			sources = append(sources, AcceptedSource{SampleID: found.sample.ID, ContentRevision: found.sample.ContentRevision})
		}
	}
	return excerpts, sources
}

// topicTokens are the retrieval text's words of two or more characters, punctuation stripped.
func topicTokens(text string) []string {
	var out []string
	for _, field := range strings.Fields(text) {
		token := strings.TrimFunc(field, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) })
		if len([]rune(token)) >= 2 && !containsString(out, token) {
			out = append(out, token)
		}
	}
	return out
}

func pct(share float64) string { return strconv.Itoa(int(math.Round(share*100))) + "%" }

func count(value float64) string { return strconv.Itoa(int(math.Round(value))) }

// decimal drops a trailing .0: `2` rather than `2.0`, `2.5` as it is.
func decimal(value float64) string {
	return strconv.FormatFloat(math.Round(value*10)/10, 'f', -1, 64)
}

func paragraphRange(sh Shape) string {
	if sh.ParagraphMin == sh.ParagraphMax {
		return fmt.Sprintf("%d문장", sh.ParagraphMin)
	}
	return fmt.Sprintf("%d~%d문장", sh.ParagraphMin, sh.ParagraphMax)
}

func paragraphSpan(sh Shape) string {
	if sh.ParagraphMin == sh.ParagraphMax {
		return strconv.Itoa(sh.ParagraphMin)
	}
	return fmt.Sprintf("%d–%d", sh.ParagraphMin, sh.ParagraphMax)
}

func quoted(lines []string) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, `"`+line+`"`)
	}
	return strings.Join(parts, ", ")
}

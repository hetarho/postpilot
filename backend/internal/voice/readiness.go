package voice

// ReadySentences is VOICE_READY_SENTENCES: the prose sentences a voice needs before 말투
// 만들기 (VOICE-32).
const ReadySentences = 60

// InitialQuestionCount is the first personal voice session, independently of sentence count.
const InitialQuestionCount = 10

// Readiness is how far a voice's 학습 글 are from what an analysis needs (VOICE-32).
type Readiness struct {
	// Percent is the larger question/sentence share, held below 100 while a part is missing.
	Percent   int
	Sentences int
	Needed    int
	// MissingParts names the parts no 학습 글 covers yet, in reading order.
	MissingParts      []PromptPart
	AnsweredQuestions int
	RequiredQuestions int
}

// Ready is 100%: enough sentences and every part.
func (r Readiness) Ready() bool { return r.Percent >= 100 }

// ReadinessOf counts the voice's 학습 글 by the fingerprint's own segmentation: a pasted post
// counts as all three parts, an answer as its prompt's part (VOICE-32, VOICE-60).
func ReadinessOf(samples []Sample) Readiness {
	sentences := 0
	covered := map[PromptPart]bool{}
	answered := map[string]bool{}
	for _, sample := range samples {
		prose := ProseSentences(sample.Body)
		if len(prose) == 0 || !containsKoreanProse(prose) {
			continue
		}
		sentences += len(prose)
		if sample.Kind == SampleKindAnswer {
			if prompt, ok := PromptByKey(sample.PromptKey); ok {
				covered[prompt.Part] = true
				answered[prompt.Key] = true
			}
			continue
		}
		for _, part := range Parts() {
			covered[part] = true
		}
	}
	missing := []PromptPart{}
	for _, part := range Parts() {
		if !covered[part] {
			missing = append(missing, part)
		}
	}
	percent := max(min(sentences, ReadySentences)*100/ReadySentences, min(len(answered), InitialQuestionCount)*100/InitialQuestionCount)
	if len(missing) > 0 && percent > 99 {
		percent = 99
	}
	return Readiness{Percent: percent, Sentences: sentences, Needed: ReadySentences, MissingParts: missing, AnsweredQuestions: len(answered), RequiredQuestions: InitialQuestionCount}
}

// Compatibility jamo alone (ㅎㅎ/ㅠㅠ) convey no personal prose.
func containsKoreanProse(sentences []string) bool {
	for _, sentence := range sentences {
		for _, r := range sentence {
			if r >= 0xAC00 && r <= 0xD7A3 {
				return true
			}
		}
	}
	return false
}

package voice

// ReadySentences is VOICE_READY_SENTENCES: the prose sentences a voice needs before 말투
// 만들기 (VOICE-32).
const ReadySentences = 60

// Readiness is how far a voice's 학습 글 are from what an analysis needs (VOICE-32).
type Readiness struct {
	// Percent is the sentence share, held below 100 while a part is missing.
	Percent   int
	Sentences int
	Needed    int
	// MissingParts names the parts no 학습 글 covers yet, in reading order.
	MissingParts []PromptPart
}

// Ready is 100%: enough sentences and every part.
func (r Readiness) Ready() bool { return r.Percent >= 100 }

// ReadinessOf counts the voice's 학습 글 by the fingerprint's own segmentation: a pasted post
// counts as all three parts, an answer as its prompt's part (VOICE-32, VOICE-60).
func ReadinessOf(samples []Sample) Readiness {
	sentences := 0
	covered := map[PromptPart]bool{}
	for _, sample := range samples {
		sentences += len(ProseSentences(sample.Body))
		if sample.Kind == SampleKindAnswer {
			if prompt, ok := PromptByKey(sample.PromptKey); ok {
				covered[prompt.Part] = true
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
	percent := min(sentences, ReadySentences) * 100 / ReadySentences
	if len(missing) > 0 && percent > 99 {
		percent = 99
	}
	return Readiness{Percent: percent, Sentences: sentences, Needed: ReadySentences, MissingParts: missing}
}

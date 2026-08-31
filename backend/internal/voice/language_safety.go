package voice

import "context"

func contentLanguageMismatch(content, source Language) error {
	return &ContentLanguageMismatchError{ContentLanguage: content, SourceLanguage: source}
}

// requireContentLanguageMatch is the single equality rule for post-derived voice
// evidence. Invalid/absent tags are refused as a mismatch as well: a learning path must
// never infer a legacy default from prose.
func requireContentLanguageMatch(content, declaredSource, activeSource Language) error {
	if !content.Valid() || !declaredSource.Valid() || !activeSource.Valid() || content != declaredSource || declaredSource != activeSource {
		return contentLanguageMismatch(content, activeSource)
	}
	return nil
}

func (s *Service) learningVoice(ctx context.Context, event LearningEvent) (Voice, error) {
	active, err := s.activeVoice(ctx, event.UserID, event.VoiceID)
	if err != nil {
		return Voice{}, err
	}
	if err := requireContentLanguageMatch(event.ContentLanguage, event.SourceLanguage, active.SourceLanguage); err != nil {
		return Voice{}, err
	}
	return active, nil
}

func requireSourceLanguageMatch(source AuthoredSource, active Voice) error {
	if !source.SourceLanguage.Valid() || source.SourceLanguage != active.SourceLanguage {
		return contentLanguageMismatch(source.SourceLanguage, active.SourceLanguage)
	}
	return nil
}

func authoredSourcesForLanguage(sources []AuthoredSource, language Language) []AuthoredSource {
	matched := make([]AuthoredSource, 0, len(sources))
	for _, source := range sources {
		if source.SourceLanguage == language {
			matched = append(matched, source)
		}
	}
	return matched
}

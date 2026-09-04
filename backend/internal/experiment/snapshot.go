package experiment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func FreezeSnapshot(snapshot Snapshot) (Snapshot, string, error) {
	if len(snapshot.Content) == 0 || snapshot.PromptVersion == "" {
		return Snapshot{}, "", fmt.Errorf("snapshot content and prompt version are required")
	}
	if snapshot.TargetLanguage != nil && !snapshot.TargetLanguage.Valid() {
		return Snapshot{}, "", ErrLanguageRequired
	}
	hasher := sha256.New()
	_, _ = hasher.Write(snapshot.Content)
	if snapshot.TargetLanguage != nil {
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write([]byte(snapshot.TargetLanguage.String()))
	}
	return Snapshot{
		Content: append([]byte(nil), snapshot.Content...), PromptVersion: snapshot.PromptVersion,
		VoiceID: snapshot.VoiceID, TemplateName: snapshot.TemplateName, TargetLanguage: cloneLanguage(snapshot.TargetLanguage),
	}, hex.EncodeToString(hasher.Sum(nil)), nil
}

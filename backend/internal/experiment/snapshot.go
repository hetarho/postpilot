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
	digest := sha256.Sum256(snapshot.Content)
	return Snapshot{Content: append([]byte(nil), snapshot.Content...), PromptVersion: snapshot.PromptVersion, VoiceID: snapshot.VoiceID}, hex.EncodeToString(digest[:]), nil
}

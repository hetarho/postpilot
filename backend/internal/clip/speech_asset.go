package clip

import "time"

// SpeechAsset is private project-owned media. ObjectKey is used only by the
// authenticated playback/export adapters, never the correction wire contract.
type SpeechAsset struct {
	ID, OwnerID, ProjectID, ObjectKey, Text string
	Speech                                  SpeechRef
	CreatedAt                               time.Time
}

package clip

import "time"

// SpeechAsset is private project-owned media. ObjectKey is used only by the
// authenticated playback/export adapters, never the correction wire contract.
const SpeechAudioPrefix = "private/clip-speech/"

type SpeechAsset struct {
	ID, OwnerID, ProjectID, ObjectKey, Text string
	Bytes                                   int64
	Speech                                  SpeechRef
	CreatedAt                               time.Time
}

type SpeechCleanup struct {
	ID, ObjectKey string
	CreatedAt     time.Time
}

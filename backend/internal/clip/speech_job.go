package clip

import (
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

// SpeechVoice is a private synthesis binding, never part of a customer projection.
type SpeechVoice struct {
	Binding         SpokenVoiceBinding
	ProfileID       string
	ProfileRevision int64
	Model           llm.ModelRef
	Handle          llm.VoiceHandle
	Settings        llm.SpeechSettings
}
type SpeechCall struct {
	SegmentID, Text, InputHash string
	Request                    llm.SpeechRequest
	Budget                     usage.UnitBudget
}
type SpeechRun struct {
	ID, OwnerID, ProjectID, RequestKey, RequestDigest, JobID, ScopeDigest string
	Revision                                                              int
	Voice                                                                 SpeechVoice
	Calls                                                                 []SpeechCall
}

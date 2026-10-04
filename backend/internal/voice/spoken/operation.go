package spoken

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/llm"
	"time"
)

const (
	JobKindDesign       = "voice_design"
	JobKindConfirm      = "voice_confirm"
	JobKindProbe        = "voice_reuse_probe"
	OperationReserved   = "reserved"
	OperationQueued     = "queued"
	OperationClaimed    = "claimed"
	OperationReceived   = "received"
	OperationPublished  = "published"
	OperationFailed     = "failed"
	OperationUnresolved = "unresolved"
	OperationCancelled  = "cancelled"
)

var ErrOperationUnresolved = errors.New("supplier confirmation outcome needs reconciliation")
var ErrOperationStopped = errors.New("spoken operation stopped")

func IsJobKind(kind string) bool {
	return kind == JobKindDesign || kind == JobKindConfirm || kind == JobKindProbe
}

type Operation struct {
	SampleAssetID                                                               string
	ID, OwnerID, Kind, State, JobID, IdempotencyKey, RequestDigest, ScopeDigest string
	DraftID, VoiceID, CandidateID                                               string
	ExpectedRevision                                                            int64
	Profile                                                                     Profile
	Name, Description, PreviewText, QualificationSessionID                      string
	CandidateHandle                                                             llm.CandidateHandle
	VoiceHandle                                                                 llm.VoiceHandle
	ReceivedHandle                                                              llm.VoiceHandle
	Texts                                                                       [2]string
	SpeechInputs                                                                [2]string
	AssetIDs                                                                    [2]string
	Calling                                                                     [2]bool
	Evidence                                                                    []llm.SpeechEvidence
	FailureReason                                                               string
	ResultID                                                                    string
	CreatedAt, UpdatedAt                                                        time.Time
}

func (o Operation) Terminal() bool {
	return o.State == OperationPublished || o.State == OperationFailed || o.State == OperationUnresolved || o.State == OperationCancelled
}

type ProbeAudio struct {
	OperationID, JobID                     string
	OwnerID, VoiceID, InputDigest, AssetID string
	Evidence                               llm.SpeechEvidence
	Timing                                 []llm.CharacterTiming
}

// All claim/publication writes can be bound to the coordinator's transaction.
// Network calls and object uploads never run through these metadata methods.
type OperationLedger interface {
	ReserveOperation(context.Context, Operation) (Operation, bool, error)
	GetOperationRequest(context.Context, string, string, string) (Operation, error)
	GetOperation(context.Context, string, string) (Operation, error)
	ListRecoverableOperations(context.Context) ([]Operation, error)
	BindOperation(context.Context, string, string, string) error
	ClaimOperation(context.Context, string, string, string) error
	ClaimProbeCall(context.Context, string, string, int) error
	RecordConfirmationResult(context.Context, string, string, llm.VoiceHandle, llm.SpeechEvidence) error
	RecordOperationEvidence(context.Context, string, string, int, llm.SpeechEvidence) error
	PublishOperation(context.Context, string, string, string) error
	FailOperation(context.Context, string, string, string, bool) error
	CancelOperation(context.Context, string, string) error
	GetProbeAudio(context.Context, string, string, string) (ProbeAudio, error)
	SaveProbeAudio(context.Context, Operation, int, Asset, ProbeAudio) error
}
type OperationStorage interface {
	Storage
	OperationLedger
}

package authoring

import (
	"errors"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

type Kind string

const (
	PostTemplate   Kind = "post_template"
	VideoTemplate  Kind = "video_template"
	PostGuideline  Kind = "post_guideline"
	VideoGuideline Kind = "video_guideline"
	WritingVoice   Kind = "writing_voice"
	JobKind             = "configuration_authoring"
	CandidateCount      = 8
	MaxTurns            = 20
	MaxPromptChars      = 2000
)

func (k Kind) Valid() bool {
	return k == PostTemplate || k == VideoTemplate || k == PostGuideline || k == VideoGuideline || k == WritingVoice
}

type Mode string

const (
	Recommend Mode = "recommend"
	Refine    Mode = "refine"
)

func (m Mode) Valid() bool { return m == Recommend || m == Refine }

type Artifact struct {
	ID          string
	Name        string
	Description string
	Body        string
	TitleArea   string
}
type Turn struct {
	ID      string
	Request string
	Reply   string
	JobID   string
	Status  string
}
type SavedRef struct {
	Kind Kind
	ID   string
	Name string
}
type Session struct {
	ID              string
	UserID          string
	Kind            Kind
	Revision        uint32
	Phase           string
	Candidates      []Artifact
	Selected        *Artifact
	Turns           []Turn
	ActiveJobID     string
	Saved           *SavedRef
	TargetID        string
	TargetVersion   string
	FailureReason   string
	PendingRequest  string
	ForkVoice       bool
	SourceContext   string
	Purpose         string
	WriteModel      string
	ActiveRequestID string
	Publication     *Publication
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
type Seed struct {
	Artifact      *Artifact
	TargetVersion string
	ForkVoice     bool
	SourceContext string
}
type Publication struct {
	Key           string
	UserID        string
	SessionID     string
	Kind          Kind
	Revision      uint32
	Artifact      Artifact
	TargetID      string
	TargetVersion string
	MakeDefault   bool
	WriteModel    string
}
type Operation struct {
	ID, UserID, SessionID, RequestID, Fingerprint, JobID, Status, FailureReason string
	Mode                                                                        Mode
	BaseRevision                                                                uint32
	Payload                                                                     []byte
	CreatedAt                                                                   time.Time
}
type OperationResult struct {
	Candidates []Artifact
	Selected   *Artifact
	Reply      string
	Purpose    string
	WriteModel string
}
type Run struct {
	ID, UserID, WriteModel string
	Payload                []byte
}
type Start struct {
	SessionID        string
	ExpectedRevision uint32
	RequestID        string
	Mode             Mode
	Prompt           string
	WriteModel       llm.ModelRef
}
type Estimate struct {
	Credits         int
	Free, Available bool
}

var (
	ErrNotFound       = errors.New("authoring session not found")
	ErrInvalidKind    = errors.New("authoring kind is invalid")
	ErrInvalid        = errors.New("authoring request is invalid")
	ErrStale          = errors.New("authoring revision changed")
	ErrBusy           = errors.New("authoring operation is active")
	ErrNoSelection    = errors.New("authoring draft not selected")
	ErrHistoryFull    = errors.New("authoring conversation is full")
	ErrOutput         = errors.New("authoring output is invalid")
	ErrModel          = errors.New("authoring writing model is unavailable")
	ErrTargetConflict = errors.New("authoring target changed")
	ErrPublication    = errors.New("authoring publication is unconfirmed")
)

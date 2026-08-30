package generation

import (
	"fmt"
	"time"
)

type BlockType string

const (
	BlockText    BlockType = "TEXT"
	BlockHeading BlockType = "HEADING"
	BlockImage   BlockType = "IMAGE"
	BlockQuote   BlockType = "QUOTE"
	BlockList    BlockType = "LIST"
)

type Block struct {
	Type    BlockType
	Content string
	Level   int32
	File    string
	Alt     string
	Caption string
	Items   []string
}

type PostContent struct {
	Title   string
	Summary string
	Tags    []string
	Blocks  []Block
}

type Observation struct {
	File          string
	Scene         string
	Mood          string
	VisibleText   string
	Objects       []string
	PeoplePresent bool
}

type Image struct {
	Filename string
	Key      string
}

// VoiceRef is the post's voice as the post context projects it. Deleted is what makes a
// start or a handler refuse before any provider call.
type VoiceRef struct {
	ID      string
	Name    string
	Deleted bool
}

type PostInput struct {
	Slug         string
	UserID       string
	Voice        VoiceRef
	Title        string
	Memo         string
	Images       []Image
	Content      *PostContent
	TargetLength *int
}

type Profile struct {
	Styleguide           string
	ActiveRules          string
	Excerpts             []string
	Rules                string
	EndingMaxConsecutive int
}

// StartRequest.VoiceID is filled by the service from the owned post and frozen into the
// job, so the handler can prove the post still belongs to the voice it was queued for.
type StartRequest struct {
	UserID       string
	PostSlug     string
	VoiceID      string
	ObserveModel string
	WriteModel   string
	TargetLength *int
}

type GenerateJob struct {
	UserID       string
	PostSlug     string
	VoiceID      string
	ObserveModel string
	WriteModel   string
	TargetLength *int
}

type StartRevisionRequest struct {
	UserID      string
	PostSlug    string
	VoiceID     string
	Instruction string
	SaveAsRule  bool
	WriteModel  string
}

type RevisionJob struct {
	UserID     string
	PostSlug   string
	VoiceID    string
	WriteModel string
	Payload    []byte
}

type JobSummary struct {
	ID            string
	Kind          string
	Status        string
	Stage         string
	ProgressDone  int
	ProgressTotal int
	Error         string
	PostSlug      string
	ObserveModel  string
	WriteModel    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return fmt.Sprintf("generation job %s is already in progress", e.ActiveID)
}

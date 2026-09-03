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
	// Model is the ref that observed this photo, stamped where the batch ran. It is what
	// lets the picker say whose eyesight it is offering to reuse; empty means unknown.
	Model string
}

type Image struct {
	Filename string
	Key      string
}

// VoiceRef is the post's voice as the post context projects it. Deleted is what makes a
// start or a handler refuse before any provider call.
type VoiceRef struct {
	ID             string
	Name           string
	Deleted        bool
	SourceLanguage Language
}

// PurposeBrief is the post's 용도 as the writer needs it: three authored strings and no id.
// It carries no id on purpose — once frozen into a job payload or an experiment snapshot it
// must stay readable after the purpose it came from is renamed or deleted.
type PurposeBrief struct {
	Name         string
	Description  string
	Instructions string
}

type PostInput struct {
	Slug   string
	UserID string
	Voice  VoiceRef
	// PurposeID is what the post currently points at, read only at enqueue time. Handlers
	// never resolve it: they use Purpose, which the job payload froze.
	PurposeID string
	Purpose   *PurposeBrief
	// Guidelines is the frozen 작문 지침 material in injection order, filled at enqueue from
	// PurposeID like Purpose is. Handlers never resolve guidelines live either.
	Guidelines []string
	Title      string
	Memo       string
	Images     []Image
	// Observations is the post's stored observation snapshot, read only at enqueue so the
	// selection and what it carries over can both be frozen there. No handler reads a live
	// snapshot: it reads the payload, which is what makes the frozen decision hold.
	Observations    []Observation
	Content         *PostContent
	TargetLanguage  Language
	ContentLanguage *Language
	TargetLength    *int
}

type Profile struct {
	Styleguide           string
	ActiveRules          string
	Excerpts             []string
	Rules                string
	EndingMaxConsecutive int
	SourceLanguage       Language
	TargetLanguage       Language
	Portable             bool
}

// StartRequest.VoiceID is filled by the service from the owned post and frozen into the
// job, so the handler can prove the post still belongs to the voice it was queued for.
type StartRequest struct {
	UserID         string
	PostSlug       string
	VoiceID        string
	ObserveModel   string
	WriteModel     string
	TargetLanguage Language
	TargetLength   *int
	// Purpose is resolved from the post server-side at Start and frozen into the payload;
	// the request never carries one. Nil means the post had none, or it was deleted first.
	Purpose *PurposeBrief
	// Guidelines is resolved the same way, from the same purpose id, and frozen alongside.
	Guidelines []string
	// ObserveCalls is how many observation calls the photos will take, resolved at Start
	// where the post is already in hand. Observation batches photos, so this is not the
	// photo count — and the credit hold has to price every call, not every photo.
	ObserveCalls int
	// ObserveFiles is the re-observation picker's answer on the way in, and the RESOLVED
	// frozen set on the way out of Start: unknown names dropped, photos with nothing to
	// reuse forced in. Nil is a client that sent no picker answer, which observes everything.
	ObserveFiles *[]string
	// Observations is the reusable snapshot as it stood at Start, frozen into the payload
	// beside ObserveFiles. Which of its entries survive the run is decided by ObserveFiles
	// alone, so a photo waiting for its batch can keep the entry it already had.
	Observations []Observation
}

type GenerateJob struct {
	UserID         string
	PostSlug       string
	VoiceID        string
	ObserveModel   string
	WriteModel     string
	TargetLanguage Language
	TargetLength   *int
	Purpose        *PurposeBrief
	Guidelines     []string
	// ObserveFiles carries PRESENCE, not just emptiness. Nil is a job queued before this
	// contract existed and keeps the observe-everything behavior; non-nil but empty is the
	// frozen decision to observe nothing at all.
	ObserveFiles *[]string
	// Observations is the reusable snapshot frozen at enqueue, beside ObserveFiles and from
	// the same read. It is the run's ONLY view of what was already known — no handler reads
	// a live snapshot.
	Observations []Observation
}

type StartRevisionRequest struct {
	UserID          string
	PostSlug        string
	VoiceID         string
	Instruction     string
	SaveAsRule      bool
	WriteModel      string
	ContentLanguage Language
	Purpose         *PurposeBrief
	Guidelines      []string
}

type RevisionJob struct {
	UserID     string
	PostSlug   string
	VoiceID    string
	WriteModel string
	Payload    []byte
}

type JobSummary struct {
	ID             string
	Kind           string
	Status         string
	Stage          string
	ProgressDone   int
	ProgressTotal  int
	Failure        *Failure
	PostSlug       string
	ObserveModel   string
	WriteModel     string
	TargetLanguage Language
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Failure is generation's consumer-owned projection of a durable job failure. It keeps
// the generation context independent from the job context's persistence types.
type Failure struct {
	Reason          string
	Params          map[string]string
	TechnicalDetail string
}

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return fmt.Sprintf("generation job %s is already in progress", e.ActiveID)
}

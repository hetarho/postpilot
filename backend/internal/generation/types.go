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
	// Slot is set only on an unfilled template slot. It rides on a TEXT block rather than
	// being a sixth BlockType so every existing switch — reading view, block editor, the
	// four export mappings, revision, the validator — stays correct without changing; only
	// the surfaces that render a slot specially need to know it exists. [I2] holds either
	// way: the canonical post is still a block array.
	Slot *BlockSlot
}

// BlockSlot is a position a template reserved for content the app cannot invent. Kind is one
// of the grammar's slot kinds; Label is the template author's own words, shown to the person
// who has to fill it and never an instruction to a model.
type BlockSlot struct {
	Kind  string
	Label string
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

// TemplateBrief is the post's 템플릿 as the writer needs it: the name, the body ALREADY
// expanded for this post's photos and rendered into prompt text, and the slots that body
// declared in document order.
//
// It carries no id. Once frozen into a job payload or an experiment snapshot it must stay
// readable after the template it came from is renamed or deleted, and re-resolving an id
// would defeat the freeze.
//
// The body is expanded rather than raw so that attaching a photo after the start cannot
// change what the model was asked for — the expansion is part of what gets frozen.
type TemplateBrief struct {
	Name string
	Body string
	// Slots are the unfilled kinds (place · link) in the order the body's {{slot:n}} tokens
	// number them, so the post-processing pass can resolve a token back to its kind and label.
	Slots []TemplateSlot
}

// TemplateSlot is one position the app cannot fill by itself. It stays honest rather than
// filled: the model is told not to write prose there, and a person fills it after export.
type TemplateSlot struct {
	Kind  string
	Label string
}

type PostInput struct {
	Slug   string
	UserID string
	Voice  VoiceRef
	// TemplateID is what the post currently points at, read only at enqueue time. Handlers
	// never resolve it: they use Template, which the job payload froze.
	TemplateID string
	Template   *TemplateBrief
	// Guidelines is the frozen 작문 지침 material in injection order, filled at enqueue from
	// TemplateID like Template is. Handlers never resolve guidelines live either.
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
	// WriteNativeEffort is frozen from the selected catalog model at enqueue, so the hold
	// and a delayed execution use the same completion budget even if curation changes.
	WriteNativeEffort bool
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
	// Template is resolved from the post server-side at Start and frozen into the payload;
	// the request never carries one. Nil means the post had none, or it was deleted first.
	Template *TemplateBrief
	// Guidelines is resolved the same way, from the same template id, and frozen alongside.
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
	Observations      []Observation
	WriteNativeEffort bool
}

type GenerateJob struct {
	UserID         string
	PostSlug       string
	VoiceID        string
	ObserveModel   string
	WriteModel     string
	TargetLanguage Language
	TargetLength   *int
	Template       *TemplateBrief
	Guidelines     []string
	// ObserveFiles carries PRESENCE, not just emptiness. Nil is a job queued before this
	// contract existed and keeps the observe-everything behavior; non-nil but empty is the
	// frozen decision to observe nothing at all.
	ObserveFiles *[]string
	// Observations is the reusable snapshot frozen at enqueue, beside ObserveFiles and from
	// the same read. It is the run's ONLY view of what was already known — no handler reads
	// a live snapshot.
	Observations      []Observation
	WriteNativeEffort bool
}

type StartRevisionRequest struct {
	UserID          string
	PostSlug        string
	VoiceID         string
	Instruction     string
	SaveAsRule      bool
	WriteModel      string
	ContentLanguage Language
	Template        *TemplateBrief
	Guidelines      []string
	// The enqueue adapter uses the same frozen length facts as the revision handler to price
	// the completion budget. Neither field is sent by the client or persisted independently.
	TargetLength      *int
	ContentChars      int
	WriteNativeEffort bool
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

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
	// BlockVideo carries the IMAGE fields and none of its own (VIDEO-2). Its File names an
	// attached VIDEO: a filename is unique across the two kinds, so a mismatch is the wrong
	// block type rather than an unknown file.
	BlockVideo BlockType = "VIDEO"
	// BlockGallery is a photo group (GEN-77): Files names the photos in the order they stand,
	// Layout how they stand, and the one Alt and Caption belong to the whole group.
	BlockGallery BlockType = "GALLERY"
)

// The two photo-group layouts, in the protojson spelling the model writes (GEN-77). Anything else
// the model writes reads as GalleryCollage (GEN-78).
const (
	GalleryCollage = "COLLAGE"
	GallerySlide   = "SLIDE"
)

type Block struct {
	Type    BlockType
	Content string
	Level   int32
	File    string
	Alt     string
	Caption string
	Items   []string
	// Files and Layout belong to a GALLERY block alone.
	Files  []string
	Layout string
}

type PostContent struct {
	Title   string
	Summary string
	Tags    []string
	Blocks  []Block
}

// WriteAnswer is what the write pass returns: the content, plus what it says about that
// content beside it. Nouns are the distinct nouns the title and the body use, already bounded
// by the parser (GEN-55); none is a real answer and never a failed write.
type WriteAnswer struct {
	Content PostContent
	Nouns   []string
	// Storyline is the plan the write answered before the post (GEN-67), bounded by the parser.
	// nil only for a comparison candidate recorded before the storyline existed, which then
	// keeps the post's own (GEN-72).
	Storyline *Storyline
}

// Storyline is a write's storyline: its paragraphs in order, and the attachment names the
// writing stage was shown (GEN-12), which is what the post reads an added attachment against.
type Storyline struct {
	Paragraphs []StorylineParagraph
	MadeWith   []string
}

// StorylineParagraph is one part of the storyline: a short plan of what it shows and says, and
// the attachment names it uses.
type StorylineParagraph struct {
	Text  string
	Files []string
}

// WriteAnnotations is what a write hands the post beside its content: its nouns and its
// storyline. Handed as a pointer, where nil keeps what the post holds; a nil Storyline inside
// keeps the post's storyline alone.
type WriteAnnotations struct {
	Nouns     []string
	Storyline *Storyline
}

// Annotations is this answer's, always non-nil: a write replaces what the last one said, and
// a write with no nouns clears them rather than keeping stale ones (GEN-55).
func (a WriteAnswer) Annotations() *WriteAnnotations {
	return &WriteAnnotations{Nouns: a.Nouns, Storyline: a.Storyline}
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
	// Events and Speech are what a still frame cannot carry, so only a VIDEO entry has them:
	// what happens in the clip in order, and what is said or heard, summarized (VIDEO-9).
	Events []string
	Speech string
	// Rotation is the clockwise turn, in degrees (0, 90, 180, 270), that makes a photo's scene
	// upright (GEN-79); always 0 for a video.
	Rotation int
}

// AttachmentKind is which kind of attachment an Image entry describes. The generation
// context speaks of one attachment list — the selection, freezing and merge functions all
// iterate it by filename — and this is what tells a clip from a photo inside it.
type AttachmentKind string

const (
	AttachmentPhoto AttachmentKind = "photo"
	AttachmentVideo AttachmentKind = "video"
)

// Image is one attached thing the post can be written from: a photo, or a video with the
// three fields a photo has no use for. An empty Kind reads as a photo, which is what every
// entry built before videos existed is.
type Image struct {
	Filename string
	Key      string
	Kind     AttachmentKind
	// ContentType is the object's stored type; DurationMs is zero for a photo.
	ContentType string
	DurationMs  int64
	// Width and Height are a photo's dimensions on record, which decide its orientation for
	// grouping (GEN-77); zero for a video.
	Width, Height int32
	// Rotation is the photo's clockwise turn in degrees and RotationByOwner whether the owner
	// set it (POST-107): an owner's turn stands over any observation's.
	Rotation        int32
	RotationByOwner bool
}

// VoiceRef is the post's voice as the post context projects it. Deleted is what makes a
// start or a handler refuse before any provider call.
type VoiceRef struct {
	ID      string
	Name    string
	Deleted bool
	// Made is whether the voice has a published analysis; a run needs a made voice (GEN-23).
	Made bool
}

// TemplateBrief is the post's 템플릿 as the writer needs it: the name and the body ALREADY
// resolved with the post's answers and rendered into prompt text, its photo places unbound.
//
// It carries no id. Once frozen into a job payload or an experiment snapshot it must stay
// readable after the template it came from is renamed or deleted, and re-resolving an id
// would defeat the freeze.
//
// The body is rendered rather than raw so that an answer edited after the start cannot change
// what the model was asked for — the render is part of what gets frozen.
type TemplateBrief struct {
	Name string
	Body string
	// Facts are the data fields the freeze resolved, in body order — the values already
	// substituted into Body and fenced there. It is carried beside the body so the prompt
	// builder can tell whether this brief holds any fact at all without re-parsing it, which
	// is what decides one legend line (TMPL-45, TMPL-46).
	Facts []TemplateFact
	// TitleArea is the rendered title form, empty when the template authored none (TMPL-50).
	// The experiment snapshot marshals the brief by field name, so omitempty is what keeps
	// every snapshot frozen before the member existed byte-identical, hash and all.
	TitleArea string
}

// RequiredTemplateAnswerError crosses the template port for a new-write start. Label is the
// first missing field in title-then-body order and is safe to show beside the editor input.
type RequiredTemplateAnswerError struct{ Label string }

func (e *RequiredTemplateAnswerError) Error() string {
	return "a required template answer is missing: " + e.Label
}

// TemplateFact is one data field that survived the freeze: the title the author asked under
// and the text the post's author typed.
type TemplateFact struct {
	Label string
	Value string
}

// TemplateAnswer is one answer the post gives to a data field its template declared. It
// reaches this context only to be handed to the template context's render at enqueue.
type TemplateAnswer struct {
	Label   string
	Text    string
	Enabled bool
}

type PostInput struct {
	Slug   string
	UserID string
	Voice  VoiceRef
	// TemplateID is what the post currently points at, read only at enqueue time. Handlers
	// never resolve it: they use Template, which the job payload froze.
	TemplateID string
	Template   *TemplateBrief
	// Guidelines is the owner's frozen 작문 지침 in injection order, filled at enqueue from
	// TemplateID and Field like Template is. Handlers never resolve guidelines live either.
	Guidelines []string
	// DefaultGuidelines are the frozen 기본 지침 texts, rendered ahead of the owner's (GUIDE-14),
	// in the run's target language.
	DefaultGuidelines []string
	// UseMemory is the post's opt-in, read at enqueue. It is the ONLY thing that decides
	// whether this context asks the memory context anything at all (MEM-18).
	UseMemory bool
	// Memories is the frozen 기억 material, retrieved at enqueue when UseMemory is set and
	// never re-read: a memory edited or deleted afterwards cannot change queued work.
	Memories []string
	// QualityRules is the frozen text of each quality rule the owner ticked, already rendered
	// in the target language, in the order the write prompt lists them (GEN-51).
	QualityRules []string
	// TemplateAnswers is what the post answers to its template's data fields, read at
	// enqueue like TemplateID. The freeze resolves them into the rendered brief, so no
	// handler ever reads one: the payload already carries the result (POST-62, TMPL-45).
	TemplateAnswers []TemplateAnswer
	Title           string
	Memo            string
	Images          []Image
	// Observations is the post's stored observation snapshot, read only at enqueue so the
	// selection and what it carries over can both be frozen there. No handler reads a live
	// snapshot: it reads the payload, which is what makes the frozen decision hold.
	Observations []Observation
	// Storyline is the post's stored storyline, nil for none. It is read at enqueue — by the
	// storyline request for its paragraphs and by a from-storyline start to freeze them — and by
	// the storyline request's write for what it was made with (GEN-69, GEN-70).
	Storyline *Storyline
	// FollowStoryline is the frozen storyline a from-storyline run writes along (GEN-70), empty
	// for every other run. Only the generate payload carries it: a comparison never reads the
	// storyline (GEN-72).
	FollowStoryline []StorylineParagraph
	Content         *PostContent
	TargetLanguage  Language
	ContentLanguage *Language
	TargetLength    *int
	// TagCount is how many tags the prompt asks for (POST-63). Read from the post at enqueue
	// and frozen like TargetLength; a write-experiment snapshot carries it, so a different
	// count is a different input hash. 0 is only ever a legacy decode and resolves to the
	// default.
	TagCount int
	// WriteNativeEffort is frozen from the selected catalog model at enqueue, so the hold
	// and a delayed execution use the same completion budget even if curation changes.
	WriteNativeEffort bool
	// Field and QualityRuleIDs are the post's 분야 and its quality ticks, read at enqueue as
	// inputs to resolve and never frozen themselves: the frozen guideline texts and rule texts
	// they resolve into carry the run's identity, so the snapshot's wire struct
	// (experiment_snapshot.go) has no member for them.
	Field          string
	QualityRuleIDs []string
	// Published is the post's lock (POST-74): no run starts on it and no result lands in it.
	// It is read from the post and never frozen; the snapshot's wire struct has no member for
	// it either.
	Published bool
}

// Profile is the voice's projection as the writer receives it (VOICE-46): the section text as
// given, the excerpts a Korean target gets, and whether it is the portable one.
type Profile struct {
	// NoVoice is 말투 없음: the prompt carries no voice bytes at all (GEN-74).
	NoVoice  bool
	Text     string
	Excerpts []string
	Portable bool
}

// StartRequest.VoiceID is filled by the service from the owned post and frozen into the
// job, so the handler can prove the post still belongs to the voice it was queued for.
type StartRequest struct {
	UserID       string
	PostSlug     string
	VoiceID      string
	ObserveModel string
	WriteModel   string
	// TargetLanguage, VoiceID and WriteNativeEffort are resolved by Start from the post and the
	// catalog; the enqueue adapter reads them for the row and the hold.
	TargetLanguage Language
	TargetLength   *int
	// ObserveCalls is how many observation calls the photos will take, resolved at Start
	// where the post is already in hand. Observation batches photos, so this is not the
	// photo count — and the credit hold has to price every call, not every photo.
	ObserveCalls int
	// ObserveFiles is the re-observation picker's answer on the way in. Nil is a client that
	// sent no picker answer, which observes everything. The resolved set Start freezes lives in
	// the payload, not here.
	ObserveFiles      *[]string
	WriteNativeEffort bool
	// FromStoryline is 이 스토리로 글 쓰기 / 다시 쓰기: the run writes along the stored storyline
	// and observes exactly what it holds (GEN-70).
	FromStoryline bool
}

// GenerateJob is one queued generate as the worker hands it over: the row's routing plus the
// payload Start encoded.
type GenerateJob struct {
	UserID       string
	PostSlug     string
	VoiceID      string
	ObserveModel string
	WriteModel   string
	// Payload is the frozen options exactly as Start encoded them. Generate decodes them, and
	// nothing else does.
	Payload []byte
}

type StartRevisionRequest struct {
	UserID            string
	PostSlug          string
	VoiceID           string
	Instruction       string
	WriteModel        string
	ContentLanguage   Language
	Template          *TemplateBrief
	Guidelines        []string
	DefaultGuidelines []string
	// The enqueue adapter uses the same frozen length facts as the revision handler to price
	// the completion budget. Neither field is sent by the client or persisted independently.
	TargetLength *int
	ContentChars int
	// TagCount is frozen into the revision payload at Start (GEN-46); the handler prompts
	// with the payload value, not the live post's.
	TagCount          int
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

// ExperimentPendingError is GEN-23's refusal: an editor write comparison holds the post. It
// names the comparison, which is not a job, so its id never passes for an active job id.
type ExperimentPendingError struct{ ExperimentID string }

func (e *ExperimentPendingError) Error() string {
	return fmt.Sprintf("write comparison %s holds the post", e.ExperimentID)
}

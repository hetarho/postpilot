// Package post is the drafting context: posts, the photos attached to them, and the
// upload handshake that puts a photo in object storage without the API touching the
// bytes ([I6]).
package post

import (
	"errors"
	"fmt"
	"time"
)

// Status values a post can hold. `review` is set by the generation pipeline (plan 06);
// this context only ever writes `draft`.
const (
	StatusDraft     = "draft"
	StatusReview    = "review"
	StatusFinalized = "finalized"
)

// uploadContentType is what a presigned PUT is signed for. The browser pipeline
// re-encodes every photo to JPEG before upload (PRD §6.2), and the signature covers
// this header — a PUT with any other Content-Type is rejected as a signature mismatch.
const uploadContentType = "image/jpeg"

// maxImageDimension bounds the width and height a client may report. The browser
// pipeline caps the long edge at 1024 px; this is generous enough to survive a change
// there while still refusing a value that could only be a bug or an attack.
const maxImageDimension = 20000

var (
	// ErrNotFound is a slug or id that does not exist.
	ErrNotFound = errors.New("not found")
	// ErrForbidden is a slug or id that exists but belongs to someone else. It is
	// deliberately distinguishable from ErrNotFound: the PRD (§7) specifies 403 here,
	// and at two users there is no enumeration concern worth hiding it for.
	ErrForbidden = errors.New("forbidden")
	// ErrDuplicateFilename is a filename already attached to the post.
	ErrDuplicateFilename = errors.New("filename already used in this post")
	// ErrTooManyPhotos is an upload that would take the post past its photo ceiling.
	ErrTooManyPhotos = errors.New("post already holds the maximum number of photos")
	// ErrDuplicateSlug is a slug another post already holds. Only the store raises it,
	// and only createPost sees it — as the signal to mint the next candidate.
	ErrDuplicateSlug = errors.New("slug already used")
	// ErrInvalidImage is a confirm whose dimensions or size cannot describe a photo.
	ErrInvalidImage = errors.New("invalid image")
	// ErrObjectMissing is a confirm for an object that never landed in storage.
	ErrObjectMissing = errors.New("uploaded object not found in storage")
	// ErrPostBusy prevents deleting a source while a handler could still write new
	// experiment output after the privacy purge.
	ErrPostBusy = errors.New("post has an active job")
	// ErrPostPublishing prevents deleting a post whose publication is still in flight.
	// It is deliberately distinct from ErrPostBusy because the remedy differs: the user
	// cancels or finishes a publication, they do not wait for a generation to end.
	ErrPostPublishing       = errors.New("post has a live publish job")
	ErrStaleContentRevision = errors.New("post content revision is stale")
	ErrInvalidContent       = errors.New("invalid post content")
	ErrNoMachineBaseline    = errors.New("post has no machine baseline to finalize")
	ErrPostNotFinalized     = errors.New("post content is not finalized")
	// ErrVoiceRequired: a create (or a present voice_id) arrived without a concrete voice.
	// The server never substitutes the default — that choice belongs to the client's dropdown.
	ErrVoiceRequired = errors.New("a voice is required")
	// ErrVoiceNotFound covers unknown and foreign voices alike; a foreign id must not be
	// distinguishable from a nonexistent one.
	ErrVoiceNotFound = errors.New("voice not found")
	ErrVoiceDeleted  = errors.New("voice is deleted")
	// ErrPurposeNotFound covers unknown and foreign purposes alike, like ErrVoiceNotFound.
	// There is deliberately no ErrPurposeRequired: a post may have none, and clearing the
	// assignment is a valid save rather than a missing value.
	ErrPurposeNotFound = errors.New("purpose not found")
	// ErrLanguageRequired is returned at create/machine-write boundaries when the
	// caller supplies no concrete supported language.
	ErrLanguageRequired = errors.New("a content language is required")
)

// Language is the post context's pure, canonical language value. Proto enums and SQL
// strings are converted only in rpc/ and store/ respectively.
type Language string

const (
	LanguageKorean  Language = "ko"
	LanguageEnglish Language = "en"
)

func ParseLanguage(value string) (Language, error) {
	language := Language(value)
	if !language.Valid() {
		return "", fmt.Errorf("%w: %q", ErrLanguageRequired, value)
	}
	return language, nil
}

func (l Language) Valid() bool { return l == LanguageKorean || l == LanguageEnglish }

type InvalidContentError struct{ Reason string }

func (e *InvalidContentError) Error() string {
	return fmt.Sprintf("%s: %s", ErrInvalidContent, e.Reason)
}
func (e *InvalidContentError) Unwrap() error { return ErrInvalidContent }

// VoiceRef is the voice a post is written in, as the post context needs it: the id it
// stores plus the name/tombstone the voice context publishes. A deleted voice still names
// itself here so the post stays readable and exportable while AI actions refuse.
type VoiceRef struct {
	ID             string
	Name           string
	Deleted        bool
	SourceLanguage Language
}

// PurposeRef is the purpose a post is written for, as the post context needs it: the id it
// stores plus the name the purpose context publishes. An empty ID is the ordinary case —
// a post without a purpose — not a missing value.
type PurposeRef struct {
	ID   string
	Name string
}

// Post is the aggregate exposed by the drafting context. Generation may replace its
// canonical content and observations only through Service's published behaviors. The post
// stores only VoiceID; Voice is enriched on read through the VoiceDirectory port.
type Post struct {
	Slug    string
	UserID  string
	VoiceID string
	Voice   VoiceRef
	// PurposeID is empty when the post has no purpose. Like VoiceID the post stores only
	// the id; Purpose is enriched on read through the PurposeDirectory port.
	PurposeID               string
	Purpose                 PurposeRef
	TargetLanguage          Language
	ContentLanguage         *Language
	Title                   string
	Memo                    string
	Status                  string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	Content                 *PostContent
	ContentRevision         int64
	MachineBaselineRevision int64
	// MachineBaselineVoiceID is the voice the latest machine result was written under.
	// Reassignment clears it, so finalization learning can prove the baseline and the
	// current voice agree.
	MachineBaselineVoiceID string
	TargetLength           *int
	FinalizedRevision      int64
	FinalizedAt            *time.Time
	Observations           []Observation

	// Images is populated by Get, not by the store's post lookup.
	Images              []Image
	ActiveJob           *ActiveJob
	PendingExperimentID string
}

// LearningSnapshot is the post context's ownership-checked hand-off to voice. The
// voice context never reads post tables and cannot mutate either snapshot.
type LearningSnapshot struct {
	PostSlug               string
	UserID                 string
	VoiceID                string
	MachineBaselineVoiceID string
	Current                PostContent
	ContentRevision        int64
	MachineBaseline        PostContent
	BaselineRevision       int64
	TargetLength           *int
	FinalizedAt            time.Time
	UpdatedAt              time.Time
	ContentLanguage        Language
	VoiceSourceLanguage    Language
}

// PublishingSnapshot is the post context's immutable, ownership-checked hand-off.
// Reading it never finalizes, learns, signs URLs, or mutates the post; publishing copies
// the named already-normalized JPEG objects through its own storage port.
type PublishingSnapshot struct {
	PostSlug            string
	UserID              string
	CreatedAt           time.Time
	Content             PostContent
	ContentRevision     int64
	FinalizedRevision   int64
	Images              []Image
	TargetLanguage      Language
	ContentLanguage     Language
	VoiceSourceLanguage Language
}

// BlockType is kept as the LLM/protojson spelling at the domain boundary.
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

// Summary is a row of the post list.
type Summary struct {
	Slug                string
	VoiceID             string
	Voice               VoiceRef
	PurposeID           string
	Purpose             PurposeRef
	Title               string
	Status              string
	UpdatedAt           time.Time
	ActiveJob           *ActiveJob
	PendingExperimentID string
	TargetLanguage      Language
	ContentLanguage     *Language
}

// ActiveJob is the snapshot the post context publishes on read models. It is owned by
// the consumer so the post domain does not depend on job persistence or transport.
type ActiveJob struct {
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
	CreatedAt      time.Time
	UpdatedAt      time.Time
	TargetLanguage Language
}

// Failure is the post read model's consumer-owned durable job failure projection.
type Failure struct {
	Reason          string
	Params          map[string]string
	TechnicalDetail string
}

// Image is a photo attached to a post. The bytes live in object storage; this is the
// record that names them.
type Image struct {
	ID        string
	PostSlug  string
	Filename  string
	Key       string
	Width     int32
	Height    int32
	Bytes     int64
	CreatedAt time.Time

	// ViewURL is a short-lived presigned GET, minted per read and never stored.
	ViewURL string
}

// Upload is a presigned PUT that has not been confirmed yet.
type Upload struct {
	ID        string
	PostSlug  string
	Filename  string
	Key       string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// ObjectKey is the storage key for a photo (PRD §5). The image id is in the key rather
// than the filename so that a rename or a duplicate name can never collide, and so the
// key is not attacker-influenced.
func ObjectKey(postSlug, imageID string) string {
	return "posts/" + postSlug + "/" + imageID + ".jpg"
}

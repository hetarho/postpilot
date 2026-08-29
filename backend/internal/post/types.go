// Package post is the drafting context: posts, the photos attached to them, and the
// upload handshake that puts a photo in object storage without the API touching the
// bytes ([I6]).
package post

import (
	"errors"
	"time"
)

// Status values a post can hold. `review` is set by the generation pipeline (plan 06);
// this context only ever writes `draft`.
const (
	StatusDraft  = "draft"
	StatusReview = "review"
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
)

// Post is the aggregate exposed by the drafting context. Generation may replace its
// canonical content and observations only through Service's published behaviors.
type Post struct {
	Slug         string
	UserID       string
	Title        string
	Memo         string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Content      *PostContent
	Observations []Observation

	// Images is populated by Get, not by the store's post lookup.
	Images              []Image
	ActiveJob           *ActiveJob
	PendingExperimentID string
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
	Title               string
	Status              string
	UpdatedAt           time.Time
	ActiveJob           *ActiveJob
	PendingExperimentID string
}

// ActiveJob is the snapshot the post context publishes on read models. It is owned by
// the consumer so the post domain does not depend on job persistence or transport.
type ActiveJob struct {
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

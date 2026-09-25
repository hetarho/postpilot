// Package post is the drafting context: posts, the photos attached to them, and the
// upload handshake that puts a photo in object storage without the API touching the
// bytes ([I6]).
package post

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status values a post can hold, and who writes each: the create writes draft; generation,
// revision and an applied comparison write review; FinalizePost writes finalized;
// SavePublishedURL writes published, and clearing the address writes finalized again
// (POST-73, POST-75).
const (
	StatusDraft     = "draft"
	StatusReview    = "review"
	StatusFinalized = "finalized"
	StatusPublished = "published"
)

// uploadContentType is what a presigned PUT is signed for. The browser pipeline
// re-encodes every photo to JPEG before upload (PRD §6.2), and the signature covers
// this header — a PUT with any other Content-Type is rejected as a signature mismatch.
const uploadContentType = "image/jpeg"

// maxImageDimension bounds the width and height a client may report, for a photo and for
// a video frame alike. The browser pipeline caps a photo's long edge at 1024 px and a clip
// comes off a phone; this is generous enough to survive a change there while still refusing
// a value that could only be a bug or an attack.
const maxImageDimension = 20000

// AttachmentKind is which kind of attachment an upload becomes. It is the post context's
// own vocabulary — the proto enum and the `uploads.kind` column are converted at the edges.
type AttachmentKind string

const (
	AttachmentPhoto AttachmentKind = "photo"
	AttachmentVideo AttachmentKind = "video"
)

// Valid reports a kind this context can act on. The zero value is deliberately NOT valid:
// a caller that means a photo says so, and the transport substitutes it for an unspecified
// wire value so the defaulting happens in exactly one place.
func (k AttachmentKind) Valid() bool { return k == AttachmentPhoto || k == AttachmentVideo }

// VideoContentTypes maps an accepted container extension (lower case, no dot) to the
// Content-Type the PUT is signed for and the row records. It is code rather than config
// (VIDEO-4): adding a container changes what the browser gate, the player and the model
// delivery all have to handle, which is a change to the product, not to a deployment.
var VideoContentTypes = map[string]string{
	"mp4":  "video/mp4",
	"mov":  "video/quicktime",
	"m4v":  "video/x-m4v",
	"webm": "video/webm",
}

// VideoContentType resolves a filename's extension to its container type. The second
// result is false for anything outside the four accepted containers.
func VideoContentType(filename string) (extension, contentType string, ok bool) {
	dot := strings.LastIndex(filename, ".")
	if dot < 0 || dot == len(filename)-1 {
		return "", "", false
	}
	extension = strings.ToLower(filename[dot+1:])
	contentType, ok = VideoContentTypes[extension]
	return extension, contentType, ok
}

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
	// ErrTooManyVideos is an upload that would take the post past its video ceiling. It is
	// separate from ErrTooManyPhotos because the ceilings differ by an order of magnitude
	// and the user is told which one they reached.
	ErrTooManyVideos = errors.New("post already holds the maximum number of videos")
	// ErrUnsupportedVideo is a filename whose extension is outside the accepted containers.
	ErrUnsupportedVideo = errors.New("unsupported video container")
	// ErrDuplicateSlug is a slug another post already holds. Only the store raises it,
	// and only createPost sees it — as the signal to mint the next candidate.
	ErrDuplicateSlug = errors.New("slug already used")
	// ErrInvalidImage is a confirm whose dimensions or size cannot describe a photo.
	ErrInvalidImage = errors.New("invalid image")
	// ErrInvalidVideo is a confirm whose duration, dimensions, size or stored content type
	// cannot describe one of this post's clips.
	ErrInvalidVideo = errors.New("invalid video")
	// ErrObjectMissing is a confirm for an object that never landed in storage.
	ErrObjectMissing = errors.New("uploaded object not found in storage")
	// ErrPostBusy prevents deleting a source while a handler could still write new
	// experiment output after the privacy purge, and saving a published address while a job
	// that writes the post's content is still running.
	ErrPostBusy             = errors.New("post has an active job")
	ErrStaleContentRevision = errors.New("post content revision is stale")
	ErrInvalidContent       = errors.New("invalid post content")
	ErrInvalidTagCount      = errors.New("tag count out of range")
	ErrNoMachineBaseline    = errors.New("post has no machine baseline to finalize")
	ErrPostNotFinalized     = errors.New("post content is not finalized")
	// ErrInvalidListRequest is a list request the browser never builds: a page token it was
	// not handed, a status that is not one, or a negative page size.
	ErrInvalidListRequest = errors.New("invalid post list request")
	// ErrVoiceRequired: a create (or a present voice_id) arrived without a concrete voice.
	// The server never substitutes the default — that choice belongs to the client's dropdown.
	ErrVoiceRequired = errors.New("a voice is required")
	// ErrVoiceNotFound covers unknown and foreign voices alike; a foreign id must not be
	// distinguishable from a nonexistent one.
	ErrVoiceNotFound = errors.New("voice not found")
	ErrVoiceDeleted  = errors.New("voice is deleted")
	// ErrTemplateNotFound covers unknown and foreign templates alike, like ErrVoiceNotFound.
	// There is deliberately no ErrTemplateRequired: a post may have none, and clearing the
	// assignment is a valid save rather than a missing value.
	ErrTemplateNotFound = errors.New("template not found")
	// ErrLanguageRequired is returned at create/machine-write boundaries when the
	// caller supplies no concrete supported language.
	ErrLanguageRequired = errors.New("a content language is required")
	// ErrTemplateAnswerInvalid is an answer with no label, or two answers in one request
	// naming the same one. Both are the client sending something no screen can produce, so
	// they share a refusal rather than each carrying their own wire reason.
	ErrTemplateAnswerInvalid = errors.New("a template answer needs a label, and one label at most once")
	// ErrPostPublished is any write to a published post other than replacing or clearing its
	// address and deleting it (POST-86 names the way out: clear the address).
	ErrPostPublished = errors.New("post is published")
	// ErrPublishedURLInvalid is an address that is not a Naver Blog post's (POST-77).
	ErrPublishedURLInvalid = errors.New("not a naver blog post address")
	// ErrFieldNotFound is a 분야 that is not on the product's list (QUAL-23).
	ErrFieldNotFound = errors.New("blog field not found")
	// ErrQualityRuleInvalid is a quality tick that names none of the four metrics.
	ErrQualityRuleInvalid = errors.New("quality rule is invalid")
)

// TemplateAnswer is what a post answers to one data field its template declared (POST-62).
//
// Label is the field's title AND the key it is stored under: it is text the template
// authored, not an id, so a rename or a template swap leaves the answer alone rather than
// destroying what someone typed.
//
// Enabled false means "I have nothing for this". Text is kept either way — the switch and an
// empty Text mean the same thing to the enqueue, and losing what was typed would make trying
// a field twice expensive.
type TemplateAnswer struct {
	Label   string
	Text    string
	Enabled bool
}

// TemplateAnswerTooLongError names which half was too long and both counts, so the handler
// builds one message without re-deriving the limit.
type TemplateAnswerTooLongError struct {
	Field string
	Chars int
	Max   int
}

func (e *TemplateAnswerTooLongError) Error() string {
	return fmt.Sprintf("template answer %s has %d characters; at most %d are allowed", e.Field, e.Chars, e.Max)
}

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

// TemplateRef is the template a post is written for, as the post context needs it: the id it
// stores plus the name the template context publishes. An empty ID is the ordinary case —
// a post without a template — not a missing value.
type TemplateRef struct {
	ID   string
	Name string
	// The two generation numbers the template authored (TEMPLATE-47). nil is "no opinion":
	// assigning this template then leaves the post's own option alone. They are read here
	// only to SEED an assignment - nothing projects them onto a read model, and no prompt
	// ever sees a template's number.
	TargetLength *int
	TagCount     *int
}

// TemplateNumbers is what an assignment seeds. A nil member is a number the template has no
// opinion about, which keeps the post's own value (TEMPLATE-48).
type TemplateNumbers struct {
	TargetLength *int
	TagCount     *int
}

// Seeds reports what this template writes onto a post it is assigned to.
func (r TemplateRef) Seeds() TemplateNumbers {
	return TemplateNumbers{TargetLength: r.TargetLength, TagCount: r.TagCount}
}

// Post is the aggregate exposed by the drafting context. Generation may replace its
// canonical content and observations only through Service's published behaviors. The post
// stores only VoiceID; Voice is enriched on read through the VoiceDirectory port.
type Post struct {
	Slug    string
	UserID  string
	VoiceID string
	Voice   VoiceRef
	// TemplateID is empty when the post has no template. Like VoiceID the post stores only
	// the id; Template is enriched on read through the TemplateDirectory port.
	TemplateID              string
	Template                TemplateRef
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
	// TagCount is how many tags a run asks for (POST-63). Always concrete: the store reads a
	// row never saved with one as TagCountRange.Default, so no caller sees "unset". The
	// one exception is a Post being CREATED, where 0 means "nobody named one" and the column
	// stays NULL - a create seeds it only when the template it names has an opinion.
	TagCount int
	// UseMemory is whether a run may carry the account's memories (MEM-18). A generation
	// option, default false, and false is what every row saved before memories existed
	// reads as — which is what keeps such a post's prompt byte-identical to today's.
	UseMemory         bool
	FinalizedRevision int64
	FinalizedAt       *time.Time
	// PublishedURL is the post's Naver Blog address, stored normalized, and "" when it has
	// none; PublishedAt is when it was recorded, restamped by a replacement (POST-73, POST-75).
	PublishedURL string
	PublishedAt  *time.Time
	// ContentNouns are the distinct nouns the latest write pass returned for this content, nil
	// when none were returned (GEN-55).
	ContentNouns []string
	// ReplacementCandidates are the spans the latest generation offered 분야 phrases for, in
	// the order it returned them, nil for none (GEN-53). Like the nouns they live beside the
	// content, never in it: the content is what the baseline, exports and learning copy (R29).
	ReplacementCandidates []ReplacementCandidate
	// Field is the post's 분야 as its ASCII id, "" for 없음 (QUAL-23). Like the quality ticks
	// below it is an input of the next run, never an edit of the post (POST-82).
	Field string
	// QualityRules are the quality metrics the owner ticked for the next run, as metric ids in
	// canonical order, nil for none (POST-81).
	QualityRules []string
	Observations []Observation

	// TemplateAnswers is what this post answers to its template's data fields, by label,
	// ordered by label. Populated by Get like Images and Videos are.
	TemplateAnswers []TemplateAnswer

	// Images and Videos are populated by Get, not by the store's post lookup.
	Images              []Image
	Videos              []Video
	ActiveJob           *ActiveJob
	PendingExperimentID string
}

// FinalizedAtCurrentRevision is whether the post's current revision is its finalized one — a
// finalized post, or a published one, which is a finalized post with an address.
func (p Post) FinalizedAtCurrentRevision() bool {
	return finalizedAt(p.Status, p.FinalizedRevision, p.ContentRevision)
}

// ReplacementSurface is where a replacement candidate stands in the content (GEN-53).
type ReplacementSurface string

const (
	ReplacementSurfaceTitle ReplacementSurface = "title"
	ReplacementSurfaceTag   ReplacementSurface = "tag"
	ReplacementSurfaceBody  ReplacementSurface = "body"
)

func (s ReplacementSurface) Valid() bool {
	switch s {
	case ReplacementSurfaceTitle, ReplacementSurfaceTag, ReplacementSurfaceBody:
		return true
	}
	return false
}

// ReplacementCandidate is one span a write offered listed 분야 phrases for: its surface, the
// tag's index for a tag or the block's for body and 0 for the title, the exact text written
// there, and the phrases offered in its place (GEN-53).
type ReplacementCandidate struct {
	Surface ReplacementSurface
	Index   int
	Source  string
	Phrases []string
}

// WriteAnnotations is what a machine write says about its content beside it: the nouns it
// used (GEN-55) and the spans it offered replacements for (GEN-53). Empty is a real answer —
// none — and clears what the post held.
type WriteAnnotations struct {
	Nouns      []string
	Candidates []ReplacementCandidate
}

// ContentSnapshot is one owned post's current content with what a reader that measures it needs
// beside it, read from the post row alone.
type ContentSnapshot struct {
	Slug            string
	ContentRevision int64
	Content         *PostContent // nil before the first write
	ContentLanguage *Language
	TargetLanguage  Language
	Nouns           []string
}

// PublishedPost is one published post as the quality context reads the account's window
// (QUAL-39). A nil ContentLanguage is for the reader to interpret; post does not default it.
type PublishedPost struct {
	Slug            string
	ContentRevision int64
	Content         PostContent
	ContentLanguage *Language
	Nouns           []string
	PublishedAt     time.Time
}

// LearningSnapshot is the post context's ownership-checked hand-off to voice. The
// voice context never reads post tables and cannot mutate either snapshot. Status and
// FinalizedRevision are the row it was read from, so the service can apply the finalization
// rule to that read rather than to an earlier one.
type LearningSnapshot struct {
	PostSlug               string
	UserID                 string
	VoiceID                string
	MachineBaselineVoiceID string
	Status                 string
	Current                PostContent
	ContentRevision        int64
	FinalizedRevision      int64
	MachineBaseline        PostContent
	BaselineRevision       int64
	TargetLength           *int
	FinalizedAt            time.Time
	UpdatedAt              time.Time
	ContentLanguage        Language
	VoiceSourceLanguage    Language
}

// FinalizedAtCurrentRevision is whether the snapshot's row holds a finalized current revision.
func (s LearningSnapshot) FinalizedAtCurrentRevision() bool {
	return finalizedAt(s.Status, s.FinalizedRevision, s.ContentRevision)
}

// finalizedAt is the one rule for "the current revision is the finalized one". A published
// post is a finalized one with an address (POST-21, POST-73).
func finalizedAt(status string, finalizedRevision, contentRevision int64) bool {
	return (status == StatusFinalized || status == StatusPublished) && finalizedRevision == contentRevision
}

// BlockType is kept as the LLM/protojson spelling at the domain boundary.
type BlockType string

const (
	BlockText    BlockType = "TEXT"
	BlockHeading BlockType = "HEADING"
	BlockImage   BlockType = "IMAGE"
	BlockQuote   BlockType = "QUOTE"
	BlockList    BlockType = "LIST"
	// BlockVideo carries the IMAGE fields and none of its own (VIDEO-2). Its File names an
	// attached VIDEO, never a photo — the two are one filename namespace, so a mismatch is
	// the wrong block type rather than an unknown file.
	BlockVideo BlockType = "VIDEO"
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
	// Model is the ref that produced this entry. Per-entry rather than per-snapshot: a run
	// may re-observe only some photos, so one snapshot can hold two models' work. Empty on
	// an entry written before this field existed, which reads as unknown.
	Model string
	// Events and Speech are what a still frame cannot carry, so only a video entry has them:
	// what happens in the clip in order, and what is said or heard, summarized (VIDEO-9).
	// They are omitted from the stored JSON when empty, so every row written before videos
	// existed decodes unchanged.
	Events []string
	Speech string
}

// Summary is a row of the post list.
type Summary struct {
	Slug                string
	VoiceID             string
	Voice               VoiceRef
	TemplateID          string
	Template            TemplateRef
	Title               string
	Status              string
	UpdatedAt           time.Time
	ActiveJob           *ActiveJob
	PendingExperimentID string
	TargetLanguage      Language
	ContentLanguage     *Language
	// Tags of the current content revision (POST-65). The list narrows by them, and a post
	// whose content has not been written yet simply carries none.
	Tags []string
	// Cursor is the row's position in the list order as the store holds it. It becomes the
	// next page token and is never shown (POST-90).
	Cursor ListCursor
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
	// WritesContent marks a job that writes this post's content or observations when it
	// completes. The composition root sets it, so this context never names a job kind.
	WritesContent bool
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

// Video is a clip attached to a post. Like an Image this is the record that names bytes
// living in object storage — but the bytes are exactly what the user picked, because
// nothing transcodes, downscales or thumbnails a video anywhere (VIDEO-4).
type Video struct {
	ID       string
	PostSlug string
	Filename string
	Key      string
	// ContentType is the container's type. It is part of the PUT signature, so it is also
	// what the stored object must report back at confirm.
	ContentType string
	Bytes       int64
	// DurationMs and the dimensions are client-reported: the server never opens a
	// container. A forged confirm can only spend the forger's own credits (VIDEO-5).
	DurationMs int64
	Width      int32
	Height     int32
	CreatedAt  time.Time

	// ViewURL is a short-lived presigned GET, minted per read and never stored.
	ViewURL string
}

// Attachment is what one confirm produced: exactly one of Image or Video, named by Kind.
// The kind comes from the UPLOAD row rather than from the confirming request, so a client
// cannot turn a photo reservation into a video by asking.
type Attachment struct {
	Kind  AttachmentKind
	Image Image
	Video Video
}

// Upload is a presigned PUT that has not been confirmed yet.
type Upload struct {
	ID       string
	PostSlug string
	Filename string
	Key      string
	// Kind decides which table the confirm writes to and which rules it applies. Rows
	// written before videos existed read as photo, which is what they are.
	Kind AttachmentKind
	// ContentType is what the PUT was signed for: always image/jpeg for a photo, the
	// container's own type for a video.
	ContentType string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// ObjectKey is the storage key for a photo (PRD §5). The image id is in the key rather
// than the filename so that a rename or a duplicate name can never collide, and so the
// key is not attacker-influenced.
func ObjectKey(postSlug, imageID string) string {
	return "posts/" + postSlug + "/" + imageID + ".jpg"
}

// VideoObjectKey is the storage key for a video: the same prefix and the same
// id-not-filename rule as a photo, with the container's own extension so the object is
// what it says it is. Sharing the prefix is what lets the orphan sweep keep one listing.
func VideoObjectKey(postSlug, videoID, extension string) string {
	return "posts/" + postSlug + "/" + videoID + "." + extension
}

// The quality metrics a post may tick, by their ASCII ids (R3). They mirror the quality
// context's own four as a closed list, because post imports no other context (ARCH-7).
const (
	QualityRuleTitleSaturation  = "title_saturation"
	QualityRuleCrossPostPhrases = "cross_post_phrases"
	QualityRuleInPostRepetition = "in_post_repetition"
	QualityRuleComposition      = "composition"
)

// qualityRules is the canonical order, the one the quality context renders rows in.
var qualityRules = []string{
	QualityRuleTitleSaturation, QualityRuleCrossPostPhrases, QualityRuleInPostRepetition, QualityRuleComposition,
}

// NormalizeQualityRules validates a tick set and returns its distinct ids in canonical order,
// which makes an equality check blind to the order they were ticked in. An id outside the four
// is ErrQualityRuleInvalid; an empty set is an empty, non-nil slice.
func NormalizeQualityRules(ids []string) ([]string, error) {
	ticked := make(map[string]bool, len(ids))
	for _, id := range ids {
		known := false
		for _, rule := range qualityRules {
			if id == rule {
				known = true
				break
			}
		}
		if !known {
			return nil, ErrQualityRuleInvalid
		}
		ticked[id] = true
	}
	out := make([]string, 0, len(ticked))
	for _, rule := range qualityRules {
		if ticked[rule] {
			out = append(out, rule)
		}
	}
	return out, nil
}

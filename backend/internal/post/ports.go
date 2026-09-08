package post

import (
	"context"
	"errors"
	"time"
)

// ErrObjectNotFound is what an ObjectStore reports for a key that is not there. It is
// the store's job to translate whatever its SDK raises into this.
var ErrObjectNotFound = errors.New("object not found")

// ObjectHead is what a HEAD reports about a stored object. The content type is part of
// it because a video's PUT was signed for its container type, so what the object reports
// back is the one server-side check that the bytes are the kind of thing that was
// reserved — a photo's type is fixed and cannot say anything.
type ObjectHead struct {
	Size        int64
	ContentType string
}

// Object is one entry of a storage listing.
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// ObjectStore is the object storage this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). The implementation lives in internal/storage; nothing here knows
// it is S3, R2, or anything else.
//
// Note what is absent: there is no Put and no Get. Photo bytes never pass through this
// process ([I6]) — the browser talks to storage directly with a URL this port signs.
type ObjectStore interface {
	// PresignPut returns a URL the browser may PUT to. The contentType is part of the
	// signature, so the caller must send the same one back to the client.
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	// PresignGet returns a short-lived read URL for a private object.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Head returns what storage knows about the object, or ErrObjectNotFound.
	Head(ctx context.Context, key string) (ObjectHead, error)
	// Delete removes the object. Deleting a key that is not there is not an error.
	Delete(ctx context.Context, key string) error
	// List returns every object under a prefix.
	List(ctx context.Context, prefix string) ([]Object, error)
}

// ActiveJobFinder is the one published behavior post reads from the job context. The
// post store never reaches across the context boundary to generation_jobs.
type ActiveJobFinder interface {
	ActiveForPost(ctx context.Context, slug string) (*ActiveJob, error)
}

// LivePublishFinder answers one question for DeletePost: is a publication of this exact
// post incarnation still in flight? The post context must not import internal/publishing,
// so the port speaks only in primitives and the composition root adapts it. createdAt
// distinguishes this post from an earlier one that held the same slug.
type LivePublishFinder interface {
	LiveForPost(ctx context.Context, userID, slug string, createdAt time.Time) (bool, error)
}

type PendingExperimentFinder interface {
	PendingForPost(ctx context.Context, userID, slug string) (string, error)
}

// VoiceDirectory is the voice context's published directory, consumed here to validate an
// assignment and to name a post's voice on read models. It lists tombstones too, so a post
// whose voice was deleted can still be projected; the post context never reads voice tables.
type VoiceDirectory interface {
	Voices(ctx context.Context, userID string) ([]VoiceRef, error)
}

// TemplateDirectory is the template context's published directory, consumed here to validate
// an assignment and to name a post's template on read models. It is a directory rather than a
// single lookup for the same reason VoiceDirectory is: the list read model needs every name
// at once, and the post context never reads the templates table.
//
// A template has no tombstone, so a deleted one simply stops appearing — and the composite
// foreign key has already set the assignment to NULL by then.
type TemplateDirectory interface {
	Templates(ctx context.Context, userID string) ([]TemplateRef, error)
}

// ExperimentContentPurger is the required privacy hook for post deletion. The post
// context calls it before the FK detaches experiment history from the source slug.
type ExperimentContentPurger interface {
	PurgePost(ctx context.Context, userID, postSlug string) error
}

// GuidelineCandidateDetacher drops the post link from any guideline candidate that named a
// post being deleted (change 26). Only the link goes: a candidate is a receipt for something
// the user wrote, and nothing references its origin, so the text stays reviewable without it.
type GuidelineCandidateDetacher interface {
	DetachPost(ctx context.Context, userID, postSlug string) error
}

// Store is the persistence this context needs.
//
// Ownership is a property of the query, not of a check the caller must remember: the
// post-scoped lookups take the post, and the service resolves the post's owner first.
type Store interface {
	CreatePost(ctx context.Context, p Post) error
	UpdateDraft(ctx context.Context, slug, userID, title, memo string, targetLanguage *Language, updatedAt time.Time) (bool, error)
	UpdateObservations(ctx context.Context, slug, userID string, observations []Observation, updatedAt time.Time) (bool, error)
	UpdateGeneratedContent(ctx context.Context, slug, userID string, content PostContent, language Language, updatedAt time.Time) (bool, error)
	GetPost(ctx context.Context, slug string) (Post, error)
	// ReassignVoice moves the post to another voice in one statement that also drops the
	// machine baseline's voice association — the part of the post that belonged to the old
	// voice. Content, revisions, photos and finalization state are untouched. It reports
	// false when the post already carried that voice.
	ReassignVoice(ctx context.Context, slug, userID, voiceID string, updatedAt time.Time) (bool, error)
	// AssignTemplate sets or clears (nil) the post's template. It writes that column and
	// updated_at and nothing else: a template is never learned from, so unlike a voice
	// reassignment it must not disturb content, revisions, the machine baseline or
	// finalization, and it is allowed in every status.
	AssignTemplate(ctx context.Context, slug, userID string, templateID *string, updatedAt time.Time) (bool, error)
	// UpsertTemplateAnswers writes one row per answer in ONE transaction, keyed by label.
	// It never deletes: clearing an answer is an empty Text, which the enqueue reads the way
	// it reads a switched-off field, and a label the current template no longer declares is
	// kept so a rename or a swap back does not destroy what was typed.
	UpsertTemplateAnswers(ctx context.Context, slug string, answers []TemplateAnswer, updatedAt time.Time) error
	// ListTemplateAnswers returns the post's answers ordered by label.
	ListTemplateAnswers(ctx context.Context, slug string) ([]TemplateAnswer, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	ListPosts(ctx context.Context, userID string) ([]Summary, error)
	DeletePost(ctx context.Context, slug, userID string) (bool, error)

	ListImages(ctx context.Context, postSlug string) ([]Image, error)
	GetImage(ctx context.Context, id string) (Image, error)
	DeleteImage(ctx context.Context, id string) error
	// ImageFilenameTaken reports a CONFIRMED photo with this name. A pending upload
	// does not count — that case is a retry, which CreateUpload replaces.
	ImageFilenameTaken(ctx context.Context, postSlug, filename string) (bool, error)
	// ImageKeyInUse reports whether a photo row points at this object key. The sweep
	// asks before deleting anything an uploads row named.
	ImageKeyInUse(ctx context.Context, key string) (bool, error)

	ListVideos(ctx context.Context, postSlug string) ([]Video, error)
	GetVideo(ctx context.Context, id string) (Video, error)
	DeleteVideo(ctx context.Context, id string) error
	// VideoFilenameTaken reports a CONFIRMED video with this name. CreateUpload asks it
	// alongside ImageFilenameTaken: one post has ONE filename namespace across both kinds.
	VideoFilenameTaken(ctx context.Context, postSlug, filename string) (bool, error)
	// CountVideos is the ceiling check. Unlike the photo one it counts in SQL: the row
	// carries bytes and a duration nobody is reading here.
	CountVideos(ctx context.Context, postSlug string) (int, error)
	// VideoKeyInUse is ImageKeyInUse for the other table, asked for the same reason.
	VideoKeyInUse(ctx context.Context, key string) (bool, error)

	CreateUpload(ctx context.Context, u Upload) error
	GetUpload(ctx context.Context, id string) (Upload, error)
	GetUploadByFilename(ctx context.Context, postSlug, filename string) (Upload, error)
	DeleteUpload(ctx context.Context, id string) error
	ListUploadsExpiredBefore(ctx context.Context, t time.Time) ([]Upload, error)

	// ConfirmVideoUpload records the video and drops the upload row ATOMICALLY, for the
	// same reason ConfirmUpload does — see the note below.
	ConfirmVideoUpload(ctx context.Context, video Video, uploadID string) error

	// ConfirmUpload records the photo and drops the upload row ATOMICALLY.
	//
	// The two must not be separate statements. If the image landed and the upload row
	// survived, that row would still name the live photo's object — and the sweep,
	// seeing it expired, would delete the bytes out from under a photo that looks fine
	// in the database.
	ConfirmUpload(ctx context.Context, img Image, uploadID string) error

	// AllReferencedKeys is every object key the database still points at — images, videos
	// and in-flight uploads together, read as one consistent snapshot. The sweep deletes
	// what is missing from this set, so a key lost to a race here is a deleted photo.
	AllReferencedKeys(ctx context.Context) (map[string]struct{}, error)
}

// ContentStore is the progressive editor capability. It is separated from the base
// drafting store so upload/sweeper collaborators do not acquire unrelated methods.
type ContentStore interface {
	SaveContent(ctx context.Context, slug, userID string, content PostContent, expectedRevision int64, updatedAt time.Time) (bool, error)
	SaveGenerationOptions(ctx context.Context, slug, userID string, targetLength *int, updatedAt time.Time) (bool, error)
	// Finalize also writes title, which the caller has already resolved: the confirmed content's
	// title, or the post's existing one when that is empty. The copy rides the same guarded
	// statement as the finalization, so it can never land without it.
	Finalize(ctx context.Context, slug, userID, title string, expectedRevision int64, finalizedAt time.Time) (bool, error)
	LearningSnapshot(ctx context.Context, slug, userID string) (LearningSnapshot, error)
}

package post

import (
	"context"
	"errors"
	"time"
)

// ErrObjectNotFound is what an ObjectStore reports for a key that is not there. It is
// the store's job to translate whatever its SDK raises into this.
var ErrObjectNotFound = errors.New("object not found")

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
	// Head returns the stored size, or ErrObjectNotFound.
	Head(ctx context.Context, key string) (int64, error)
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

type PendingExperimentFinder interface {
	PendingForPost(ctx context.Context, userID, slug string) (string, error)
}

// ExperimentContentPurger is the required privacy hook for post deletion. The post
// context calls it before the FK detaches experiment history from the source slug.
type ExperimentContentPurger interface {
	PurgePost(ctx context.Context, userID, postSlug string) error
}

// Store is the persistence this context needs.
//
// Ownership is a property of the query, not of a check the caller must remember: the
// post-scoped lookups take the post, and the service resolves the post's owner first.
type Store interface {
	CreatePost(ctx context.Context, p Post) error
	UpdateDraft(ctx context.Context, slug, userID, title, memo string, updatedAt time.Time) (bool, error)
	UpdateObservations(ctx context.Context, slug, userID string, observations []Observation, updatedAt time.Time) (bool, error)
	UpdateGeneratedContent(ctx context.Context, slug, userID string, content PostContent, updatedAt time.Time) (bool, error)
	GetPost(ctx context.Context, slug string) (Post, error)
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

	CreateUpload(ctx context.Context, u Upload) error
	GetUpload(ctx context.Context, id string) (Upload, error)
	GetUploadByFilename(ctx context.Context, postSlug, filename string) (Upload, error)
	DeleteUpload(ctx context.Context, id string) error
	ListUploadsExpiredBefore(ctx context.Context, t time.Time) ([]Upload, error)

	// ConfirmUpload records the photo and drops the upload row ATOMICALLY.
	//
	// The two must not be separate statements. If the image landed and the upload row
	// survived, that row would still name the live photo's object — and the sweep,
	// seeing it expired, would delete the bytes out from under a photo that looks fine
	// in the database.
	ConfirmUpload(ctx context.Context, img Image, uploadID string) error

	// AllReferencedKeys is every object key the database still points at — images and
	// in-flight uploads together, read as one consistent snapshot. The sweep deletes
	// what is missing from this set, so a key lost to a race here is a deleted photo.
	AllReferencedKeys(ctx context.Context) (map[string]struct{}, error)
}

// ContentStore is the progressive editor capability. It is separated from the base
// drafting store so upload/sweeper collaborators do not acquire unrelated methods.
type ContentStore interface {
	SaveContent(ctx context.Context, slug, userID string, content PostContent, expectedRevision int64, targetLength int, updatedAt time.Time) (bool, error)
	LearningSnapshot(ctx context.Context, slug, userID string) (LearningSnapshot, error)
}

type MachineContentStore interface {
	UpdateGeneratedContentWithTarget(ctx context.Context, slug, userID string, content PostContent, targetLength int, updatedAt time.Time) (bool, error)
}

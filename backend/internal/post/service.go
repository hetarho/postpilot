package post

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Service is the drafting context's behavior. Every method takes the acting user id
// from the caller (the interceptor put it in the context) and never from a payload.
type Service struct {
	store    Store
	blobs    ObjectStore
	putTTL   time.Duration
	getTTL   time.Duration
	maxBytes int64
	jobs     ActiveJobFinder

	// now and newID are seams for tests in this package, not configuration.
	now   func() time.Time
	newID func() string
}

// NewService wires the context with its store, its object storage, the presigned URL
// lifetimes, and the largest object it will accept as a photo.
func NewService(store Store, blobs ObjectStore, putTTL, getTTL time.Duration, maxBytes int64, jobs ...ActiveJobFinder) *Service {
	svc := &Service{
		store:    store,
		blobs:    blobs,
		putTTL:   putTTL,
		getTTL:   getTTL,
		maxBytes: maxBytes,
		now:      time.Now,
		newID:    newObjectID,
	}
	if len(jobs) > 0 {
		svc.jobs = jobs[0]
	}
	return svc
}

// SaveDraft creates the post when slug is empty, otherwise updates the caller's own.
//
// It is the autosave endpoint, so it is called every second or so while someone types:
// repeated saves are plain idempotent updates, and only the very first one mints a slug.
func (s *Service) SaveDraft(ctx context.Context, userID, slug, title, memo string) (Post, error) {
	if slug == "" {
		return s.createPost(ctx, userID, title, memo)
	}

	if _, err := s.ownedPost(ctx, userID, slug); err != nil {
		return Post{}, err
	}

	now := s.now()
	updated, err := s.store.UpdateDraft(ctx, slug, userID, title, memo, now)
	if err != nil {
		return Post{}, fmt.Errorf("update draft: %w", err)
	}
	if !updated {
		// ownedPost just succeeded, so a miss here means the row vanished between the
		// two statements. Report it as gone rather than inventing a post.
		return Post{}, ErrNotFound
	}

	return s.Get(ctx, userID, slug)
}

// slugAttempts bounds the mint-and-insert retry. Minting reads the slugs that exist,
// so a retry only happens when another request took the candidate in between — rare,
// and each attempt sees one more taken slug, so it converges immediately.
const slugAttempts = 5

func (s *Service) createPost(ctx context.Context, userID, title, memo string) (Post, error) {
	now := s.now()

	// Mint-then-insert is a check-then-act, so the insert is what actually decides:
	// two first-saves of the same title on the same day can mint the same candidate,
	// and the loser must get the next serial rather than an error.
	for attempt := 0; attempt < slugAttempts; attempt++ {
		// exists is consulted per candidate rather than up front: the collision case is
		// rare, and pre-loading every slug for the day to avoid one query would be worse.
		var lookupErr error
		slug := MintSlug(now.UTC().Format("20060102"), title, func(candidate string) bool {
			if lookupErr != nil {
				return false
			}
			taken, err := s.store.SlugExists(ctx, candidate)
			if err != nil {
				lookupErr = err
			}
			return taken
		})
		if lookupErr != nil {
			return Post{}, fmt.Errorf("check slug: %w", lookupErr)
		}

		created := Post{
			Slug:      slug,
			UserID:    userID,
			Title:     title,
			Memo:      memo,
			Status:    StatusDraft,
			CreatedAt: now,
			UpdatedAt: now,
		}
		err := s.store.CreatePost(ctx, created)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, ErrDuplicateSlug) {
			return Post{}, fmt.Errorf("create post: %w", err)
		}
		// Someone took it between the check and the insert. The next mint sees it.
	}

	return Post{}, fmt.Errorf("create post: could not mint a free slug in %d attempts", slugAttempts)
}

// Get returns the caller's post with a fresh view URL on every image.
//
// The URLs are minted here, per read, rather than stored: the bucket is private and a
// stored URL would either be long-lived (a durable capability to a private object) or
// already expired by the time it is used.
func (s *Service) Get(ctx context.Context, userID, slug string) (Post, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return Post{}, err
	}

	images, err := s.store.ListImages(ctx, slug)
	if err != nil {
		return Post{}, fmt.Errorf("list images: %w", err)
	}
	for i := range images {
		url, err := s.blobs.PresignGet(ctx, images[i].Key, s.getTTL)
		if err != nil {
			return Post{}, fmt.Errorf("presign view url for %s: %w", images[i].Filename, err)
		}
		images[i].ViewURL = url
	}

	found.Images = images
	if s.jobs != nil {
		found.ActiveJob, err = s.jobs.ActiveForPost(ctx, slug)
		if err != nil {
			return Post{}, fmt.Errorf("load active job: %w", err)
		}
	}
	return found, nil
}

// List returns the caller's posts, newest first.
func (s *Service) List(ctx context.Context, userID string) ([]Summary, error) {
	summaries, err := s.store.ListPosts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	if s.jobs != nil {
		for i := range summaries {
			summaries[i].ActiveJob, err = s.jobs.ActiveForPost(ctx, summaries[i].Slug)
			if err != nil {
				return nil, fmt.Errorf("load active job for %s: %w", summaries[i].Slug, err)
			}
		}
	}
	return summaries, nil
}

// AttachedImages returns an ownership-checked generation snapshot without presigning
// browser URLs. Object keys remain backend-only and are consumed by the generation
// context's storage port.
func (s *Service) AttachedImages(ctx context.Context, userID, slug string) (Post, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return Post{}, err
	}
	found.Images, err = s.store.ListImages(ctx, slug)
	if err != nil {
		return Post{}, fmt.Errorf("list attached images: %w", err)
	}
	return found, nil
}

// SetObservations replaces the persisted contact sheet snapshot for one owned post.
func (s *Service) SetObservations(ctx context.Context, userID, slug string, observations []Observation) error {
	if _, err := s.ownedPost(ctx, userID, slug); err != nil {
		return err
	}
	updated, err := s.store.UpdateObservations(ctx, slug, userID, observations, s.now())
	if err != nil {
		return err
	}
	if !updated {
		return ErrNotFound
	}
	return nil
}

// SetGeneratedContent atomically replaces canonical content and moves the post to review.
func (s *Service) SetGeneratedContent(ctx context.Context, userID, slug string, content PostContent) error {
	if _, err := s.ownedPost(ctx, userID, slug); err != nil {
		return err
	}
	updated, err := s.store.UpdateGeneratedContent(ctx, slug, userID, content, s.now())
	if err != nil {
		return err
	}
	if !updated {
		return ErrNotFound
	}
	return nil
}

// CreateUpload reserves a filename and hands back a presigned PUT.
//
// The image id is minted now, not at confirm time, because the object key contains it:
// the browser has to PUT to the final key, and the server has to be able to find that
// object again from an upload_id alone after a restart.
func (s *Service) CreateUpload(ctx context.Context, userID, postSlug, filename string) (Upload, string, string, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return Upload{}, "", "", fmt.Errorf("filename is required")
	}
	if _, err := s.ownedPost(ctx, userID, postSlug); err != nil {
		return Upload{}, "", "", err
	}

	// A CONFIRMED photo with this name is a real conflict — the filename is how the
	// model and the exporters address a photo, so two cannot share one.
	taken, err := s.store.ImageFilenameTaken(ctx, postSlug, filename)
	if err != nil {
		return Upload{}, "", "", fmt.Errorf("check filename: %w", err)
	}
	if taken {
		return Upload{}, "", "", ErrDuplicateFilename
	}
	// A PENDING upload with this name is not a conflict, it is a retry: the plan gives
	// every photo a retry button that restarts from here with a fresh id. Refusing
	// would strand the user until the sweep ran, up to an hour later.
	if err := s.replacePendingUpload(ctx, postSlug, filename); err != nil {
		return Upload{}, "", "", err
	}

	now := s.now()
	upload := Upload{
		ID:        s.newID(),
		PostSlug:  postSlug,
		Filename:  filename,
		ExpiresAt: now.Add(s.putTTL),
		CreatedAt: now,
	}
	upload.Key = ObjectKey(postSlug, upload.ID)

	url, err := s.blobs.PresignPut(ctx, upload.Key, uploadContentType, s.putTTL)
	if err != nil {
		return Upload{}, "", "", fmt.Errorf("presign upload url: %w", err)
	}
	// The row is written after the presign succeeds: a row with no usable URL would be
	// swept later as an orphan for no reason.
	if err := s.store.CreateUpload(ctx, upload); err != nil {
		// The UNIQUE(post_slug, filename) constraint fired, which means another request
		// claimed the name between the checks above and this insert.
		if errors.Is(err, ErrDuplicateFilename) {
			return Upload{}, "", "", ErrDuplicateFilename
		}
		return Upload{}, "", "", fmt.Errorf("create upload: %w", err)
	}

	return upload, url, uploadContentType, nil
}

// replacePendingUpload clears an unconfirmed upload holding this filename so a retry
// can take it. Its object goes too — nothing will ever reference that key again.
func (s *Service) replacePendingUpload(ctx context.Context, postSlug, filename string) error {
	pending, err := s.store.GetUploadByFilename(ctx, postSlug, filename)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check pending upload: %w", err)
	}

	// Row first, then object. The reverse order could delete the bytes and then fail,
	// leaving a row that promises an object which is not there.
	if err := s.store.DeleteUpload(ctx, pending.ID); err != nil {
		return fmt.Errorf("clear pending upload: %w", err)
	}
	if err := s.blobs.Delete(ctx, pending.Key); err != nil {
		// Not fatal: the object is now unreferenced, which is exactly what the sweep
		// collects. Failing the retry over it would be the worse outcome.
		slog.Warn("could not delete a replaced upload's object", "key", pending.Key, "err", err)
	}
	return nil
}

// ConfirmUpload turns a landed object into a photo.
//
// The size comes from storage, never from the client: the browser reports width and
// height because only it decoded the image ([I6]), but bytes is something the server
// can check, so it does — and it refuses an object too large to be one of our photos.
//
// It is idempotent. A client that never saw the response retries, and a retry has to
// return the photo rather than a primary-key failure.
func (s *Service) ConfirmUpload(ctx context.Context, userID, uploadID string, width, height int32) (Image, error) {
	if width <= 0 || height <= 0 || width > maxImageDimension || height > maxImageDimension {
		return Image{}, fmt.Errorf("%w: dimensions %dx%d", ErrInvalidImage, width, height)
	}

	upload, err := s.store.GetUpload(ctx, uploadID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// No upload row: either this id never existed, or it was already confirmed
			// and the client is retrying.
			return s.alreadyConfirmed(ctx, userID, uploadID)
		}
		return Image{}, fmt.Errorf("load upload: %w", err)
	}
	if _, err := s.ownedPost(ctx, userID, upload.PostSlug); err != nil {
		return Image{}, err
	}

	size, err := s.blobs.Head(ctx, upload.Key)
	if err != nil {
		if errors.Is(err, ErrObjectNotFound) {
			// The PUT never landed, or landed elsewhere. Leaving the uploads row in
			// place lets the client retry with the same id, and the sweep collects it
			// if the client gives up.
			return Image{}, ErrObjectMissing
		}
		return Image{}, fmt.Errorf("head uploaded object: %w", err)
	}
	// The browser caps size before uploading, but a presigned PUT is a URL an
	// authenticated client can use however it likes — so the cap is enforced where it
	// can actually be trusted. The object is dropped rather than left for the sweep,
	// which would keep paying for it for an hour.
	if size <= 0 || size > s.maxBytes {
		if err := s.blobs.Delete(ctx, upload.Key); err != nil {
			slog.Warn("could not delete a rejected upload's object", "key", upload.Key, "err", err)
		}
		if err := s.store.DeleteUpload(ctx, upload.ID); err != nil {
			slog.Warn("could not delete a rejected upload row", "upload_id", upload.ID, "err", err)
		}
		return Image{}, fmt.Errorf("%w: %d bytes", ErrInvalidImage, size)
	}

	image := Image{
		ID:        upload.ID,
		PostSlug:  upload.PostSlug,
		Filename:  upload.Filename,
		Key:       upload.Key,
		Width:     width,
		Height:    height,
		Bytes:     size,
		CreatedAt: s.now(),
	}
	// One transaction: see the note on Store.ConfirmUpload for why the two writes must
	// not be separable.
	if err := s.store.ConfirmUpload(ctx, image, upload.ID); err != nil {
		if errors.Is(err, ErrDuplicateFilename) {
			return Image{}, ErrDuplicateFilename
		}
		return Image{}, fmt.Errorf("confirm upload: %w", err)
	}

	return image, nil
}

// alreadyConfirmed answers a retry whose upload row is gone because the first attempt
// succeeded. Anything else is a genuinely unknown id.
func (s *Service) alreadyConfirmed(ctx context.Context, userID, uploadID string) (Image, error) {
	image, err := s.store.GetImage(ctx, uploadID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Image{}, ErrNotFound
		}
		return Image{}, fmt.Errorf("load image: %w", err)
	}
	if _, err := s.ownedPost(ctx, userID, image.PostSlug); err != nil {
		return Image{}, err
	}
	return image, nil
}

// DeleteImage removes the photo and its object.
//
// Storage first, then the row: the reverse order would drop the only reference to the
// object if the delete failed, leaving bytes nobody can name. This way a failure leaves
// a row whose object is gone, which the user can retry.
func (s *Service) DeleteImage(ctx context.Context, userID, imageID string) error {
	image, err := s.store.GetImage(ctx, imageID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("load image: %w", err)
	}
	if _, err := s.ownedPost(ctx, userID, image.PostSlug); err != nil {
		return err
	}

	if err := s.blobs.Delete(ctx, image.Key); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	if err := s.store.DeleteImage(ctx, imageID); err != nil {
		return fmt.Errorf("delete image row: %w", err)
	}
	return nil
}

// ownedPost resolves a slug to the caller's post, or says why not.
//
// Unknown and foreign are distinguished on purpose (PRD §7 specifies 403 for a foreign
// slug). At two users there is nothing to enumerate.
func (s *Service) ownedPost(ctx context.Context, userID, slug string) (Post, error) {
	found, err := s.store.GetPost(ctx, slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Post{}, ErrNotFound
		}
		return Post{}, fmt.Errorf("load post: %w", err)
	}
	if found.UserID != userID {
		return Post{}, ErrForbidden
	}
	return found, nil
}

// newObjectID mints an image/upload id. 16 random bytes rather than a counter or a
// timestamp: the id lands in an object key, so it must not be guessable from another
// user's id or reveal how many photos exist.
func newObjectID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("post: cannot read random bytes for an object id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

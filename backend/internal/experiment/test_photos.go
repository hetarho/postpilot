package experiment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ErrTestPhotoLimit is an owner already holding the most unattached photos one post may hold.
// It carries the failure reason a post upload uses for its photo limit, so the client already
// has its copy (→POST-31).
var ErrTestPhotoLimit = &TestRefusal{reason: "POST_PHOTO_LIMIT", message: "the owner already holds the maximum number of unattached test photos"}

// ErrTestObjectNotFound is a HEAD of a key the object store does not hold.
var ErrTestObjectNotFound = errors.New("test photo object not found")

// TestPhotoObject is one entry of a storage listing.
type TestPhotoObject struct {
	Key          string
	LastModified time.Time
}

// TestPhotoObjects is the private bucket as this context needs it: the post port's shape,
// adapted over internal/storage in cmd/api (ARCH-6). Photo bytes never pass through the API;
// the browser talks to storage with a URL this port signs (→POST-34).
type TestPhotoObjects interface {
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Head reports the stored size, or ErrTestObjectNotFound.
	Head(ctx context.Context, key string) (int64, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]TestPhotoObject, error)
}

// TestPhotoLedger is the owner's photo rows as the upload handshake reads and writes them.
type TestPhotoLedger interface {
	CreateTestPhoto(ctx context.Context, photo TestPhoto) error
	GetTestPhoto(ctx context.Context, user, id string) (TestPhoto, error)
	ConfirmTestPhoto(ctx context.Context, user, id string, bytes, width, height int64) (TestPhoto, error)
	CountUnattachedTestPhotos(ctx context.Context, user string) (int, error)
	DeleteUnattachedTestPhoto(ctx context.Context, user, id string) (bool, error)
}

// TestPhotoLimits are the post upload limits read by name, plus how long an unattached photo
// lives. A zero PhotoTTL is DefaultTestPhotoTTL, and a zero MaxUnattached lifts the cap.
type TestPhotoLimits struct {
	PutTTL, GetTTL, PhotoTTL time.Duration
	MaxBytes                 int64
	MaxUnattached            int
}

// TestPhotoUpload is one issued upload handshake: the pending row and the URL to PUT to.
type TestPhotoUpload struct {
	Photo       TestPhoto
	PutURL      string
	ContentType string
	PutExpires  time.Time
}

// TestPhotos writes, confirms and serves the photos of a photo observation test (MODEL-97).
// Every read and write derives the owner from the caller, and a foreign id is indistinguishable
// from an unknown one (MODEL-41).
type TestPhotos struct {
	ledger  TestPhotoLedger
	objects TestPhotoObjects
	limits  TestPhotoLimits
	now     func() time.Time
	newID   func() string
}

func NewTestPhotos(ledger TestPhotoLedger, objects TestPhotoObjects, limits TestPhotoLimits) *TestPhotos {
	if ledger == nil || objects == nil {
		panic("experiment: test photos need a ledger and an object store")
	}
	if limits.PhotoTTL <= 0 {
		limits.PhotoTTL = DefaultTestPhotoTTL
	}
	return &TestPhotos{ledger: ledger, objects: objects, limits: limits, now: time.Now, newID: newID}
}

// testPhotoKey is `tests/{user}/{id}.jpg`: not guessable and revealing no count (→POST-38).
func testPhotoKey(user, id string) string { return TestPhotoPrefix + user + "/" + id + ".jpg" }

// Create presigns one photo's PUT and records it pending and unattached. The row is written
// after the presign succeeds, so a failed presign leaves no reservation nothing can fill.
func (s *TestPhotos) Create(ctx context.Context, user, filename string) (TestPhotoUpload, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return TestPhotoUpload{}, ErrTestMaterialInvalid
	}
	if s.limits.MaxUnattached > 0 {
		waiting, err := s.ledger.CountUnattachedTestPhotos(ctx, user)
		if err != nil {
			return TestPhotoUpload{}, fmt.Errorf("count unattached test photos: %w", err)
		}
		if waiting >= s.limits.MaxUnattached {
			return TestPhotoUpload{}, ErrTestPhotoLimit
		}
	}
	id, now := s.newID(), s.now()
	photo := TestPhoto{ID: id, UserID: user, Filename: filename, Key: testPhotoKey(user, id), Status: TestPhotoPending, ExpiresAt: now.Add(s.limits.PhotoTTL), CreatedAt: now}
	url, err := s.objects.PresignPut(ctx, photo.Key, TestPhotoContentType, s.limits.PutTTL)
	if err != nil {
		return TestPhotoUpload{}, fmt.Errorf("presign test photo: %w", err)
	}
	if err := s.ledger.CreateTestPhoto(ctx, photo); err != nil {
		return TestPhotoUpload{}, fmt.Errorf("record test photo: %w", err)
	}
	return TestPhotoUpload{Photo: photo, PutURL: url, ContentType: TestPhotoContentType, PutExpires: now.Add(s.limits.PutTTL)}, nil
}

// Confirm turns a landed object into a ready photo. The size comes from storage, never from the
// client: the browser reports the dimensions because only it decoded the file (→POST-34), but
// bytes is something the server can check. It is idempotent, so a client that never saw the
// answer can repeat it.
func (s *TestPhotos) Confirm(ctx context.Context, user, id string, width, height int64) (TestPhoto, error) {
	photo, err := s.ledger.GetTestPhoto(ctx, user, id)
	if errors.Is(err, ErrTestPhotoNotFound) {
		return TestPhoto{}, ErrTestNotFound
	}
	if err != nil {
		return TestPhoto{}, fmt.Errorf("load test photo: %w", err)
	}
	if photo.Status == TestPhotoReady {
		return photo, nil
	}
	if width <= 0 || height <= 0 || width > maxTestPhotoDimension || height > maxTestPhotoDimension {
		return TestPhoto{}, ErrTestMaterialInvalid
	}
	size, err := s.objects.Head(ctx, photo.Key)
	if errors.Is(err, ErrTestObjectNotFound) {
		// The row stays so the PUT can be retried with the same id.
		return TestPhoto{}, ErrTestMaterialInvalid
	}
	if err != nil {
		return TestPhoto{}, fmt.Errorf("head test photo: %w", err)
	}
	// A presigned PUT is a URL the client can use however it likes, so the ceiling is enforced
	// where it can be trusted, and a rejected object is dropped at once rather than kept for the
	// sweep (→POST-36).
	if size <= 0 || size > s.limits.MaxBytes {
		s.dropRejected(ctx, photo)
		return TestPhoto{}, ErrTestMaterialInvalid
	}
	confirmed, err := s.ledger.ConfirmTestPhoto(ctx, user, id, size, width, height)
	if errors.Is(err, ErrTestPhotoNotFound) {
		return TestPhoto{}, ErrTestNotFound
	}
	if err != nil {
		return TestPhoto{}, fmt.Errorf("confirm test photo: %w", err)
	}
	return confirmed, nil
}

// dropRejected removes the row first and then its object (→POST-39): a dangling row would
// promise an object that is gone, while a leftover object is what the stray sweep reclaims.
func (s *TestPhotos) dropRejected(ctx context.Context, photo TestPhoto) {
	removed, err := s.ledger.DeleteUnattachedTestPhoto(ctx, photo.UserID, photo.ID)
	if err != nil {
		slog.WarnContext(ctx, "rejected test photo row not removed; the sweep reclaims it", "err", err)
		return
	}
	if !removed {
		return
	}
	if err := s.objects.Delete(ctx, photo.Key); err != nil {
		slog.WarnContext(ctx, "rejected test photo object not removed; the sweep reclaims it", "err", err)
	}
}

// ViewURL is a short-lived read URL for one stored photo, with the instant it stops working.
// The caller has already proved the key belongs to the owner; the key never crosses the wire.
func (s *TestPhotos) ViewURL(ctx context.Context, key string) (string, time.Time, error) {
	expires := s.now().Add(s.limits.GetTTL)
	url, err := s.objects.PresignGet(ctx, key, s.limits.GetTTL)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("presign test photo view: %w", err)
	}
	return url, expires, nil
}

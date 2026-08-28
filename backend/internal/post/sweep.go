package post

import (
	"context"
	"log/slog"
	"time"
)

// objectPrefix is everything this app owns in the bucket. The sweep never looks outside
// it, so a bucket shared with something else is not at risk from a bug here.
const objectPrefix = "posts/"

// Sweeper reclaims storage nothing points at any more.
//
// Two kinds of leak exist, and they need different evidence:
//   - An upload that was presigned but never confirmed. The uploads row names the key.
//   - An object with no row at all — a PUT that landed while the confirm was lost, or a
//     row deleted while the object delete failed. Only a listing finds these.
type Sweeper struct {
	store  Store
	blobs  ObjectStore
	minAge time.Duration

	now func() time.Time
}

// NewSweeper builds the sweeper. minAge is the grace period an object gets before it
// counts as stray.
func NewSweeper(store Store, blobs ObjectStore, minAge time.Duration) *Sweeper {
	return &Sweeper{store: store, blobs: blobs, minAge: minAge, now: time.Now}
}

// Run sweeps every interval until the context is cancelled. It does NOT sweep on start:
// a deploy loop would otherwise turn every restart into a full bucket listing.
func (s *Sweeper) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SweepOnce(ctx)
		}
	}
}

// SweepOnce runs one pass. It logs rather than returns errors: nothing upstream can act
// on a failed sweep, and one bad key must not stop the rest of the pass.
func (s *Sweeper) SweepOnce(ctx context.Context) {
	expired := s.sweepExpiredUploads(ctx)
	stray := s.sweepStrayObjects(ctx)
	if expired > 0 || stray > 0 {
		slog.Info("orphan sweep", "expired_uploads", expired, "stray_objects", stray)
	}
}

// sweepExpiredUploads drops uploads whose window closed, with their objects.
func (s *Sweeper) sweepExpiredUploads(ctx context.Context) int {
	// minAge past expiry, not at expiry: a client that PUT just before the URL died may
	// still be sending its confirm.
	cutoff := s.now().Add(-s.minAge)

	uploads, err := s.store.ListUploadsExpiredBefore(ctx, cutoff)
	if err != nil {
		slog.Error("orphan sweep: list expired uploads failed", "err", err)
		return 0
	}

	swept := 0
	for _, upload := range uploads {
		// A confirmed photo can point at this very key: confirm moves the key from
		// uploads to images, and a row that outlived its move would otherwise name a
		// live object. Deleting it would destroy the bytes while the photo row went on
		// looking healthy, which is silent and permanent.
		inUse, err := s.store.ImageKeyInUse(ctx, upload.Key)
		if err != nil {
			slog.Error("orphan sweep: check image key failed", "key", upload.Key, "err", err)
			continue
		}
		if inUse {
			// Drop the stale row only; the object belongs to the photo now.
			if err := s.store.DeleteUpload(ctx, upload.ID); err != nil {
				slog.Error("orphan sweep: delete stale upload row failed", "upload_id", upload.ID, "err", err)
			}
			continue
		}

		if err := s.blobs.Delete(ctx, upload.Key); err != nil {
			// Leave the row: it is the only record of the key, so dropping it now would
			// turn a retryable failure into an object nothing can name.
			slog.Error("orphan sweep: delete object failed", "key", upload.Key, "err", err)
			continue
		}
		if err := s.store.DeleteUpload(ctx, upload.ID); err != nil {
			slog.Error("orphan sweep: delete upload row failed", "upload_id", upload.ID, "err", err)
			continue
		}
		swept++
	}
	return swept
}

// sweepStrayObjects deletes objects the database does not reference.
func (s *Sweeper) sweepStrayObjects(ctx context.Context) int {
	// The referenced set is read BEFORE the listing. In that order a row written during
	// the pass yields an object that looks unreferenced but is too young to touch; in
	// the other order a row could be deleted after being seen, and its object would
	// survive as a leak. Both are safe, but only this order is safe without the age
	// check doing the work.
	referenced, err := s.store.AllReferencedKeys(ctx)
	if err != nil {
		slog.Error("orphan sweep: read referenced keys failed", "err", err)
		return 0
	}

	objects, err := s.blobs.List(ctx, objectPrefix)
	if err != nil {
		slog.Error("orphan sweep: list objects failed", "err", err)
		return 0
	}

	// Anything younger than minAge is skipped outright: an upload in flight has an
	// object and no row yet, and deleting it would race the user's own PUT.
	cutoff := s.now().Add(-s.minAge)
	swept := 0
	for _, object := range objects {
		if _, ok := referenced[object.Key]; ok {
			continue
		}
		if object.LastModified.After(cutoff) {
			continue
		}
		if err := s.blobs.Delete(ctx, object.Key); err != nil {
			slog.Error("orphan sweep: delete stray object failed", "key", object.Key, "err", err)
			continue
		}
		swept++
	}
	return swept
}

package voice

import (
	"context"
	"log/slog"
	"time"
)

// PhotoSweeper reclaims photo-prompt objects nothing points at any more, on the post sweep's
// interval and rules (→POST-40): an expired pending upload with its object, and a `voices/`
// object no 학습 글 or pending upload names.
type PhotoSweeper struct {
	rows    PhotoSweepLedger
	objects ObjectStore
	minAge  time.Duration
	now     func() time.Time
}

func NewPhotoSweeper(rows PhotoSweepLedger, objects ObjectStore, minAge time.Duration) *PhotoSweeper {
	return &PhotoSweeper{rows: rows, objects: objects, minAge: minAge, now: time.Now}
}

// Run sweeps every interval until the context is cancelled. It does NOT sweep on start: a
// deploy loop would otherwise turn every restart into a full bucket listing.
func (s *PhotoSweeper) Run(ctx context.Context, interval time.Duration) {
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

// SweepOnce runs one pass; it logs rather than returns errors, and one bad key does not stop
// the rest of the pass.
func (s *PhotoSweeper) SweepOnce(ctx context.Context) {
	expired := s.sweepExpiredUploads(ctx)
	stray := s.sweepStrayObjects(ctx)
	if expired > 0 || stray > 0 {
		slog.Info("voice photo sweep", "expired_uploads", expired, "stray_objects", stray)
	}
}

func (s *PhotoSweeper) sweepExpiredUploads(ctx context.Context) int {
	// minAge past expiry: a client that PUT just before the URL died may still be answering.
	uploads, err := s.rows.ListPhotoUploadsExpiredBefore(ctx, s.now().Add(-s.minAge))
	if err != nil {
		slog.Error("voice photo sweep: list expired uploads failed", "err", err)
		return 0
	}
	swept := 0
	for _, upload := range uploads {
		// An answer can own this very key: the answer and the row's drop are one transaction,
		// but a row that outlived it must never take the answer's photo with it.
		inUse, err := s.rows.PhotoKeyInUse(ctx, upload.Key)
		if err != nil {
			slog.Error("voice photo sweep: check key failed", "err", err)
			continue
		}
		if !inUse {
			if err := s.objects.Delete(ctx, upload.Key); err != nil {
				// The row is the only record of the key; it stays for the next pass.
				slog.Error("voice photo sweep: delete object failed", "err", err)
				continue
			}
		}
		if err := s.rows.DeletePhotoUpload(ctx, upload.ID); err != nil {
			slog.Error("voice photo sweep: delete upload row failed", "err", err)
			continue
		}
		swept++
	}
	return swept
}

func (s *PhotoSweeper) sweepStrayObjects(ctx context.Context) int {
	// The referenced set is read BEFORE the listing, so a row written during the pass yields
	// an object too young to touch rather than a deleted live photo.
	referenced, err := s.rows.AllReferencedPhotoKeys(ctx)
	if err != nil {
		slog.Error("voice photo sweep: read referenced keys failed", "err", err)
		return 0
	}
	objects, err := s.objects.List(ctx, photoObjectPrefix)
	if err != nil {
		// A failed listing deletes nothing.
		slog.Error("voice photo sweep: list objects failed", "err", err)
		return 0
	}
	cutoff := s.now().Add(-s.minAge)
	swept := 0
	for _, object := range objects {
		if _, ok := referenced[object.Key]; ok || object.LastModified.After(cutoff) {
			continue
		}
		if err := s.objects.Delete(ctx, object.Key); err != nil {
			slog.Error("voice photo sweep: delete stray object failed", "err", err)
			continue
		}
		swept++
	}
	return swept
}

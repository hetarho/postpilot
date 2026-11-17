package experiment

import (
	"context"
	"log/slog"
	"time"
)

// TestPhotoSweepLedger is the photo rows as the sweep reads them.
type TestPhotoSweepLedger interface {
	ListExpiredUnattachedTestPhotos(ctx context.Context, before time.Time, limit int) ([]TestPhoto, error)
	DeleteUnattachedTestPhoto(ctx context.Context, user, id string) (bool, error)
	ListTestPhotoKeys(ctx context.Context) ([]string, error)
}

// TestPhotoSweeper reclaims what no photo observation test uses any more, on the post sweep's
// interval and rules (→POST-40): an expired unattached row with its object, and a `tests/`
// object no row names. A photo a test holds is frozen test input and is never touched here; the
// test's own payload purge removes it (MODEL-42).
type TestPhotoSweeper struct {
	rows    TestPhotoSweepLedger
	objects TestPhotoObjects
	minAge  time.Duration
	now     func() time.Time
}

func NewTestPhotoSweeper(rows TestPhotoSweepLedger, objects TestPhotoObjects, minAge time.Duration) *TestPhotoSweeper {
	return &TestPhotoSweeper{rows: rows, objects: objects, minAge: minAge, now: time.Now}
}

// Run sweeps every interval until the context is cancelled. It does NOT sweep on start: a
// deploy loop would otherwise turn every restart into a full bucket listing.
func (s *TestPhotoSweeper) Run(ctx context.Context, interval time.Duration) {
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
func (s *TestPhotoSweeper) SweepOnce(ctx context.Context) {
	expired := s.sweepExpiredRows(ctx)
	stray := s.sweepStrayObjects(ctx)
	if expired > 0 || stray > 0 {
		slog.Info("test photo sweep", "expired_photos", expired, "stray_objects", stray)
	}
}

func (s *TestPhotoSweeper) sweepExpiredRows(ctx context.Context) int {
	// minAge past expiry: a client that PUT just before its URL died may still be answering.
	before := s.now().Add(-s.minAge)
	swept := 0
	for {
		photos, err := s.rows.ListExpiredUnattachedTestPhotos(ctx, before, testPhotoSweepBatch)
		if err != nil {
			slog.Error("test photo sweep: list expired photos failed", "err", err)
			return swept
		}
		removedNow := 0
		for _, photo := range photos {
			// Row first, then object (→POST-39): the delete is conditional on the photo still
			// being unattached, so a photo a test claimed in the meantime keeps its object, and
			// an object whose delete fails below is a stray the second half of this pass reclaims.
			removed, err := s.rows.DeleteUnattachedTestPhoto(ctx, photo.UserID, photo.ID)
			if err != nil {
				slog.Error("test photo sweep: delete photo row failed", "err", err)
				continue
			}
			if !removed {
				continue
			}
			removedNow++
			if err := s.objects.Delete(ctx, photo.Key); err != nil {
				slog.Error("test photo sweep: delete photo object failed", "err", err)
			}
		}
		swept += removedNow
		// A pass that removed nothing would list the same rows again.
		if removedNow == 0 || len(photos) < testPhotoSweepBatch {
			return swept
		}
	}
}

func (s *TestPhotoSweeper) sweepStrayObjects(ctx context.Context) int {
	// The referenced set is read BEFORE the listing, so a row written during the pass yields an
	// object too young to touch rather than a deleted live photo.
	keys, err := s.rows.ListTestPhotoKeys(ctx)
	if err != nil {
		slog.Error("test photo sweep: read referenced keys failed", "err", err)
		return 0
	}
	referenced := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		referenced[key] = struct{}{}
	}
	objects, err := s.objects.List(ctx, TestPhotoPrefix)
	if err != nil {
		// A failed listing deletes nothing.
		slog.Error("test photo sweep: list objects failed", "err", err)
		return 0
	}
	cutoff := s.now().Add(-s.minAge)
	swept := 0
	for _, object := range objects {
		if _, ok := referenced[object.Key]; ok || object.LastModified.After(cutoff) {
			continue
		}
		if err := s.objects.Delete(ctx, object.Key); err != nil {
			slog.Error("test photo sweep: delete stray object failed", "err", err)
			continue
		}
		swept++
	}
	return swept
}

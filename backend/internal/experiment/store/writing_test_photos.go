package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/experiment/store/sqlc"
	"github.com/postpilot/backend/internal/platform/dbtime"
)

// The photos a photo observation test owns (MODEL-97): rows only. The stored objects live
// under their own prefix and are removed by the stray sweep, never by these methods.

func testPhotoFromRow(row sqlc.WritingTestPhoto) (experiment.TestPhoto, error) {
	expires, err := dbtime.Parse(row.ExpiresAt)
	if err != nil {
		return experiment.TestPhoto{}, err
	}
	created, err := dbtime.Parse(row.CreatedAt)
	if err != nil {
		return experiment.TestPhoto{}, err
	}
	return experiment.TestPhoto{ID: row.ID, UserID: row.UserID, TestID: row.TestID.String, Filename: row.Filename, Key: row.ObjectKey, Status: row.Status, Bytes: row.Bytes, Width: row.Width, Height: row.Height, ExpiresAt: expires, CreatedAt: created}, nil
}
func testPhotosFromRows(rows []sqlc.WritingTestPhoto) ([]experiment.TestPhoto, error) {
	photos := make([]experiment.TestPhoto, 0, len(rows))
	for _, row := range rows {
		photo, err := testPhotoFromRow(row)
		if err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}
	return photos, nil
}

// CreateTestPhoto records an owner's upload handshake: pending, unattached, expiring.
func (s *Store) CreateTestPhoto(ctx context.Context, photo experiment.TestPhoto) error {
	return s.write.InsertWritingTestPhoto(ctx, sqlc.InsertWritingTestPhotoParams{ID: photo.ID, UserID: photo.UserID, Filename: photo.Filename, ObjectKey: photo.Key, ExpiresAt: dbtime.Format(photo.ExpiresAt), CreatedAt: dbtime.Format(photo.CreatedAt)})
}

// GetTestPhoto reads one photo of its owner; an unknown or foreign id is the same refusal.
func (s *Store) GetTestPhoto(ctx context.Context, user, id string) (experiment.TestPhoto, error) {
	row, err := s.read.GetWritingTestPhoto(ctx, sqlc.GetWritingTestPhotoParams{UserID: user, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return experiment.TestPhoto{}, experiment.ErrTestPhotoNotFound
	}
	if err != nil {
		return experiment.TestPhoto{}, err
	}
	return testPhotoFromRow(row)
}

// ConfirmTestPhoto turns a pending photo ready with the measured object. Confirming a photo
// that is already ready returns it unchanged, so a lost response can be repeated.
func (s *Store) ConfirmTestPhoto(ctx context.Context, user, id string, bytes, width, height int64) (experiment.TestPhoto, error) {
	changed, err := s.write.ConfirmWritingTestPhoto(ctx, sqlc.ConfirmWritingTestPhotoParams{Bytes: bytes, Width: width, Height: height, UserID: user, ID: id})
	if err != nil {
		return experiment.TestPhoto{}, err
	}
	row, err := s.write.GetWritingTestPhoto(ctx, sqlc.GetWritingTestPhotoParams{UserID: user, ID: id})
	if errors.Is(err, sql.ErrNoRows) || (err == nil && changed == 0 && row.Status != experiment.TestPhotoReady) {
		return experiment.TestPhoto{}, experiment.ErrTestPhotoNotFound
	}
	if err != nil {
		return experiment.TestPhoto{}, err
	}
	return testPhotoFromRow(row)
}

// ListReadyTestPhotos returns the owner's confirmed photos among ids, oldest first. A photo
// that is unknown, foreign or still pending is simply absent.
func (s *Store) ListReadyTestPhotos(ctx context.Context, user string, ids []string) ([]experiment.TestPhoto, error) {
	encoded, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.ListReadyWritingTestPhotos(ctx, sqlc.ListReadyWritingTestPhotosParams{UserID: user, Ids: string(encoded)})
	if err != nil {
		return nil, err
	}
	return testPhotosFromRows(rows)
}

// AttachTestPhotos binds the owner's ready, unattached photos to one test inside the caller's
// transaction, so the admission that freezes them commits or rolls back with them. Any id
// that is repeated, unknown, foreign, pending or already attached leaves the count short and
// is refused; the caller must then roll its transaction back.
func AttachTestPhotos(ctx context.Context, q *sqlc.Queries, user, testID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			return experiment.ErrTestPhotoUnavailable
		}
		seen[id] = true
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	attached, err := q.AttachWritingTestPhotos(ctx, sqlc.AttachWritingTestPhotosParams{TestID: nullString(testID), UserID: user, Ids: string(encoded)})
	if err != nil {
		return err
	}
	if attached != int64(len(ids)) {
		return experiment.ErrTestPhotoUnavailable
	}
	return nil
}

// CountUnattachedTestPhotos counts the owner's uploads that no test holds yet, pending and
// ready alike, for the per-owner ceiling on what can wait.
func (s *Store) CountUnattachedTestPhotos(ctx context.Context, user string) (int, error) {
	count, err := s.read.CountUnattachedWritingTestPhotos(ctx, user)
	return int(count), err
}

// ListExpiredUnattachedTestPhotos is the sweep's work list: uploads no test claimed in time.
func (s *Store) ListExpiredUnattachedTestPhotos(ctx context.Context, before time.Time, limit int) ([]experiment.TestPhoto, error) {
	rows, err := s.read.ListUnattachedWritingTestPhotosExpired(ctx, sqlc.ListUnattachedWritingTestPhotosExpiredParams{ExpiresAt: dbtime.Format(before), Limit: int64(limit)})
	if err != nil {
		return nil, err
	}
	return testPhotosFromRows(rows)
}

// DeleteUnattachedTestPhoto removes one unattached row of its owner and reports whether it
// existed. A photo a test holds is frozen test input and is never removed on its own.
func (s *Store) DeleteUnattachedTestPhoto(ctx context.Context, user, id string) (bool, error) {
	removed, err := s.write.DeleteUnattachedWritingTestPhoto(ctx, sqlc.DeleteUnattachedWritingTestPhotoParams{UserID: user, ID: id})
	return removed > 0, err
}

// ListTestPhotoKeys returns every object key a photo row still references, so the stray
// sweep can tell a referenced object from an abandoned one.
func (s *Store) ListTestPhotoKeys(ctx context.Context) ([]string, error) {
	return s.read.ListWritingTestPhotoKeys(ctx)
}

// ListTestPhotos returns the photos a test holds, oldest first.
func (s *Store) ListTestPhotos(ctx context.Context, user, testID string) ([]experiment.TestPhoto, error) {
	rows, err := s.read.ListWritingTestPhotosForTest(ctx, sqlc.ListWritingTestPhotosForTestParams{UserID: user, TestID: nullString(testID)})
	if err != nil {
		return nil, err
	}
	return testPhotosFromRows(rows)
}

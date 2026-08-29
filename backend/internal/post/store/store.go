// Package store persists the post context. It is the anti-corruption boundary on the
// database side (ARCHITECTURE §2.2): sqlc row structs and driver errors stop here, and
// only post domain types travel inward.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store/sqlc"
)

// writeLayout is how timestamps live in SQLite: fixed-width RFC3339 in UTC. The width
// matters because ORDER BY and `expires_at < ?` are plain string comparisons —
// time.RFC3339Nano trims trailing zeros and would sort "…08.5Z" after "…08.51Z".
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// Store implements post.Store over SQLite. Writes go through the single serialized
// writer connection, reads through the pool (ARCHITECTURE §2.4).
type Store struct {
	writer *sql.DB
	reader *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

// New builds the store over the process's writer and reader pools.
func New(writer, reader *sql.DB) *Store {
	return &Store{
		writer: writer,
		reader: reader,
		write:  sqlc.New(writer),
		read:   sqlc.New(reader),
	}
}

// --- posts ---

func (s *Store) CreatePost(ctx context.Context, p post.Post) error {
	err := s.write.CreatePost(ctx, sqlc.CreatePostParams{
		Slug:      p.Slug,
		UserID:    p.UserID,
		Title:     p.Title,
		Memo:      p.Memo,
		CreatedAt: formatTime(p.CreatedAt),
		UpdatedAt: formatTime(p.UpdatedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return post.ErrDuplicateSlug
		}
		return fmt.Errorf("insert post: %w", err)
	}
	return nil
}

func (s *Store) UpdateDraft(ctx context.Context, slug, userID, title, memo string, updatedAt time.Time) (bool, error) {
	n, err := s.write.UpdatePostDraft(ctx, sqlc.UpdatePostDraftParams{
		Title:     title,
		Memo:      memo,
		UpdatedAt: formatTime(updatedAt),
		Slug:      slug,
		UserID:    userID,
	})
	if err != nil {
		return false, fmt.Errorf("update post: %w", err)
	}
	return n > 0, nil
}

func (s *Store) UpdateObservations(ctx context.Context, slug, userID string, observations []post.Observation, updatedAt time.Time) (bool, error) {
	encoded, err := marshalObservations(observations)
	if err != nil {
		return false, fmt.Errorf("encode observations: %w", err)
	}
	n, err := s.write.UpdatePostObservations(ctx, sqlc.UpdatePostObservationsParams{
		Observations: sql.NullString{String: encoded, Valid: true}, UpdatedAt: formatTime(updatedAt),
		Slug: slug, UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("update observations: %w", err)
	}
	return n > 0, nil
}

func (s *Store) UpdateGeneratedContent(ctx context.Context, slug, userID string, content post.PostContent, updatedAt time.Time) (bool, error) {
	encoded, err := marshalContent(content)
	if err != nil {
		return false, fmt.Errorf("encode content: %w", err)
	}
	n, err := s.write.UpdateGeneratedContent(ctx, sqlc.UpdateGeneratedContentParams{
		Content: sql.NullString{String: encoded, Valid: true}, MachineBaseline: sql.NullString{String: encoded, Valid: true}, UpdatedAt: formatTime(updatedAt),
		Slug: slug, UserID: userID, Content_2: sql.NullString{String: encoded, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("update generated content: %w", err)
	}
	return n > 0, nil
}

func (s *Store) SaveContent(ctx context.Context, slug, userID string, content post.PostContent, expectedRevision int64, updatedAt time.Time) (bool, error) {
	encoded, err := marshalContent(content)
	if err != nil {
		return false, fmt.Errorf("encode content: %w", err)
	}
	n, err := s.write.SavePostContent(ctx, sqlc.SavePostContentParams{
		Content: sql.NullString{String: encoded, Valid: true}, UpdatedAt: formatTime(updatedAt),
		Slug: slug, UserID: userID, ContentRevision: expectedRevision,
	})
	if err != nil {
		return false, fmt.Errorf("save content: %w", err)
	}
	return n == 1, nil
}

func (s *Store) SaveGenerationOptions(ctx context.Context, slug, userID string, targetLength *int, updatedAt time.Time) (bool, error) {
	n, err := s.write.SavePostGenerationOptions(ctx, sqlc.SavePostGenerationOptionsParams{
		TargetLength: optionalInt64(targetLength), UpdatedAt: formatTime(updatedAt), Slug: slug, UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("save post generation options: %w", err)
	}
	return n == 1, nil
}

func (s *Store) Finalize(ctx context.Context, slug, userID string, expectedRevision int64, finalizedAt time.Time) (bool, error) {
	stamp := formatTime(finalizedAt)
	n, err := s.write.FinalizePost(ctx, sqlc.FinalizePostParams{
		FinalizedAt: sql.NullString{String: stamp, Valid: true}, UpdatedAt: stamp,
		Slug: slug, UserID: userID, ContentRevision: expectedRevision,
	})
	if err != nil {
		return false, fmt.Errorf("finalize post: %w", err)
	}
	return n == 1, nil
}

func (s *Store) LearningSnapshot(ctx context.Context, slug, userID string) (post.LearningSnapshot, error) {
	row, err := s.read.GetLearningSnapshot(ctx, sqlc.GetLearningSnapshotParams{Slug: slug, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return post.LearningSnapshot{}, post.ErrNotFound
	}
	if err != nil {
		return post.LearningSnapshot{}, fmt.Errorf("select learning snapshot: %w", err)
	}
	current, err := unmarshalContent(row.Content.String)
	if err != nil {
		return post.LearningSnapshot{}, err
	}
	baseline, err := unmarshalContent(row.MachineBaseline.String)
	if err != nil {
		return post.LearningSnapshot{}, err
	}
	if current == nil || baseline == nil {
		return post.LearningSnapshot{}, post.ErrNoMachineBaseline
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return post.LearningSnapshot{}, err
	}
	if row.Status != post.StatusFinalized || !row.FinalizedRevision.Valid || row.FinalizedRevision.Int64 != row.ContentRevision || !row.FinalizedAt.Valid {
		return post.LearningSnapshot{}, post.ErrPostNotFinalized
	}
	finalizedAt, err := parseTime(row.FinalizedAt.String)
	if err != nil {
		return post.LearningSnapshot{}, err
	}
	return post.LearningSnapshot{PostSlug: row.Slug, UserID: row.UserID, Current: *current,
		ContentRevision: row.ContentRevision, MachineBaseline: *baseline,
		BaselineRevision: row.MachineBaselineRevision, TargetLength: optionalInt(row.TargetLength),
		FinalizedAt: finalizedAt, UpdatedAt: updated}, nil
}

func (s *Store) GetPost(ctx context.Context, slug string) (post.Post, error) {
	row, err := s.read.GetPost(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return post.Post{}, post.ErrNotFound
		}
		return post.Post{}, fmt.Errorf("select post: %w", err)
	}
	return toPost(row)
}

func (s *Store) SlugExists(ctx context.Context, slug string) (bool, error) {
	taken, err := s.read.PostSlugExists(ctx, slug)
	if err != nil {
		return false, fmt.Errorf("check slug: %w", err)
	}
	return taken, nil
}

func (s *Store) ListPosts(ctx context.Context, userID string) ([]post.Summary, error) {
	rows, err := s.read.ListPostsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select posts: %w", err)
	}

	summaries := make([]post.Summary, 0, len(rows))
	for _, row := range rows {
		updatedAt, err := parseTime(row.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("post %s: %w", row.Slug, err)
		}
		title := row.Title
		if strings.TrimSpace(title) == "" && row.Content.Valid {
			content, err := unmarshalContent(row.Content.String)
			if err != nil {
				return nil, fmt.Errorf("post %s: %w", row.Slug, err)
			}
			if content != nil {
				title = content.Title
			}
		}
		summaries = append(summaries, post.Summary{
			Slug:      row.Slug,
			Title:     title,
			Status:    row.Status,
			UpdatedAt: updatedAt,
		})
	}
	return summaries, nil
}

func (s *Store) DeletePost(ctx context.Context, slug, userID string) (bool, error) {
	count, err := s.write.DeletePost(ctx, sqlc.DeletePostParams{Slug: slug, UserID: userID})
	return count == 1, err
}

// --- images ---

func (s *Store) CreateImage(ctx context.Context, img post.Image) error {
	err := s.write.CreateImage(ctx, sqlc.CreateImageParams{
		ID:        img.ID,
		PostSlug:  img.PostSlug,
		Filename:  img.Filename,
		R2Key:     img.Key,
		Width:     int64(img.Width),
		Height:    int64(img.Height),
		Bytes:     img.Bytes,
		CreatedAt: formatTime(img.CreatedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return post.ErrDuplicateFilename
		}
		return fmt.Errorf("insert image: %w", err)
	}
	return nil
}

func (s *Store) ListImages(ctx context.Context, postSlug string) ([]post.Image, error) {
	rows, err := s.read.ListImagesByPost(ctx, postSlug)
	if err != nil {
		return nil, fmt.Errorf("select images: %w", err)
	}

	images := make([]post.Image, 0, len(rows))
	for _, row := range rows {
		img, err := toImage(row)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

func (s *Store) GetImage(ctx context.Context, id string) (post.Image, error) {
	row, err := s.read.GetImage(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return post.Image{}, post.ErrNotFound
		}
		return post.Image{}, fmt.Errorf("select image: %w", err)
	}
	return toImage(row)
}

func (s *Store) DeleteImage(ctx context.Context, id string) error {
	if err := s.write.DeleteImage(ctx, id); err != nil {
		return fmt.Errorf("delete image: %w", err)
	}
	return nil
}

// ImageFilenameTaken reports a CONFIRMED photo with this name. A pending upload is
// deliberately not "taken": that is the retry case, and CreateUpload replaces it.
func (s *Store) ImageFilenameTaken(ctx context.Context, postSlug, filename string) (bool, error) {
	taken, err := s.read.ImageFilenameTaken(ctx, sqlc.ImageFilenameTakenParams{
		PostSlug: postSlug,
		Filename: filename,
	})
	if err != nil {
		return false, fmt.Errorf("check image filename: %w", err)
	}
	return taken, nil
}

// ImageKeyInUse reports whether a photo points at this object key.
func (s *Store) ImageKeyInUse(ctx context.Context, key string) (bool, error) {
	inUse, err := s.read.ImageKeyInUse(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check image key: %w", err)
	}
	return inUse, nil
}

// --- uploads ---

func (s *Store) CreateUpload(ctx context.Context, u post.Upload) error {
	err := s.write.CreateUpload(ctx, sqlc.CreateUploadParams{
		ID:        u.ID,
		PostSlug:  u.PostSlug,
		Filename:  u.Filename,
		R2Key:     u.Key,
		ExpiresAt: formatTime(u.ExpiresAt),
		CreatedAt: formatTime(u.CreatedAt),
	})
	if err != nil {
		// The UNIQUE(post_slug, filename) constraint is what actually closes the race
		// between two concurrent CreateUpload calls; the service's precheck only makes
		// the common case a clean error instead of a constraint failure.
		if isUniqueViolation(err) {
			return post.ErrDuplicateFilename
		}
		return fmt.Errorf("insert upload: %w", err)
	}
	return nil
}

func (s *Store) GetUpload(ctx context.Context, id string) (post.Upload, error) {
	row, err := s.read.GetUpload(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return post.Upload{}, post.ErrNotFound
		}
		return post.Upload{}, fmt.Errorf("select upload: %w", err)
	}
	return toUpload(row)
}

func (s *Store) GetUploadByFilename(ctx context.Context, postSlug, filename string) (post.Upload, error) {
	row, err := s.read.GetUploadByFilename(ctx, sqlc.GetUploadByFilenameParams{
		PostSlug: postSlug,
		Filename: filename,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return post.Upload{}, post.ErrNotFound
		}
		return post.Upload{}, fmt.Errorf("select upload by filename: %w", err)
	}
	return toUpload(row)
}

func (s *Store) DeleteUpload(ctx context.Context, id string) error {
	if err := s.write.DeleteUpload(ctx, id); err != nil {
		return fmt.Errorf("delete upload: %w", err)
	}
	return nil
}

func (s *Store) ListUploadsExpiredBefore(ctx context.Context, t time.Time) ([]post.Upload, error) {
	rows, err := s.read.ListUploadsExpiredBefore(ctx, formatTime(t))
	if err != nil {
		return nil, fmt.Errorf("select expired uploads: %w", err)
	}

	uploads := make([]post.Upload, 0, len(rows))
	for _, row := range rows {
		upload, err := toUpload(row)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, upload)
	}
	return uploads, nil
}

// ConfirmUpload writes the photo and drops the upload row in one transaction.
//
// Atomicity is not a nicety here. A key is referenced by exactly one of the two tables,
// and confirm is the hand-off. If the image were written and the upload row survived,
// that row would still name the live photo's object — and the sweep, finding it
// expired, would delete the bytes while the photo row went on looking healthy.
func (s *Store) ConfirmUpload(ctx context.Context, img post.Image, uploadID string) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin confirm: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	q := s.write.WithTx(tx)
	err = q.CreateImage(ctx, sqlc.CreateImageParams{
		ID:        img.ID,
		PostSlug:  img.PostSlug,
		Filename:  img.Filename,
		R2Key:     img.Key,
		Width:     int64(img.Width),
		Height:    int64(img.Height),
		Bytes:     img.Bytes,
		CreatedAt: formatTime(img.CreatedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return post.ErrDuplicateFilename
		}
		return fmt.Errorf("insert image: %w", err)
	}
	if err := q.DeleteUpload(ctx, uploadID); err != nil {
		return fmt.Errorf("delete upload: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit confirm: %w", err)
	}
	return nil
}

// AllReferencedKeys reads every key both tables point at, as ONE snapshot.
//
// The read transaction is load-bearing, not tidiness. The sweep deletes objects missing
// from this set, and a confirm moves a key from uploads to images: read separately, a
// confirm landing between the two queries can leave a live key in neither result — and
// the sweep would then delete a photo nobody could get back.
func (s *Store) AllReferencedKeys(ctx context.Context) (map[string]struct{}, error) {
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin key snapshot: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // read-only

	q := s.read.WithTx(tx)
	imageKeys, err := q.ListAllImageKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("select image keys: %w", err)
	}
	uploadKeys, err := q.ListAllUploadKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("select upload keys: %w", err)
	}

	keys := make(map[string]struct{}, len(imageKeys)+len(uploadKeys))
	for _, key := range imageKeys {
		keys[key] = struct{}{}
	}
	for _, key := range uploadKeys {
		keys[key] = struct{}{}
	}
	return keys, nil
}

// --- mapping ---

func toPost(row sqlc.Post) (post.Post, error) {
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return post.Post{}, fmt.Errorf("post %s created_at: %w", row.Slug, err)
	}
	updatedAt, err := parseTime(row.UpdatedAt)
	if err != nil {
		return post.Post{}, fmt.Errorf("post %s updated_at: %w", row.Slug, err)
	}
	content, err := unmarshalContent(row.Content.String)
	if err != nil {
		return post.Post{}, fmt.Errorf("post %s: %w", row.Slug, err)
	}
	observations, err := unmarshalObservations(row.Observations.String)
	if err != nil {
		return post.Post{}, fmt.Errorf("post %s: %w", row.Slug, err)
	}
	var finalizedAt *time.Time
	if row.FinalizedAt.Valid {
		value, parseErr := parseTime(row.FinalizedAt.String)
		if parseErr != nil {
			return post.Post{}, fmt.Errorf("post %s finalized_at: %w", row.Slug, parseErr)
		}
		finalizedAt = &value
	}
	return post.Post{
		Slug:                    row.Slug,
		UserID:                  row.UserID,
		Title:                   row.Title,
		Memo:                    row.Memo,
		Status:                  row.Status,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
		Content:                 content,
		ContentRevision:         row.ContentRevision,
		MachineBaselineRevision: row.MachineBaselineRevision,
		TargetLength:            optionalInt(row.TargetLength),
		FinalizedRevision:       row.FinalizedRevision.Int64,
		FinalizedAt:             finalizedAt,
		Observations:            observations,
	}, nil
}

func optionalInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func optionalInt64(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func toImage(row sqlc.Image) (post.Image, error) {
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return post.Image{}, fmt.Errorf("image %s: %w", row.ID, err)
	}
	return post.Image{
		ID:        row.ID,
		PostSlug:  row.PostSlug,
		Filename:  row.Filename,
		Key:       row.R2Key,
		Width:     int32(row.Width),
		Height:    int32(row.Height),
		Bytes:     row.Bytes,
		CreatedAt: createdAt,
	}, nil
}

func toUpload(row sqlc.Upload) (post.Upload, error) {
	expiresAt, err := parseTime(row.ExpiresAt)
	if err != nil {
		return post.Upload{}, fmt.Errorf("upload %s expires_at: %w", row.ID, err)
	}
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return post.Upload{}, fmt.Errorf("upload %s created_at: %w", row.ID, err)
	}
	return post.Upload{
		ID:        row.ID,
		PostSlug:  row.PostSlug,
		Filename:  row.Filename,
		Key:       row.R2Key,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
	}, nil
}

// formatTime normalizes to UTC so stored values sort against each other regardless of
// the offset the caller's clock carried.
func formatTime(t time.Time) string { return t.UTC().Format(writeLayout) }

// parseTime reads with RFC3339Nano rather than writeLayout: it accepts any fraction
// width, so a row written before the width was pinned still loads.
func parseTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", v, err)
	}
	return t, nil
}

// isUniqueViolation detects a PRIMARY KEY or UNIQUE collision.
//
// It matches on the message rather than a driver error code: modernc.org/sqlite returns
// its own error type, and importing the driver here just to read a code would pull it
// into the one package whose job is to keep drivers at the edge. The message text is
// part of SQLite's stable public behavior.
func isUniqueViolation(err error) bool {
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "UNIQUE CONSTRAINT FAILED") ||
		strings.Contains(msg, "PRIMARY KEY CONSTRAINT FAILED")
}

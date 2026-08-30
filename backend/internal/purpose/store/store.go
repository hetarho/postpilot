// Package store persists the purpose context. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/purpose"
	"github.com/postpilot/backend/internal/purpose/store/sqlc"
)

// The fixed-width UTC layout every context writes timestamps in, so string comparison
// and ORDER BY agree with chronological order.
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

func (s *Store) Insert(ctx context.Context, p purpose.Purpose) error {
	err := s.write.InsertPurpose(ctx, sqlc.InsertPurposeParams{
		ID: p.ID, UserID: p.UserID, Name: p.Name, Description: p.Description,
		Instructions: p.Instructions, CreatedAt: formatTime(p.CreatedAt), UpdatedAt: formatTime(p.UpdatedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return purpose.ErrDuplicateName
		}
		return fmt.Errorf("insert purpose: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, userID string) ([]purpose.Purpose, error) {
	rows, err := s.read.ListPurposes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select purposes: %w", err)
	}
	out := make([]purpose.Purpose, 0, len(rows))
	for _, row := range rows {
		value, err := toPurpose(row.ID, row.UserID, row.Name, row.Description, row.Instructions, row.CreatedAt, row.UpdatedAt, row.PostCount)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func (s *Store) Get(ctx context.Context, userID, id string) (purpose.Purpose, error) {
	return s.get(ctx, s.read, userID, id)
}

func (s *Store) get(ctx context.Context, q *sqlc.Queries, userID, id string) (purpose.Purpose, error) {
	row, err := q.GetPurpose(ctx, sqlc.GetPurposeParams{ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return purpose.Purpose{}, purpose.ErrNotFound
	}
	if err != nil {
		return purpose.Purpose{}, fmt.Errorf("select purpose: %w", err)
	}
	return toPurpose(row.ID, row.UserID, row.Name, row.Description, row.Instructions, row.CreatedAt, row.UpdatedAt, row.PostCount)
}

// Update runs one statement per present field inside a single transaction, so a field the
// patch does not carry is never written — not even back to the value this call read.
func (s *Store) Update(ctx context.Context, userID, id string, patch purpose.Patch, updatedAt time.Time) (purpose.Purpose, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return purpose.Purpose{}, fmt.Errorf("begin update purpose: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	stamp := formatTime(updatedAt)

	// The first statement that runs also answers "does this purpose exist and is it mine":
	// zero rows means the id is unknown or belongs to another account, which read the same.
	touched := false
	if patch.Name != nil {
		n, err := q.UpdatePurposeName(ctx, sqlc.UpdatePurposeNameParams{Name: *patch.Name, UpdatedAt: stamp, ID: id, UserID: userID})
		if err != nil {
			if isUniqueViolation(err) {
				return purpose.Purpose{}, purpose.ErrDuplicateName
			}
			return purpose.Purpose{}, fmt.Errorf("update purpose name: %w", err)
		}
		if n == 0 {
			return purpose.Purpose{}, purpose.ErrNotFound
		}
		touched = true
	}
	if patch.Description != nil {
		n, err := q.UpdatePurposeDescription(ctx, sqlc.UpdatePurposeDescriptionParams{Description: *patch.Description, UpdatedAt: stamp, ID: id, UserID: userID})
		if err != nil {
			return purpose.Purpose{}, fmt.Errorf("update purpose description: %w", err)
		}
		if n == 0 {
			return purpose.Purpose{}, purpose.ErrNotFound
		}
		touched = true
	}
	if patch.Instructions != nil {
		n, err := q.UpdatePurposeInstructions(ctx, sqlc.UpdatePurposeInstructionsParams{Instructions: *patch.Instructions, UpdatedAt: stamp, ID: id, UserID: userID})
		if err != nil {
			return purpose.Purpose{}, fmt.Errorf("update purpose instructions: %w", err)
		}
		if n == 0 {
			return purpose.Purpose{}, purpose.ErrNotFound
		}
		touched = true
	}
	if !touched {
		return purpose.Purpose{}, purpose.ErrNotFound
	}

	// Read back inside the same transaction: the response must describe what this edit
	// produced, not a row another writer may have moved on since.
	updated, err := s.get(ctx, q, userID, id)
	if err != nil {
		return purpose.Purpose{}, err
	}
	if err := tx.Commit(); err != nil {
		return purpose.Purpose{}, fmt.Errorf("commit update purpose: %w", err)
	}
	return updated, nil
}

// Delete counts the posts pointing at the purpose and removes it in one transaction. The
// detach itself is the schema's BEFORE DELETE trigger, which runs inside this same
// transaction, so the count and the clearing cannot disagree: both see one snapshot of posts.
func (s *Store) Delete(ctx context.Context, userID, id string) (int, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin delete purpose: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)

	attached, err := q.CountPostsForPurpose(ctx, sqlc.CountPostsForPurposeParams{PurposeID: sql.NullString{String: id, Valid: true}, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("count posts for purpose: %w", err)
	}
	n, err := q.DeletePurpose(ctx, sqlc.DeletePurposeParams{ID: id, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("delete purpose: %w", err)
	}
	if n == 0 {
		return 0, purpose.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete purpose: %w", err)
	}
	return int(attached), nil
}

func toPurpose(id, userID, name, description, instructions, createdAt, updatedAt string, postCount int64) (purpose.Purpose, error) {
	created, err := parseTime(createdAt)
	if err != nil {
		return purpose.Purpose{}, fmt.Errorf("parse purpose created_at: %w", err)
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return purpose.Purpose{}, fmt.Errorf("parse purpose updated_at: %w", err)
	}
	return purpose.Purpose{
		ID: id, UserID: userID, Name: name, Description: description, Instructions: instructions,
		PostCount: int(postCount), CreatedAt: created, UpdatedAt: updated,
	}, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE CONSTRAINT FAILED")
}

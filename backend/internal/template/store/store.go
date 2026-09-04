// Package store persists the template context. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/template/store/sqlc"
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

// Insert counts and writes in one transaction. The cap is checked here rather than in the
// service because a service-side count is a read the next writer does not see: two creates
// racing at the limit would both find room.
func (s *Store) Insert(ctx context.Context, t template.Template, maxPerAccount int) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert template: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)

	held, err := q.CountTemplates(ctx, t.UserID)
	if err != nil {
		return fmt.Errorf("count templates: %w", err)
	}
	if int(held) >= maxPerAccount {
		return template.ErrTooMany
	}
	err = q.InsertTemplate(ctx, sqlc.InsertTemplateParams{
		ID: t.ID, UserID: t.UserID, Name: t.Name, Description: t.Description,
		Body: t.Body, CreatedAt: formatTime(t.CreatedAt), UpdatedAt: formatTime(t.UpdatedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return template.ErrDuplicateName
		}
		return fmt.Errorf("insert template: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert template: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, userID string) ([]template.Template, error) {
	rows, err := s.read.ListTemplates(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select templates: %w", err)
	}
	out := make([]template.Template, 0, len(rows))
	for _, row := range rows {
		value, err := toTemplate(row.ID, row.UserID, row.Name, row.Description, row.Body, row.CreatedAt, row.UpdatedAt, row.PostCount)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func (s *Store) Get(ctx context.Context, userID, id string) (template.Template, error) {
	return s.get(ctx, s.read, userID, id)
}

func (s *Store) get(ctx context.Context, q *sqlc.Queries, userID, id string) (template.Template, error) {
	row, err := q.GetTemplate(ctx, sqlc.GetTemplateParams{ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return template.Template{}, template.ErrNotFound
	}
	if err != nil {
		return template.Template{}, fmt.Errorf("select template: %w", err)
	}
	return toTemplate(row.ID, row.UserID, row.Name, row.Description, row.Body, row.CreatedAt, row.UpdatedAt, row.PostCount)
}

// Update runs one statement per present field inside a single transaction, so a field the
// patch does not carry is never written — not even back to the value this call read.
func (s *Store) Update(ctx context.Context, userID, id string, patch template.Patch, updatedAt time.Time) (template.Template, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return template.Template{}, fmt.Errorf("begin update template: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	stamp := formatTime(updatedAt)

	// The first statement that runs also answers "does this template exist and is it mine":
	// zero rows means the id is unknown or belongs to another account, which read the same.
	touched := false
	if patch.Name != nil {
		n, err := q.UpdateTemplateName(ctx, sqlc.UpdateTemplateNameParams{Name: *patch.Name, UpdatedAt: stamp, ID: id, UserID: userID})
		if err != nil {
			if isUniqueViolation(err) {
				return template.Template{}, template.ErrDuplicateName
			}
			return template.Template{}, fmt.Errorf("update template name: %w", err)
		}
		if n == 0 {
			return template.Template{}, template.ErrNotFound
		}
		touched = true
	}
	if patch.Description != nil {
		n, err := q.UpdateTemplateDescription(ctx, sqlc.UpdateTemplateDescriptionParams{Description: *patch.Description, UpdatedAt: stamp, ID: id, UserID: userID})
		if err != nil {
			return template.Template{}, fmt.Errorf("update template description: %w", err)
		}
		if n == 0 {
			return template.Template{}, template.ErrNotFound
		}
		touched = true
	}
	if patch.Body != nil {
		n, err := q.UpdateTemplateBody(ctx, sqlc.UpdateTemplateBodyParams{Body: *patch.Body, UpdatedAt: stamp, ID: id, UserID: userID})
		if err != nil {
			return template.Template{}, fmt.Errorf("update template body: %w", err)
		}
		if n == 0 {
			return template.Template{}, template.ErrNotFound
		}
		touched = true
	}
	if !touched {
		return template.Template{}, template.ErrNotFound
	}

	// Read back inside the same transaction: the response must describe what this edit
	// produced, not a row another writer may have moved on since.
	updated, err := s.get(ctx, q, userID, id)
	if err != nil {
		return template.Template{}, err
	}
	if err := tx.Commit(); err != nil {
		return template.Template{}, fmt.Errorf("commit update template: %w", err)
	}
	return updated, nil
}

// Delete counts the posts pointing at the template and removes it in one transaction. The
// detach itself is the schema's BEFORE DELETE trigger, which runs inside this same
// transaction, so the count and the clearing cannot disagree: both see one snapshot of posts.
func (s *Store) Delete(ctx context.Context, userID, id string) (int, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin delete template: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)

	attached, err := q.CountPostsForTemplate(ctx, sqlc.CountPostsForTemplateParams{TemplateID: sql.NullString{String: id, Valid: true}, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("count posts for template: %w", err)
	}
	n, err := q.DeleteTemplate(ctx, sqlc.DeleteTemplateParams{ID: id, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("delete template: %w", err)
	}
	if n == 0 {
		return 0, template.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete template: %w", err)
	}
	return int(attached), nil
}

func toTemplate(id, userID, name, description, body, createdAt, updatedAt string, postCount int64) (template.Template, error) {
	created, err := parseTime(createdAt)
	if err != nil {
		return template.Template{}, fmt.Errorf("parse template created_at: %w", err)
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return template.Template{}, fmt.Errorf("parse template updated_at: %w", err)
	}
	return template.Template{
		ID: id, UserID: userID, Name: name, Description: description, Body: body,
		PostCount: int(postCount), CreatedAt: created, UpdatedAt: updated,
	}, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE CONSTRAINT FAILED")
}

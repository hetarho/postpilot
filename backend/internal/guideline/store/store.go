// Package store persists the guideline context. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/guideline/store/sqlc"
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
// service because two concurrent creates would otherwise both read max-1 and both commit.
func (s *Store) Insert(ctx context.Context, g guideline.Guideline, maxPerAccount int) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert guideline: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)

	held, err := q.CountGuidelines(ctx, g.UserID)
	if err != nil {
		return fmt.Errorf("count guidelines: %w", err)
	}
	if int(held) >= maxPerAccount {
		return &guideline.AccountCapError{Max: maxPerAccount}
	}
	err = q.InsertGuideline(ctx, sqlc.InsertGuidelineParams{
		ID: g.ID, UserID: g.UserID, Text: g.Text, Scope: string(g.Scope),
		CreatedAt: formatTime(g.CreatedAt), UpdatedAt: formatTime(g.UpdatedAt),
	})
	if err != nil {
		if isDuplicateText(err) {
			return guideline.ErrDuplicateText
		}
		return fmt.Errorf("insert guideline: %w", err)
	}
	if err := insertScope(ctx, q, g.UserID, g.ID, g.TemplateIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert guideline: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, userID string) ([]guideline.Guideline, error) {
	rows, err := s.read.ListGuidelines(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select guidelines: %w", err)
	}
	// One link read for the whole account rather than one per guideline: the list is a
	// screen, and N+1 reads of a two-column join table buy nothing.
	links, err := s.read.ListGuidelineTemplateLinks(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select guideline scope links: %w", err)
	}
	scoped := make(map[string][]string, len(links))
	for _, link := range links {
		scoped[link.GuidelineID] = append(scoped[link.GuidelineID], link.TemplateID)
	}
	out := make([]guideline.Guideline, 0, len(rows))
	for _, row := range rows {
		value, err := toGuideline(row)
		if err != nil {
			return nil, err
		}
		value.TemplateIDs = scoped[row.ID]
		out = append(out, value)
	}
	return out, nil
}

func (s *Store) Get(ctx context.Context, userID, id string) (guideline.Guideline, error) {
	return s.get(ctx, s.read, userID, id)
}

func (s *Store) get(ctx context.Context, q *sqlc.Queries, userID, id string) (guideline.Guideline, error) {
	row, err := q.GetGuideline(ctx, sqlc.GetGuidelineParams{ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return guideline.Guideline{}, guideline.ErrNotFound
	}
	if err != nil {
		return guideline.Guideline{}, fmt.Errorf("select guideline: %w", err)
	}
	value, err := toGuideline(row)
	if err != nil {
		return guideline.Guideline{}, err
	}
	ids, err := q.ListGuidelineScope(ctx, sqlc.ListGuidelineScopeParams{GuidelineID: id, UserID: userID})
	if err != nil {
		return guideline.Guideline{}, fmt.Errorf("select guideline scope: %w", err)
	}
	value.TemplateIDs = ids
	return value, nil
}

// Update writes only the present parts of the patch, in one transaction. A patch without a
// scope names no scope statement at all, so a text edit cannot revert a scope replacement
// that landed from another tab in between.
func (s *Store) Update(ctx context.Context, userID, id string, patch guideline.Patch, updatedAt time.Time) (guideline.Guideline, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return guideline.Guideline{}, fmt.Errorf("begin update guideline: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	stamp := formatTime(updatedAt)

	// The first statement that runs also answers "does this guideline exist and is it mine":
	// zero rows means the id is unknown or belongs to another account, which read the same.
	touched := false
	if patch.Text != nil {
		n, err := q.UpdateGuidelineText(ctx, sqlc.UpdateGuidelineTextParams{Text: *patch.Text, UpdatedAt: stamp, ID: id, UserID: userID})
		if err != nil {
			if isDuplicateText(err) {
				return guideline.Guideline{}, guideline.ErrDuplicateText
			}
			return guideline.Guideline{}, fmt.Errorf("update guideline text: %w", err)
		}
		if n == 0 {
			return guideline.Guideline{}, guideline.ErrNotFound
		}
		touched = true
	}
	if patch.Scope != nil {
		n, err := q.UpdateGuidelineScope(ctx, sqlc.UpdateGuidelineScopeParams{Scope: string(patch.Scope.Scope), UpdatedAt: stamp, ID: id, UserID: userID})
		if err != nil {
			return guideline.Guideline{}, fmt.Errorf("update guideline scope: %w", err)
		}
		if n == 0 {
			return guideline.Guideline{}, guideline.ErrNotFound
		}
		// The kind and the link set are replaced together: a scope is one value, so a crash
		// between the two must not leave 'global' still carrying links, or the reverse.
		if err := q.DeleteGuidelineScope(ctx, sqlc.DeleteGuidelineScopeParams{GuidelineID: id, UserID: userID}); err != nil {
			return guideline.Guideline{}, fmt.Errorf("clear guideline scope: %w", err)
		}
		if err := insertScope(ctx, q, userID, id, patch.Scope.TemplateIDs); err != nil {
			return guideline.Guideline{}, err
		}
		touched = true
	}
	if !touched {
		return guideline.Guideline{}, guideline.ErrNotFound
	}

	// Read back inside the same transaction: the response must describe what this edit
	// produced, not a row another writer may have moved on since.
	updated, err := s.get(ctx, q, userID, id)
	if err != nil {
		return guideline.Guideline{}, err
	}
	if err := tx.Commit(); err != nil {
		return guideline.Guideline{}, fmt.Errorf("commit update guideline: %w", err)
	}
	return updated, nil
}

func (s *Store) Delete(ctx context.Context, userID, id string) error {
	n, err := s.write.DeleteGuideline(ctx, sqlc.DeleteGuidelineParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete guideline: %w", err)
	}
	if n == 0 {
		return guideline.ErrNotFound
	}
	return nil
}

func (s *Store) ApplicableTexts(ctx context.Context, userID, templateID string) ([]string, error) {
	texts, err := s.read.ListApplicableGuidelineTexts(ctx, sqlc.ListApplicableGuidelineTextsParams{UserID: userID, TemplateID: templateID})
	if err != nil {
		return nil, fmt.Errorf("select applicable guidelines: %w", err)
	}
	return texts, nil
}

// insertScope writes the link rows. A foreign template id is refused by the composite foreign
// key, not by a check here, so the account boundary holds even if a service check is bypassed.
func insertScope(ctx context.Context, q *sqlc.Queries, userID, guidelineID string, templateIDs []string) error {
	for _, templateID := range templateIDs {
		err := q.InsertGuidelineScopeLink(ctx, sqlc.InsertGuidelineScopeLinkParams{
			GuidelineID: guidelineID, TemplateID: templateID, UserID: userID,
		})
		if err != nil {
			if isForeignKeyViolation(err) {
				return guideline.ErrTemplateNotFound
			}
			return fmt.Errorf("insert guideline scope link: %w", err)
		}
	}
	return nil
}

func toGuideline(row sqlc.Guideline) (guideline.Guideline, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return guideline.Guideline{}, fmt.Errorf("parse guideline created_at: %w", err)
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return guideline.Guideline{}, fmt.Errorf("parse guideline updated_at: %w", err)
	}
	scope := guideline.Scope(row.Scope)
	if !scope.Valid() {
		return guideline.Guideline{}, fmt.Errorf("unknown guideline scope %q", row.Scope)
	}
	return guideline.Guideline{
		ID: row.ID, UserID: row.UserID, Text: row.Text, Scope: scope,
		CreatedAt: created, UpdatedAt: updated,
	}, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

// isDuplicateText names the constraint rather than accepting any UNIQUE failure: an id
// collision is a bug in id generation, not a rule the user broke, and must not be reported to
// them as "that text already exists".
func isDuplicateText(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToUpper(err.Error())
	return strings.Contains(message, "UNIQUE CONSTRAINT FAILED") && strings.Contains(message, "GUIDELINES.TEXT")
}

func isForeignKeyViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "FOREIGN KEY CONSTRAINT FAILED")
}

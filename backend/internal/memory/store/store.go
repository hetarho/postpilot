// Package store persists the memory context. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/memory"
	"github.com/postpilot/backend/internal/memory/store/sqlc"
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

// Insert is the whole create rule in one transaction: the exact-text lookup, the cap, the
// row and its tags, then the source link. None of it can move up into the service — two
// concurrent creates would otherwise both read max-1 and both commit, and two approvals of
// one text would both insert.
func (s *Store) Insert(ctx context.Context, m memory.Memory, sourcePostSlug string, maxPerAccount int) (memory.Memory, bool, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return memory.Memory{}, false, fmt.Errorf("begin insert memory: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)

	existing, err := q.MemoryByText(ctx, sqlc.MemoryByTextParams{UserID: m.UserID, Text: m.Text})
	switch {
	case err == nil:
		// Exact after trim and nothing else (MEM-9). The kind and the tags of the row that
		// is already there are left alone: a re-approval is a second sighting of a fact,
		// not an edit of one, and overwriting them would discard what the user authored.
		if _, err := q.TouchMemory(ctx, sqlc.TouchMemoryParams{
			LastSeenAt: formatTime(m.LastSeenAt), ID: existing.ID, UserID: m.UserID,
		}); err != nil {
			return memory.Memory{}, false, fmt.Errorf("touch memory: %w", err)
		}
		if err := linkSource(ctx, q, m.UserID, existing.ID, sourcePostSlug, m.CreatedAt); err != nil {
			return memory.Memory{}, false, err
		}
		stored, err := loadOne(ctx, q, m.UserID, existing.ID)
		if err != nil {
			return memory.Memory{}, false, err
		}
		stored.LastSeenAt = m.LastSeenAt
		if err := tx.Commit(); err != nil {
			return memory.Memory{}, false, fmt.Errorf("commit insert memory: %w", err)
		}
		return stored, true, nil
	case !errors.Is(err, sql.ErrNoRows):
		return memory.Memory{}, false, fmt.Errorf("select memory by text: %w", err)
	}

	held, err := q.CountMemories(ctx, m.UserID)
	if err != nil {
		return memory.Memory{}, false, fmt.Errorf("count memories: %w", err)
	}
	// Read inside the transaction, so the count cannot be stale by the time the row lands.
	// Nothing is evicted at the cap: the account keeps exactly what it chose to keep.
	if int(held) >= maxPerAccount {
		return memory.Memory{}, false, &memory.AccountCapError{Max: maxPerAccount}
	}
	if err := q.InsertMemory(ctx, sqlc.InsertMemoryParams{
		ID: m.ID, UserID: m.UserID, Text: m.Text, Kind: string(m.Kind),
		CreatedAt: formatTime(m.CreatedAt), UpdatedAt: formatTime(m.UpdatedAt),
		LastSeenAt: formatTime(m.LastSeenAt),
	}); err != nil {
		return memory.Memory{}, false, fmt.Errorf("insert memory: %w", err)
	}
	if err := insertTags(ctx, q, m.UserID, m.ID, m.Tags); err != nil {
		return memory.Memory{}, false, err
	}
	if err := linkSource(ctx, q, m.UserID, m.ID, sourcePostSlug, m.CreatedAt); err != nil {
		return memory.Memory{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return memory.Memory{}, false, fmt.Errorf("commit insert memory: %w", err)
	}
	stored := m
	if sourcePostSlug != "" {
		stored.SourcePostSlugs = []string{sourcePostSlug}
	}
	return stored, false, nil
}

func (s *Store) List(ctx context.Context, userID string) ([]memory.Memory, error) {
	rows, err := s.read.ListMemories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	tags, err := s.read.ListMemoryTags(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list memory tags: %w", err)
	}
	sources, err := s.read.ListMemorySources(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list memory sources: %w", err)
	}
	// Two whole-account reads rather than two per row: the account's ceiling is 300, and a
	// per-row query would be 601 statements to render one screen.
	tagsByMemory := make(map[string][]string, len(rows))
	for _, row := range tags {
		tagsByMemory[row.MemoryID] = append(tagsByMemory[row.MemoryID], row.Tag)
	}
	sourcesByMemory := make(map[string][]string, len(rows))
	for _, row := range sources {
		sourcesByMemory[row.MemoryID] = append(sourcesByMemory[row.MemoryID], row.PostSlug)
	}
	out := make([]memory.Memory, 0, len(rows))
	for _, row := range rows {
		m, err := toMemory(row)
		if err != nil {
			return nil, err
		}
		m.Tags = tagsByMemory[row.ID]
		m.SourcePostSlugs = sourcesByMemory[row.ID]
		out = append(out, m)
	}
	return out, nil
}

func (s *Store) Get(ctx context.Context, userID, id string) (memory.Memory, error) {
	return loadOne(ctx, s.read, userID, id)
}

// Update applies only the present parts of the patch, each as its own statement, in one
// transaction. An edit that carried no text never names the text column at all, so two tabs
// editing two halves of one memory cannot overwrite each other.
func (s *Store) Update(ctx context.Context, userID, id string, patch memory.Patch, updatedAt time.Time) (memory.Memory, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return memory.Memory{}, fmt.Errorf("begin update memory: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	stamp := formatTime(updatedAt)

	if patch.Text != nil {
		n, err := q.UpdateMemoryText(ctx, sqlc.UpdateMemoryTextParams{
			Text: *patch.Text, UpdatedAt: stamp, ID: id, UserID: userID,
		})
		if err != nil {
			if isDuplicateText(err) {
				// The account already holds this exact text on another row. Merging the two
				// would discard one of them, so the edit is refused and the user decides.
				return memory.Memory{}, memory.ErrDuplicateText
			}
			return memory.Memory{}, fmt.Errorf("update memory text: %w", err)
		}
		if n == 0 {
			return memory.Memory{}, memory.ErrNotFound
		}
	}
	if patch.Kind != nil {
		n, err := q.UpdateMemoryKind(ctx, sqlc.UpdateMemoryKindParams{
			Kind: string(*patch.Kind), UpdatedAt: stamp, ID: id, UserID: userID,
		})
		if err != nil {
			return memory.Memory{}, fmt.Errorf("update memory kind: %w", err)
		}
		if n == 0 {
			return memory.Memory{}, memory.ErrNotFound
		}
	}
	if patch.Tags != nil {
		// A tag-only edit still stamps updated_at: replacing the set is an authored change
		// even though no column of `memories` carries it.
		n, err := q.TouchMemoryUpdatedAt(ctx, sqlc.TouchMemoryUpdatedAtParams{
			UpdatedAt: stamp, ID: id, UserID: userID,
		})
		if err != nil {
			return memory.Memory{}, fmt.Errorf("touch memory: %w", err)
		}
		if n == 0 {
			return memory.Memory{}, memory.ErrNotFound
		}
		if err := q.DeleteMemoryTags(ctx, sqlc.DeleteMemoryTagsParams{MemoryID: id, UserID: userID}); err != nil {
			return memory.Memory{}, fmt.Errorf("clear memory tags: %w", err)
		}
		if err := insertTags(ctx, q, userID, id, *patch.Tags); err != nil {
			return memory.Memory{}, err
		}
	}
	updated, err := loadOne(ctx, q, userID, id)
	if err != nil {
		return memory.Memory{}, err
	}
	if err := tx.Commit(); err != nil {
		return memory.Memory{}, fmt.Errorf("commit update memory: %w", err)
	}
	return updated, nil
}

func (s *Store) Delete(ctx context.Context, userID, id string) error {
	n, err := s.write.DeleteMemory(ctx, sqlc.DeleteMemoryParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	if n == 0 {
		return memory.ErrNotFound
	}
	return nil
}

// DropPostSources is MEM-17 in one transaction: the memories the post fed are read FIRST,
// then its links go, then exactly those that just lost their last link are deleted. The
// delete is scoped to that read set on purpose — a memory written by hand has no links at
// all, and a rule phrased as "delete every memory with no sources" would take it too.
func (s *Store) DropPostSources(ctx context.Context, userID, postSlug string) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin drop memory sources: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)

	ids, err := q.ListMemoryIDsForPost(ctx, sqlc.ListMemoryIDsForPostParams{UserID: userID, PostSlug: postSlug})
	if err != nil {
		return fmt.Errorf("list memories of post: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if err := q.DeleteMemorySourcesForPost(ctx, sqlc.DeleteMemorySourcesForPostParams{UserID: userID, PostSlug: postSlug}); err != nil {
		return fmt.Errorf("drop memory sources of post: %w", err)
	}
	for _, id := range ids {
		if _, err := q.DeleteMemoryIfUnsourced(ctx, sqlc.DeleteMemoryIfUnsourcedParams{ID: id, UserID: userID}); err != nil {
			return fmt.Errorf("delete orphaned memory: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit drop memory sources: %w", err)
	}
	return nil
}

func insertTags(ctx context.Context, q *sqlc.Queries, userID, memoryID string, tags []string) error {
	for _, tag := range tags {
		if err := q.InsertMemoryTag(ctx, sqlc.InsertMemoryTagParams{MemoryID: memoryID, UserID: userID, Tag: tag}); err != nil {
			return fmt.Errorf("insert memory tag: %w", err)
		}
	}
	return nil
}

// linkSource records the post this fact was approved from. An empty slug is a memory written
// by hand, which links nothing.
func linkSource(ctx context.Context, q *sqlc.Queries, userID, memoryID, postSlug string, at time.Time) error {
	if postSlug == "" {
		return nil
	}
	if err := q.InsertMemorySource(ctx, sqlc.InsertMemorySourceParams{
		MemoryID: memoryID, UserID: userID, PostSlug: postSlug, CreatedAt: formatTime(at),
	}); err != nil {
		return fmt.Errorf("link memory source: %w", err)
	}
	return nil
}

func loadOne(ctx context.Context, q *sqlc.Queries, userID, id string) (memory.Memory, error) {
	row, err := q.GetMemory(ctx, sqlc.GetMemoryParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return memory.Memory{}, memory.ErrNotFound
		}
		return memory.Memory{}, fmt.Errorf("get memory: %w", err)
	}
	m, err := toMemory(row)
	if err != nil {
		return memory.Memory{}, err
	}
	if m.Tags, err = q.ListTagsOfMemory(ctx, sqlc.ListTagsOfMemoryParams{MemoryID: id, UserID: userID}); err != nil {
		return memory.Memory{}, fmt.Errorf("list tags of memory: %w", err)
	}
	if m.SourcePostSlugs, err = q.ListSourcesOfMemory(ctx, sqlc.ListSourcesOfMemoryParams{MemoryID: id, UserID: userID}); err != nil {
		return memory.Memory{}, fmt.Errorf("list sources of memory: %w", err)
	}
	return m, nil
}

// toMemory parses the stored kind rather than casting it. The CHECK constraint already
// refuses an unknown one, so a failure here means the column was written around the schema
// — which is worth an error rather than a zero value that retrieval would silently gate.
func toMemory(row sqlc.Memory) (memory.Memory, error) {
	kind, err := memory.ParseKind(row.Kind)
	if err != nil {
		return memory.Memory{}, fmt.Errorf("memory %s carries kind %q: %w", row.ID, row.Kind, err)
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return memory.Memory{}, err
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return memory.Memory{}, err
	}
	lastSeen, err := parseTime(row.LastSeenAt)
	if err != nil {
		return memory.Memory{}, err
	}
	return memory.Memory{
		ID: row.ID, UserID: row.UserID, Text: row.Text, Kind: kind,
		CreatedAt: created, UpdatedAt: updated, LastSeenAt: lastSeen,
	}, nil
}

func formatTime(t time.Time) string { return t.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse memory timestamp %q: %w", value, err)
	}
	return parsed, nil
}

// isDuplicateText recognizes the UNIQUE(user_id, text) violation without depending on the
// driver's error type, the way the guideline store does for the same constraint shape.
func isDuplicateText(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToUpper(err.Error())
	return strings.Contains(message, "UNIQUE CONSTRAINT FAILED") && strings.Contains(message, "MEMORIES.TEXT")
}

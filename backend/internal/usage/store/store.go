// Package store persists the usage context. It is the anti-corruption boundary on the
// database side (ARCHITECTURE §2.2): sqlc row structs and driver errors stop here.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/usage/store/sqlc"
)

// writeLayout is the fixed-width RFC3339 the whole database uses. The width matters here
// more than anywhere: every quota window is a lexicographic BETWEEN over these strings, so
// a variable-width fraction would sort a row into the wrong day.
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// Store implements usage.Store over SQLite.
//
// writer is kept alongside the query set because a transaction must be opened on the same
// handle the queries run against — and that handle is capped at one connection, so a
// transaction-scoped store must never fall back to the pool or it would wait on itself.
type Store struct {
	writer *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

// InWriteTx runs fn against a store bound to one write transaction.
//
// BEGIN IMMEDIATE is the point: SQLite would otherwise start a deferred transaction that
// takes its write lock only at the first write, which is exactly the window in which two
// admissions could both read the same count and both pass.
func (s *Store) InWriteTx(ctx context.Context, fn func(usage.Store) error) error {
	if s.writer == nil {
		// Already inside a transaction — nesting would deadlock on the single writer.
		return fn(s)
	}
	conn, err := s.writer.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire writer: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin write transaction: %w", err)
	}
	scoped := &Store{write: sqlc.New(conn), read: sqlc.New(conn)}
	if err := fn(scoped); err != nil {
		if _, rollbackErr := conn.ExecContext(ctx, "ROLLBACK"); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback: %w", rollbackErr))
		}
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit write transaction: %w", err)
	}
	return nil
}

func (s *Store) CountAdmissions(ctx context.Context, userID string, from, to time.Time) (int64, error) {
	// Admission reads happen on the writer: the count they produce is about to decide a
	// write, and WAL's readers may still be a commit behind.
	n, err := s.write.CountAdmissionsInWindow(ctx, sqlc.CountAdmissionsInWindowParams{
		UserID: userID, CreatedAt: formatTime(from), CreatedAt_2: formatTime(to),
	})
	if err != nil {
		return 0, fmt.Errorf("count admissions: %w", err)
	}
	return n, nil
}

func (s *Store) InsertAdmission(ctx context.Context, admission usage.Admission) error {
	err := s.write.InsertAdmission(ctx, sqlc.InsertAdmissionParams{
		UserID:    admission.UserID,
		Kind:      admission.Kind,
		JobID:     admission.JobID,
		CreatedAt: formatTime(admission.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert admission: %w", err)
	}
	return nil
}

func (s *Store) DeleteAdmissionForJob(ctx context.Context, jobID string) error {
	if err := s.write.DeleteAdmissionForJob(ctx, jobID); err != nil {
		return fmt.Errorf("delete admission: %w", err)
	}
	return nil
}

func (s *Store) SumCost(ctx context.Context, userID string, from, to time.Time) (int64, error) {
	total, err := s.write.SumCostInWindow(ctx, sqlc.SumCostInWindowParams{
		UserID: userID, CreatedAt: formatTime(from), CreatedAt_2: formatTime(to),
	})
	if err != nil {
		return 0, fmt.Errorf("sum cost: %w", err)
	}
	return total, nil
}

func (s *Store) InsertEvent(ctx context.Context, event usage.Event) error {
	err := s.write.InsertEvent(ctx, sqlc.InsertEventParams{
		UserID:           event.UserID,
		Kind:             event.Kind,
		JobID:            event.JobID,
		Stage:            event.Stage,
		Model:            event.Model,
		PromptTokens:     event.PromptTokens,
		CompletionTokens: event.CompletionTokens,
		CostMicrousd:     event.CostMicrousd,
		CostSource:       string(event.CostSource),
		CreatedAt:        formatTime(event.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert usage event: %w", err)
	}
	return nil
}

// formatTime normalizes to UTC before formatting so stored values sort against each other
// regardless of the offset the caller's clock carried. The Asia/Seoul window boundaries
// arrive here as ordinary instants and become their UTC equivalents.
func formatTime(t time.Time) string { return t.UTC().Format(writeLayout) }

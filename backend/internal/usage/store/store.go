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
// more than anywhere: a lot's expiry is compared and ordered as a string, so a
// variable-width fraction would sort a grant into the wrong position in the consumption
// order — or hide one that has not actually lapsed.
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

func (s *Store) LotsInConsumptionOrder(
	ctx context.Context, userID string, now time.Time,
) ([]usage.Lot, error) {
	// Lot reads happen on the writer: the balance they produce is about to decide a write,
	// and WAL's readers may still be a commit behind.
	rows, err := s.write.LotsInConsumptionOrder(ctx, sqlc.LotsInConsumptionOrderParams{
		UserID: userID, ExpiresAt: sql.NullString{String: formatTime(now), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("read credit lots: %w", err)
	}
	lots := make([]usage.Lot, 0, len(rows))
	for _, row := range rows {
		lot, err := toLot(row)
		if err != nil {
			return nil, err
		}
		lots = append(lots, lot)
	}
	return lots, nil
}

func (s *Store) ActiveMonthlyLot(
	ctx context.Context, userID string, now time.Time,
) (usage.Lot, bool, error) {
	row, err := s.write.ActiveMonthlyLot(ctx, sqlc.ActiveMonthlyLotParams{
		UserID: userID, ExpiresAt: sql.NullString{String: formatTime(now), Valid: true},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return usage.Lot{}, false, nil
	}
	if err != nil {
		return usage.Lot{}, false, fmt.Errorf("read monthly lot: %w", err)
	}
	lot, err := toLot(row)
	if err != nil {
		return usage.Lot{}, false, err
	}
	return lot, true, nil
}

func (s *Store) InsertLot(ctx context.Context, lot usage.Lot) error {
	expires := sql.NullString{}
	if lot.ExpiresAt != nil {
		expires = sql.NullString{String: formatTime(*lot.ExpiresAt), Valid: true}
	}
	err := s.write.InsertLot(ctx, sqlc.InsertLotParams{
		ID:        lot.ID,
		UserID:    lot.UserID,
		Kind:      string(lot.Kind),
		Granted:   int64(lot.Granted),
		Remaining: int64(lot.Remaining),
		ExpiresAt: expires,
		CreatedAt: formatTime(lot.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert credit lot: %w", err)
	}
	return nil
}

func (s *Store) InsertLotIfAbsent(ctx context.Context, lot usage.Lot) error {
	expires := sql.NullString{}
	if lot.ExpiresAt != nil {
		expires = sql.NullString{String: formatTime(*lot.ExpiresAt), Valid: true}
	}
	err := s.write.InsertLotIfAbsent(ctx, sqlc.InsertLotIfAbsentParams{
		ID:        lot.ID,
		UserID:    lot.UserID,
		Kind:      string(lot.Kind),
		Granted:   int64(lot.Granted),
		Remaining: int64(lot.Remaining),
		ExpiresAt: expires,
		CreatedAt: formatTime(lot.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert credit lot if absent: %w", err)
	}
	return nil
}

func (s *Store) SpendFromLot(ctx context.Context, lotID string, credits int) error {
	err := s.write.SpendFromLot(ctx, sqlc.SpendFromLotParams{
		Remaining: int64(credits), ID: lotID, Remaining_2: int64(credits),
	})
	if err != nil {
		return fmt.Errorf("spend from credit lot: %w", err)
	}
	return nil
}

func (s *Store) RefundToLot(ctx context.Context, lotID string, credits int) error {
	err := s.write.RefundToLot(ctx, sqlc.RefundToLotParams{
		Remaining: int64(credits), ID: lotID, Remaining_2: int64(credits),
	})
	if err != nil {
		return fmt.Errorf("refund to credit lot: %w", err)
	}
	return nil
}

func (s *Store) InsertAdmission(ctx context.Context, admission usage.Admission) error {
	err := s.write.InsertAdmission(ctx, sqlc.InsertAdmissionParams{
		UserID:      admission.UserID,
		Kind:        admission.Kind,
		JobID:       admission.JobID,
		HoldCredits: int64(admission.HoldCredits),
		CreatedAt:   formatTime(admission.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert admission: %w", err)
	}
	return nil
}

func (s *Store) InsertHoldDebits(ctx context.Context, jobID string, debits []usage.LotDebit) error {
	for _, debit := range debits {
		err := s.write.InsertHoldDebit(ctx, sqlc.InsertHoldDebitParams{
			JobID: jobID, LotID: debit.LotID, Credits: int64(debit.Credits),
		})
		if err != nil {
			return fmt.Errorf("insert hold debit: %w", err)
		}
	}
	return nil
}

func (s *Store) HoldForJob(
	ctx context.Context, jobID string,
) (usage.Admission, []usage.LotDebit, bool, error) {
	row, err := s.write.OpenAdmissionForJob(ctx, jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return usage.Admission{}, nil, false, nil
	}
	if err != nil {
		return usage.Admission{}, nil, false, fmt.Errorf("read open admission: %w", err)
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return usage.Admission{}, nil, false, err
	}

	debitRows, err := s.write.HoldDebitsForJob(ctx, jobID)
	if err != nil {
		return usage.Admission{}, nil, false, fmt.Errorf("read hold debits: %w", err)
	}
	debits := make([]usage.LotDebit, 0, len(debitRows))
	for _, debit := range debitRows {
		debits = append(debits, usage.LotDebit{LotID: debit.LotID, Credits: int(debit.Credits)})
	}

	return usage.Admission{
		UserID: row.UserID, Kind: row.Kind, JobID: row.JobID,
		HoldCredits: int(row.HoldCredits), CreatedAt: created,
	}, debits, true, nil
}

func (s *Store) MarkSettled(ctx context.Context, jobID string, credits int, at time.Time) error {
	err := s.write.MarkAdmissionSettled(ctx, sqlc.MarkAdmissionSettledParams{
		SettledCredits: sql.NullInt64{Int64: int64(credits), Valid: true},
		SettledAt:      sql.NullString{String: formatTime(at), Valid: true},
		JobID:          jobID,
	})
	if err != nil {
		return fmt.Errorf("mark admission settled: %w", err)
	}
	return nil
}

func (s *Store) UnsettledHoldJobs(ctx context.Context) ([]string, error) {
	jobs, err := s.read.UnsettledHoldJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("read unsettled holds: %w", err)
	}
	return jobs, nil
}

func (s *Store) DeleteAdmissionForJob(ctx context.Context, jobID string) error {
	if err := s.write.DeleteAdmissionForJob(ctx, jobID); err != nil {
		return fmt.Errorf("delete admission: %w", err)
	}
	return nil
}

func (s *Store) SumCostForJob(ctx context.Context, jobID string) (int64, error) {
	total, err := s.write.SumCostForJob(ctx, jobID)
	if err != nil {
		return 0, fmt.Errorf("sum job cost: %w", err)
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
		ReasoningTokens:  event.ReasoningTokens,
		CostMicrousd:     event.CostMicrousd,
		CostSource:       string(event.CostSource),
		CreatedAt:        formatTime(event.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert usage event: %w", err)
	}
	return nil
}

// toLot maps one stored row. A NULL expiry is a lot that does not expire, which the
// domain models as a nil pointer rather than a sentinel instant — a far-future date would
// sort correctly but read as a real deadline everywhere it was displayed.
func toLot(row sqlc.CreditLot) (usage.Lot, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return usage.Lot{}, err
	}
	lot := usage.Lot{
		ID: row.ID, UserID: row.UserID, Kind: usage.LotKind(row.Kind),
		Granted: int(row.Granted), Remaining: int(row.Remaining), CreatedAt: created,
	}
	if row.ExpiresAt.Valid {
		expires, err := parseTime(row.ExpiresAt.String)
		if err != nil {
			return usage.Lot{}, err
		}
		lot.ExpiresAt = &expires
	}
	return lot, nil
}

// ReasoningSpend reads through the READ pool: it is a diagnostic aggregate over a window of
// rows, not part of any write path, so it must not queue behind the single writer.
func (s *Store) ReasoningSpend(ctx context.Context, stage string, since time.Time) ([]usage.ReasoningSpend, error) {
	rows, err := s.read.ReasoningSpendByStage(ctx, sqlc.ReasoningSpendByStageParams{
		Stage: stage, CreatedAt: formatTime(since),
	})
	if err != nil {
		return nil, fmt.Errorf("aggregate reasoning spend: %w", err)
	}
	out := make([]usage.ReasoningSpend, 0, len(rows))
	for _, row := range rows {
		out = append(out, usage.ReasoningSpend{
			Model: row.Model, Stage: stage, Calls: row.Calls,
			ReasoningTokens: row.ReasoningTokens, CompletionTokens: row.CompletionTokens,
		})
	}
	return out, nil
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(writeLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored instant %q: %w", value, err)
	}
	return parsed, nil
}

// formatTime normalizes to UTC before formatting so stored values sort against each other
// regardless of the offset the caller's clock carried. The Asia/Seoul window boundaries
// arrive here as ordinary instants and become their UTC equivalents.
func formatTime(t time.Time) string { return t.UTC().Format(writeLayout) }

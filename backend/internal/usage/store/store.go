// Package store persists the usage context. It is the anti-corruption boundary on the
// database side (ARCHITECTURE §2.2): sqlc row structs and driver errors stop here.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/usage/store/sqlc"
)

// writeLayout is the fixed-width RFC3339 the whole database uses. The width matters here
// more than anywhere: a lot's expiry is compared and ordered as a string, so a
// variable-width fraction would sort a grant into the wrong position in the consumption
// order — or hide one that has not actually lapsed.
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func nullableRate(rate int64) sql.NullInt64 { return sql.NullInt64{Int64: rate, Valid: rate > 0} }
func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

// Store implements usage.Storage over SQLite.
//
// writer is kept alongside the query set because a transaction must be opened on the same
// handle the queries run against — and that handle is capped at one connection, so a
// transaction-scoped store must never fall back to the pool or it would wait on itself.
type Store struct {
	writer       *sql.DB
	write        *sqlc.Queries
	read         *sqlc.Queries
	raw          sqlc.DBTX
	exports      usage.ExportWindowLedger
	exportsForTx func(*sql.Tx) usage.ExportWindowLedger
	// exportsOpened answers the renewal probe about the window exportsForTx would open.
	exportsOpened usage.ExportWindowReader
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader), raw: writer}
}

// NewWithExports makes lazy credit and export grants part of the same writer
// transaction. The factory is required for production's paid-entitlement ledger, and so
// is opened, the read-pool answer to whether that window is already open, which lets a
// balance read skip the writer when nothing needs granting.
func NewWithExports(writer, reader *sql.DB, factory func(*sql.Tx) usage.ExportWindowLedger, opened usage.ExportWindowReader) *Store {
	if factory == nil || opened == nil {
		panic("usage: export window transaction factory and reader are required")
	}
	store := New(writer, reader)
	store.exportsForTx, store.exportsOpened = factory, opened
	return store
}

// NewTx binds usage operations to a transaction connection owned by a composition-level
// coordinator. It exists for money flows that must update billing rows and credit lots in
// one SQLite transaction without either context reading the other's tables.
func NewTx(tx *sql.Tx) *Store {
	return &Store{write: sqlc.New(tx), read: sqlc.New(tx), raw: tx}
}

// InWriteTx runs fn against a store bound to one write transaction.
//
// The writer pool opens immediate transactions (platform/db), so the write lock is held
// from BEGIN — SQLite's deferred default would take it only at the first write, which is
// exactly the window in which two admissions could both read the same count and both pass.
func (s *Store) InWriteTx(ctx context.Context, fn func(usage.Storage) error) error {
	if s.writer == nil {
		// Already inside a transaction — nesting would deadlock on the single writer.
		return fn(s)
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin write transaction: %w", err)
	}
	// database/sql owns the undo: it rolls the transaction back when the caller's context
	// dies, and this deferred call is a no-op once Commit has landed. Issuing ROLLBACK by
	// hand on the request's own context was what left the single writer connection inside
	// an open transaction, failing every later write in the process until a restart.
	defer func() { _ = tx.Rollback() }()
	var exports usage.ExportWindowLedger
	if s.exportsForTx != nil {
		exports = s.exportsForTx(tx)
	}
	if err := fn(&Store{write: sqlc.New(tx), read: sqlc.New(tx), raw: tx, exports: exports}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit write transaction: %w", err)
	}
	return nil
}

func (s *Store) OpenExportWindow(ctx context.Context, window usage.ExportWindow) error {
	if s.exports == nil {
		return nil
	} // Credit-only adapters compose exports in their owner.
	return s.exports.OpenExportWindow(ctx, window)
}

// ExportWindowOpened reports whether OpenExportWindow would leave window as it is. A store
// that composes no export window never writes one; inside a transaction there is no read
// pool to ask, so the answer there is "it would write".
func (s *Store) ExportWindowOpened(ctx context.Context, window usage.ExportWindow) (bool, error) {
	if s.exports == nil && s.exportsForTx == nil {
		return true, nil
	}
	if s.exportsOpened == nil {
		return false, nil
	}
	return s.exportsOpened.ExportWindowOpened(ctx, window)
}

// WindowGrantsOpened compares each grant's stored expiry text with the one it would be
// opened with, exactly as InsertLotIfAbsent's conflict clause does.
func (s *Store) WindowGrantsOpened(ctx context.Context, grants []usage.Lot) (bool, error) {
	if len(grants) == 0 {
		return true, nil
	}
	ids := make([]string, 0, len(grants))
	for _, grant := range grants {
		ids = append(ids, grant.ID)
	}
	rows, err := s.read.WindowLotExpiries(ctx, ids)
	if err != nil {
		return false, fmt.Errorf("read window grant expiries: %w", err)
	}
	stored := make(map[string]sql.NullString, len(rows))
	for _, row := range rows {
		stored[row.ID] = row.ExpiresAt
	}
	for _, grant := range grants {
		expires, found := stored[grant.ID]
		if !found || !expires.Valid || grant.ExpiresAt == nil || expires.String < formatTime(*grant.ExpiresAt) {
			return false, nil
		}
	}
	return true, nil
}

func (s *Store) LotsInConsumptionOrder(
	ctx context.Context, userID string, now time.Time,
) ([]usage.Lot, error) {
	// Lot reads happen on the writer: the balance they produce is about to decide a write,
	// and WAL's readers may still be a commit behind.
	return lotsInConsumptionOrder(ctx, s.write, userID, now)
}

// ReadLotsInConsumptionOrder is the same read on the read pool, for a balance that is
// shown rather than spent.
func (s *Store) ReadLotsInConsumptionOrder(
	ctx context.Context, userID string, now time.Time,
) ([]usage.Lot, error) {
	return lotsInConsumptionOrder(ctx, s.read, userID, now)
}

func lotsInConsumptionOrder(ctx context.Context, q *sqlc.Queries, userID string, now time.Time) ([]usage.Lot, error) {
	rows, err := q.LotsInConsumptionOrder(ctx, sqlc.LotsInConsumptionOrderParams{
		UserID: userID, ExpiresAt: sql.NullString{String: formatTime(now), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("read credit lots: %w", err)
	}
	lots := make([]usage.Lot, 0, len(rows))
	for _, row := range rows {
		lot, err := toLot(sqlc.ActiveMonthlyLotRow(row))
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
		ID:         lot.ID,
		UserID:     lot.UserID,
		Kind:       string(lot.Kind),
		Granted:    int64(lot.Granted),
		Remaining:  int64(lot.Remaining),
		ExpiresAt:  expires,
		CreatedAt:  formatTime(lot.CreatedAt),
		CoverageID: nullable(lot.CoverageID), WindowStart: nullableTime(lot.WindowStart),
		IssuanceCause: nullable(lot.IssuanceCause), CorrelationID: nullable(lot.CorrelationID),
	})
	if err != nil {
		return fmt.Errorf("insert credit lot: %w", err)
	}
	return nil
}

func (s *Store) InsertLotIfAbsent(ctx context.Context, lot usage.Lot) (bool, error) {
	expires := sql.NullString{}
	if lot.ExpiresAt != nil {
		expires = sql.NullString{String: formatTime(*lot.ExpiresAt), Valid: true}
	}
	rows, err := s.write.InsertLotIfAbsent(ctx, sqlc.InsertLotIfAbsentParams{
		ID:         lot.ID,
		UserID:     lot.UserID,
		Kind:       string(lot.Kind),
		Granted:    int64(lot.Granted),
		Remaining:  int64(lot.Remaining),
		ExpiresAt:  expires,
		CreatedAt:  formatTime(lot.CreatedAt),
		CoverageID: nullable(lot.CoverageID), WindowStart: nullableTime(lot.WindowStart),
		IssuanceCause: nullable(lot.IssuanceCause), CorrelationID: nullable(lot.CorrelationID),
	})
	if err != nil {
		return false, fmt.Errorf("insert credit lot if absent: %w", err)
	}
	return rows > 0, nil
}

func (s *Store) RaiseLot(ctx context.Context, lotID string, credits int) error {
	err := s.write.RaiseLot(ctx, sqlc.RaiseLotParams{
		Granted: int64(credits), Remaining: int64(credits), ID: lotID,
	})
	if err != nil {
		return fmt.Errorf("raise credit lot: %w", err)
	}
	return nil
}

func (s *Store) UntouchedPurchasedLots(ctx context.Context, lotIDs []string) ([]string, error) {
	if len(lotIDs) == 0 {
		return nil, nil
	}
	ids, err := s.read.UntouchedPurchasedLots(ctx, lotIDs)
	if err != nil {
		return nil, fmt.Errorf("read untouched purchased lots: %w", err)
	}
	return ids, nil
}

func (s *Store) ExpireVoucherLot(ctx context.Context, lotID string, at time.Time) (bool, error) {
	instant := sql.NullString{String: formatTime(at), Valid: true}
	rows, err := s.write.ExpireVoucherLot(ctx, sqlc.ExpireVoucherLotParams{
		ExpiresAt: instant, ID: lotID, ExpiresAt_2: instant,
	})
	if err != nil {
		return false, fmt.Errorf("expire voucher credit lot: %w", err)
	}
	return rows > 0, nil
}

func (s *Store) VoucherLots(ctx context.Context, lotIDs []string) ([]usage.Lot, error) {
	if len(lotIDs) == 0 {
		return nil, nil
	}
	rows, err := s.read.VoucherLots(ctx, lotIDs)
	if err != nil {
		return nil, fmt.Errorf("read voucher credit lots: %w", err)
	}
	lots := make([]usage.Lot, 0, len(rows))
	for _, row := range rows {
		lot, err := toLot(sqlc.ActiveMonthlyLotRow(row))
		if err != nil {
			return nil, err
		}
		lots = append(lots, lot)
	}
	return lots, nil
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
	models, err := json.Marshal(admission.AdmittedModels)
	if err != nil {
		return fmt.Errorf("encode admitted models: %w", err)
	}
	if len(admission.AdmittedModels) == 0 {
		models = []byte("[]")
	}
	err = s.write.InsertAdmission(ctx, sqlc.InsertAdmissionParams{
		UserID:                    admission.UserID,
		Kind:                      admission.Kind,
		JobID:                     admission.JobID,
		HoldCredits:               int64(admission.HoldCredits),
		CreatedAt:                 formatTime(admission.CreatedAt),
		ApprovedMaxCredits:        nullableCredits(admission.ApprovedMaxCredits),
		CancellationPolicyVersion: int64(admission.CancellationPolicyVersion),
		CoverageID:                nullable(admission.CoverageID),
		DailyWindowStart:          nullableTime(admission.DailyWindowStart),
		BenefitWindowStart:        nullableTime(admission.BenefitWindowStart),
		FxSource:                  nullable(admission.Rate.Source),
		FxPublicationDate:         nullable(admission.Rate.PublicationDate),
		FxReferenceE4:             nullableRate(admission.Rate.ReferenceE4),
		FxAppliedE4:               nullableRate(admission.Rate.AppliedE4),
		FxTemporary:               boolInt(admission.Rate.Temporary),
		AdmittedPlan:              nullable(string(admission.AdmittedPlan)),
		AdmittedModelsJson:        string(models),
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
			OriginCoverageID:  nullable(debit.OriginCoverageID),
			OriginWindowStart: nullableTime(debit.OriginWindowStart),
		})
		if err != nil {
			return fmt.Errorf("insert hold debit: %w", err)
		}
	}
	return nil
}

func (s *Store) InsertEligibleLots(ctx context.Context, jobID string, lots []usage.Lot) error {
	for _, lot := range lots {
		if err := s.write.InsertEligibleLot(ctx, sqlc.InsertEligibleLotParams{JobID: jobID, LotID: lot.ID}); err != nil {
			return fmt.Errorf("snapshot eligible lot: %w", err)
		}
	}
	return nil
}

func (s *Store) EligibleLotsForJob(ctx context.Context, jobID string) ([]usage.Lot, error) {
	rows, err := s.write.EligibleLotsForJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("read eligible lots: %w", err)
	}
	lots := make([]usage.Lot, 0, len(rows))
	for _, row := range rows {
		lot, err := toLot(sqlc.ActiveMonthlyLotRow(row))
		if err != nil {
			return nil, err
		}
		lots = append(lots, lot)
	}
	return lots, nil
}

func (s *Store) DeleteEligibleLotsForJob(ctx context.Context, jobID string) error {
	if err := s.write.DeleteEligibleLotsForJob(ctx, jobID); err != nil {
		return fmt.Errorf("delete eligible lots: %w", err)
	}
	return nil
}

// HoldForJob reads on the read pool outside a transaction: every metered provider call asks
// for its job's admission, and must not queue behind the single writer for it. Inside a
// write transaction both query sets are the transaction, so the re-check that refuses a
// second hold still reads what that transaction is about to decide on.
func (s *Store) HoldForJob(
	ctx context.Context, jobID string,
) (usage.Admission, []usage.LotDebit, bool, error) {
	row, err := s.read.OpenAdmissionForJob(ctx, jobID)
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

	debitRows, err := s.read.HoldDebitsForJob(ctx, jobID)
	if err != nil {
		return usage.Admission{}, nil, false, fmt.Errorf("read hold debits: %w", err)
	}
	debits := make([]usage.LotDebit, 0, len(debitRows))
	for _, debit := range debitRows {
		debits = append(debits, usage.LotDebit{LotID: debit.LotID, Credits: int(debit.Credits)})
	}

	admission := usage.Admission{
		UserID: row.UserID, Kind: row.Kind, JobID: row.JobID,
		HoldCredits: int(row.HoldCredits), CreatedAt: created,
		AdmittedPlan:              plan.Plan(row.AdmittedPlan.String),
		ApprovedMaxCredits:        optionalCredits(row.ApprovedMaxCredits),
		CancellationPolicyVersion: int(row.CancellationPolicyVersion),
		CoverageID:                row.CoverageID.String,
		Rate: plan.RateSnapshot{Source: row.FxSource.String, PublicationDate: row.FxPublicationDate.String,
			ReferenceE4: row.FxReferenceE4.Int64, AppliedE4: row.FxAppliedE4.Int64,
			Temporary: row.FxTemporary != 0},
	}
	if err := json.Unmarshal([]byte(row.AdmittedModelsJson), &admission.AdmittedModels); err != nil {
		return usage.Admission{}, nil, false, fmt.Errorf("decode admitted models: %w", err)
	}
	if row.DailyWindowStart.Valid {
		value, err := parseTime(row.DailyWindowStart.String)
		if err != nil {
			return usage.Admission{}, nil, false, err
		}
		admission.DailyWindowStart = &value
	}
	if row.BenefitWindowStart.Valid {
		value, err := parseTime(row.BenefitWindowStart.String)
		if err != nil {
			return usage.Admission{}, nil, false, err
		}
		admission.BenefitWindowStart = &value
	}
	return admission, debits, true, nil
}

func (s *Store) MarkSettled(ctx context.Context, jobID string, settlement usage.Settlement, at time.Time) error {
	err := s.write.MarkAdmissionSettled(ctx, sqlc.MarkAdmissionSettledParams{
		SettledCredits:         sql.NullInt64{Int64: int64(settlement.Credits), Valid: true},
		SettledAt:              sql.NullString{String: formatTime(at), Valid: true},
		SettlementReason:       sql.NullString{String: string(settlement.Reason), Valid: settlement.Reason != ""},
		ConfirmedChargeCredits: nullableCredits(settlement.ConfirmedCharge),
		CancellationFeeCredits: nullableCredits(settlement.CancellationFee),
		SettlementCause:        nullable(settlement.Cause),
		CompensationCredits:    nullableRate(int64(settlement.CompensationCredits)),
		CompensationLotID:      nullable(settlement.CompensationLotID),
		CompensationExpiresAt:  nullableTime(settlement.CompensationExpiresAt),
		JobID:                  jobID,
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

func (s *Store) CostForJob(ctx context.Context, jobID string) (usage.JobCost, error) {
	cost, err := s.write.CostForJob(ctx, jobID)
	if err != nil {
		return usage.JobCost{}, fmt.Errorf("read job cost: %w", err)
	}
	return usage.JobCost{TotalMicrousd: cost.TotalMicrousd, ConfirmedMicrousd: cost.ConfirmedMicrousd}, nil
}

func (s *Store) InsertEvent(ctx context.Context, event usage.Event) error {
	if event.Units != nil {
		return s.InsertUnitEvent(ctx, event, *event.Units)
	}
	err := s.write.InsertEvent(ctx, sqlc.InsertEventParams{
		UserID:             event.UserID,
		Kind:               event.Kind,
		JobID:              event.JobID,
		Stage:              event.Stage,
		Model:              event.Model,
		PromptTokens:       event.PromptTokens,
		CompletionTokens:   event.CompletionTokens,
		ReasoningTokens:    event.ReasoningTokens,
		ReasoningTruncated: boolToInt64(event.ReasoningTruncated),
		CostMicrousd:       event.CostMicrousd,
		CostSource:         string(event.CostSource),
		CreatedAt:          formatTime(event.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert usage event: %w", err)
	}
	return nil
}

// toLot maps one stored row. A NULL expiry is a lot that does not expire, which the
// domain models as a nil pointer rather than a sentinel instant — a far-future date would
// sort correctly but read as a real deadline everywhere it was displayed.
func toLot(row sqlc.ActiveMonthlyLotRow) (usage.Lot, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return usage.Lot{}, err
	}
	lot := usage.Lot{
		ID: row.ID, UserID: row.UserID, Kind: usage.LotKind(row.Kind),
		CoverageID: row.CoverageID.String, IssuanceCause: row.IssuanceCause.String,
		CorrelationID: row.CorrelationID.String,
		Granted:       int(row.Granted), Remaining: int(row.Remaining), CreatedAt: created,
	}
	if row.ExpiresAt.Valid {
		expires, err := parseTime(row.ExpiresAt.String)
		if err != nil {
			return usage.Lot{}, err
		}
		lot.ExpiresAt = &expires
	}
	if row.WindowStart.Valid {
		start, err := parseTime(row.WindowStart.String)
		if err != nil {
			return usage.Lot{}, err
		}
		lot.WindowStart = &start
	}
	return lot, nil
}

func nullable(value string) sql.NullString { return sql.NullString{String: value, Valid: value != ""} }
func nullableTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*value), Valid: true}
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
			ReasoningTruncations: row.ReasoningTruncations,
		})
	}
	return out, nil
}

// PostStageCosts reads through the READ pool for the same reason ReasoningSpend does: it is
// an aggregate over a window, never part of a write.
func (s *Store) PostStageCosts(ctx context.Context, since time.Time) ([]usage.PostStageCost, error) {
	rows, err := s.read.RecentPostStageCosts(ctx, sql.NullString{String: formatTime(since), Valid: true})
	if err != nil {
		return nil, fmt.Errorf("read recent post stage costs: %w", err)
	}
	out := make([]usage.PostStageCost, 0, len(rows))
	for _, row := range rows {
		providerID, modelID, ok := strings.Cut(row.Model, "/")
		if !ok {
			continue
		}
		out = append(out, usage.PostStageCost{
			JobID: row.JobID, UserID: row.UserID, Stage: row.Stage,
			Model:        llm.ModelRef{ProviderID: providerID, ModelID: modelID},
			CostMicrousd: row.CostMicrousd,
			Rate: plan.RateSnapshot{Source: row.FxSource.String, PublicationDate: row.FxPublicationDate.String,
				ReferenceE4: row.FxReferenceE4.Int64, AppliedE4: row.FxAppliedE4.Int64},
		})
	}
	return out, nil
}

// parseTime reads with RFC3339Nano rather than writeLayout, as the post and auth stores
// do: it accepts any fraction width, so a row written by hand or by a test fixture with a
// trimmed fraction still loads. Only the WRITE side pins the width, and that is what the
// string ordering above depends on.
func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored instant %q: %w", value, err)
	}
	return parsed, nil
}

// formatTime normalizes to UTC before formatting so stored values sort against each other
// regardless of the offset the caller's clock carried. The Asia/Seoul window boundaries
// arrive here as ordinary instants and become their UTC equivalents.
func formatTime(t time.Time) string { return t.UTC().Format(writeLayout) }

func boolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

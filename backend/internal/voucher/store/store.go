// Package store maps the voucher domain to its SQLite-owned schema.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/voucher"
	"github.com/postpilot/backend/internal/voucher/store/sqlc"
)

// writeLayout pins the fraction width so stored instants sort as strings.
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer       *sql.DB
	write        *sqlc.Queries
	read         *sqlc.Queries
	creditsForTx func(*sql.Tx) voucher.Credits
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

// SetCreditsForTx attaches the ledger adapter at the composition root. The factory binds it
// to this store's transaction, so a redemption's lot and its row commit together without
// either store importing the other.
func (s *Store) SetCreditsForTx(factory func(*sql.Tx) voucher.Credits) {
	s.creditsForTx = factory
}

func (s *Store) InWriteTx(ctx context.Context, fn func(voucher.Store, voucher.Credits) error) error {
	if s.writer == nil || s.creditsForTx == nil {
		return errors.New("voucher transaction is not configured")
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin voucher transaction: %w", err)
	}
	// database/sql rolls the transaction back when the caller's context dies, which a
	// hand-written ROLLBACK on that same dead context could not.
	defer func() { _ = tx.Rollback() }()
	credits := s.creditsForTx(tx)
	if credits == nil {
		return errors.New("voucher transaction credit factory returned nil")
	}
	scoped := &Store{write: sqlc.New(tx), read: sqlc.New(tx)}
	if err := fn(scoped, credits); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit voucher transaction: %w", err)
	}
	return nil
}

func (s *Store) InsertVoucher(ctx context.Context, v voucher.Voucher) error {
	params := sqlc.InsertVoucherParams{
		ID: v.ID, Token: v.Token, Credits: int64(v.Credits), ValidityDays: int64(v.ValidityDays),
		Message: v.Message, IssuedBy: v.IssuedBy,
		IssuedAt: formatTime(v.IssuedAt), LinkExpiresAt: formatTime(v.LinkExpiresAt),
	}
	if v.Sale != nil {
		params.SaleKrw = sql.NullInt64{Int64: v.Sale.AmountKRW, Valid: true}
		params.PayerName = sql.NullString{String: v.Sale.Payer, Valid: true}
	}
	if err := s.write.InsertVoucher(ctx, params); err != nil {
		return fmt.Errorf("insert voucher: %w", err)
	}
	return nil
}

// VoucherByToken reads on the writer: inside a redemption it is the read the write depends
// on, and outside one a WAL reader may still be a commit behind a just-issued voucher.
func (s *Store) VoucherByToken(ctx context.Context, token string) (voucher.Voucher, bool, error) {
	row, err := s.write.VoucherByToken(ctx, token)
	return oneVoucher(row, err)
}

func (s *Store) VoucherByID(ctx context.Context, id string) (voucher.Voucher, bool, error) {
	row, err := s.write.VoucherByID(ctx, id)
	return oneVoucher(row, err)
}

// Vouchers reads on the read pool: the operator's list decides nothing.
func (s *Store) Vouchers(ctx context.Context) ([]voucher.Voucher, error) {
	rows, err := s.read.Vouchers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vouchers: %w", err)
	}
	out := make([]voucher.Voucher, 0, len(rows))
	for _, row := range rows {
		v, err := toVoucher(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) MarkRedeemed(ctx context.Context, id, userID, lotID string, at time.Time) (bool, error) {
	rows, err := s.write.MarkVoucherRedeemed(ctx, sqlc.MarkVoucherRedeemedParams{
		RedeemedBy: sql.NullString{String: userID, Valid: true},
		RedeemedAt: sql.NullString{String: formatTime(at), Valid: true},
		LotID:      sql.NullString{String: lotID, Valid: true},
		ID:         id,
	})
	if err != nil {
		return false, fmt.Errorf("mark voucher redeemed: %w", err)
	}
	return rows > 0, nil
}

func (s *Store) MarkRevoked(ctx context.Context, id string, at time.Time) (bool, error) {
	rows, err := s.write.MarkVoucherRevoked(ctx, sqlc.MarkVoucherRevokedParams{
		RevokedAt: sql.NullString{String: formatTime(at), Valid: true}, ID: id,
	})
	if err != nil {
		return false, fmt.Errorf("mark voucher revoked: %w", err)
	}
	return rows > 0, nil
}

// oneVoucher maps a single-row read. The error names no token: a lookup error must not
// carry the gift link's secret into a log.
func oneVoucher(row sqlc.Voucher, err error) (voucher.Voucher, bool, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return voucher.Voucher{}, false, nil
	}
	if err != nil {
		return voucher.Voucher{}, false, fmt.Errorf("read voucher: %w", err)
	}
	v, err := toVoucher(row)
	if err != nil {
		return voucher.Voucher{}, false, err
	}
	return v, true, nil
}

func toVoucher(row sqlc.Voucher) (voucher.Voucher, error) {
	issued, err := parseTime(row.IssuedAt)
	if err != nil {
		return voucher.Voucher{}, err
	}
	linkExpires, err := parseTime(row.LinkExpiresAt)
	if err != nil {
		return voucher.Voucher{}, err
	}
	v := voucher.Voucher{
		ID: row.ID, Token: row.Token, Credits: int(row.Credits), ValidityDays: int(row.ValidityDays),
		Message: row.Message, IssuedBy: row.IssuedBy, IssuedAt: issued, LinkExpiresAt: linkExpires,
		RedeemedBy: row.RedeemedBy.String, LotID: row.LotID.String,
	}
	if row.SaleKrw.Valid {
		v.Sale = &voucher.Sale{AmountKRW: row.SaleKrw.Int64, Payer: row.PayerName.String}
	}
	if v.RedeemedAt, err = optionalTime(row.RedeemedAt); err != nil {
		return voucher.Voucher{}, err
	}
	if v.RevokedAt, err = optionalTime(row.RevokedAt); err != nil {
		return voucher.Voucher{}, err
	}
	return v, nil
}

func optionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored instant %q: %w", value, err)
	}
	return parsed, nil
}

func formatTime(t time.Time) string { return t.UTC().Format(writeLayout) }

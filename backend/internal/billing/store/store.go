// Package store maps the billing domain to its SQLite-owned schema.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/billing/store/sqlc"
	"github.com/postpilot/backend/internal/plan"
)

const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

func (s *Store) InWriteTx(ctx context.Context, fn func(billing.Store) error) error {
	if s.writer == nil {
		return fn(s)
	}
	conn, err := s.writer.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire billing writer: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin billing transaction: %w", err)
	}
	scoped := &Store{write: sqlc.New(conn), read: sqlc.New(conn)}
	if err := fn(scoped); err != nil {
		if rollbackErr := rollback(ctx, conn); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit billing transaction: %w", err)
	}
	return nil
}

func rollback(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		return fmt.Errorf("rollback billing transaction: %w", err)
	}
	return nil
}

func (s *Store) Subscription(ctx context.Context, userID string) (billing.Subscription, bool, error) {
	row, err := s.read.GetSubscription(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return billing.Subscription{}, false, nil
	}
	if err != nil {
		return billing.Subscription{}, false, fmt.Errorf("read subscription: %w", err)
	}
	result, err := toSubscription(row)
	return result, err == nil, err
}

func (s *Store) PaymentMethod(ctx context.Context, userID string) (billing.PaymentMethod, bool, error) {
	row, err := s.read.GetPaymentMethod(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return billing.PaymentMethod{}, false, nil
	}
	if err != nil {
		return billing.PaymentMethod{}, false, fmt.Errorf("read payment method: %w", err)
	}
	registered, err := parseTime(row.RegisteredAt)
	if err != nil {
		return billing.PaymentMethod{}, false, err
	}
	return billing.PaymentMethod{UserID: row.UserID, Provider: row.Provider, BillingKey: row.BillingKey, CustomerKey: row.CustomerKey, CardLabel: row.CardLabel, RegisteredAt: registered}, true, nil
}

func (s *Store) Events(ctx context.Context, userID string, limit int) ([]billing.Event, error) {
	rows, err := s.read.ListBillingEvents(ctx, sqlc.ListBillingEventsParams{UserID: userID, Limit: int64(limit)})
	if err != nil {
		return nil, fmt.Errorf("list billing events: %w", err)
	}
	result := make([]billing.Event, 0, len(rows))
	for _, row := range rows {
		mapped, err := toEvent(row)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, nil
}

func (s *Store) Purchases(ctx context.Context, userID string) ([]billing.Purchase, error) {
	rows, err := s.read.ListCreditPurchases(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list credit purchases: %w", err)
	}
	result := make([]billing.Purchase, 0, len(rows))
	for _, row := range rows {
		charged, err := parseTime(row.ChargedAt)
		if err != nil {
			return nil, err
		}
		purchase := billing.Purchase{ID: row.ID, UserID: row.UserID, LotID: row.LotID, Credits: int(row.Credits), USDCents: int(row.UsdCents), KRW: int(row.Krw), ProviderPaymentKey: row.ProviderPaymentKey, OrderID: row.OrderID, ChargedAt: charged}
		if row.RefundedAt.Valid {
			refunded, err := parseTime(row.RefundedAt.String)
			if err != nil {
				return nil, err
			}
			purchase.RefundedAt = &refunded
		}
		result = append(result, purchase)
	}
	return result, nil
}

func (s *Store) InsertProviderNotification(ctx context.Context, n billing.ProviderNotification) error {
	err := s.write.InsertProviderNotification(ctx, sqlc.InsertProviderNotificationParams{
		Provider: n.Provider, EventType: n.EventType,
		PaymentKey: nullString(n.PaymentKey), OrderID: nullString(n.OrderID), Status: nullString(n.Status),
		Payload: n.Payload, ReceivedAt: formatTime(n.ReceivedAt),
	})
	if err != nil {
		return fmt.Errorf("insert provider notification: %w", err)
	}
	return nil
}

func toSubscription(row sqlc.Subscription) (billing.Subscription, error) {
	tier, err := plan.Parse(row.Tier)
	if err != nil {
		return billing.Subscription{}, err
	}
	anchor, err := parseTime(row.AnchorAt)
	if err != nil {
		return billing.Subscription{}, err
	}
	start, err := parseTime(row.TermStart)
	if err != nil {
		return billing.Subscription{}, err
	}
	end, err := parseTime(row.TermEnd)
	if err != nil {
		return billing.Subscription{}, err
	}
	next, err := parseTime(row.NextGrantAt)
	if err != nil {
		return billing.Subscription{}, err
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return billing.Subscription{}, err
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return billing.Subscription{}, err
	}
	result := billing.Subscription{UserID: row.UserID, Tier: tier, Term: billing.Term(row.Term), AnchorAt: anchor, TermStart: start, TermEnd: end, NextGrantAt: next, AutoRenew: row.AutoRenew != 0, Status: row.Status, CreatedAt: created, UpdatedAt: updated}
	if row.ScheduledTier.Valid {
		value, err := plan.Parse(row.ScheduledTier.String)
		if err != nil {
			return billing.Subscription{}, err
		}
		result.ScheduledTier = &value
	}
	if row.ScheduledTerm.Valid {
		value := billing.Term(row.ScheduledTerm.String)
		result.ScheduledTerm = &value
	}
	return result, nil
}

func toEvent(row sqlc.BillingEvent) (billing.Event, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return billing.Event{}, err
	}
	result := billing.Event{ID: row.ID, UserID: row.UserID, Kind: row.Kind, CreatedAt: created}
	if row.Tier.Valid {
		value, err := plan.Parse(row.Tier.String)
		if err != nil {
			return billing.Event{}, err
		}
		result.Tier = &value
	}
	if row.Term.Valid {
		value := billing.Term(row.Term.String)
		result.Term = &value
	}
	result.Credits = intPtr(row.Credits)
	result.USDCents = intPtr(row.UsdCents)
	result.KRW = intPtr(row.Krw)
	if row.KrwPerUsdE4.Valid {
		value := row.KrwPerUsdE4.Int64
		result.KRWPerUSDE4 = &value
	}
	result.RateDate = stringPtr(row.RateDate)
	result.ProviderPaymentKey = stringPtr(row.ProviderPaymentKey)
	result.OrderID = stringPtr(row.OrderID)
	result.Note = stringPtr(row.Note)
	return result, nil
}

func intPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	mapped := int(value.Int64)
	return &mapped
}
func stringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	mapped := value.String
	return &mapped
}
func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored instant %q: %w", value, err)
	}
	return parsed, nil
}
func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

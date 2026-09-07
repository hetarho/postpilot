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
	writer       *sql.DB
	write        *sqlc.Queries
	read         *sqlc.Queries
	credits      billing.Credits
	creditsForTx func(*sql.Conn) billing.Credits
	plans        billing.Plans
	plansForTx   func(*sql.Conn) billing.Plans
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

// SetCreditsForTx attaches the usage adapter at the composition root. The factory binds
// that adapter to this store's transaction connection, so billing can atomically record a
// method registration and its once-only credit lot without either store importing the other.
func (s *Store) SetCreditsForTx(factory func(*sql.Conn) billing.Credits) {
	s.creditsForTx = factory
}

func (s *Store) SetPlansForTx(factory func(*sql.Conn) billing.Plans) {
	s.plansForTx = factory
}

func (s *Store) InWriteTx(ctx context.Context, fn func(billing.Store, billing.Credits, billing.Plans) error) error {
	if s.writer == nil {
		if s.credits == nil || s.plans == nil {
			return errors.New("billing transaction adapters are not configured")
		}
		return fn(s, s.credits, s.plans)
	}
	if s.creditsForTx == nil || s.plansForTx == nil {
		return errors.New("billing transaction adapter factories are not configured")
	}
	conn, err := s.writer.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire billing writer: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin billing transaction: %w", err)
	}
	credits := s.creditsForTx(conn)
	plans := s.plansForTx(conn)
	if credits == nil || plans == nil {
		_ = rollback(ctx, conn)
		return errors.New("billing transaction adapter factory returned nil")
	}
	scoped := &Store{write: sqlc.New(conn), read: sqlc.New(conn), credits: credits, plans: plans}
	if err := fn(scoped, credits, plans); err != nil {
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

func (s *Store) UpsertPaymentMethod(ctx context.Context, method billing.PaymentMethod) error {
	err := s.write.UpsertPaymentMethod(ctx, sqlc.UpsertPaymentMethodParams{
		UserID: method.UserID, Provider: method.Provider, BillingKey: method.BillingKey,
		CustomerKey: method.CustomerKey, CardLabel: method.CardLabel,
		RegisteredAt: formatTime(method.RegisteredAt),
	})
	if err != nil {
		return fmt.Errorf("upsert payment method: %w", err)
	}
	return nil
}

func (s *Store) DeletePaymentMethod(ctx context.Context, userID string) error {
	if err := s.write.DeletePaymentMethod(ctx, userID); err != nil {
		return fmt.Errorf("delete payment method: %w", err)
	}
	return nil
}

func (s *Store) InsertEvent(ctx context.Context, event billing.Event) error {
	err := s.write.InsertBillingEvent(ctx, sqlc.InsertBillingEventParams{
		UserID: event.UserID, Kind: event.Kind, Tier: planNull(event.Tier),
		Term: termNull(event.Term), Credits: nullableInt(event.Credits),
		UsdCents: nullableInt(event.USDCents), KrwPerUsdE4: nullableInt64(event.KRWPerUSDE4),
		RateDate: stringNull(event.RateDate), Krw: nullableInt(event.KRW),
		ProviderPaymentKey: stringNull(event.ProviderPaymentKey), OrderID: stringNull(event.OrderID),
		Note: stringNull(event.Note), CreatedAt: formatTime(event.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert billing event: %w", err)
	}
	return nil
}

func (s *Store) UpsertSubscription(ctx context.Context, subscription billing.Subscription) error {
	err := s.write.UpsertSubscription(ctx, sqlc.UpsertSubscriptionParams{
		UserID: subscription.UserID, Tier: subscription.Tier.String(), Term: string(subscription.Term),
		AnchorAt: formatTime(subscription.AnchorAt), TermStart: formatTime(subscription.TermStart),
		TermEnd: formatTime(subscription.TermEnd), NextGrantAt: formatTime(subscription.NextGrantAt),
		AutoRenew: boolInt(subscription.AutoRenew), ScheduledTier: planNull(subscription.ScheduledTier),
		ScheduledTerm: termNull(subscription.ScheduledTerm), Status: subscription.Status,
		CreatedAt: formatTime(subscription.CreatedAt), UpdatedAt: formatTime(subscription.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	return nil
}

func (s *Store) DueSubscriptions(ctx context.Context, at time.Time) ([]billing.Subscription, error) {
	rows, err := s.read.ListDueSubscriptions(ctx, formatTime(at))
	if err != nil {
		return nil, fmt.Errorf("list due subscriptions: %w", err)
	}
	result := make([]billing.Subscription, 0, len(rows))
	for _, row := range rows {
		mapped, err := toSubscription(row)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, nil
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
func stringNull(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}
func nullableInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}
func nullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}
func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
func planNull(value *plan.Plan) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: value.String(), Valid: true}
}
func termNull(value *billing.Term) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*value), Valid: true}
}
func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored instant %q: %w", value, err)
	}
	return parsed, nil
}
func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

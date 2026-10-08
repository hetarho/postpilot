package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/plan"
)

func (s *Store) PutQuote(ctx context.Context, q billing.QuoteRecord) error {
	_, err := s.writeDB.ExecContext(ctx, `INSERT INTO billing_quotes
      (id,user_id,tier,term,krw,applied_now,effective_at,subscription_updated_at,quoted_at,expires_at)
      VALUES (?,?,?,?,?,?,?,?,?,?)`, q.ID, q.UserID, q.Tier, q.Term, q.KRW,
		boolInt(q.AppliedNow), formatTime(q.EffectiveAt), formatTime(q.SubscriptionUpdatedAt),
		formatTime(q.QuotedAt), formatTime(q.ExpiresAt))
	return err
}

func (s *Store) Quote(ctx context.Context, id string) (billing.QuoteRecord, bool, error) {
	var q billing.QuoteRecord
	var tier, term, effective, updated, quoted, expires string
	var applied int
	err := s.readDB.QueryRowContext(ctx, `SELECT id,user_id,tier,term,krw,applied_now,
      effective_at,subscription_updated_at,quoted_at,expires_at FROM billing_quotes WHERE id=?`, id).
		Scan(&q.ID, &q.UserID, &tier, &term, &q.KRW, &applied, &effective, &updated, &quoted, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return billing.QuoteRecord{}, false, nil
	}
	if err != nil {
		return billing.QuoteRecord{}, false, err
	}
	q.Tier, err = plan.Parse(tier)
	if err != nil {
		return billing.QuoteRecord{}, false, err
	}
	q.Term, q.AppliedNow = billing.Term(term), applied != 0
	for _, item := range []struct {
		raw string
		out *time.Time
	}{
		{effective, &q.EffectiveAt}, {updated, &q.SubscriptionUpdatedAt},
		{quoted, &q.QuotedAt}, {expires, &q.ExpiresAt},
	} {
		*item.out, err = parseTime(item.raw)
		if err != nil {
			return billing.QuoteRecord{}, false, err
		}
	}
	return q, true, nil
}

func (s *Store) PurgeExpiredQuotes(ctx context.Context, expiredBefore time.Time) (int, error) {
	result, err := s.writeDB.ExecContext(ctx, `DELETE FROM billing_quotes WHERE expires_at<?`, formatTime(expiredBefore))
	if err != nil {
		return 0, fmt.Errorf("purge expired billing quotes: %w", err)
	}
	n, err := result.RowsAffected()
	return int(n), err
}

func (s *Store) InsertIntent(ctx context.Context, i billing.Intent) error {
	_, err := s.writeDB.ExecContext(ctx, `INSERT INTO billing_intents
      (order_id,user_id,kind,tier,term,pack_id,billing_key,customer_key,krw,quote_id,quoted_at,
       subscription_updated_at,effective_at,status,created_at,updated_at)
      VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, i.OrderID, i.UserID, i.Kind,
		optionalString(string(i.Tier)), optionalString(string(i.Term)), optionalString(i.PackID),
		i.BillingKey, i.CustomerKey, i.KRW, optionalString(i.QuoteID), formatTime(i.QuotedAt), optionalTime(i.SubscriptionUpdatedAt),
		optionalTime(i.EffectiveAt), "pending", formatTime(i.CreatedAt), formatTime(i.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert billing intent: %w", err)
	}
	return nil
}

const intentColumns = `order_id,user_id,kind,tier,term,pack_id,billing_key,customer_key,krw,quote_id,quoted_at,
  subscription_updated_at,effective_at,status,provider_status,provider_payment_key,created_at,updated_at`

func (s *Store) Intent(ctx context.Context, orderID string) (billing.Intent, bool, error) {
	return scanIntent(s.readDB.QueryRowContext(ctx, `SELECT `+intentColumns+` FROM billing_intents WHERE order_id=?`, orderID))
}

func (s *Store) PendingIntent(ctx context.Context, userID string) (billing.Intent, bool, error) {
	return scanIntent(s.readDB.QueryRowContext(ctx, `SELECT `+intentColumns+` FROM billing_intents
      WHERE user_id=? AND status IN ('pending','review') LIMIT 1`, userID))
}

func (s *Store) DueIntents(ctx context.Context, createdBefore time.Time) ([]billing.Intent, error) {
	return s.listIntents(ctx, `SELECT `+intentColumns+` FROM billing_intents
      WHERE status='pending' AND created_at<=? ORDER BY created_at LIMIT 100`, formatTime(createdBefore))
}

func (s *Store) ReviewIntents(ctx context.Context, limit int) ([]billing.Intent, error) {
	return s.listIntents(ctx, `SELECT `+intentColumns+` FROM billing_intents
      WHERE status='review' ORDER BY created_at LIMIT ?`, limit)
}

func (s *Store) listIntents(ctx context.Context, query string, args ...any) ([]billing.Intent, error) {
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []billing.Intent
	for rows.Next() {
		i, _, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (s *Store) MarkIntent(ctx context.Context, orderID, status, providerStatus, paymentKey string, at time.Time) (bool, error) {
	result, err := s.writeDB.ExecContext(ctx, `UPDATE billing_intents SET status=?,provider_status=?,
      provider_payment_key=?,applied_at=?,updated_at=? WHERE order_id=? AND status='pending'`,
		status, optionalString(providerStatus), optionalString(paymentKey),
		optionalTime(at), formatTime(at), orderID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (s *Store) FailReviewIntent(ctx context.Context, orderID, providerStatus string, at time.Time) (bool, error) {
	result, err := s.writeDB.ExecContext(ctx, `UPDATE billing_intents SET status='failed',provider_status=?,updated_at=?
      WHERE order_id=? AND status='review'`, optionalString(providerStatus), formatTime(at), orderID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

type rowScanner interface{ Scan(...any) error }

func scanIntent(row rowScanner) (billing.Intent, bool, error) {
	var i billing.Intent
	var tier, term, pack, quote, updated, effective, providerStatus, paymentKey sql.NullString
	var quoted, created, changed string
	err := row.Scan(&i.OrderID, &i.UserID, &i.Kind, &tier, &term, &pack, &i.BillingKey, &i.CustomerKey, &i.KRW, &quote,
		&quoted, &updated, &effective, &i.Status, &providerStatus, &paymentKey, &created, &changed)
	if errors.Is(err, sql.ErrNoRows) {
		return billing.Intent{}, false, nil
	}
	if err != nil {
		return billing.Intent{}, false, err
	}
	if tier.Valid {
		i.Tier, err = plan.Parse(tier.String)
		if err != nil {
			return billing.Intent{}, false, err
		}
	}
	i.Term, i.PackID, i.QuoteID = billing.Term(term.String), pack.String, quote.String
	i.ProviderStatus, i.PaymentKey = providerStatus.String, paymentKey.String
	for _, item := range []struct {
		raw string
		out *time.Time
	}{
		{quoted, &i.QuotedAt}, {created, &i.CreatedAt}, {changed, &i.UpdatedAt},
	} {
		*item.out, err = parseTime(item.raw)
		if err != nil {
			return billing.Intent{}, false, err
		}
	}
	if updated.Valid {
		i.SubscriptionUpdatedAt, err = parseTime(updated.String)
		if err != nil {
			return billing.Intent{}, false, err
		}
	}
	if effective.Valid {
		i.EffectiveAt, err = parseTime(effective.String)
		if err != nil {
			return billing.Intent{}, false, err
		}
	}
	return i, true, nil
}

func optionalTime(at time.Time) any {
	if at.IsZero() {
		return nil
	}
	return formatTime(at)
}

func optionalString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/plan"
)

func (s *Store) RefundBenefits() billing.RefundBenefits { return s.refundBenefits }

func (s *Store) SetIntentFunding(ctx context.Context, orderID, coverageID string, end time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE billing_intents SET coverage_id=?,funding_end_at=?
		WHERE order_id=? AND kind<>'pack' AND status='pending'`, coverageID, formatTime(end), orderID)
	return err
}

func (s *Store) RefundPayment(ctx context.Context, userID, orderID string) (billing.RefundPayment, bool, error) {
	var payment billing.RefundPayment
	var tier, term, chargedAt, effectiveAt, coverageID, fundingEnd sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT i.order_id,i.user_id,i.kind,i.provider_payment_key,i.krw,
		i.tier,i.term,i.applied_at,i.effective_at,i.coverage_id,i.funding_end_at,
		COALESCE((SELECT p.lot_id FROM credit_purchases p WHERE p.order_id=i.order_id),'')
		FROM billing_intents i WHERE i.order_id=? AND i.user_id=? AND i.status='applied'`, orderID, userID).
		Scan(&payment.OrderID, &payment.UserID, &payment.Kind, &payment.PaymentKey, &payment.KRW,
			&tier, &term, &chargedAt, &effectiveAt, &coverageID, &fundingEnd, &payment.PackLotID)
	if errors.Is(err, sql.ErrNoRows) {
		return billing.RefundPayment{}, false, nil
	}
	if err != nil {
		return billing.RefundPayment{}, false, fmt.Errorf("read refundable payment: %w", err)
	}
	if chargedAt.Valid {
		payment.ChargedAt, err = parseTime(chargedAt.String)
		if err != nil {
			return billing.RefundPayment{}, false, err
		}
	}
	if effectiveAt.Valid {
		payment.EffectiveAt, err = parseTime(effectiveAt.String)
		if err != nil {
			return billing.RefundPayment{}, false, err
		}
	}
	if payment.EffectiveAt.IsZero() {
		payment.EffectiveAt = payment.ChargedAt
	}
	if tier.Valid {
		payment.Tier, err = plan.Parse(tier.String)
		if err != nil {
			return billing.RefundPayment{}, false, err
		}
	}
	if term.Valid {
		payment.Term = billing.Term(term.String)
	}
	payment.CoverageID = coverageID.String
	if payment.Kind != "pack" {
		if payment.CoverageID == "" {
			return billing.RefundPayment{}, false, errors.New("refundable payment has no funding coverage")
		}
		if payment.Kind == "upgrade" {
			var transitionAt string
			if err := s.db.QueryRowContext(ctx, `SELECT effective_at FROM entitlement_tier_transitions
				WHERE correlation_id=?`, orderID).Scan(&transitionAt); err != nil {
				return billing.RefundPayment{}, false, err
			}
			at, err := parseTime(transitionAt)
			if err != nil {
				return billing.RefundPayment{}, false, err
			}
			payment.PriorTier, err = s.TierAt(ctx, userID, payment.CoverageID, at.Add(-time.Nanosecond))
			if err != nil {
				return billing.RefundPayment{}, false, err
			}
		}
		if fundingEnd.Valid {
			payment.FundedTermEnd, err = parseTime(fundingEnd.String)
			if err != nil {
				return billing.RefundPayment{}, false, err
			}
		}
		var next string
		err = s.db.QueryRowContext(ctx, `SELECT effective_at FROM billing_intents
			WHERE user_id=? AND kind IN ('renew','subscribe') AND status='applied'
			AND effective_at>? ORDER BY effective_at LIMIT 1`,
			userID, formatTime(payment.EffectiveAt)).Scan(&next)
		if err == nil {
			payment.NextBaseStart, err = parseTime(next)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return billing.RefundPayment{}, false, err
		}
	}
	return payment, true, nil
}

func (s *Store) InsertRefundRequest(ctx context.Context, request billing.RefundRequest) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO billing_refund_requests
		(id,user_id,order_id,reason,status,requested_at) VALUES (?,?,?,?,'requested',?)`,
		request.ID, request.UserID, request.OrderID, request.Reason, formatTime(request.RequestedAt))
	return err
}

func (s *Store) OpenRefundForOrder(ctx context.Context, orderID string) (bool, error) {
	var open int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM billing_refund_requests
		WHERE order_id=? AND status IN ('requested','processing'))`, orderID).Scan(&open)
	return open != 0, err
}

func (s *Store) RefundRequest(ctx context.Context, id string) (billing.RefundRequest, bool, error) {
	var request billing.RefundRequest
	var requestedAt string
	var reviewedBy, reviewedAt, amount, disposition, idempotency, providerStatus, transactionKey, confirmedAmount, confirmedAt sql.NullString
	var balanceBefore sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,order_id,reason,status,requested_at,
		reviewed_by,reviewed_at,CAST(reviewed_amount_krw AS TEXT),disposition_json,idempotency_key,provider_balance_before_krw,
		provider_status,provider_transaction_key,CAST(confirmed_amount_krw AS TEXT),confirmed_at
		FROM billing_refund_requests WHERE id=?`, id).
		Scan(&request.ID, &request.UserID, &request.OrderID, &request.Reason, &request.Status, &requestedAt,
			&reviewedBy, &reviewedAt, &amount, &disposition, &idempotency, &balanceBefore, &providerStatus,
			&transactionKey, &confirmedAmount, &confirmedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return billing.RefundRequest{}, false, nil
	}
	if err != nil {
		return billing.RefundRequest{}, false, err
	}
	if err := mapRefundRequest(&request, requestedAt, reviewedBy, reviewedAt, amount, disposition,
		idempotency, providerStatus, transactionKey, confirmedAmount, confirmedAt); err != nil {
		return billing.RefundRequest{}, false, err
	}
	request.ProviderBalanceBeforeKRW = int(balanceBefore.Int64)
	return request, true, nil
}

func (s *Store) Refunds(ctx context.Context, userID string) ([]billing.RefundRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM billing_refund_requests
		WHERE (?='' OR user_id=?) ORDER BY requested_at DESC,id DESC LIMIT 200`, userID, userID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := make([]billing.RefundRequest, 0, len(ids))
	for _, id := range ids {
		request, ok, err := s.RefundRequest(ctx, id)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, request)
		}
	}
	return result, nil
}

func (s *Store) ProcessingRefundIDs(ctx context.Context, since time.Time, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM billing_refund_requests
		WHERE status='processing' AND (provider_transaction_key IS NOT NULL OR reviewed_at>?)
		ORDER BY reviewed_at,id LIMIT ?`, formatTime(since), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ReviewedEvidence(ctx context.Context, requestID string) (billing.RefundEvidence, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT evidence_json FROM billing_refund_decisions
		WHERE request_id=? ORDER BY created_at DESC,id DESC LIMIT 1`, requestID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return billing.RefundEvidence{}, false, nil
	}
	if err != nil {
		return billing.RefundEvidence{}, false, err
	}
	var evidence billing.RefundEvidence
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		return billing.RefundEvidence{}, false, err
	}
	return evidence, true, nil
}

func (s *Store) RecordRefundDecision(ctx context.Context, request billing.RefundRequest, decision billing.RefundDecision) error {
	status := "rejected"
	if decision.Outcome == "approve" {
		status = "processing"
	}
	result, err := s.db.ExecContext(ctx, `UPDATE billing_refund_requests SET status=?,reviewed_by=?,reviewed_at=?,
		reviewed_amount_krw=?,disposition_json=?,idempotency_key=?,provider_balance_before_krw=?
		WHERE id=? AND status='requested'`,
		status, decision.ReviewerID, formatTime(decision.CreatedAt), decision.AmountKRW,
		decision.DispositionJSON, nullableText(request.IdempotencyKey), request.ProviderBalanceBeforeKRW, request.ID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return billing.ErrRefundConflict
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO billing_refund_decisions
		(id,request_id,reviewer_id,outcome,reviewed_amount_krw,evidence_json,disposition_json,created_at)
		VALUES (?,?,?,?,?,?,?,?)`, decision.ID, decision.RequestID, decision.ReviewerID,
		decision.Outcome, decision.AmountKRW, decision.EvidenceJSON, decision.DispositionJSON,
		formatTime(decision.CreatedAt))
	return err
}

func (s *Store) RecordRefundProviderAttempt(ctx context.Context, requestID, transactionKey string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE billing_refund_requests SET provider_transaction_key=?
		WHERE id=? AND status='processing' AND (provider_transaction_key IS NULL OR provider_transaction_key=?)`,
		transactionKey, requestID, transactionKey)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return billing.ErrRefundConflict
	}
	return nil
}

func (s *Store) RecordRefundOutcome(ctx context.Context, request billing.RefundRequest, payment billing.Payment, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE billing_refund_requests SET status='completed',
		provider_status=?,provider_transaction_key=?,confirmed_amount_krw=?,confirmed_at=?
		WHERE id=? AND status='processing'`, payment.Status, request.ProviderTransactionKey,
		request.ConfirmedAmountKRW, formatTime(at), request.ID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return billing.ErrRefundConflict
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO billing_refund_provider_outcomes
		(id,request_id,provider_status,transaction_key,confirmed_amount_krw,observed_at)
		VALUES (?,?,?,?,?,?)`, request.ID, request.ID, payment.Status,
		request.ProviderTransactionKey, request.ConfirmedAmountKRW, formatTime(at))
	return err
}

func (s *Store) FailRefund(ctx context.Context, requestID, providerStatus string, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE billing_refund_requests SET status='failed',provider_status=?
		WHERE id=? AND status='processing'`, providerStatus, requestID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return billing.ErrRefundConflict
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO billing_refund_provider_outcomes
		(id,request_id,provider_status,confirmed_amount_krw,observed_at)
		VALUES (?,?,?,0,?)`, requestID+":failed", requestID, providerStatus, formatTime(at))
	return err
}

func (s *Store) ConfirmedRefundTotal(ctx context.Context, orderID string) (int, error) {
	var amount sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT SUM(confirmed_amount_krw) FROM billing_refund_requests
		WHERE order_id=? AND status='completed'`, orderID).Scan(&amount)
	return int(amount.Int64), err
}

func (s *Store) HasUnresolvedDependentUpgrade(ctx context.Context, payment billing.RefundPayment) (bool, error) {
	if payment.Kind == "pack" || payment.Kind == "upgrade" {
		return false, nil
	}
	funding := payment.Funding()
	var unresolved int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM billing_intents u
		WHERE u.user_id=? AND u.kind='upgrade' AND u.status='applied'
		AND u.coverage_id=? AND u.effective_at>=? AND u.effective_at<?
		AND NOT EXISTS(SELECT 1 FROM billing_refund_requests r
			WHERE r.order_id=u.order_id AND r.status='completed'))`,
		payment.UserID, funding.CoverageID, formatTime(funding.Start), formatTime(funding.End)).Scan(&unresolved)
	return unresolved != 0, err
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func mapRefundRequest(request *billing.RefundRequest, requestedAt string,
	reviewedBy, reviewedAt, amount, disposition, idempotency, providerStatus,
	transactionKey, confirmedAmount, confirmedAt sql.NullString) error {
	var err error
	request.RequestedAt, err = parseTime(requestedAt)
	if err != nil {
		return err
	}
	request.ReviewedBy = reviewedBy.String
	request.DispositionJSON = disposition.String
	request.IdempotencyKey = idempotency.String
	request.ProviderStatus = providerStatus.String
	request.ProviderTransactionKey = transactionKey.String
	if reviewedAt.Valid {
		at, err := parseTime(reviewedAt.String)
		if err != nil {
			return err
		}
		request.ReviewedAt = &at
	}
	if confirmedAt.Valid {
		at, err := parseTime(confirmedAt.String)
		if err != nil {
			return err
		}
		request.ConfirmedAt = &at
	}
	if amount.Valid {
		if _, err := fmt.Sscan(amount.String, &request.ReviewedAmountKRW); err != nil {
			return err
		}
	}
	if confirmedAmount.Valid {
		if _, err := fmt.Sscan(confirmedAmount.String, &request.ConfirmedAmountKRW); err != nil {
			return err
		}
	}
	return nil
}

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/usage/store/sqlc"
)

func (s *Store) SaveUnitQuote(ctx context.Context, q usage.UnitQuote, now time.Time) error {
	data, err := encodeUnitQuote(q)
	if err != nil {
		return err
	}
	if err := s.write.PurgeExpiredUnitQuotes(ctx, formatTime(now)); err != nil {
		return err
	}
	return s.write.SaveUnitQuote(ctx, sqlc.SaveUnitQuoteParams{ID: q.ID, UserID: q.UserID, Kind: q.Kind, Digest: q.Digest, QuoteJson: data, ExpiresAt: formatTime(q.ExpiresAt)})
}
func (s *Store) GetUnitQuote(ctx context.Context, owner, id string) (usage.UnitQuote, error) {
	r, err := s.read.GetUnitQuote(ctx, sqlc.GetUnitQuoteParams{ID: id, UserID: owner})
	if err != nil {
		return usage.UnitQuote{}, err
	}
	q, err := decodeUnitQuote(r.QuoteJson)
	q.ConsumedJobID = r.ConsumedJobID
	return q, err
}
func (s *Store) ConsumeUnitQuote(ctx context.Context, id, job string) (bool, error) {
	n, err := s.write.ConsumeUnitQuote(ctx, sqlc.ConsumeUnitQuoteParams{ConsumedJobID: job, ID: id})
	return n == 1, err
}
func (s *Store) SaveUnitAdmission(ctx context.Context, job, quoteID string, calls []usage.UnitBudget) error {
	data, err := json.Marshal(unitBudgetsRecord{Version: 1, Calls: budgetsToRecord(calls)})
	if err != nil {
		return err
	}
	return s.write.SaveUnitAdmission(ctx, sqlc.SaveUnitAdmissionParams{JobID: job, QuoteID: quoteID, BudgetsJson: string(data)})
}
func (s *Store) UnitAdmission(ctx context.Context, job string) (string, []usage.UnitBudget, error) {
	r, err := s.read.GetUnitAdmission(ctx, job)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	var record unitBudgetsRecord
	if err := json.Unmarshal([]byte(r.BudgetsJson), &record); err != nil {
		return "", nil, err
	}
	if record.Version != 1 {
		return "", nil, usage.ErrUnitApproval
	}
	calls, err := budgetsFromRecord(record.Calls)
	return r.QuoteID, calls, err
}
func (s *Store) ClaimUnitCall(ctx context.Context, id, job, fingerprint string, maximum int, now time.Time) (bool, error) {
	n, err := s.write.ClaimUnitCall(ctx, sqlc.ClaimUnitCallParams{ID: id, JobID: job, Fingerprint: fingerprint, CreatedAt: formatTime(now), MaximumCalls: int64(maximum)})
	return n == 1, err
}
func (s *Store) InsertUnitEvent(ctx context.Context, event usage.Event, unit usage.UnitEvent) error {
	c, err := s.read.GetUnitClaim(ctx, unit.ClaimID)
	if err != nil {
		return err
	}
	if c.JobID != event.JobID || c.UserID != event.UserID || c.Kind != event.Kind || c.BudgetFingerprint != unit.BudgetFingerprint {
		return usage.ErrUnitCall
	}
	count, err := s.read.UnitEventRecorded(ctx, unit.ClaimID)
	if err != nil || count > 0 {
		return err
	}
	records := make([]unitEvidenceRecord, 0, len(unit.Evidence))
	for _, e := range unit.Evidence {
		records = append(records, unitEvidenceRecord{string(e.Unit), e.Quantity})
	}
	data, err := json.Marshal(records)
	if err != nil {
		return err
	}
	id, err := s.write.InsertUnitBaseEvent(ctx, sqlc.InsertUnitBaseEventParams{UserID: event.UserID, Kind: event.Kind, JobID: event.JobID, Stage: event.Stage, Model: event.Model,
		CostMicrousd: event.CostMicrousd, CostSource: string(event.CostSource), CreatedAt: formatTime(event.CreatedAt)})
	if err != nil {
		return err
	}
	providerID, _, _ := strings.Cut(event.Model, "/")
	return s.write.InsertUnitEvidence(ctx, sqlc.InsertUnitEvidenceParams{EventID: id, ClaimID: unit.ClaimID, ProviderID: providerID, RequestID: unit.SupplierRequestID, Fingerprint: unit.BudgetFingerprint,
		EvidenceJson: string(data), ReportedUsd: unit.ReportedUSD, ExactUsd: unit.ExactUSD, CostSource: string(unit.CostSource)})
}
func (s *Store) UnitCostForJob(ctx context.Context, job string) (string, error) {
	rows, err := s.read.UnitCostsForJob(ctx, job)
	if err != nil {
		return "", err
	}
	total := new(big.Rat)
	for _, r := range rows {
		n, ok := new(big.Rat).SetString(r)
		if !ok || n.Sign() < 0 {
			return "", usage.ErrUnitPricing
		}
		total.Add(total, n)
	}
	return total.RatString(), nil
}

func (s *Store) ClaimBoundedUnitCall(ctx context.Context, id, job, fingerprint string, count, total, characters int, digest string, at time.Time) (bool, error) {
	n, e := s.write.ClaimBoundedUnitCall(ctx, sqlc.ClaimBoundedUnitCallParams{ID: id, Job: job, Fingerprint: fingerprint, Now: formatTime(at), Characters: int64(characters), InputDigest: digest, MaxCalls: int64(count), MaxCharacters: int64(total)})
	return n == 1, e
}

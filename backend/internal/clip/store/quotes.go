package store

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"github.com/postpilot/backend/internal/platform/config"
)

var _ clip.QuoteStore = (*Store)(nil)

func quoteRow(r sqlc.ClipGenerationQuote) (clip.GenerationQuote, error) {
	var pricing clip.GenerationPricing
	if err := strictJSON(r.PricingJson, &pricing); err != nil {
		return clip.GenerationQuote{}, err
	}
	if pricing.MaxCredits != int(r.MaxCredits) {
		return clip.GenerationQuote{}, clip.ErrQuoteChanged
	}
	expires, err := time.Parse(time.RFC3339Nano, r.ExpiresAt)
	return clip.GenerationQuote{ID: r.ID, UserID: r.UserID, ProjectID: r.ProjectID, BatchID: r.BatchID, InputDigest: r.InputDigest, Pricing: pricing, ExpiresAt: expires, ConsumedJobID: r.ConsumedJobID.String}, err
}

func (s *Store) GetQuote(ctx context.Context, user, id string) (clip.GenerationQuote, error) {
	r, err := s.read.GetClipQuote(ctx, sqlc.GetClipQuoteParams{UserID: user, ID: id})
	if err != nil {
		return clip.GenerationQuote{}, dbError(err)
	}
	return quoteRow(r)
}

func validateQuoteInputs(ctx context.Context, q *sqlc.Queries, quote clip.GenerationQuote, now time.Time) error {
	if !now.Before(quote.ExpiresAt) {
		return clip.ErrQuoteExpired
	}
	p, err := getProject(ctx, q, quote.UserID, quote.ProjectID)
	if err != nil {
		return err
	}
	t, err := getTemplate(ctx, q, quote.UserID, p.VideoTemplateID)
	if err != nil {
		return err
	}
	b, err := getSourceBatch(ctx, q, quote.UserID, quote.BatchID)
	if err != nil {
		return err
	}
	if !clip.ValidQuoteBatch(b, quote.UserID, quote.ProjectID, now) {
		return clip.ErrSourceState
	}
	if quote.ExpiresAt.After(b.ExpiresAt) || quote.InputDigest != clip.QuoteInputDigest(p, t, b, quote.Pricing) {
		return clip.ErrQuoteChanged
	}
	return clip.RequiredAnswers(t, p, config.ClipCompositionLimits())
}

func (s *Store) SaveQuote(ctx context.Context, quote clip.GenerationQuote, now time.Time) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		if err := validateQuoteInputs(ctx, q, quote, now); err != nil {
			return struct{}{}, err
		}
		pricing, err := json.Marshal(quote.Pricing)
		if err != nil {
			return struct{}{}, err
		}
		n, err := q.SaveClipQuote(ctx, sqlc.SaveClipQuoteParams{ID: quote.ID, UserID: quote.UserID, ProjectID: quote.ProjectID, BatchID: quote.BatchID, InputDigest: quote.InputDigest, PricingJson: string(pricing), MaxCredits: int64(quote.Pricing.MaxCredits), ExpiresAt: stamp(quote.ExpiresAt)})
		if err == nil && n != 1 {
			err = clip.ErrQuoteChanged
		}
		return struct{}{}, err
	})
	return err
}

func (s *Store) LinkApprovedSourceJob(ctx context.Context, quote clip.GenerationQuote, job string, now time.Time) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		r, err := q.GetClipQuote(ctx, sqlc.GetClipQuoteParams{UserID: quote.UserID, ID: quote.ID})
		if err != nil {
			return struct{}{}, err
		}
		stored, err := quoteRow(r)
		if err != nil {
			return struct{}{}, err
		}
		if stored.ConsumedJobID != "" {
			if stored.ConsumedJobID == job {
				return struct{}{}, nil
			}
			return struct{}{}, clip.ErrQuoteChanged
		}
		if !reflect.DeepEqual(stored, quote) {
			return struct{}{}, clip.ErrQuoteChanged
		}
		if err = validateQuoteInputs(ctx, q, stored, now); err != nil {
			return struct{}{}, err
		}
		n, err := q.ConsumeClipQuote(ctx, sqlc.ConsumeClipQuoteParams{ConsumedJobID: nullable(job), UserID: quote.UserID, ID: quote.ID, ExpiresAt: stamp(now)})
		if err != nil {
			return struct{}{}, err
		}
		if n != 1 {
			return struct{}{}, clip.ErrQuoteChanged
		}
		n, err = q.LinkSourceJob(ctx, sqlc.LinkSourceJobParams{JobID: nullable(job), ID: quote.BatchID, UserID: quote.UserID, ExpiresAt: stamp(now)})
		if err == nil && n != 1 {
			err = clip.ErrSourceState
		}
		return struct{}{}, err
	})
	return err
}

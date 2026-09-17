package store

import (
	"context"
	"encoding/json"
	"errors"
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
	if p.Finalized != nil {
		return clip.ErrFinalized
	}
	// A project may carry no template at all (CLIP-5), and the digest the
	// service signed was computed over the same zero recipe.
	var t clip.VideoTemplate
	if p.VideoTemplateID != "" {
		if t, err = getTemplate(ctx, q, quote.UserID, p.VideoTemplateID); err != nil {
			return err
		}
	}
	b, err := getSourceBatch(ctx, q, quote.UserID, quote.BatchID)
	if err != nil {
		return err
	}
	access, e := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: quote.ProjectID, UserID: quote.UserID})
	if e != nil {
		return e
	}
	if access.Deleting != 0 || access.SourceAccessRevokedAt.Valid || access.SourceBatchID.String != b.ID {
		return clip.ErrSourceState
	}

	if !clip.ValidQuoteBatch(b, quote.UserID, quote.ProjectID, now) {
		return clip.ErrSourceState
	}
	if quote.ExpiresAt.After(b.ExpiresAt) || quote.InputDigest != clip.QuoteInputDigest(p, t, b, quote.Pricing) {
		return clip.ErrQuoteChanged
	}
	raw, err := q.GetClipRecovery(ctx, sqlc.GetClipRecoveryParams{UserID: quote.UserID, ProjectID: quote.ProjectID})
	if err != nil && !errors.Is(dbError(err), clip.ErrNotFound) {
		return err
	}
	var recovery *clip.RecoveryState
	if err == nil {
		recovery = &clip.RecoveryState{}
		if json.Unmarshal([]byte(raw.StateJson), recovery) != nil {
			return clip.ErrQuoteChanged
		}
	}
	if clip.RecoveryDigest(recovery) != quote.Pricing.RecoveryDigest {
		return clip.ErrQuoteChanged
	}
	return clip.RequiredAnswers(t, p, config.ClipCompositionLimits())
}

func (s *Store) SaveQuote(ctx context.Context, quote clip.GenerationQuote, now time.Time) error {
	return s.saveQuote(ctx, quote, now, true)
}

// SaveRevisionQuote stores a REVISION's quote. Its digest binds the saved plan
// and the owner's words rather than the generation inputs, so the generation's
// own input check would refuse a perfectly good quote; the service has already
// checked what this quote is about (CLIP-131).
func (s *Store) SaveRevisionQuote(ctx context.Context, quote clip.GenerationQuote, now time.Time) error {
	return s.saveQuote(ctx, quote, now, false)
}

func (s *Store) saveQuote(ctx context.Context, quote clip.GenerationQuote, now time.Time, generation bool) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		if generation {
			if err := validateQuoteInputs(ctx, q, quote, now); err != nil {
				return struct{}{}, err
			}
		} else if !now.Before(quote.ExpiresAt) {
			return struct{}{}, clip.ErrQuoteExpired
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

// LinkRevisionJob consumes a REVISION's quote and renews the originals'
// retention (CLIP-73). It repeats none of the generation's input check: a
// revision's quote binds the saved plan and the owner's words, which the
// service verified against the current project before it enqueued anything,
// and neither the batch nor the template decides what this job rewrites.
func (s *Store) LinkRevisionJob(ctx context.Context, quote clip.GenerationQuote, job string, now time.Time) error {
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
		if !now.Before(stored.ExpiresAt) {
			return struct{}{}, clip.ErrQuoteExpired
		}
		n, err := q.ConsumeClipQuote(ctx, sqlc.ConsumeClipQuoteParams{ConsumedJobID: nullable(job), UserID: quote.UserID, ID: quote.ID, ExpiresAt: stamp(now)})
		if err != nil {
			return struct{}{}, err
		}
		if n != 1 {
			return struct{}{}, clip.ErrQuoteChanged
		}
		return struct{}{}, bindSourceAttempt(ctx, q, quote.UserID, quote.BatchID, job, now)
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
		return struct{}{}, bindSourceAttempt(ctx, q, quote.UserID, quote.BatchID, job, now)
	})
	return err
}

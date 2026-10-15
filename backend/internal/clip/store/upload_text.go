package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

var _ clip.UploadTextStore = (*Store)(nil)

func uploadTextRow(r sqlc.ClipUploadText) (clip.UploadText, error) {
	at, err := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	if err != nil {
		return clip.UploadText{}, err
	}
	tags := []string{}
	if err = json.Unmarshal([]byte(r.Tags), &tags); err != nil {
		return clip.UploadText{}, err
	}
	return clip.UploadText{ProjectID: r.ProjectID, UserID: r.UserID, Channel: r.Channel, Body: r.Body, Tags: tags, Edited: r.Edited != 0, Revision: int(r.Revision), UpdatedAt: at, LastJobID: r.LastJobID}, nil
}

func (s *Store) ListUploadTexts(ctx context.Context, user, id string) ([]clip.UploadText, error) {
	if _, err := s.GetProject(ctx, user, id); err != nil {
		return nil, err
	}
	rows, err := s.read.ListClipUploadTexts(ctx, sqlc.ListClipUploadTextsParams{ProjectID: id, UserID: user})
	if err != nil {
		return nil, err
	}
	out := make([]clip.UploadText, 0, len(rows))
	for _, r := range rows {
		v, e := uploadTextRow(r)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

func readUploadText(ctx context.Context, q *sqlc.Queries, user, id, channel string) (clip.UploadText, error) {
	r, err := q.GetClipUploadText(ctx, sqlc.GetClipUploadTextParams{ProjectID: id, UserID: user, Channel: channel})
	if errors.Is(dbError(err), clip.ErrNotFound) {
		return clip.UploadText{ProjectID: id, UserID: user, Channel: channel, Tags: []string{}}, nil
	}
	if err != nil {
		return clip.UploadText{}, err
	}
	return uploadTextRow(r)
}

func (s *Store) SaveUploadText(ctx context.Context, in clip.UploadTextSave) (clip.UploadText, error) {
	// Generated publication only runs through the transaction-bound finisher.
	if in.ExpectedRevision < 0 || (in.JobID == "") != in.Edited || in.JobID != "" && s.writer != nil {
		return clip.UploadText{}, clip.ErrInvalid
	}
	return transact(ctx, s, func(q *sqlc.Queries) (clip.UploadText, error) {
		p, err := getProject(ctx, q, in.UserID, in.ProjectID)
		if err != nil {
			return clip.UploadText{}, err
		}
		if in.JobID == "" {
			active, e := q.HasActiveClipJob(ctx, nullable(in.ProjectID))
			if e != nil {
				return clip.UploadText{}, e
			}
			if active > 0 {
				return clip.UploadText{}, clip.ErrBusy
			}
		} else if p.EditPlanRevision != in.ExpectedPlanRevision {
			return clip.UploadText{}, clip.ErrPlanConflict
		}
		checked, err := clip.ValidateUploadTextEdit(in.Channel, p.Language, p.Campaign, in.Body, in.Tags)
		if err != nil {
			return clip.UploadText{}, err
		}
		var n int64
		if in.ExpectedRevision == 0 {
			n, err = q.InsertClipUploadText(ctx, sqlc.InsertClipUploadTextParams{ProjectID: in.ProjectID, UserID: in.UserID, Channel: in.Channel, Body: checked.Body, Tags: encodeGuideItems(checked.Tags), Edited: flag(in.Edited), UpdatedAt: stamp(in.Now), LastJobID: in.JobID})
		} else {
			n, err = q.UpdateClipUploadText(ctx, sqlc.UpdateClipUploadTextParams{Body: checked.Body, Tags: encodeGuideItems(checked.Tags), Edited: flag(in.Edited), UpdatedAt: stamp(in.Now), LastJobID: in.JobID, ProjectID: in.ProjectID, UserID: in.UserID, Channel: in.Channel, Revision: int64(in.ExpectedRevision)})
		}
		if err != nil {
			return clip.UploadText{}, err
		}
		if n != 1 {
			return clip.UploadText{}, clip.ErrPlanConflict
		}
		return readUploadText(ctx, q, in.UserID, in.ProjectID, in.Channel)
	})
}

func uploadQuoteRow(r sqlc.ClipUploadTextQuote) (clip.UploadTextQuote, error) {
	var pricing clip.GenerationPricing
	if strictJSON(r.PricingJson, &pricing) != nil || !pricing.UploadText || !pricing.Valid() || pricing.MaxCredits != int(r.MaxCredits) {
		return clip.UploadTextQuote{}, clip.ErrQuoteChanged
	}
	expires, err := time.Parse(time.RFC3339Nano, r.ExpiresAt)
	return clip.UploadTextQuote{GenerationQuote: clip.GenerationQuote{ID: r.ID, UserID: r.UserID, ProjectID: r.ProjectID, InputDigest: r.InputDigest, Pricing: pricing, ExpiresAt: expires, ConsumedJobID: r.ConsumedJobID.String}, Channel: r.Channel, ExpectedRevision: int(r.ExpectedRevision)}, err
}

func (s *Store) GetUploadTextQuote(ctx context.Context, user, id string) (clip.UploadTextQuote, error) {
	r, err := s.read.GetClipUploadTextQuote(ctx, sqlc.GetClipUploadTextQuoteParams{UserID: user, ID: id})
	if err != nil {
		return clip.UploadTextQuote{}, dbError(err)
	}
	return uploadQuoteRow(r)
}

func (s *Store) SaveUploadTextQuote(ctx context.Context, quote clip.UploadTextQuote, now time.Time) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		p, e := getProject(ctx, q, quote.UserID, quote.ProjectID)
		if e != nil {
			return struct{}{}, e
		}
		if !quote.Pricing.UploadText || !quote.Pricing.Valid() {
			return struct{}{}, clip.ErrPricingUnavailable
		}
		if !now.Before(quote.ExpiresAt) {
			return struct{}{}, clip.ErrQuoteExpired
		}
		if _, e = clip.UploadTextMaterial(p, quote.Channel); e != nil {
			return struct{}{}, e
		}
		active, e := q.HasActiveClipJob(ctx, nullable(p.ID))
		if e != nil {
			return struct{}{}, e
		}
		if active > 0 {
			return struct{}{}, clip.ErrBusy
		}
		text, e := readUploadText(ctx, q, quote.UserID, quote.ProjectID, quote.Channel)
		if e != nil {
			return struct{}{}, e
		}
		if text.Revision != quote.ExpectedRevision {
			return struct{}{}, clip.ErrQuoteChanged
		}
		raw, e := json.Marshal(quote.Pricing)
		if e != nil {
			return struct{}{}, e
		}
		n, e := q.SaveClipUploadTextQuote(ctx, sqlc.SaveClipUploadTextQuoteParams{ID: quote.ID, UserID: quote.UserID, ProjectID: quote.ProjectID, Channel: quote.Channel, ExpectedRevision: int64(quote.ExpectedRevision), InputDigest: quote.InputDigest, PricingJson: string(raw), MaxCredits: int64(quote.Pricing.MaxCredits), ExpiresAt: stamp(quote.ExpiresAt)})
		if e != nil {
			return struct{}{}, e
		}
		if n != 1 {
			return struct{}{}, clip.ErrQuoteChanged
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Store) LinkUploadTextJob(ctx context.Context, quote clip.UploadTextQuote, job string, frozen clip.UploadTextSnapshot, now time.Time) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		r, e := q.GetClipUploadTextQuote(ctx, sqlc.GetClipUploadTextQuoteParams{UserID: quote.UserID, ID: quote.ID})
		if e != nil {
			return struct{}{}, dbError(e)
		}
		stored, e := uploadQuoteRow(r)
		if e != nil {
			return struct{}{}, e
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
		p, e := getProject(ctx, q, quote.UserID, quote.ProjectID)
		if e != nil {
			return struct{}{}, e
		}
		material, e := clip.UploadTextMaterial(p, quote.Channel)
		if e != nil {
			return struct{}{}, e
		}
		text, e := readUploadText(ctx, q, quote.UserID, quote.ProjectID, quote.Channel)
		if e != nil {
			return struct{}{}, e
		}
		if frozen.UserID != quote.UserID || frozen.ProjectID != quote.ProjectID || frozen.Input.Channel != quote.Channel || frozen.PlanRevision != p.EditPlanRevision || frozen.TextRevision != text.Revision || text.Revision != quote.ExpectedRevision ||
			frozen.MaterialDigest != clip.UploadMaterialDigest(material) || quote.InputDigest != clip.UploadQuoteDigest(frozen.Input, text.Revision, frozen.ConfirmEdited, quote.Pricing) {
			return struct{}{}, clip.ErrQuoteChanged
		}
		if text.Edited && !frozen.ConfirmEdited {
			return struct{}{}, clip.ErrUploadTextConfirmation
		}
		n, e := q.ConsumeClipUploadTextQuote(ctx, sqlc.ConsumeClipUploadTextQuoteParams{ConsumedJobID: nullable(job), UserID: quote.UserID, ID: quote.ID, ExpiresAt: stamp(now)})
		if e != nil {
			return struct{}{}, e
		}
		if n != 1 {
			return struct{}{}, clip.ErrQuoteChanged
		}
		return struct{}{}, nil
	})
	return err
}

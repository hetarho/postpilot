package clip

import (
	"context"
	"errors"
	"time"
)

var ErrSourceExpired = errors.New("clip source retention expired")
var ErrSourceMissing = errors.New("clip source object missing")

type SourcePlayback struct {
	URL       string
	ExpiresAt time.Time
}

func SourceAvailability(b SourceBatch, v SourceLease, now time.Time) string {
	if b.State == "cleanup_pending" || v.CleanupPending {
		return "cleanup_pending"
	}
	if v.State != "ready" {
		if !now.Before(b.UploadExpiresAt) {
			return "expired"
		}
		return "uploading"
	}
	if !now.Before(v.ExpiresAt) {
		if b.State == "consuming" {
			return "active"
		}
		return "expired"
	}
	if b.State == "consuming" {
		return "active"
	}
	return "available"
}

// GetSources returns metadata and availability only. HEAD requests never read pixels.
func (s *SourceService) GetSources(ctx context.Context, user, project string) ([]SourceBatch, error) {
	batches, err := s.store.ProjectSourceBatches(ctx, user, project)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for i := range batches {
		for j := range batches[i].Sources {
			v := &batches[i].Sources[j]
			v.Availability = SourceAvailability(batches[i], *v, now)
			if v.Availability == "available" || v.Availability == "active" {
				info, e := s.objects.HeadSource(ctx, v.Key)
				if errors.Is(e, ErrNotFound) {
					v.Availability = "missing"
					continue
				}
				if e != nil {
					return nil, errors.New("clip source availability check failed")
				}
				if info.Bytes != v.Bytes || info.ContentType != v.ContentType {
					v.Availability = "missing"
				}
			}
		}
	}
	return batches, nil
}

func (s *SourceService) Playback(ctx context.Context, user, project, id, fingerprint string) (SourcePlayback, error) {
	find := func() (SourceLease, error) {
		batches, err := s.store.ProjectSourceBatches(ctx, user, project)
		if err != nil {
			return SourceLease{}, err
		}
		found := false
		for _, b := range batches {
			for _, v := range b.Sources {
				if v.ID != id || v.Fingerprint != fingerprint {
					continue
				}
				found = true
				if b.State != "cleanup_pending" && !v.CleanupPending && v.State == "ready" && s.now().Before(v.ExpiresAt) {
					return v, nil
				}
			}
		}
		if found {
			return SourceLease{}, ErrSourceExpired
		}
		return SourceLease{}, ErrNotFound
	}
	v, err := find()
	if err != nil {
		return SourcePlayback{}, err
	}
	info, err := s.objects.HeadSource(ctx, v.Key)
	if errors.Is(err, ErrNotFound) {
		return SourcePlayback{}, ErrSourceMissing
	}
	if err != nil {
		return SourcePlayback{}, errors.New("clip source availability check failed")
	}
	if info.Bytes != v.Bytes || info.ContentType != v.ContentType {
		return SourcePlayback{}, ErrSourceMissing
	}
	checked, err := find()
	if err != nil {
		return SourcePlayback{}, err
	}
	if checked.Key != v.Key {
		return SourcePlayback{}, ErrSourceState
	}
	now := s.now()
	ttl := min(s.config.PlaybackTTL, v.ExpiresAt.Sub(now)).Truncate(time.Second)
	if ttl <= 0 {
		return SourcePlayback{}, ErrSourceExpired
	}
	url, err := s.objects.PresignSourcePlayback(ctx, v.Key, v.ContentType, ttl)
	if err != nil {
		return SourcePlayback{}, errors.New("clip source playback signing failed")
	}
	// An SDK call may overlap replacement, expiry or a permanent access fence.
	checked, err = find()
	if err != nil {
		return SourcePlayback{}, err
	}
	if checked.Key != v.Key || !s.now().Before(now.Add(ttl)) {
		return SourcePlayback{}, ErrSourceState
	}
	return SourcePlayback{URL: url, ExpiresAt: now.Add(ttl)}, nil
}

func (s *SourceService) AvailableBatch(ctx context.Context, user, id string, plans ...EditPlan) (SourceBatch, error) {
	b, err := s.store.GetSourceBatch(ctx, user, id)
	if err != nil {
		return b, err
	}
	if b.AccessDenied || !ValidQuoteBatch(b, user, b.ProjectID, s.now()) {
		return b, ErrSourceState
	}
	checked := b
	if len(plans) > 0 {
		if err := MatchRenderBatch(plans[0], b); err != nil {
			return b, err
		}
		checked = renderBatchSources(plans[0], b)
	}

	for _, v := range checked.Sources {
		info, e := s.objects.HeadSource(ctx, v.Key)
		if errors.Is(e, ErrNotFound) {
			return b, ErrSourceMissing
		}
		if e != nil {
			return b, errors.New("clip source availability check failed")
		}
		if info.Bytes != v.Bytes || info.ContentType != v.ContentType {
			return b, ErrSourceMissing
		}
	}
	return b, nil
}

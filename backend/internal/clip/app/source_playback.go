package app

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// GetSources returns metadata and availability only. HEAD requests never read pixels.
func (s *SourceService) GetSources(ctx context.Context, user, project string) ([]clip.SourceBatch, error) {
	batches, err := s.store.ProjectSourceBatches(ctx, user, project)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for i := range batches {
		for j := range batches[i].Sources {
			v := &batches[i].Sources[j]
			v.Availability = clip.SourceAvailability(batches[i], *v, now)
			if v.Availability == "available" || v.Availability == "active" {
				info, e := s.objects.HeadSource(ctx, v.Key)
				if errors.Is(e, clip.ErrNotFound) {
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

func (s *SourceService) Playback(ctx context.Context, user, project, id, fingerprint string) (clip.SourcePlayback, error) {
	find := func() (clip.SourceLease, error) {
		batches, err := s.store.ProjectSourceBatches(ctx, user, project)
		if err != nil {
			return clip.SourceLease{}, err
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
			return clip.SourceLease{}, clip.ErrSourceExpired
		}
		return clip.SourceLease{}, clip.ErrNotFound
	}
	v, err := find()
	if err != nil {
		return clip.SourcePlayback{}, err
	}
	info, err := s.objects.HeadSource(ctx, v.Key)
	if errors.Is(err, clip.ErrNotFound) {
		return clip.SourcePlayback{}, clip.ErrSourceMissing
	}
	if err != nil {
		return clip.SourcePlayback{}, errors.New("clip source availability check failed")
	}
	if info.Bytes != v.Bytes || info.ContentType != v.ContentType {
		return clip.SourcePlayback{}, clip.ErrSourceMissing
	}
	checked, err := find()
	if err != nil {
		return clip.SourcePlayback{}, err
	}
	if checked.Key != v.Key {
		return clip.SourcePlayback{}, clip.ErrSourceState
	}
	now := s.now()
	ttl := min(s.config.PlaybackTTL, v.ExpiresAt.Sub(now)).Truncate(time.Second)
	if ttl <= 0 {
		return clip.SourcePlayback{}, clip.ErrSourceExpired
	}
	url, err := s.objects.PresignSourcePlayback(ctx, v.Key, v.ContentType, ttl)
	if err != nil {
		return clip.SourcePlayback{}, errors.New("clip source playback signing failed")
	}
	// An SDK call may overlap replacement, expiry or a permanent access fence.
	checked, err = find()
	if err != nil {
		return clip.SourcePlayback{}, err
	}
	if checked.Key != v.Key || !s.now().Before(now.Add(ttl)) {
		return clip.SourcePlayback{}, clip.ErrSourceState
	}
	return clip.SourcePlayback{URL: url, ExpiresAt: now.Add(ttl)}, nil
}

func (s *SourceService) AvailableBatch(ctx context.Context, user, id string, plans ...clip.EditPlan) (clip.SourceBatch, error) {
	b, err := s.store.GetSourceBatch(ctx, user, id)
	if err != nil {
		return b, err
	}
	if b.AccessDenied || !clip.ValidQuoteBatch(b, user, b.ProjectID, s.now()) {
		return b, clip.ErrSourceState
	}
	checked := b
	if len(plans) > 0 {
		if err := clip.MatchRenderBatch(plans[0], b); err != nil {
			return b, err
		}
		checked = renderBatchSources(plans[0], b)
	}

	for _, v := range checked.Sources {
		info, e := s.objects.HeadSource(ctx, v.Key)
		if errors.Is(e, clip.ErrNotFound) {
			return b, clip.ErrSourceMissing
		}
		if e != nil {
			return b, errors.New("clip source availability check failed")
		}
		if info.Bytes != v.Bytes || info.ContentType != v.ContentType {
			return b, clip.ErrSourceMissing
		}
	}
	return b, nil
}

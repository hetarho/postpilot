package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type availabilityErrorStore struct {
	clip.SourceStore
	batch clip.SourceBatch
}

func (s availabilityErrorStore) ProjectSourceBatches(context.Context, string, string) ([]clip.SourceBatch, error) {
	return []clip.SourceBatch{s.batch}, nil
}
func (s availabilityErrorStore) GetSourceBatch(context.Context, string, string) (clip.SourceBatch, error) {
	return s.batch, nil
}

type availabilityErrorObjects struct {
	clip.ObjectStore
	cause error
}

func (s availabilityErrorObjects) HeadSource(context.Context, string) (clip.SourceObjectInfo, error) {
	return clip.SourceObjectInfo{}, fmt.Errorf("SDK HEAD failed: %w", s.cause)
}

func TestSourceAvailabilityPreservesCancellationAndCause(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, errors.New("storage proxy failed")} {
		for _, action := range []string{"directory", "playback", "quote"} {
			t.Run(action+"/"+cause.Error(), func(t *testing.T) {
				now := time.Now()
				source := clip.SourceLease{ID: "source", Key: "owned", State: "ready", ActualBytes: 2,
					ExpiresAt: now.Add(time.Hour), SourceMetadata: clip.SourceMetadata{Bytes: 2, Fingerprint: "fingerprint"}}
				batch := clip.SourceBatch{ID: "batch", UserID: "alice", ProjectID: "project", State: "ready",
					ExpiresAt: now.Add(time.Hour), Sources: []clip.SourceLease{source}}
				service := NewSourceService(availabilityErrorStore{batch: batch}, availabilityErrorObjects{cause: cause},
					clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: time.Minute}), func() time.Time { return now })
				var err error
				switch action {
				case "directory":
					_, err = service.GetSources(t.Context(), "alice", "project")
				case "playback":
					_, err = service.Playback(t.Context(), "alice", "project", "source", "fingerprint")
				case "quote":
					_, err = service.AvailableBatch(t.Context(), "alice", "batch")
				}
				if !errors.Is(err, cause) {
					t.Fatalf("availability read discarded its cause: %v", err)
				}
			})
		}
	}
}

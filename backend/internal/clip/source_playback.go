package clip

import (
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

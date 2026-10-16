// Package capturetime normalizes optional original-media instants without guessing a clock.
package capturetime

import (
	"errors"
	"time"
)

func Normalize(at *time.Time) (*time.Time, error) {
	if at == nil || at.IsZero() {
		return nil, nil
	}
	value := at.UTC().Round(0)
	if value.Year() < 1 || value.Year() > 9999 {
		return nil, errors.New("capture time is outside RFC3339 range")
	}
	return &value, nil
}
func Parse(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339Nano, *raw)
	if err != nil {
		return nil, errors.New("invalid RFC3339 capture time")
	}
	return Normalize(&at)
}
func Format(at *time.Time) *string {
	if at == nil || at.IsZero() {
		return nil
	}
	raw := at.UTC().Format(time.RFC3339Nano)
	return &raw
}

// Unset/1904-epoch movie times are not evidence of a recording date.
func Video(at *time.Time) *time.Time {
	if at == nil || at.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return nil
	}
	return at
}

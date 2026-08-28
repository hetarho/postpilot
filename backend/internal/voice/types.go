// Package voice owns one account's editable writing profile and its source samples.
package voice

import (
	"errors"
	"fmt"
	"time"
)

const AnalysisJobKind = "analyze_voice"

var (
	ErrAnalyzeModelRequired = errors.New("an enabled analyze model is required")
	ErrSampleNotFound       = errors.New("voice sample not found")
	ErrSampleMutation       = errors.New("voice sample change could not schedule analysis")
)

type SampleTooShortError struct{ Chars int }

func (e *SampleTooShortError) Error() string {
	return fmt.Sprintf("sample has %d characters; at least %d are required", e.Chars, SampleMinChars)
}

type Sample struct {
	ID        string
	UserID    string
	Label     string
	Body      string
	Chars     int
	CreatedAt time.Time
}

type Profile struct {
	UserID      string
	Styleguide  string
	Rules       string
	UpdatedAt   time.Time
	Samples     []Sample
	ActiveJobID string
}

type AnalysisJob struct {
	UserID     string
	WriteModel string
}

type AnalysisJobRequest struct {
	UserID     string
	WriteModel string
}

type ActiveJob struct{ ID string }

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return fmt.Sprintf("analysis job %s is already in progress", e.ActiveID)
}

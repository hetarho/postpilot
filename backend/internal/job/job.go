// Package job owns durable long-running work and the in-process worker that consumes it.
package job

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	KindGenerate     = "generate"
	KindRevise       = "revise"
	KindAnalyzeVoice = "analyze_voice"

	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

var (
	ErrNotFound       = errors.New("job not found")
	ErrForbidden      = errors.New("job belongs to another user")
	ErrActiveConflict = errors.New("an active job already exists")
	ErrInvalidTarget  = errors.New("job target does not belong to user")
)

// ErrAlreadyInProgress identifies the durable job a caller should attach to.
type ErrAlreadyInProgress struct {
	ActiveID string
}

func (e *ErrAlreadyInProgress) Error() string {
	return fmt.Sprintf("job %s is already in progress", e.ActiveID)
}

func (e *ErrAlreadyInProgress) Unwrap() error { return ErrActiveConflict }

// NewJob is the immutable input recorded before the worker is woken.
type NewJob struct {
	Kind         string
	UserID       string
	PostSlug     *string
	ObserveModel string
	WriteModel   string
	Payload      []byte
}

// Job is the worker-facing record, including the kind-specific payload.
type Job struct {
	ID            string
	Kind          string
	UserID        string
	PostSlug      *string
	Status        string
	Stage         string
	ProgressDone  int
	ProgressTotal int
	Error         string
	ObserveModel  string
	WriteModel    string
	Payload       []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

// JobSummary is the public view returned to other contexts and the RPC edge.
type JobSummary struct {
	ID            string
	Kind          string
	UserID        string
	PostSlug      *string
	Status        string
	Stage         string
	ProgressDone  int
	ProgressTotal int
	Error         string
	ObserveModel  string
	WriteModel    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func summarize(found Job) *JobSummary {
	return &JobSummary{
		ID: found.ID, Kind: found.Kind, UserID: found.UserID, PostSlug: found.PostSlug,
		Status: found.Status, Stage: found.Stage, ProgressDone: found.ProgressDone,
		ProgressTotal: found.ProgressTotal, Error: found.Error,
		ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
		CreatedAt: found.CreatedAt, UpdatedAt: found.UpdatedAt,
	}
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("job: cannot read random bytes for a job id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

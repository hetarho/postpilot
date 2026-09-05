package guideline

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// CandidateStatus is where a recorded instruction stands. All three states are kept rather
// than deleted, because the row itself is what stops the same instruction from being
// recorded a second time: a dismissed correction must not come back, and an approved one
// must not be offered again.
type CandidateStatus string

const (
	CandidateStatusPending   CandidateStatus = "pending"
	CandidateStatusApproved  CandidateStatus = "approved"
	CandidateStatusDismissed CandidateStatus = "dismissed"
)

func (s CandidateStatus) Valid() bool {
	return s == CandidateStatusPending || s == CandidateStatusApproved || s == CandidateStatusDismissed
}

// CandidateTextMaxChars is the bound a candidate is STORED at: the revision instruction
// bound, not the guideline bound. Refusing a long instruction at recording time would lose
// exactly the most specific corrections, so the guideline bound is enforced at approval,
// where the user can shorten the text with a live counter (change 26).
//
// It is code rather than config for the same reason the revision bound is: the two must not
// be able to drift apart, since a candidate is only ever a copy of one instruction.
const CandidateTextMaxChars = 500

var (
	// ErrCandidateNotFound covers unknown and foreign ids alike, like ErrNotFound.
	ErrCandidateNotFound = errors.New("guideline candidate not found")
	// ErrCandidateTextInvalid is the blank-after-trim case. Recording it would put an empty
	// row in the review list that could never become a guideline.
	ErrCandidateTextInvalid = errors.New("guideline candidate text is required")
)

// Candidate is a receipt for something the user wrote, awaiting review. It carries no scope
// at all: scope is a durable decision about every future post, made at approval time. That
// absence is what lets recording be automatic without anything being applied.
type Candidate struct {
	ID          string
	UserID      string
	Text        string
	PostSlug    string
	Status      CandidateStatus
	Occurrences int
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

// CandidateRecording is what the store must do for one instruction. It is a decision, not a
// suggestion: the store carries it out inside the recording transaction, where the dedupe
// read and the pending count that produced it cannot have moved.
type CandidateRecording int

const (
	// RecordCandidateSkip records nothing: the text is already a saved guideline, or the
	// pending queue is full.
	RecordCandidateSkip CandidateRecording = iota
	// RecordCandidateInsert is a first sighting.
	RecordCandidateInsert
	// RecordCandidateCount is a repeat of a PENDING candidate: the count is the signal that
	// a one-off correction has become a standing rule, so it belongs on a row the user is
	// still being asked about.
	RecordCandidateCount
)

// DecideRecording is the recording rule as a pure decision over the three facts the store
// reads inside the recording transaction: the status of an existing candidate with this
// exact text (empty when there is none), whether a guideline already holds it, and how many
// pending candidates the account holds.
//
// An approved or dismissed candidate is a Skip, not a Count: change 26 groups it with the
// saved-guideline case — an instruction the user already ruled on records nothing and
// revives nothing, and a count nobody can see would be the only difference.
func DecideRecording(existing CandidateStatus, existingGuideline bool, pendingHeld, maxPending int) CandidateRecording {
	switch existing {
	case CandidateStatusPending:
		return RecordCandidateCount
	case CandidateStatusApproved, CandidateStatusDismissed:
		return RecordCandidateSkip
	}
	if existingGuideline {
		return RecordCandidateSkip
	}
	if pendingHeld >= maxPending {
		return RecordCandidateSkip
	}
	return RecordCandidateInsert
}

// validCandidateText trims and bounds an instruction at the CANDIDATE bound. The text is
// otherwise untouched: nothing normalizes case, punctuation, spacing or wording, because a
// candidate is the user's own sentence and the dedupe below is exact-after-trim.
func validCandidateText(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrCandidateTextInvalid
	}
	if utf8.RuneCountInString(trimmed) > CandidateTextMaxChars {
		return "", &TextTooLongError{Chars: utf8.RuneCountInString(trimmed), Max: CandidateTextMaxChars}
	}
	return trimmed, nil
}

package guideline

import (
	"context"
	"time"
)

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). Ownership is a property of the query rather than of a check the
// caller must remember: every method takes the account and scopes its SQL by it, so a
// same-shaped id from another account reads as missing.
type Store interface {
	// Insert refuses past maxPerAccount INSIDE its own transaction. The cap cannot be
	// checked in the service and enforced here: two concurrent creates would both read
	// max-1 and both win.
	//
	// approval marks the candidate this create came from, in the same transaction, so a
	// refused create leaves no candidate approved and a saved guideline never leaves its
	// candidate pending. A named id that matches no candidate of the account is refused —
	// approving something that is not there is a request that did not happen.
	Insert(ctx context.Context, g Guideline, maxPerAccount int, approval CandidateApproval) error
	// List returns the account's guidelines in injection order — the global group, then the
	// template group, then the 분야 group, each by created_at then id (GUIDE-14) — with
	// TemplateIDs and Fields populated.
	List(ctx context.Context, userID string, kind Kind) ([]Guideline, error)
	Get(ctx context.Context, userID, id string) (Guideline, error)
	// Update applies only the present parts of the patch in one transaction. A present scope
	// replaces the kind, the template links and the 분야 links together, so a rescope leaves no
	// link of the other kind; an absent one is not written at all, so a text edit cannot
	// revert a scope another tab saved.
	Update(ctx context.Context, userID, id string, patch Patch, updatedAt time.Time) (Guideline, error)
	Delete(ctx context.Context, userID, id string) error
	// ApplicableTexts returns the texts that apply to one post, in injection order (GUIDE-14):
	// the account's global guidelines, then those linked to templateID, then those linked to
	// field. An empty templateID is a post with no template and an empty field a post with no
	// 분야; each matches no link, so a post missing either receives only the groups it has.
	ApplicableTexts(ctx context.Context, userID, templateID, field string) ([]string, error)
	// ClipApplicableTexts is ApplicableTexts for a clip: the account's global clip guidelines, then
	// those linked to videoTemplateID, each by created_at then id. An empty id is a clip with no
	// video template.
	ClipApplicableTexts(ctx context.Context, userID, videoTemplateID string) ([]string, error)
	// DefaultsOff returns the 기본 지침 keys the account switched off for one kind; a row means
	// off, so an account that never switched anything has none (GUIDE-43).
	DefaultsOff(ctx context.Context, userID string, kind Kind) ([]string, error)
	// SetDefaultOff writes one switch, idempotently either way.
	SetDefaultOff(ctx context.Context, userID string, kind Kind, key string, off bool, at time.Time) error

	// RecordCandidate carries out the whole recording rule in ONE transaction: it reads the
	// existing candidate's status, whether a guideline already holds the text and the pending
	// count, asks DecideRecording, and then inserts or counts up. The decision cannot be made
	// in the service and applied here — two concurrent recordings would both read max-1
	// pending, and both would insert. `recorded` is false for a skip, which is an ordinary
	// outcome and not an error.
	RecordCandidate(ctx context.Context, c Candidate, maxPending int) (recorded bool, err error)
	// ListPendingCandidates returns the pending ones in review order (occurrences desc, then
	// last-seen desc) together with the account's pending count, so the caller can say
	// whether the queue is full without owning the bound.
	ListPendingCandidates(ctx context.Context, userID string, kind Kind) ([]Candidate, int, error)
	// SetCandidateStatus moves one candidate out of pending. An unknown or foreign id reads
	// as missing, like every other method here.
	SetCandidateStatus(ctx context.Context, userID, id string, status CandidateStatus) error
	// DropCandidatePostSlug detaches every candidate of the account that named one post.
	DropCandidatePostSlug(ctx context.Context, userID, postSlug string) error
	// DropCandidateClipID detaches every candidate of the account that named one clip project.
	DropCandidateClipID(ctx context.Context, userID, clipID string) error
}

// CandidateApproval is the create's approval side effect, applied inside the insert's own
// transaction so a saved guideline and the candidate it came from can never disagree.
//
// ID is set only when the user approved a candidate they EDITED first, whose text no longer
// matches the row. Text is always set, and is what marks the candidate an on-the-spot
// 지침으로 저장 recorded without the client having to learn its id.
type CandidateApproval struct {
	ID   string
	Text string
}

// TemplateDirectory is the template context's published directory, consumed here for two
// things and nothing else: proving a scoped id is owned by the account at write time, and
// projecting names on read. It is a port rather than a join because templates are another
// context's table (ARCHITECTURE §2.2).
type TemplateDirectory interface {
	Templates(ctx context.Context, userID string) ([]TemplateRef, error)
}

// VideoTemplateDirectory is the clip context's video templates, consumed exactly as
// TemplateDirectory is, for a clip guideline's scope (GUIDE-5). The guideline context reads no
// clip table: the composite key is the schema's only link (ARCH-7).
type VideoTemplateDirectory interface {
	VideoTemplates(ctx context.Context, userID string) ([]TemplateRef, error)
}

// FieldDirectory answers whether a 분야 id is on the product's list, consumed to validate a
// fields scope before anything is written. It is declared here by its
// consumer, so this context never imports the one that owns the list (ARCH-6, ARCH-7).
type FieldDirectory interface {
	Known(id string) bool
}

// TestedSettings commits a frozen winner and its receipt atomically under ordinary caps.
// A retry reads the committed receipt even if its target was later edited or deleted.
type TestedSettings interface {
	PublishTestWinner(context.Context, TestedPublication) (TestedPublicationReceipt, error)
}

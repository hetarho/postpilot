package memory

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). Ownership is a property of the query rather than of a check the
// caller must remember: every method takes the account and scopes its SQL by it, so a
// same-shaped id from another account reads as missing.
type Store interface {
	// Insert applies the whole create rule in ONE transaction: the exact-text lookup, the
	// cap and the write. It cannot be split — the cap read in the service and the insert
	// here would let two concurrent creates both read max-1 and both commit, and a dedupe
	// checked in the service would let two approvals of one text both insert.
	//
	// A text the account already holds inserts nothing: the source link is added, the
	// memory's last use advances to seenAt, and the EXISTING row comes back with
	// deduplicated true. Its kind and tags are left exactly as they were — a re-approval
	// is a second sighting of a fact, not an edit of it (MEM-9).
	//
	// sourcePostSlug is empty for a memory written by hand, which then has no link at all.
	Insert(ctx context.Context, m Memory, sourcePostSlug string, maxPerAccount int) (stored Memory, deduplicated bool, err error)
	// List returns the account's memories in injection order — most recently used first,
	// then most recently created — with tags and source links populated.
	List(ctx context.Context, userID string) ([]Memory, error)
	Get(ctx context.Context, userID, id string) (Memory, error)
	// Update applies only the present parts of the patch in one transaction. A present tag
	// set replaces the whole set; an absent one is not written at all, so a text edit
	// cannot revert tags another tab saved.
	Update(ctx context.Context, userID, id string, patch Patch, updatedAt time.Time) (Memory, error)
	Delete(ctx context.Context, userID, id string) error
	// DropPostSources carries out MEM-17 in one transaction: it drops every link naming one
	// post and then deletes exactly those memories the dropped link was the last one of. A
	// memory written by hand has no links at all and is never touched by it, which is why
	// the deletion is scoped to the memories that just lost a link rather than to every
	// memory with none.
	DropPostSources(ctx context.Context, userID, postSlug string) error
}

// Models is the account's analyze selection and the provider call, exactly as the voice
// context consumes them: extraction reads finished prose, which is what the analyze stage
// is for, so no new per-account model setting appears (MEM-13).
type Models interface {
	AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error)
	Resolve(ref llm.ModelRef) (llm.ModelInfo, bool)
	Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error)
}

// Posts is the one thing this context asks of the post context: the finished post to read,
// ownership already checked. It crosses as TEXT — this context never learns what a block is.
type Posts interface {
	ExtractionSource(ctx context.Context, userID, slug string) (ExtractionSource, error)
}

// ExtractionJobs is the durable job the extraction runs as. Enqueue passes the shared credit
// gate at the queue's own seam (QUOTA-13), so this context neither prices nor charges
// anything; SaveCandidates writes the result onto the job row, and Candidates reads it back
// for its owner — a job of another account reads as missing there.
type ExtractionJobs interface {
	Enqueue(ctx context.Context, request ExtractionRequest) (string, error)
	SaveCandidates(ctx context.Context, jobID string, payload []byte) error
	Candidates(ctx context.Context, userID, jobID string) ([]byte, error)
}

// ExtractionRequest is one enqueue: the post it reads, the frozen analyze model, and the
// frozen source. The source rides the job row so an edit made while the job waits cannot
// change what was extracted.
type ExtractionRequest struct {
	UserID   string
	PostSlug string
	Model    string
	Source   ExtractionSource
}

// Package modelcatalog is the curated model catalog: which of the provider's models this
// installation offers, and at which plan tier. It owns catalog_models.
//
// It is the operator's half of the model story. The provider context remembers what a USER
// picked; this context decides what there is to pick from, and the llm registry reads it
// live so a curation change applies to the next request rather than the next deploy.
package modelcatalog

import (
	"errors"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

var (
	// ErrNotFound: the model is neither curated nor offered upstream.
	ErrNotFound = errors.New("model not found")
	// ErrInvalidReasoning: the requested per-model override is not a known effort.
	ErrInvalidReasoning = errors.New("invalid reasoning effort")
)

// Model is one curated row: the operator's decisions plus the snapshot of upstream facts
// taken when the row was written or last refreshed.
type Model struct {
	ModelID             string
	ProviderSlug        string
	Label               string
	Vision              bool
	StructuredOutput    bool
	ContextTokens       int64
	InputUSDPerMillion  string
	OutputUSDPerMillion string
	PricingCheckedAt    string
	Reasoning           llm.ReasoningEffort
	Enabled             bool
	// Listed: the upstream catalog still offered this model at the last successful refresh.
	Listed     bool
	LastSeenAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Candidate is one model the upstream catalog currently offers. It carries no curation —
// that is what a Model adds.
type Candidate struct {
	ModelID             string
	ProviderSlug        string
	Label               string
	Description         string
	Vision              bool
	StructuredOutput    bool
	ContextTokens       int64
	InputUSDPerMillion  string
	OutputUSDPerMillion string
	// SourceCreatedAt is the upstream publication time in epoch seconds. It is what orders
	// a provider's models newest-first.
	SourceCreatedAt int64
}

// Snapshot is one read of the upstream catalog.
type Snapshot struct {
	Candidates []Candidate
	FetchedAt  time.Time
	FromCache  bool
}

// Entry is one row of the operator's browse list: an upstream candidate, a curated model,
// or both. A curated model with no candidate is one the upstream catalog has stopped
// offering — the case the screen has to make visible.
type Entry struct {
	Candidate
	// Curated: a catalog_models row exists, so Reasoning and Enabled are decisions somebody
	// made rather than defaults.
	Curated   bool
	Enabled   bool
	Reasoning llm.ReasoningEffort
	Listed    bool
}

// Browse is the whole answer to "what can I curate": the merged list plus everything the
// screen needs to say about where it came from.
type Browse struct {
	Entries   []Entry
	FetchedAt time.Time
	FromCache bool
	// FetchError is the upstream failure, when there was one. Entries then carry curated
	// rows only, and no availability bookkeeping was written.
	FetchError string
}

// Patch is a partial curation edit. A nil member is not being changed, which is what lets
// two operators edit different fields of one model without overwriting each other.
type Patch struct {
	Enabled   *bool
	Reasoning *llm.ReasoningEffort
}

// ProviderSlugOf is the vendor segment of an upstream model id — "openai" in
// "openai/gpt-5.6-sol". It groups and filters the browse list; it is NOT the registry's
// provider id, which is the same for every row.
func ProviderSlugOf(modelID string) string {
	slug, _, ok := strings.Cut(modelID, "/")
	if !ok || slug == "" {
		return modelID
	}
	return slug
}

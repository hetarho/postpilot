// Package modelcatalog is the curated model catalog: which of the provider's models this
// installation offers, and for which purposes. It owns catalog_models and
// catalog_model_purposes.
//
// It is the operator's half of the model story. The provider context remembers what a USER
// picked; this context decides what there is to pick from, and the llm registry reads it
// live so a curation change applies to the next request rather than the next deploy.
package modelcatalog

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

var (
	// ErrNotFound: the model is neither curated nor offered upstream.
	ErrNotFound = errors.New("model not found")
	// ErrInvalidReasoning: the requested per-model override is not a known effort.
	ErrInvalidReasoning = errors.New("invalid reasoning effort")
	// ErrUnknownPurpose: the requested purpose is not one of the five this catalog curates.
	ErrUnknownPurpose = errors.New("unknown purpose")
	// ErrPurposeIneligible: the model lacks the capability the purpose requires
	// (photo-analysis needs vision; a generation purpose needs the matching output).
	ErrPurposeIneligible = errors.New("model not capable of purpose")
)

// Purpose is one use the product puts a model to. Registration is per purpose (change 20):
// the same model may serve several, and each user-facing stage lists only the models
// registered to ITS purpose. The two generation purposes are curated ahead of the features
// that will consume them — nothing outside the operator screen reads them yet.
type Purpose string

const (
	PurposePhotoAnalysis   Purpose = "photo-analysis"
	PurposeStyleAnalysis   Purpose = "style-analysis"
	PurposeWriting         Purpose = "writing"
	PurposeImageGeneration Purpose = "image-generation"
	PurposeVideoGeneration Purpose = "video-generation"
)

// Purposes in display order — the order the operator tabs use.
var Purposes = []Purpose{
	PurposePhotoAnalysis, PurposeStyleAnalysis, PurposeWriting,
	PurposeImageGeneration, PurposeVideoGeneration,
}

// SortPurposes orders a registration set in display order, in place. The one comparator
// for every reader, so a write's answer and the next read cannot order differently.
func SortPurposes(purposes []Purpose) {
	slices.SortFunc(purposes, func(a, b Purpose) int {
		return slices.Index(Purposes, a) - slices.Index(Purposes, b)
	})
}

// ParsePurpose accepts the stored/wire form.
func ParsePurpose(s string) (Purpose, error) {
	for _, p := range Purposes {
		if string(p) == s {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownPurpose, s)
}

// Stage is the user-facing stage this purpose feeds, in the stable string form the llm
// boundary already carries ("observe"/"write"/"analyze"), or "" for the generation
// purposes, which no stage consumes yet.
func (p Purpose) Stage() string {
	switch p {
	case PurposePhotoAnalysis:
		return llm.StageNameObserve
	case PurposeStyleAnalysis:
		return llm.StageNameAnalyze
	case PurposeWriting:
		return llm.StageNameWrite
	default:
		return ""
	}
}

// EligibleFor is the capability gate: whether this model can serve the purpose at all. It
// is enforced at REGISTRATION, which is what lets the stage-side check stay a pure
// membership test — an observe model is vision-capable because it could not have been
// registered otherwise. It takes the whole row rather than three same-typed bools, so a
// caller cannot transpose the image/video flags without a compile error.
func (p Purpose) EligibleFor(m Model) bool {
	switch p {
	case PurposePhotoAnalysis:
		return m.Vision
	case PurposeImageGeneration:
		return m.ImageOutput
	case PurposeVideoGeneration:
		return m.VideoOutput
	default:
		return true
	}
}

// Model is one curated row: the operator's decisions plus the snapshot of upstream facts
// taken when the row was written or last refreshed.
type Model struct {
	ModelID          string
	ProviderSlug     string
	Label            string
	Vision           bool
	StructuredOutput bool
	// ImageOutput/VideoOutput: what the model can produce, from the source's
	// architecture.output_modalities — the generation purposes gate on them.
	ImageOutput         bool
	VideoOutput         bool
	ContextTokens       int64
	InputUSDPerMillion  string
	OutputUSDPerMillion string
	PricingCheckedAt    string
	Reasoning           llm.ReasoningEffort
	// Purposes this model is registered to. Zero purposes is the kept-but-served-to-nobody
	// state the old `enabled = 0` used to be: the reasoning override survives, users see
	// nothing.
	Purposes []Purpose
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
	ImageOutput         bool
	VideoOutput         bool
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
	// Curated: a catalog_models row exists, so Reasoning and Purposes are decisions somebody
	// made rather than defaults.
	Curated   bool
	Purposes  []Purpose
	Reasoning llm.ReasoningEffort
	Listed    bool
}

// EntryOf presents a curated row in the browse-list shape — the one projection both the
// merge and a curation write's answer use, so the two can never carry different fields.
// The upstream description is absent: it lives only in the live candidate list.
func EntryOf(m Model) Entry {
	return Entry{
		Candidate: Candidate{
			ModelID: m.ModelID, ProviderSlug: m.ProviderSlug, Label: m.Label,
			Vision: m.Vision, StructuredOutput: m.StructuredOutput,
			ImageOutput: m.ImageOutput, VideoOutput: m.VideoOutput,
			ContextTokens:      m.ContextTokens,
			InputUSDPerMillion: m.InputUSDPerMillion, OutputUSDPerMillion: m.OutputUSDPerMillion,
		},
		Curated: true, Purposes: m.Purposes,
		Reasoning: m.Reasoning, Listed: m.Listed,
	}
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
// Purpose registration is not a patch — it has its own write (SetPurpose).
type Patch struct {
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

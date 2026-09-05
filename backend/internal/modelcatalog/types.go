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
	// ErrPurposeNotRegistered: an effort was set for a purpose this model does not serve.
	// The control only appears once registered, and the server holds the same rule.
	ErrPurposeNotRegistered = errors.New("model is not registered to purpose")
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
	ReasoningCapability
	// Purposes this model is registered to. Zero purposes is the kept-but-served-to-nobody
	// state the old `enabled = 0` used to be: the row survives, users see nothing.
	Purposes []Purpose
	// Reasoning is the operator's override PER REGISTRATION, keyed by purpose. It is a
	// property of "this model doing this task", not of the model: the code-owned policy it
	// overrides is per stage, and the right effort is a measurement of a model against a
	// task (change 24). A purpose absent from the map, or present as Unspecified, defers to
	// the stage policy. An unregistered purpose has no effort to carry, which is consistent
	// — a model serves it to nobody.
	Reasoning map[Purpose]llm.ReasoningEffort
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
	ReasoningCapability
	// SourceCreatedAt is the upstream publication time in epoch seconds. It is what orders
	// a provider's models newest-first.
	SourceCreatedAt int64
}

// ReasoningCapability is what the source publishes about one model's reasoning (change 27).
// It exists so the operator chooses an effort from what the model actually accepts, instead
// of from the same eight values for every model.
//
// EVERY FIELD'S ZERO MEANS "UNKNOWN", NOT "SUPPORTS NOTHING" — the same rule the pricing
// snapshot already follows. An empty Efforts list is a model whose accepted values the
// source does not publish (only ~154 of 427 do), and the answer there is to offer all eight,
// not none.
type ReasoningCapability struct {
	// Reasons: the source carries a `reasoning` object for this model at all.
	Reasons bool
	// Efforts: the accepted effort values, VERBATIM and in the source's descending order,
	// because that order is the order a selector should offer them in.
	Efforts []string
	// DefaultEffort: what the model uses when reasoning is enabled and no effort is sent.
	// It is what `unset` actually means, so the admin can see that leaving it alone means
	// `high` rather than "off".
	DefaultEffort string
	// Mandatory: reasoning cannot be turned off, so `none` must never be offered or sent.
	Mandatory bool
	// NativeEffort: the provider receives the effort STRING itself, rather than a token
	// budget OpenRouter derived from it. Nothing here consumes it; change 29 needs it to
	// size a completion budget safely.
	NativeEffort bool
	// MaxTokens: the source offers a reasoning token budget for this model. Recorded and
	// displayed only — this change surfaces no input for it.
	MaxTokens bool
}

// Known reports whether this capability came from a read that actually looked. It is the one
// thing the six fields cannot say about themselves: `Reasons: false, Efforts: nil` is BOTH
// "the source published no reasoning object" and "nothing has ever asked" — migration 0024
// leaves every existing row in exactly that shape.
//
// It is not stored. A capability read live from the source is known by construction; one
// read back from a row is only known if it says something. Treating the ambiguous case as
// unknown is the safe direction: it offers the operator more values than the model may take
// (which the server still refuses on the way in), rather than silently removing a control
// while the stored override keeps being sent.
func (c ReasoningCapability) Known() bool {
	return c.Reasons || len(c.Efforts) > 0 || c.DefaultEffort != "" || c.Mandatory || c.MaxTokens || c.NativeEffort
}

// AcceptsEffort answers whether one effort value may be stored for this model.
//
// A model the source CONFIRMS does not reason accepts no effort at all — there is nothing for
// one to mean. An unknown capability accepts everything, and so does a known one that
// publishes no list: the source not publishing a list is not the model refusing every value.
// `none` is refused when reasoning is mandatory, and when a published list omits it.
func (c ReasoningCapability) AcceptsEffort(effort llm.ReasoningEffort) bool {
	// Clearing the override, and `unset` (send no effort key), are not claims about the
	// model's vocabulary — they are always allowed.
	if effort == llm.ReasoningUnspecified || effort == llm.ReasoningUnset {
		return true
	}
	if c.Known() && !c.Reasons {
		return false
	}
	if effort == llm.ReasoningNone && c.Mandatory {
		return false
	}
	if len(c.Efforts) == 0 {
		return true
	}
	return slices.Contains(c.Efforts, string(effort))
}

// DriftedFrom reports an override the source no longer lists — a revised model, a replaced
// slug. It is derived at read time from the two fields rather than stored, because it must
// follow the catalog; and it is a WARNING only. Plan 18's `listed = 0` precedent applies:
// the source's list changing is not a mandate to rewrite an operator's decision, so the
// value is kept and still sent.
func (c ReasoningCapability) DriftedFrom(effort llm.ReasoningEffort) bool {
	// Neither "no override" nor `unset` is a claim about what the model accepts, so neither
	// can drift.
	if effort == llm.ReasoningUnspecified || effort == llm.ReasoningUnset {
		return false
	}
	// Drift is exactly "this stored value would be refused if it were written today", which
	// is the rule above read backwards. Deriving it rather than repeating the list check
	// catches the cases a list comparison alone misses: a model that became mandatory while
	// the override is `none`, and one that stopped reasoning altogether.
	return !c.AcceptsEffort(effort)
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
	Curated  bool
	Purposes []Purpose
	// Reasoning is the effort for the PURPOSE being listed, not the model's. The browse list
	// is read one purpose tab at a time, so the evidence and the control shown on a tab
	// belong to that tab (change 24).
	Reasoning llm.ReasoningEffort
	Listed    bool
	// ReasoningSpend is the recent reasoning-vs-completion split for this model at the
	// purpose being listed, or nil when nothing has been recorded for it. It is what makes a
	// model ignoring its effort visible BEFORE it fails a user's job — the only reliable
	// check, since `supported_parameters` says a model accepts `reasoning_effort` but never
	// which values it honors.
	ReasoningSpend *ReasoningSpend
	// ReasoningDrifted: the stored override would be refused if it were written today — no
	// longer in the model's published list, `none` on a model that became mandatory, an
	// effort on a model that stopped reasoning. A warning for the row, never a correction:
	// the value is kept and still sent.
	ReasoningDrifted bool
	// ReasoningKnown: the capability above came from a read that actually looked, so
	// `reasons: false` means the source published no reasoning object rather than "nothing
	// has asked yet". False for a row served from storage — after migration 0024 and before
	// the first successful refresh, or whenever the provider catalog cannot be read — and the
	// screen must then keep offering the full effort vocabulary rather than hiding a control
	// whose stored value is still being sent.
	ReasoningKnown bool
}

// ReasoningSpend is a recent window of one model's completion budget at one purpose.
type ReasoningSpend struct {
	Calls                int64
	ReasoningTokens      int64
	CompletionTokens     int64
	ReasoningTruncations int64
}

// ReasoningShare is the fraction of the completion budget spent on reasoning, 0-1. Zero
// completion tokens reports 0 rather than dividing.
func (s ReasoningSpend) ReasoningShare() float64 {
	if s.CompletionTokens <= 0 {
		return 0
	}
	return float64(s.ReasoningTokens) / float64(s.CompletionTokens)
}

// EntryOf presents a curated row in the browse-list shape — the one projection both the
// merge and a curation write's answer use, so the two can never carry different fields.
// The upstream description is absent: it lives only in the live candidate list.
//
// `purpose` is the tab being listed, and it is what selects the effort to report. An empty
// purpose reports none, which is what a caller with no tab in hand should show.
func EntryOf(m Model, purpose Purpose) Entry {
	return Entry{
		Candidate: Candidate{
			ModelID: m.ModelID, ProviderSlug: m.ProviderSlug, Label: m.Label,
			Vision: m.Vision, StructuredOutput: m.StructuredOutput,
			ImageOutput: m.ImageOutput, VideoOutput: m.VideoOutput,
			ContextTokens:      m.ContextTokens,
			InputUSDPerMillion: m.InputUSDPerMillion, OutputUSDPerMillion: m.OutputUSDPerMillion,
			ReasoningCapability: m.ReasoningCapability,
		},
		Curated: true, Purposes: m.Purposes,
		Reasoning: m.Reasoning[purpose], Listed: m.Listed,
		ReasoningDrifted: m.DriftedFrom(m.Reasoning[purpose]),
		// A stored row can only be trusted about reasoning if it says something: see Known.
		ReasoningKnown: m.Known(),
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

// Patch is a partial curation edit for ONE (model, purpose). A nil member is not being
// changed, which is what lets two operators edit different fields of one model without
// overwriting each other. Purpose registration is not a patch — it has its own write
// (SetPurpose) — and an effort may only be set for a purpose the model is registered to,
// which is a server rule rather than a UI convention (change 24).
type Patch struct {
	Purpose   Purpose
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

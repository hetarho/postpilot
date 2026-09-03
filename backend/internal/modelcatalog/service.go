package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Service is the catalog's use-cases and, at the same time, the llm registry's model
// source.
//
// Those are one object because they are one piece of state: the registry needs the curated
// list on every request but cannot take a context or fail, so the list is held in memory
// and the writes that change it are right here. Splitting them would mean an invalidation
// message between two halves of the same decision.
type Service struct {
	store    Store
	upstream Upstream
	spend    ReasoningSpendReader
	now      func() time.Time

	mu     sync.RWMutex
	models []llm.SourceModel
	byID   map[string]llm.SourceModel
}

// NewService wires the context. The upstream catalog is attached separately because its
// address comes from the loaded registry, which is built after this.
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now, byID: map[string]llm.SourceModel{}}
}

// SetUpstream attaches the provider's own catalog. Called once at boot; without it the
// operator screen can still read and edit curated rows, it just cannot discover new ones.
func (s *Service) SetUpstream(u Upstream) { s.upstream = u }

// SetReasoningSpend wires the ledger's aggregate. Without it the curation surface simply
// carries no spend signal, which is the same outcome an account with no recorded calls has.
func (s *Service) SetReasoningSpend(r ReasoningSpendReader) { s.spend = r }

// Reload refreshes the in-memory view the registry reads. Called at boot and after every
// curation write.
func (s *Service) Reload(ctx context.Context) error {
	rows, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("load model catalog: %w", err)
	}
	s.setCache(rows)
	return nil
}

func (s *Service) setCache(rows []Model) {
	models := make([]llm.SourceModel, 0, len(rows))
	byID := make(map[string]llm.SourceModel, len(rows))
	for _, row := range rows {
		// Zero registrations is the kept-but-served-to-nobody state: the row (and its
		// reasoning override) survives, but the registry never sees it.
		if len(row.Purposes) == 0 {
			continue
		}
		model := llm.SourceModel{
			ModelID:             row.ModelID,
			Label:               row.Label,
			Vision:              row.Vision,
			StructuredOutput:    row.StructuredOutput,
			ContextTokens:       row.ContextTokens,
			InputUSDPerMillion:  row.InputUSDPerMillion,
			OutputUSDPerMillion: row.OutputUSDPerMillion,
			PricingCheckedAt:    row.PricingCheckedAt,
			Reasoning:           stageReasoningOf(row),
			Stages:              stagesOf(row),
			Delisted:            !row.Listed,
		}
		models = append(models, model)
		byID[row.ModelID] = model
	}
	s.mu.Lock()
	s.models, s.byID = models, byID
	s.mu.Unlock()
}

// Models implements llm.ModelSource. The copy is deliberate: the registry hands this slice
// to a caller that may sort or filter it, and the cache must not move underneath.
func (s *Service) Models() []llm.SourceModel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]llm.SourceModel(nil), s.models...)
}

// Lookup implements llm.ModelSource.
func (s *Service) Lookup(modelID string) (llm.SourceModel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	found, ok := s.byID[modelID]
	return found, ok
}

// Browse is the operator's view: every model the provider offers, annotated with what has
// been curated, plus every curated model the provider has stopped offering.
//
// A successful read of the live catalog also refreshes the stored snapshots and the
// availability flags. A FAILED one changes nothing and is reported instead: bookkeeping
// that treated an outage as evidence would retire the whole catalog the first time the
// network hiccuped.
// `purpose` is the tab being listed: it selects which effort each entry reports and which
// stage's spend signal is attached, so the evidence and the control the operator sees belong
// to the tab they are looking at (change 24). An empty purpose reports neither.
func (s *Service) Browse(ctx context.Context, refresh bool, purpose Purpose) (Browse, error) {
	var (
		snapshot   Snapshot
		fetchError string
	)
	if s.upstream == nil {
		fetchError = "no upstream catalog is configured"
	} else if found, err := s.upstream.Fetch(ctx, refresh); err != nil {
		// The operator sees the failure on screen; the cause belongs in the log, not in a
		// message that would carry a URL or a provider's prose to the browser.
		slog.Warn("model catalog fetch failed", "err", err)
		fetchError = "the provider catalog could not be read"
	} else {
		snapshot = found
	}

	if fetchError == "" && !snapshot.FromCache {
		if err := s.store.RefreshAvailability(ctx, snapshot.Candidates, s.now()); err != nil {
			return Browse{}, fmt.Errorf("refresh catalog availability: %w", err)
		}
	}

	rows, err := s.store.List(ctx)
	if err != nil {
		return Browse{}, fmt.Errorf("list curated models: %w", err)
	}
	// The rows were just read for the merge, so the registry's view is refreshed from them
	// rather than with a second query — and any invalidation a write failed to apply
	// self-heals the next time this screen is opened.
	s.setCache(rows)

	curated := make(map[string]Model, len(rows))
	for _, row := range rows {
		curated[row.ModelID] = row
	}

	spend := s.reasoningSpend(ctx, purpose)

	entries := make([]Entry, 0, len(snapshot.Candidates)+len(rows))
	for _, candidate := range snapshot.Candidates {
		entry := Entry{Candidate: candidate, Listed: true}
		if row, ok := curated[candidate.ModelID]; ok {
			entry.Curated, entry.Purposes = true, row.Purposes
			entry.Reasoning = row.Reasoning[purpose]
			delete(curated, candidate.ModelID)
		}
		entry.ReasoningSpend = spend[entry.ModelID]
		entries = append(entries, entry)
	}
	// What is left is curated but unoffered. It keeps the snapshot taken when it was last
	// seen, so the row still reads as a model rather than as an id.
	for _, row := range rows {
		if _, still := curated[row.ModelID]; !still {
			continue
		}
		entry := EntryOf(row, purpose)
		entry.ReasoningSpend = spend[row.ModelID]
		entries = append(entries, entry)
	}

	slices.SortFunc(entries, func(a, b Entry) int {
		if a.ProviderSlug != b.ProviderSlug {
			if a.ProviderSlug < b.ProviderSlug {
				return -1
			}
			return 1
		}
		// Newest first within a vendor: a catalog is read to find what is new.
		if a.SourceCreatedAt != b.SourceCreatedAt {
			if a.SourceCreatedAt > b.SourceCreatedAt {
				return -1
			}
			return 1
		}
		return cmpString(a.ModelID, b.ModelID)
	})

	return Browse{
		Entries: entries, FetchedAt: snapshot.FetchedAt,
		FromCache: snapshot.FromCache, FetchError: fetchError,
	}, nil
}

// SetPurpose registers or deregisters a model for one purpose — the write that decides
// what users can pick ([I3]: registering never selects anything for anyone).
//
// Registering a model still offered upstream snapshots the live entry; one that is not,
// but already has a row, is re-registered from what is stored — so an operator can undo a
// deregistration without waiting for the provider's catalog to be reachable. The purpose's
// capability gate runs here, server-side, so a tab that hid an ineligible model is a
// convenience rather than the enforcement.
func (s *Service) SetPurpose(ctx context.Context, modelID string, purpose Purpose, registered bool) (Model, error) {
	if _, err := ParsePurpose(string(purpose)); err != nil {
		return Model{}, err
	}
	existing, err := s.store.Get(ctx, modelID)
	hasRow := err == nil
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Model{}, fmt.Errorf("read curated model: %w", err)
	}

	now := s.now()
	if !registered {
		// Deregistering something that was never curated is answered honestly rather than
		// invented as a no-op row.
		if !hasRow {
			return Model{}, fmt.Errorf("%w: %s", ErrNotFound, modelID)
		}
		if !slices.Contains(existing.Purposes, purpose) {
			return existing, nil
		}
		if err := s.store.DeregisterPurpose(ctx, modelID, purpose, now); err != nil {
			return Model{}, fmt.Errorf("deregister model purpose: %w", err)
		}
		// A fresh slice rather than an in-place delete: the read row may share its backing
		// array with the store's own state.
		remaining := make([]Purpose, 0, len(existing.Purposes))
		for _, p := range existing.Purposes {
			if p != purpose {
				remaining = append(remaining, p)
			}
		}
		existing.Purposes = remaining
		existing.UpdatedAt = now
		s.invalidate(ctx)
		return existing, nil
	}

	candidate, offered := s.candidate(ctx, modelID)
	if !offered && !hasRow {
		return Model{}, fmt.Errorf("%w: %s", ErrNotFound, modelID)
	}
	// Re-checking an already-checked box with no fresh snapshot to write changes nothing —
	// answering from what is stored skips a row rewrite and a full cache rebuild.
	if !offered && slices.Contains(existing.Purposes, purpose) {
		return existing, nil
	}

	row := existing
	row.ModelID = modelID
	if offered {
		row.ProviderSlug = candidate.ProviderSlug
		row.Label = candidate.Label
		row.Vision = candidate.Vision
		row.StructuredOutput = candidate.StructuredOutput
		row.ImageOutput = candidate.ImageOutput
		row.VideoOutput = candidate.VideoOutput
		row.ContextTokens = candidate.ContextTokens
		row.InputUSDPerMillion = candidate.InputUSDPerMillion
		row.OutputUSDPerMillion = candidate.OutputUSDPerMillion
		row.PricingCheckedAt = now.UTC().Format(time.DateOnly)
		row.Listed = true
		row.LastSeenAt = now
	}
	if !purpose.EligibleFor(row) {
		return Model{}, fmt.Errorf("%w: %s for %s", ErrPurposeIneligible, modelID, purpose)
	}
	row.UpdatedAt = now
	if !hasRow {
		row.CreatedAt = now
	}

	if err := s.store.RegisterPurpose(ctx, row, purpose); err != nil {
		return Model{}, fmt.Errorf("register model purpose: %w", err)
	}
	if !slices.Contains(row.Purposes, purpose) {
		row.Purposes = append(append([]Purpose(nil), row.Purposes...), purpose)
		SortPurposes(row.Purposes)
	}
	s.invalidate(ctx)
	return row, nil
}

// stageReasoningOf projects the per-purpose overrides onto the stage keys the llm boundary
// carries. Only a REGISTERED purpose contributes: an effort on a purpose the model no
// longer serves must not reach a stage. A purpose that feeds no stage (the generation
// purposes) contributes nothing, which is why nothing outside the operator screen reads it.
func stageReasoningOf(row Model) map[string]llm.ReasoningEffort {
	if len(row.Reasoning) == 0 {
		return nil
	}
	out := make(map[string]llm.ReasoningEffort, len(row.Reasoning))
	for _, purpose := range row.Purposes {
		stage := purpose.Stage()
		effort := row.Reasoning[purpose]
		if stage == "" || effort == llm.ReasoningUnspecified {
			continue
		}
		out[stage] = effort
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// stagesOf projects purpose registrations onto the user-facing stages the llm boundary
// carries as opaque strings. Generation purposes map to no stage yet, so a model
// registered only to them is invisible to every picker.
//
// The capability gate is re-checked here, not only at registration: an operator refresh
// re-snapshots the flags from the source, and a model that LOST the capability its
// registration was gated on (a vision model that stopped taking images) must stop serving
// that stage at once. The registration row itself is kept — flagging, never auto-retiring,
// is the operator's contract — so the admin tab still shows it to be unchecked.
func stagesOf(row Model) []string {
	stages := make([]string, 0, len(row.Purposes))
	for _, purpose := range row.Purposes {
		stage := purpose.Stage()
		if stage != "" && purpose.EligibleFor(row) && !slices.Contains(stages, stage) {
			stages = append(stages, stage)
		}
	}
	return stages
}

// Update applies a partial curation edit for ONE (model, purpose) — today that is the
// reasoning override; registration has its own write (SetPurpose).
//
// The purpose is required and must be one the model is REGISTERED to: an effort on a purpose
// the model serves to nobody would be a stored decision with no effect, and the control only
// appears once registered. That was a UI rule; it is a server rule now (change 24).
func (s *Service) Update(ctx context.Context, modelID string, patch Patch) (Model, error) {
	if _, err := ParsePurpose(string(patch.Purpose)); err != nil {
		return Model{}, err
	}
	if patch.Reasoning != nil && !patch.Reasoning.Valid() {
		return Model{}, fmt.Errorf("%w: %q", ErrInvalidReasoning, *patch.Reasoning)
	}
	updated, err := s.store.Patch(ctx, modelID, patch, s.now())
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrPurposeNotRegistered) {
			return Model{}, err
		}
		return Model{}, fmt.Errorf("update model: %w", err)
	}
	s.invalidate(ctx)
	return updated, nil
}

// reasoningSpend reads the ledger's aggregate for the purpose's stage through the port, and
// answers with an empty map on any problem: the signal is evidence beside a control, so its
// absence must never be what stops an operator from curating.
func (s *Service) reasoningSpend(ctx context.Context, purpose Purpose) map[string]*ReasoningSpend {
	stage := purpose.Stage()
	if s.spend == nil || stage == "" {
		return nil
	}
	rows, err := s.spend.ReasoningSpendByModel(ctx, stage)
	if err != nil {
		slog.Warn("reasoning spend read failed", "stage", stage, "err", err)
		return nil
	}
	out := make(map[string]*ReasoningSpend, len(rows))
	for _, row := range rows {
		// A model with no recorded call for this stage is absent from the map, so the row
		// renders nothing rather than a zero that reads as a measurement.
		if row.Calls == 0 {
			continue
		}
		out[row.Model] = &ReasoningSpend{
			Calls: row.Calls, ReasoningTokens: row.ReasoningTokens,
			CompletionTokens: row.CompletionTokens,
		}
	}
	return out
}

// candidate looks the model up in the live catalog without forcing a refresh. A failure is
// not fatal here: the caller decides whether the stored row alone is enough.
func (s *Service) candidate(ctx context.Context, modelID string) (Candidate, bool) {
	if s.upstream == nil {
		return Candidate{}, false
	}
	snapshot, err := s.upstream.Fetch(ctx, false)
	if err != nil {
		slog.Warn("model catalog fetch failed", "err", err)
		return Candidate{}, false
	}
	for _, item := range snapshot.Candidates {
		if item.ModelID == modelID {
			return item, true
		}
	}
	return Candidate{}, false
}

// invalidate refreshes the registry's view after a write.
//
// A failure is logged rather than returned: the write is already durable, so reporting it
// as failed would tell the operator the opposite of what happened. The stale window closes
// at the next write, the next Browse, or the next boot.
func (s *Service) invalidate(ctx context.Context) {
	if err := s.Reload(ctx); err != nil {
		slog.Error("model catalog cache reload failed", "err", err)
	}
}

func cmpString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

var _ llm.ModelSource = (*Service)(nil)

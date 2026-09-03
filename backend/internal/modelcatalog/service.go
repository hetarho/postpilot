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
		if !row.Enabled {
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
			Reasoning:           row.Reasoning,
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
func (s *Service) Browse(ctx context.Context, refresh bool) (Browse, error) {
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

	entries := make([]Entry, 0, len(snapshot.Candidates)+len(rows))
	for _, candidate := range snapshot.Candidates {
		entry := Entry{Candidate: candidate, Listed: true}
		if row, ok := curated[candidate.ModelID]; ok {
			entry.Curated, entry.Enabled = true, row.Enabled
			entry.Reasoning = row.Reasoning
			delete(curated, candidate.ModelID)
		}
		entries = append(entries, entry)
	}
	// What is left is curated but unoffered. It keeps the snapshot taken when it was last
	// seen, so the row still reads as a model rather than as an id.
	for _, row := range rows {
		if _, still := curated[row.ModelID]; !still {
			continue
		}
		entries = append(entries, Entry{
			Candidate: Candidate{
				ModelID: row.ModelID, ProviderSlug: row.ProviderSlug, Label: row.Label,
				Vision: row.Vision, StructuredOutput: row.StructuredOutput,
				ContextTokens:      row.ContextTokens,
				InputUSDPerMillion: row.InputUSDPerMillion, OutputUSDPerMillion: row.OutputUSDPerMillion,
			},
			Curated: true, Enabled: row.Enabled,
			Reasoning: row.Reasoning, Listed: row.Listed,
		})
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

// Enable makes a model selectable.
//
// A model still offered upstream is snapshotted from the live entry. One that is not, but
// already has a row, is re-enabled from what is stored — so an operator can undo a disable
// without waiting for the provider's catalog to be reachable.
func (s *Service) Enable(ctx context.Context, modelID string) (Model, error) {
	existing, err := s.store.Get(ctx, modelID)
	hasRow := err == nil
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Model{}, fmt.Errorf("read curated model: %w", err)
	}

	candidate, offered := s.candidate(ctx, modelID)
	if !offered && !hasRow {
		return Model{}, fmt.Errorf("%w: %s", ErrNotFound, modelID)
	}

	now := s.now()
	row := existing
	row.ModelID = modelID
	if offered {
		row.ProviderSlug = candidate.ProviderSlug
		row.Label = candidate.Label
		row.Vision = candidate.Vision
		row.StructuredOutput = candidate.StructuredOutput
		row.ContextTokens = candidate.ContextTokens
		row.InputUSDPerMillion = candidate.InputUSDPerMillion
		row.OutputUSDPerMillion = candidate.OutputUSDPerMillion
		row.PricingCheckedAt = now.UTC().Format(time.DateOnly)
		row.Listed = true
		row.LastSeenAt = now
	}
	row.Enabled = true
	row.UpdatedAt = now
	if !hasRow {
		row.CreatedAt = now
	}

	if err := s.store.Upsert(ctx, row); err != nil {
		return Model{}, fmt.Errorf("enable model: %w", err)
	}
	s.invalidate(ctx)
	return row, nil
}

// Update applies a partial curation edit: enable/disable, or the per-model reasoning
// override.
func (s *Service) Update(ctx context.Context, modelID string, patch Patch) (Model, error) {
	if patch.Reasoning != nil && !patch.Reasoning.Valid() {
		return Model{}, fmt.Errorf("%w: %q", ErrInvalidReasoning, *patch.Reasoning)
	}
	updated, err := s.store.Patch(ctx, modelID, patch, s.now())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Model{}, err
		}
		return Model{}, fmt.Errorf("update model: %w", err)
	}
	s.invalidate(ctx)
	return updated, nil
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

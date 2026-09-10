package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
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
		efforts := make([]llm.ReasoningEffort, 0, len(row.Efforts))
		for _, effort := range row.Efforts {
			efforts = append(efforts, llm.ReasoningEffort(effort))
		}
		model := llm.SourceModel{
			ModelID:               row.ModelID,
			Label:                 row.Label,
			Vision:                row.Vision,
			VideoInput:            row.VideoInput,
			StructuredOutput:      row.StructuredOutput,
			ContextTokens:         row.ContextTokens,
			InputUSDPerMillion:    row.InputUSDPerMillion,
			OutputUSDPerMillion:   row.OutputUSDPerMillion,
			PricingCheckedAt:      row.PricingCheckedAt,
			Reasoning:             stageReasoningOf(row),
			ReasoningEfforts:      efforts,
			ReasoningNativeEffort: row.NativeEffort,
			Stages:                stagesOf(row),
			Levels:                stageLevelsOf(row),
			Delisted:              !row.Listed,
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
		// Read live from the source this pass, so the capability is known by construction —
		// including a `reasons: false` that really means "publishes no reasoning object".
		entry := Entry{Candidate: candidate, Listed: true, ReasoningKnown: true}
		if row, ok := curated[candidate.ModelID]; ok {
			entry.Curated, entry.Purposes = true, row.Purposes
			entry.Reasoning = row.Reasoning[purpose]
			entry.Level = row.Levels[purpose]
			// Drift is derived from the LIVE capability, not the stored snapshot: the point of
			// the warning is that the source's list moved away from what the operator chose.
			// It is a flag only — the override is kept and still sent (MODEL-22); delisting
			// is the one automatic deregistration, and it happens in the store (MODEL-20).
			entry.ReasoningDrifted = candidate.DriftedFrom(entry.Reasoning)
			delete(curated, candidate.ModelID)
		}
		entry.ReasoningSpend = spend[entry.ModelID]
		entries = append(entries, entry)
	}
	// What is left is curated but not in this snapshot. A row a successful read has
	// marked unlisted is not shown at all (MODEL-20): its registrations are already gone,
	// so there is nothing for the operator to do with it, and it comes back as a live
	// unregistered candidate when the source offers it again. Only a
	// still-listed row the snapshot lacks is emitted, which is the degraded path (a failed
	// or empty read served from the stored rows) keeping its last-seen snapshot.
	for _, row := range rows {
		if _, still := curated[row.ModelID]; !still || !row.Listed {
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
	row.UpdatedAt = now
	if !hasRow {
		row.CreatedAt = now
	}
	if offered {
		// The reasoning capability is part of the same upstream snapshot as the flags and the
		// pricing, so a register refreshes it exactly as it refreshes those (change 27).
		row = rowFromCandidate(existing, hasRow, candidate, now)
	}
	if !purpose.EligibleFor(row) {
		return Model{}, fmt.Errorf("%w: %s for %s", ErrPurposeIneligible, modelID, purpose)
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

// stageLevelsOf projects the per-purpose levels onto the stage keys the llm boundary
// carries, on exactly the terms stageReasoningOf uses: only a REGISTERED purpose
// contributes, and a purpose that feeds no stage contributes nothing. The gate is NOT
// re-checked here — a level is display metadata, and a model that lost its capability is
// removed from the stage by stagesOf, which is the one place that decision belongs.
func stageLevelsOf(row Model) map[string]string {
	if len(row.Levels) == 0 {
		return nil
	}
	out := make(map[string]string, len(row.Levels))
	for _, purpose := range row.Purposes {
		stage := purpose.Stage()
		level := row.Levels[purpose]
		if stage == "" || level == "" {
			continue
		}
		out[stage] = string(level)
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

// Update applies a partial curation edit for ONE (model, purpose) — the reasoning override
// and the level; registration has its own write (SetPurpose).
//
// The purpose is required and must be one the model is REGISTERED to: an effort on a purpose
// the model serves to nobody would be a stored decision with no effect, and the control only
// appears once registered. That was a UI rule; it is a server rule now (change 24).
func (s *Service) Update(ctx context.Context, modelID string, patch Patch) (Model, error) {
	if _, err := ParsePurpose(string(patch.Purpose)); err != nil {
		return Model{}, err
	}
	// The enum check stays the FIRST gate: a value that is not an effort at all is refused
	// before anything is read.
	if patch.Reasoning != nil && !patch.Reasoning.Valid() {
		return Model{}, fmt.Errorf("%w: %q", ErrInvalidReasoning, *patch.Reasoning)
	}
	// The level has only the enum gate: it gates nothing downstream (MODEL-58), so there is
	// no model-side rule to check it against the way an effort has one.
	if patch.Level != nil {
		if _, err := ParseLevel(string(*patch.Level)); err != nil {
			return Model{}, err
		}
	}
	// Then the model rule (change 27): an effort outside a model's published list, or `none`
	// on a model that cannot turn reasoning off, is refused. A model whose list the source
	// does not publish keeps accepting all eight — absence is unknown, not "supports
	// nothing". The frontend's option filtering is an affordance; this is the contract.
	//
	// The capability is read before the patch, so a refresh landing in between could in
	// principle accept a value the newest list no longer has. That is the same drift the
	// warning exists for and is deliberately tolerated: a stored override is never rewritten
	// by the source, so the alternative would be a transaction spanning a decision the
	// operator already made.
	if patch.Reasoning != nil {
		capability, err := s.reasoningCapabilityOf(ctx, modelID)
		if err != nil {
			return Model{}, err
		}
		if !capability.AcceptsEffort(*patch.Reasoning) {
			return Model{}, fmt.Errorf("%w: %s does not accept %q", ErrInvalidReasoning, modelID, *patch.Reasoning)
		}
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

// reasoningCapabilityOf answers what this model accepts, preferring the LIVE entry over the
// stored snapshot. The live one is authoritative and always known; the stored one may be a
// row written before this data existed, where "does not reason" and "nobody has looked" are
// the same bytes (see ReasoningCapability.Known).
//
// The read is not held against the write: a refresh landing in between could accept a value
// the newest list no longer has. That is the same drift the row's warning exists for, and it
// is deliberately tolerated — a stored override is never rewritten by the source, so the
// alternative would be a transaction spanning a decision the operator already made.
func (s *Service) reasoningCapabilityOf(ctx context.Context, modelID string) (ReasoningCapability, error) {
	if candidate, offered := s.candidate(ctx, modelID); offered {
		return candidate.ReasoningCapability, nil
	}
	row, err := s.store.Get(ctx, modelID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ReasoningCapability{}, err
		}
		return ReasoningCapability{}, fmt.Errorf("read model before reasoning update: %w", err)
	}
	return row.ReasoningCapability, nil
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
			CompletionTokens:     row.CompletionTokens,
			ReasoningTruncations: row.ReasoningTruncations,
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

// AssignCombo points one estimator combo at the two models that price it (QUOTA-39).
//
// Both must be curated AND registered to the purpose they serve: the estimate quotes what a
// real run of that combo would cost, and a model no stage may run would be quoting a price
// nothing can charge. The pair is written together, so a half-assigned combo is a client
// state rather than a row.
func (s *Service) AssignCombo(ctx context.Context, combo Combo, observeModelID, writeModelID string) error {
	if !combo.Valid() {
		return fmt.Errorf("%w: %s", ErrUnknownCombo, combo)
	}
	if err := s.requireRegistered(ctx, observeModelID, PurposePhotoAnalysis); err != nil {
		return err
	}
	if err := s.requireRegistered(ctx, writeModelID, PurposeWriting); err != nil {
		return err
	}
	assignment := ComboAssignment{Combo: combo, ObserveModelID: observeModelID, WriteModelID: writeModelID}
	if err := s.store.AssignCombo(ctx, assignment, s.now()); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

func (s *Service) requireRegistered(ctx context.Context, modelID string, purpose Purpose) error {
	model, err := s.store.Get(ctx, modelID)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("%w: %s is not registered to %s", ErrComboModelUnusable, modelID, purpose)
	}
	if err != nil {
		return fmt.Errorf("read curated model: %w", err)
	}
	if !slices.Contains(model.Purposes, purpose) {
		return fmt.Errorf("%w: %s is not registered to %s", ErrComboModelUnusable, modelID, purpose)
	}
	return nil
}

// Combos reads the operator's assignments for their own screen, all four in ladder order
// with the unassigned ones carrying empty ids.
func (s *Service) Combos(ctx context.Context) ([]ComboAssignment, error) {
	stored, err := s.store.ListCombos(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[Combo]ComboAssignment, len(stored))
	for _, assignment := range stored {
		byName[assignment.Combo] = assignment
	}
	out := make([]ComboAssignment, 0, len(Combos()))
	for _, combo := range Combos() {
		if found, ok := byName[combo]; ok {
			out = append(out, found)
			continue
		}
		out = append(out, ComboAssignment{Combo: combo})
	}
	return out, nil
}

// ComboRates prices every ASSIGNED combo for a comparison screen.
//
// A combo whose model has since lost its registration, left the catalog or published no
// price is left out rather than priced anyway (QUOTA-39): a comparison that quoted a tier
// nobody can run would be worse than one that shows fewer tiers.
func (s *Service) ComboRates(ctx context.Context) ([]ComboRates, error) {
	assignments, err := s.store.ListCombos(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[Combo]ComboAssignment, len(assignments))
	for _, assignment := range assignments {
		byName[assignment.Combo] = assignment
	}

	out := make([]ComboRates, 0, len(assignments))
	for _, combo := range Combos() {
		assignment, ok := byName[combo]
		if !ok {
			continue
		}
		observe, ok := s.priceable(ctx, assignment.ObserveModelID, PurposePhotoAnalysis)
		if !ok {
			continue
		}
		write, ok := s.priceable(ctx, assignment.WriteModelID, PurposeWriting)
		if !ok {
			continue
		}
		rates, ok := plan.EstimatorRates(pricerFor(observe), pricerFor(write))
		if !ok {
			continue
		}
		out = append(out, ComboRates{
			Combo:        combo,
			ObserveLabel: observe.Label,
			WriteLabel:   write.Label,
			Rates:        rates,
		})
	}
	return out, nil
}

func (s *Service) priceable(ctx context.Context, modelID string, purpose Purpose) (Model, bool) {
	model, err := s.store.Get(ctx, modelID)
	if err != nil || !slices.Contains(model.Purposes, purpose) {
		return Model{}, false
	}
	// A model the provider stopped offering, or one with no published price, cannot be
	// quoted: `listed = 0` is the catalog's own way of saying the run would fail.
	if !model.Listed || model.InputUSDPerMillion == "" || model.OutputUSDPerMillion == "" {
		return Model{}, false
	}
	return model, true
}

// pricerFor turns a curated row's published prices into the pricing function the plan
// package asks for, so the token assumptions stay in plan and the money arithmetic stays in
// llm — neither learns the other's job.
func pricerFor(m Model) plan.Pricer {
	return func(promptTokens, completionTokens int64) (int64, bool) {
		cost := llm.ResolveCost(llm.CostInput{
			PromptTokens:        promptTokens,
			CompletionTokens:    completionTokens,
			InputUSDPerMillion:  m.InputUSDPerMillion,
			OutputUSDPerMillion: m.OutputUSDPerMillion,
		})
		return cost.Microusd, cost.Source == llm.CostEstimated
	}
}

// DocumentPurposePlan is what applying the paste would do to one purpose. Unchanged is
// carried as well as the two deltas so the operator's diff can say "already in place"
// instead of leaving a registration unexplained.
type DocumentPurposePlan struct {
	Purpose    Purpose
	Register   []string
	Deregister []string
	Unchanged  []string
	// Relevel is the registrations the document keeps but re-grades, including to and from
	// unset. It is its own list rather than a footnote on Unchanged because a curator's
	// list pasted without levels clears every one of them, and the operator has to see that
	// before confirming (MODEL-54, MODEL-59).
	Relevel []LevelChange
}

// LevelChange is one registration's grade moving. Either side may be "" — that is what
// setting a first level and clearing one look like.
type LevelChange struct {
	ModelID string
	From    Level
	To      Level
}

// DocumentPlan is the answer to both preview and apply. Applied is false whenever anything
// was refused, and Issues then holds every reason: the two calls report identically, so a
// rejection discovered at apply time (the catalog moved between the calls) renders exactly
// like one discovered at preview.
type DocumentPlan struct {
	Purposes []DocumentPurposePlan
	Issues   []DocumentIssue
	// FetchError is set when the provider catalog could not be read. The document path
	// needs the live snapshot to create a row for an id nobody has curated yet, so an
	// unreadable catalog refuses the whole paste rather than degrading to stored rows the
	// way Browse does.
	FetchError string
	Applied    bool
}

// PreviewDocument parses, validates and reports — and writes nothing, including the
// availability bookkeeping a Browse would do (MODEL-54).
func (s *Service) PreviewDocument(ctx context.Context, text string) (DocumentPlan, error) {
	plan, _, err := s.planDocument(ctx, text)
	return plan, err
}

// ApplyDocument re-parses and re-validates the same text from scratch and then applies it
// in one transaction. It accepts no preview token by design: the catalog moves between the
// two calls, and a token would let a stale diff be committed against a catalog that no
// longer matches it.
func (s *Service) ApplyDocument(ctx context.Context, text string) (DocumentPlan, error) {
	plan, writes, err := s.planDocument(ctx, text)
	if err != nil {
		return DocumentPlan{}, err
	}
	if plan.FetchError != "" || len(plan.Issues) > 0 {
		return plan, nil
	}
	if len(writes) > 0 {
		if err := s.store.SyncPurposes(ctx, writes, s.now()); err != nil {
			return DocumentPlan{}, fmt.Errorf("sync catalog document: %w", err)
		}
		s.invalidate(ctx)
	}
	plan.Applied = true
	return plan, nil
}

// ExportDocument renders every purpose's current registrations in the same protocol, so the
// operator edits what is actually there instead of writing a document from memory
// (MODEL-55).
func (s *Service) ExportDocument(ctx context.Context) (string, error) {
	rows, err := s.store.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list curated models: %w", err)
	}
	registrations := make(map[Purpose][]DocumentEntry, len(Purposes))
	for _, row := range rows {
		for _, purpose := range row.Purposes {
			registrations[purpose] = append(registrations[purpose], DocumentEntry{
				ModelID: row.ModelID, Level: row.Levels[purpose],
			})
		}
	}
	return RenderDocument(registrations), nil
}

// planDocument is the one path preview and apply share, so the two can never disagree about
// what a document means. It returns the plan the operator sees and the writes that would
// realize it; the writes are empty whenever anything was refused.
func (s *Service) planDocument(ctx context.Context, text string) (DocumentPlan, []PurposeWrite, error) {
	doc, issues := ParseDocument(text)

	// The live read comes before anything else that could fail, and a failure ends the call:
	// an id nobody has curated has no row to register, and only the snapshot can make one.
	var snapshot Snapshot
	if s.upstream == nil {
		return DocumentPlan{Issues: issues, FetchError: "no upstream catalog is configured"}, nil, nil
	}
	found, err := s.upstream.Fetch(ctx, false)
	if err != nil {
		slog.Warn("model catalog fetch failed", "err", err)
		return DocumentPlan{Issues: issues, FetchError: "the provider catalog could not be read"}, nil, nil
	}
	snapshot = found

	rows, err := s.store.List(ctx)
	if err != nil {
		return DocumentPlan{}, nil, fmt.Errorf("list curated models: %w", err)
	}
	curated := make(map[string]Model, len(rows))
	for _, row := range rows {
		curated[row.ModelID] = row
	}
	offered := make(map[string]Candidate, len(snapshot.Candidates))
	for _, candidate := range snapshot.Candidates {
		offered[candidate.ModelID] = candidate
	}

	var (
		plan   = DocumentPlan{Issues: issues}
		writes []PurposeWrite
		now    = s.now()
	)
	for _, section := range doc.Sections {
		wanted := make(map[string]bool, len(section.Entries))
		purposePlan := DocumentPurposePlan{Purpose: section.Purpose}

		for _, entry := range section.Entries {
			modelID, line := entry.ModelID, entry.Line
			existing, hasRow := curated[modelID]
			candidate, isOffered := offered[modelID]
			if !isOffered {
				// The read succeeded, so an id it does not carry is genuinely not on offer.
				// Which of the two causes it is decides what the operator has to do: curate
				// a different model, or accept that one they already had is gone (MODEL-20).
				cause := IssueUnknownModel
				if hasRow {
					cause = IssueUnlisted
				}
				plan.Issues = append(plan.Issues, DocumentIssue{Line: line, Text: modelID, Cause: cause})
				continue
			}
			row := rowFromCandidate(existing, hasRow, candidate, now)
			if !section.Purpose.EligibleFor(row) {
				plan.Issues = append(plan.Issues, DocumentIssue{Line: line, Text: modelID, Cause: IssueIneligible})
				continue
			}
			wanted[modelID] = true
			// Every listed id is written, registered or not: the write also carries the
			// level, and setting it to what it already is costs nothing while leaving the
			// document as the single statement of the purpose's state (MODEL-52).
			writes = append(writes, PurposeWrite{
				Model: row, Purpose: section.Purpose, Register: true, Level: entry.Level,
			})
			if hasRow && slices.Contains(existing.Purposes, section.Purpose) {
				if current := existing.Levels[section.Purpose]; current != entry.Level {
					purposePlan.Relevel = append(purposePlan.Relevel, LevelChange{
						ModelID: modelID, From: current, To: entry.Level,
					})
					continue
				}
				purposePlan.Unchanged = append(purposePlan.Unchanged, modelID)
				continue
			}
			// A new registration is a Register and nothing else: its level is part of
			// arriving, not a change to something that was already there.
			purposePlan.Register = append(purposePlan.Register, modelID)
		}

		// Whatever holds the purpose today and the section does not name is dropped: the
		// section is the purpose's complete membership (MODEL-52).
		for _, row := range rows {
			if wanted[row.ModelID] || !slices.Contains(row.Purposes, section.Purpose) {
				continue
			}
			purposePlan.Deregister = append(purposePlan.Deregister, row.ModelID)
			writes = append(writes, PurposeWrite{
				Model: Model{ModelID: row.ModelID}, Purpose: section.Purpose,
			})
		}

		slices.Sort(purposePlan.Register)
		slices.Sort(purposePlan.Deregister)
		slices.Sort(purposePlan.Unchanged)
		slices.SortFunc(purposePlan.Relevel, func(a, b LevelChange) int {
			return strings.Compare(a.ModelID, b.ModelID)
		})
		plan.Purposes = append(plan.Purposes, purposePlan)
	}

	if len(plan.Issues) > 0 {
		// Nothing is applied when anything was refused, so the writes are not handed back at
		// all rather than left for a caller to remember to check.
		return plan, nil, nil
	}
	return plan, writes, nil
}

// rowFromCandidate is the row a registration would write: the stored curation, if any, with
// the live snapshot laid over it. It is the same overlay SetPurpose performs, kept in one
// place so the two paths cannot drift into writing different rows for the same model.
func rowFromCandidate(existing Model, hasRow bool, candidate Candidate, now time.Time) Model {
	row := existing
	row.ModelID = candidate.ModelID
	row.ProviderSlug = candidate.ProviderSlug
	row.Label = candidate.Label
	row.Vision = candidate.Vision
	row.StructuredOutput = candidate.StructuredOutput
	row.ImageOutput = candidate.ImageOutput
	row.VideoOutput = candidate.VideoOutput
	row.VideoInput = candidate.VideoInput
	row.ContextTokens = candidate.ContextTokens
	row.InputUSDPerMillion = candidate.InputUSDPerMillion
	row.OutputUSDPerMillion = candidate.OutputUSDPerMillion
	row.ReasoningCapability = candidate.ReasoningCapability
	row.PricingCheckedAt = now.UTC().Format(time.DateOnly)
	row.Listed = true
	row.LastSeenAt = now
	row.UpdatedAt = now
	if !hasRow {
		row.CreatedAt = now
	}
	return row
}

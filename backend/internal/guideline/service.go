package guideline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	store      Store
	templates  TemplateDirectory
	limits     Limits
	maxPending int
	now        func() time.Time
	newID      func() string
}

func NewService(store Store, limits Limits, maxPendingCandidates int) *Service {
	if !limits.valid() {
		panic("guideline: limits must be positive")
	}
	if maxPendingCandidates <= 0 {
		panic("guideline: the pending candidate bound must be positive")
	}
	return &Service{store: store, limits: limits, maxPending: maxPendingCandidates, now: time.Now, newID: newID}
}

// SetTemplateDirectory wires the template context's directory. Without it a scoped guideline
// cannot be validated or named, so scope writes are refused rather than accepted blind.
func (s *Service) SetTemplateDirectory(directory TemplateDirectory) { s.templates = directory }

func (s *Service) Limits() Limits { return s.limits }

// List returns the account's guidelines in injection order with template names projected, so
// the management screen shows exactly what the writer will be given, in that order.
func (s *Service) List(ctx context.Context, userID string) ([]Guideline, error) {
	guidelines, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list guidelines: %w", err)
	}
	if err := s.project(ctx, userID, guidelines); err != nil {
		return nil, err
	}
	return guidelines, nil
}

// Create is also the approval path for a candidate. There is deliberately no Approve
// procedure: every field rule, the text bound and the account cap already live here, and a
// second entry point would have to restate all of them to stay in agreement.
//
// fromCandidateID is set only when the user edited the candidate's text before approving,
// so the row can no longer be found by text. The text match happens either way, which is
// what marks the candidate an on-the-spot 지침으로 저장 recorded.
func (s *Service) Create(ctx context.Context, userID, text string, scope Scope, templateIDs []string, fromCandidateID string) (Guideline, error) {
	text, err := s.validText(text)
	if err != nil {
		return Guideline{}, err
	}
	ids, err := s.validScope(ctx, userID, scope, templateIDs)
	if err != nil {
		return Guideline{}, err
	}
	now := s.now()
	created := Guideline{
		ID: s.newID(), UserID: userID, Text: text, Scope: scope, TemplateIDs: ids,
		CreatedAt: now, UpdatedAt: now,
	}
	approval := CandidateApproval{ID: strings.TrimSpace(fromCandidateID), Text: text}
	if err := s.store.Insert(ctx, created, s.limits.MaxPerAccount, approval); err != nil {
		return Guideline{}, err
	}
	return s.projectOne(ctx, userID, created)
}

// RecordCandidate stores one completed revision's instruction verbatim, at the CANDIDATE
// bound (500) rather than the guideline bound (300): the guideline bound is enforced at
// approval, where the user can shorten the text. Nothing here rewrites, summarizes,
// normalizes or generalizes the instruction, and no provider is called — a candidate is a
// receipt for something the user wrote ([I4] stays entirely with voice).
//
// A skip is an ordinary outcome, not an error: the text is already a guideline, the user
// already ruled on it, or the pending queue is full.
func (s *Service) RecordCandidate(ctx context.Context, userID, postSlug, instruction string) error {
	text, err := validCandidateText(instruction)
	if err != nil {
		return err
	}
	now := s.now()
	candidate := Candidate{
		ID: s.newID(), UserID: userID, Text: text, PostSlug: strings.TrimSpace(postSlug),
		Status: CandidateStatusPending, Occurrences: 1, FirstSeenAt: now, LastSeenAt: now,
	}
	if _, err := s.store.RecordCandidate(ctx, candidate, s.maxPending); err != nil {
		return fmt.Errorf("record guideline candidate: %w", err)
	}
	return nil
}

// ListCandidates returns the pending candidates in review order plus whether the queue is at
// its bound. queueFull is derived here, from the same count recording compares, so the screen
// and the recording path cannot disagree — and the client never owns a copy of the bound.
func (s *Service) ListCandidates(ctx context.Context, userID string) (candidates []Candidate, queueFull bool, err error) {
	candidates, pending, err := s.store.ListPendingCandidates(ctx, userID)
	if err != nil {
		return nil, false, fmt.Errorf("list guideline candidates: %w", err)
	}
	return candidates, pending >= s.maxPending, nil
}

// DismissCandidate marks the row rather than deleting it: the dismissed row is what keeps the
// same instruction from being recorded again by a later revision.
func (s *Service) DismissCandidate(ctx context.Context, userID, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrCandidateNotFound
	}
	return s.store.SetCandidateStatus(ctx, userID, id, CandidateStatusDismissed)
}

// DetachCandidatePost drops the post link from every candidate that named one post, called
// when that post is deleted. The text is untouched: nothing references a candidate's origin,
// so a candidate without a link is still exactly as reviewable as one with it.
func (s *Service) DetachCandidatePost(ctx context.Context, userID, postSlug string) error {
	if strings.TrimSpace(postSlug) == "" {
		return nil
	}
	return s.store.DropCandidatePostSlug(ctx, userID, postSlug)
}

// Update applies only what the request carried. The validation runs per present part, so a
// text edit can never be refused for a scope it did not send — and never rewrites one either.
func (s *Service) Update(ctx context.Context, userID, id string, patch Patch) (Guideline, error) {
	if strings.TrimSpace(id) == "" {
		return Guideline{}, ErrNotFound
	}
	if patch.empty() {
		found, err := s.store.Get(ctx, userID, id)
		if err != nil {
			return Guideline{}, err
		}
		return s.projectOne(ctx, userID, found)
	}
	if patch.Text != nil {
		text, err := s.validText(*patch.Text)
		if err != nil {
			return Guideline{}, err
		}
		patch.Text = &text
	}
	if patch.Scope != nil {
		ids, err := s.validScope(ctx, userID, patch.Scope.Scope, patch.Scope.TemplateIDs)
		if err != nil {
			return Guideline{}, err
		}
		patch.Scope = &ScopePatch{Scope: patch.Scope.Scope, TemplateIDs: ids}
	}
	updated, err := s.store.Update(ctx, userID, id, patch, s.now())
	if err != nil {
		return Guideline{}, err
	}
	return s.projectOne(ctx, userID, updated)
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrNotFound
	}
	return s.store.Delete(ctx, userID, id)
}

// ForPrompt is this context's published behavior for prompt builders: the ordered texts that
// apply to one post, resolved from the post's CURRENT template. Absence is not an error — a
// prompt with no guidelines is a valid prompt — so the caller gets an empty slice.
//
// templateID is a pointer because "the post has no template" and "the post has template X" are
// different questions, and the first must not be spelled as the empty-string id of the second.
func (s *Service) ForPrompt(ctx context.Context, userID string, templateID *string) ([]string, error) {
	scoped := ""
	if templateID != nil {
		scoped = strings.TrimSpace(*templateID)
	}
	texts, err := s.store.ApplicableTexts(ctx, userID, scoped)
	if err != nil {
		return nil, fmt.Errorf("resolve applicable guidelines: %w", err)
	}
	return texts, nil
}

func (s *Service) projectOne(ctx context.Context, userID string, g Guideline) (Guideline, error) {
	one := []Guideline{g}
	if err := s.project(ctx, userID, one); err != nil {
		return Guideline{}, err
	}
	return one[0], nil
}

// project fills the template-name projection for the given guidelines. A scoped id with no
// directory entry is dropped rather than shown as a blank chip: it means the template was
// deleted between the link read and this read, which is the orphaned-scope state.
func (s *Service) project(ctx context.Context, userID string, guidelines []Guideline) error {
	needed := false
	for _, g := range guidelines {
		if len(g.TemplateIDs) > 0 {
			needed = true
			break
		}
	}
	if !needed {
		return nil
	}
	names, err := s.directory(ctx, userID)
	if err != nil {
		return err
	}
	for i := range guidelines {
		refs := make([]TemplateRef, 0, len(guidelines[i].TemplateIDs))
		for _, id := range guidelines[i].TemplateIDs {
			if name, ok := names[id]; ok {
				refs = append(refs, TemplateRef{ID: id, Name: name})
			}
		}
		// By name, so the chips of one guideline read in a stable order the user can predict.
		sort.Slice(refs, func(a, b int) bool {
			if refs[a].Name == refs[b].Name {
				return refs[a].ID < refs[b].ID
			}
			return refs[a].Name < refs[b].Name
		})
		guidelines[i].Templates = refs
	}
	return nil
}

func (s *Service) directory(ctx context.Context, userID string) (map[string]string, error) {
	if s.templates == nil {
		return nil, fmt.Errorf("guideline: template directory is not wired")
	}
	templates, err := s.templates.Templates(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load template directory: %w", err)
	}
	names := make(map[string]string, len(templates))
	for _, p := range templates {
		names[p.ID] = p.Name
	}
	return names, nil
}

func (s *Service) validText(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrInvalidText
	}
	if chars := utf8.RuneCountInString(trimmed); chars > s.limits.TextMaxChars {
		return "", &TextTooLongError{Chars: chars, Max: s.limits.TextMaxChars}
	}
	return trimmed, nil
}

// validScope collapses duplicate ids and proves every remaining one is an owned template. A
// `templates` scope must name at least one at creation and on every scope update: only a
// template deletion may leave the set empty (plan 16 invariant 2).
func (s *Service) validScope(ctx context.Context, userID string, scope Scope, templateIDs []string) ([]string, error) {
	if !scope.Valid() {
		return nil, ErrScopeShape
	}
	unique := make([]string, 0, len(templateIDs))
	seen := make(map[string]struct{}, len(templateIDs))
	for _, raw := range templateIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, ErrTemplateNotFound
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if scope == ScopeGlobal {
		if len(unique) > 0 {
			return nil, ErrScopeShape
		}
		return nil, nil
	}
	if len(unique) == 0 {
		return nil, ErrScopeShape
	}
	names, err := s.directory(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, id := range unique {
		if _, ok := names[id]; !ok {
			return nil, ErrTemplateNotFound
		}
	}
	return unique, nil
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("guideline: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

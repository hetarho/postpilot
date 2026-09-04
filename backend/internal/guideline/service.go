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
	store     Store
	templates TemplateDirectory
	limits    Limits
	now       func() time.Time
	newID     func() string
}

func NewService(store Store, limits Limits) *Service {
	if !limits.valid() {
		panic("guideline: limits must be positive")
	}
	return &Service{store: store, limits: limits, now: time.Now, newID: newID}
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

func (s *Service) Create(ctx context.Context, userID, text string, scope Scope, templateIDs []string) (Guideline, error) {
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
	if err := s.store.Insert(ctx, created, s.limits.MaxPerAccount); err != nil {
		return Guideline{}, err
	}
	return s.projectOne(ctx, userID, created)
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

package guideline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	store      Store
	templates  TemplateDirectory
	fields     FieldDirectory
	limits     Limits
	maxPending int
	now        func() time.Time
	newID      func() string
}

// NewService takes the field directory as a constructor argument rather than a setter: every
// fields scope and every preset write needs it, so a service without it is a wiring error
// (ARCH-40).
func NewService(store Store, fields FieldDirectory, limits Limits, maxPendingCandidates int) *Service {
	if fields == nil {
		panic("guideline: a field directory is required")
	}
	if !limits.valid() {
		panic("guideline: limits must be positive")
	}
	if maxPendingCandidates <= 0 {
		panic("guideline: the pending candidate bound must be positive")
	}
	return &Service{store: store, fields: fields, limits: limits, maxPending: maxPendingCandidates, now: time.Now, newID: newID}
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
//
// The scope is one value — a kind with both sets — so a templates set and a 분야 set cannot be
// passed in each other's place.
func (s *Service) Create(ctx context.Context, userID, text string, scope ScopePatch, fromCandidateID string) (Guideline, error) {
	text, err := s.validText(text)
	if err != nil {
		return Guideline{}, err
	}
	valid, err := s.validScope(ctx, userID, scope)
	if err != nil {
		return Guideline{}, err
	}
	now := s.now()
	created := Guideline{
		ID: s.newID(), UserID: userID, Text: text, Scope: valid.Scope, TemplateIDs: valid.TemplateIDs,
		Fields: valid.Fields, CreatedAt: now, UpdatedAt: now,
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
		// The normalized patch carries the other kind's set as nil, so a rescope between
		// templates and fields can never leave a link of the kind it left.
		valid, err := s.validScope(ctx, userID, *patch.Scope)
		if err != nil {
			return Guideline{}, err
		}
		patch.Scope = &valid
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

// Preset is the account's 상위 노출 단어 사용 state; an account that never touched it reads as off
// with no 분야 (GUIDE-34).
func (s *Service) Preset(ctx context.Context, userID string) (Preset, error) {
	preset, err := s.store.Preset(ctx, userID)
	if err != nil {
		return Preset{}, fmt.Errorf("read guideline preset: %w", err)
	}
	return preset, nil
}

// UpdatePreset is the owner's switch and 분야 set for the preset, a presence patch. A present
// set replaces the whole set, an empty one included (GUIDE-38), and every 분야 in it is proved
// before anything is written. It spends no cap, checks no text and approves no candidate
// (GUIDE-39), and nothing but the owner's procedure calls it (GUIDE-33).
func (s *Service) UpdatePreset(ctx context.Context, userID string, patch PresetPatch) (Preset, error) {
	if patch.Enabled == nil && patch.Fields == nil {
		return s.Preset(ctx, userID)
	}
	normalized := PresetPatch{Enabled: patch.Enabled}
	if patch.Fields != nil {
		fields, err := collapse(*patch.Fields, ErrFieldNotFound)
		if err != nil {
			return Preset{}, err
		}
		if err := s.knownFields(fields); err != nil {
			return Preset{}, err
		}
		normalized.Fields = &fields
	}
	preset, err := s.store.UpdatePreset(ctx, userID, normalized, s.now())
	if err != nil {
		return Preset{}, fmt.Errorf("update guideline preset: %w", err)
	}
	return preset, nil
}

// ForPrompt is this context's published behavior for prompt builders: the ordered texts that
// apply to one post, resolved from the post's CURRENT template and 분야 — global, then template,
// then 분야 (GUIDE-14) — with the preset's line last where it applies. Absence is not an error:
// a prompt with no guidelines is a valid prompt.
//
// templateID and field are pointers because "the post has none" and "the post has X" are
// different questions, and the first must not be spelled as the empty-string id of the second.
// A revision never carries the preset line (GEN-57): it rewrites the owner's own text, and the
// phrase list the line binds is not part of a revision.
func (s *Service) ForPrompt(ctx context.Context, userID string, templateID, field *string, forRevision bool) ([]string, error) {
	scoped := trimmed(templateID)
	blogField := trimmed(field)
	texts, err := s.store.ApplicableTexts(ctx, userID, scoped, blogField)
	if err != nil {
		return nil, fmt.Errorf("resolve applicable guidelines: %w", err)
	}
	// The preset is only ever read for a generation of a post that has a 분야, so a post without
	// one never pays for it and never receives it (GUIDE-17, GUIDE-29).
	if forRevision || blogField == "" {
		return texts, nil
	}
	preset, err := s.store.Preset(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read guideline preset: %w", err)
	}
	// Not deduplicated against an owner line with the same text: the two are independent
	// (GUIDE-39). Not gated on the 분야's phrase list either; the phrase freeze decides that.
	if preset.Enabled && slices.Contains(preset.Fields, blogField) {
		texts = append(texts, PresetText)
	}
	return texts, nil
}

func trimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
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

// validScope normalizes a scope and proves it: the kind, then both sets trimmed and collapsed
// in first-seen order, then the shape, then that every id exists. A `templates` scope must name
// at least one template and a `fields` scope at least one 분야, at creation and on every scope
// update: only a template deletion may leave a templates set empty (plan 16 invariant 2). The
// normalized patch carries the other kind's set as nil.
func (s *Service) validScope(ctx context.Context, userID string, patch ScopePatch) (ScopePatch, error) {
	if !patch.Scope.Valid() {
		return ScopePatch{}, ErrScopeShape
	}
	templates, err := collapse(patch.TemplateIDs, ErrTemplateNotFound)
	if err != nil {
		return ScopePatch{}, err
	}
	fields, err := collapse(patch.Fields, ErrFieldNotFound)
	if err != nil {
		return ScopePatch{}, err
	}
	switch patch.Scope {
	case ScopeGlobal:
		if len(templates) > 0 || len(fields) > 0 {
			return ScopePatch{}, ErrScopeShape
		}
		return ScopePatch{Scope: ScopeGlobal}, nil
	case ScopeTemplates:
		if len(fields) > 0 || len(templates) == 0 {
			return ScopePatch{}, ErrScopeShape
		}
		names, err := s.directory(ctx, userID)
		if err != nil {
			return ScopePatch{}, err
		}
		for _, id := range templates {
			if _, ok := names[id]; !ok {
				return ScopePatch{}, ErrTemplateNotFound
			}
		}
		return ScopePatch{Scope: ScopeTemplates, TemplateIDs: templates}, nil
	default: // ScopeFields: Valid admits no fourth kind.
		if len(templates) > 0 || len(fields) == 0 {
			return ScopePatch{}, ErrScopeShape
		}
		if err := s.knownFields(fields); err != nil {
			return ScopePatch{}, err
		}
		return ScopePatch{Scope: ScopeFields, Fields: fields}, nil
	}
}

// collapse trims each id and keeps the first of every duplicate, in order. A blank id is the
// given refusal: an id that names nothing is a request for nothing.
func collapse(values []string, blank error) ([]string, error) {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, blank
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique, nil
}

// knownFields proves every 분야 is on the product's list.
func (s *Service) knownFields(fields []string) error {
	for _, id := range fields {
		if !s.fields.Known(id) {
			return ErrFieldNotFound
		}
	}
	return nil
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("guideline: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

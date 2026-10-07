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
	store          Store
	templates      TemplateDirectory
	videoTemplates VideoTemplateDirectory
	fields         FieldDirectory
	limits         Limits
	maxPending     int
	now            func() time.Time
	newID          func() string
}

// NewService takes the field directory as a constructor argument rather than a setter: every
// fields scope needs it, so a service without it is a wiring error (ARCH-40).
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

// SetVideoTemplateDirectory wires the clip context's video templates, the scope of a clip
// guideline; without it a clip guideline scoped to video templates is refused like a post one
// without its directory.
func (s *Service) SetVideoTemplateDirectory(directory VideoTemplateDirectory) {
	s.videoTemplates = directory
}

func (s *Service) Limits() Limits { return s.limits }

// List returns the account's guidelines of one kind in injection order with template names
// projected, so the management screen shows exactly what the writer will be given, in that order.
func (s *Service) List(ctx context.Context, userID string, kind Kind) ([]Guideline, error) {
	if !kind.Valid() {
		kind = KindPost
	}
	guidelines, err := s.store.List(ctx, userID, kind)
	if err != nil {
		return nil, fmt.Errorf("list guidelines: %w", err)
	}
	if err := s.project(ctx, userID, kind, guidelines); err != nil {
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
// passed in each other's place. kind is the guideline's for good; an approval creates one of its
// candidate's kind, and a candidate of another kind reads as missing (GUIDE-11).
func (s *Service) Create(ctx context.Context, userID string, kind Kind, title, text string, scope ScopePatch, fromCandidateID string) (Guideline, error) {
	if !kind.Valid() {
		kind = KindPost
	}
	title, err := s.validTitle(title)
	if err != nil {
		return Guideline{}, err
	}
	text, err = s.validText(text)
	if err != nil {
		return Guideline{}, err
	}
	valid, err := s.validScope(ctx, userID, kind, scope)
	if err != nil {
		return Guideline{}, err
	}
	now := s.now()
	created := Guideline{
		ID: s.newID(), UserID: userID, Kind: kind, Title: title, Text: text, Scope: valid.Scope, TemplateIDs: valid.TemplateIDs,
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
//
// kind decides what source names: a post revision's post slug, or a clip revision's project id.
func (s *Service) RecordCandidate(ctx context.Context, userID string, kind Kind, source, instruction string) error {
	if !kind.Valid() {
		return fmt.Errorf("record guideline candidate: unknown kind %q", kind)
	}
	text, err := validCandidateText(instruction)
	if err != nil {
		return err
	}
	now := s.now()
	candidate := Candidate{
		ID: s.newID(), UserID: userID, Kind: kind, Text: text,
		Status: CandidateStatusPending, Occurrences: 1, FirstSeenAt: now, LastSeenAt: now,
	}
	if kind == KindClip {
		candidate.ClipID = strings.TrimSpace(source)
	} else {
		candidate.PostSlug = strings.TrimSpace(source)
	}
	if _, err := s.store.RecordCandidate(ctx, candidate, s.maxPending); err != nil {
		return fmt.Errorf("record guideline candidate: %w", err)
	}
	return nil
}

// ListCandidates returns the pending candidates in review order plus whether the queue is at
// its bound. queueFull is derived here, from the same count recording compares, so the screen
// and the recording path cannot disagree — and the client never owns a copy of the bound.
func (s *Service) ListCandidates(ctx context.Context, userID string, kind Kind) (candidates []Candidate, queueFull bool, err error) {
	if !kind.Valid() {
		kind = KindPost
	}
	candidates, pending, err := s.store.ListPendingCandidates(ctx, userID, kind)
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

// DetachCandidateClip drops the clip project link from every candidate that named one project,
// called once that project is deleted (GUIDE-13).
func (s *Service) DetachCandidateClip(ctx context.Context, userID, clipID string) error {
	if strings.TrimSpace(clipID) == "" {
		return nil
	}
	return s.store.DropCandidateClipID(ctx, userID, clipID)
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
	// The kind decides what a scope may name, so it is read first; it never changes.
	current, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Guideline{}, err
	}
	if patch.Title != nil {
		title, err := s.validTitle(*patch.Title)
		if err != nil {
			return Guideline{}, err
		}
		patch.Title = &title
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
		valid, err := s.validScope(ctx, userID, current.Kind, *patch.Scope)
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

// ForPrompt is this context's published behavior for prompt builders: the texts one run is
// given (GUIDE-14). First the enabled 기본 지침 of the kind, in the product's order and in the
// target language — a Korean-target-only one reaching a Korean target alone, a memories-only
// one reaching a run that carries memories alone — then the owner's texts that apply to the
// post, resolved from its CURRENT template and 분야: global, then template, then 분야, each by
// created_at then id. Absence is not an error: a prompt with no guidelines is a valid prompt.
//
// withMemories says whether the run's prompt carries a [기억] section — the caller froze its
// memories first and knows (GEN-73); a revision and a clip never carry one (MEM-22).
//
// A clip's owner texts are its kind's: global, then those linked to its video template —
// templateID is then the video template's id and field is unused.
//
// templateID and field are pointers because "the post has none" and "the post has X" are
// different questions, and the first must not be spelled as the empty-string id of the second.
func (s *Service) ForPrompt(ctx context.Context, userID string, kind Kind, templateID, field *string, target Language, withMemories bool) (PromptGuidelines, error) {
	defaults, err := s.Defaults(ctx, userID, kind)
	if err != nil {
		return PromptGuidelines{}, err
	}
	var out PromptGuidelines
	for _, d := range defaults {
		if !d.Enabled || (d.Default.MemoriesOnly && !withMemories) {
			continue
		}
		if text, ok := d.Default.Text(target); ok {
			out.Defaults = append(out.Defaults, text)
		}
	}
	if kind == KindClip {
		texts, err := s.store.ClipApplicableTexts(ctx, userID, trimmed(templateID))
		if err != nil {
			return PromptGuidelines{}, fmt.Errorf("resolve applicable clip guidelines: %w", err)
		}
		out.Owner = texts
		return out, nil
	}
	texts, err := s.store.ApplicableTexts(ctx, userID, trimmed(templateID), trimmed(field))
	if err != nil {
		return PromptGuidelines{}, fmt.Errorf("resolve applicable guidelines: %w", err)
	}
	out.Owner = texts
	return out, nil
}

// Defaults is every 기본 지침 of one kind with the account's switch, in the product's order. A
// stored key the product no longer carries is ignored (GUIDE-43).
func (s *Service) Defaults(ctx context.Context, userID string, kind Kind) ([]DefaultState, error) {
	if !kind.Valid() {
		return nil, ErrDefaultNotFound
	}
	off, err := s.store.DefaultsOff(ctx, userID, kind)
	if err != nil {
		return nil, err
	}
	switchedOff := make(map[string]bool, len(off))
	for _, key := range off {
		switchedOff[key] = true
	}
	registry := Defaults(kind)
	out := make([]DefaultState, 0, len(registry))
	for _, d := range registry {
		out = append(out, DefaultState{Default: d, Enabled: !switchedOff[d.Key]})
	}
	return out, nil
}

// SetDefaultEnabled switches one 기본 지침 on or off for the account and kind, idempotently. An
// unknown key is ErrDefaultNotFound and writes nothing.
func (s *Service) SetDefaultEnabled(ctx context.Context, userID string, kind Kind, key string, enabled bool) (DefaultState, error) {
	d, ok := DefaultFor(kind, key)
	if !kind.Valid() || !ok {
		return DefaultState{}, ErrDefaultNotFound
	}
	if err := s.store.SetDefaultOff(ctx, userID, kind, key, !enabled, s.now()); err != nil {
		return DefaultState{}, err
	}
	return DefaultState{Default: d, Enabled: enabled}, nil
}

func trimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (s *Service) projectOne(ctx context.Context, userID string, g Guideline) (Guideline, error) {
	one := []Guideline{g}
	if err := s.project(ctx, userID, g.Kind, one); err != nil {
		return Guideline{}, err
	}
	return one[0], nil
}

// project fills the template-name projection for the given guidelines. A scoped id with no
// directory entry is dropped rather than shown as a blank chip: it means the template was
// deleted between the link read and this read, which is the orphaned-scope state.
func (s *Service) project(ctx context.Context, userID string, kind Kind, guidelines []Guideline) error {
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
	names, err := s.directory(ctx, userID, kind)
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

// directory is the kind's template names by id: a post's templates, or a clip's video templates.
func (s *Service) directory(ctx context.Context, userID string, kind Kind) (map[string]string, error) {
	var templates []TemplateRef
	var err error
	if kind == KindClip {
		if s.videoTemplates == nil {
			return nil, fmt.Errorf("guideline: video template directory is not wired")
		}
		templates, err = s.videoTemplates.VideoTemplates(ctx, userID)
	} else {
		if s.templates == nil {
			return nil, fmt.Errorf("guideline: template directory is not wired")
		}
		templates, err = s.templates.Templates(ctx, userID)
	}
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

// validTitle trims a title and bounds it (GUIDE-46). Empty is allowed and means none; a title
// is never unique, because it is only the list's name for a rule.
func (s *Service) validTitle(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if chars := utf8.RuneCountInString(trimmed); chars > s.limits.TitleMaxChars {
		return "", &TitleTooLongError{Chars: chars, Max: s.limits.TitleMaxChars}
	}
	return trimmed, nil
}

// validScope normalizes a scope and proves it: the kind, then both sets trimmed and collapsed
// in first-seen order, then the shape, then that every id exists. A `templates` scope must name
// at least one template and a `fields` scope at least one 분야, at creation and on every scope
// update: only a template deletion may leave a templates set empty (GUIDE-5). The
// normalized patch carries the other kind's set as nil.
//
// A clip guideline's templates are video templates, and it has no 분야 scope (GUIDE-5).
func (s *Service) validScope(ctx context.Context, userID string, kind Kind, patch ScopePatch) (ScopePatch, error) {
	valid, err := validScopeShape(kind, patch)
	if err != nil {
		return ScopePatch{}, err
	}
	if valid.Scope == ScopeTemplates {
		names, err := s.directory(ctx, userID, kind)
		if err != nil {
			return ScopePatch{}, err
		}
		for _, id := range valid.TemplateIDs {
			if _, ok := names[id]; !ok {
				return ScopePatch{}, ErrTemplateNotFound
			}
		}
	}
	if valid.Scope == ScopeFields {
		if err := s.knownFields(valid.Fields); err != nil {
			return ScopePatch{}, err
		}
	}
	return valid, nil
}

// validScopeShape normalizes a requested whole scope without reading ownership.
// Directory membership remains part of validScope at canonical publication.
func validScopeShape(kind Kind, patch ScopePatch) (ScopePatch, error) {
	if !patch.Scope.Valid() || (kind == KindClip && patch.Scope == ScopeFields) {
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
		return ScopePatch{Scope: ScopeTemplates, TemplateIDs: templates}, nil
	default: // ScopeFields: Valid admits no fourth kind.
		if len(templates) > 0 || len(fields) == 0 {
			return ScopePatch{}, ErrScopeShape
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

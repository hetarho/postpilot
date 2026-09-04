package template

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	store  Store
	limits Limits
	now    func() time.Time
	newID  func() string
}

func NewService(store Store, limits Limits) *Service {
	if !limits.valid() {
		panic("template: field limits must be positive")
	}
	return &Service{store: store, limits: limits, now: time.Now, newID: newID}
}

func (s *Service) Limits() Limits { return s.limits }

func (s *Service) List(ctx context.Context, userID string) ([]Template, error) {
	templates, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	return templates, nil
}

func (s *Service) Create(ctx context.Context, userID, name, description, body string) (Template, error) {
	name, err := s.validName(name)
	if err != nil {
		return Template{}, err
	}
	description, err = s.validDescription(description)
	if err != nil {
		return Template{}, err
	}
	body, err = s.validBody(body)
	if err != nil {
		return Template{}, err
	}
	now := s.now()
	created := Template{
		ID: s.newID(), UserID: userID, Name: name, Description: description,
		Body: body, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.Insert(ctx, created, s.limits.MaxPerAccount); err != nil {
		return Template{}, err
	}
	return created, nil
}

// Update applies only the fields the request carried. The validation runs per present
// field, so an edit of `body` alone can never be refused for a `name` it did not send —
// and can never quietly rewrite one either.
func (s *Service) Update(ctx context.Context, userID, id string, patch Patch) (Template, error) {
	if strings.TrimSpace(id) == "" {
		return Template{}, ErrNotFound
	}
	if patch.empty() {
		return s.store.Get(ctx, userID, id)
	}
	if patch.Name != nil {
		name, err := s.validName(*patch.Name)
		if err != nil {
			return Template{}, err
		}
		patch.Name = &name
	}
	if patch.Description != nil {
		description, err := s.validDescription(*patch.Description)
		if err != nil {
			return Template{}, err
		}
		patch.Description = &description
	}
	if patch.Body != nil {
		body, err := s.validBody(*patch.Body)
		if err != nil {
			return Template{}, err
		}
		patch.Body = &body
	}
	return s.store.Update(ctx, userID, id, patch, s.now())
}

// Delete removes the template and reports how many posts it was detached from. The count is
// what the confirmation named, so it is produced by the same transaction that detaches —
// counting first and deleting after would report a number that was already stale.
func (s *Service) Delete(ctx context.Context, userID, id string) (int, error) {
	if strings.TrimSpace(id) == "" {
		return 0, ErrNotFound
	}
	return s.store.Delete(ctx, userID, id)
}

// RenderedFor is this context's published behavior for prompt builders: one owned template,
// expanded for this post's photos and rendered into prompt text.
//
// The photo filenames are an argument rather than something this context looks up, because
// expansion has to see exactly the attachment set the caller froze — resolving them here
// would let a photo added between the two reads change what was frozen.
//
// `ok` false is the ordinary "the post has none, or it was deleted since" case: absence is
// not an error, because a prompt without a template is a valid prompt. A body that no longer
// parses is treated the same way rather than failing the run — it can only happen if a row
// was edited outside the service, and refusing to generate would be a worse answer than
// generating without a shape.
func (s *Service) RenderedFor(ctx context.Context, userID, id string, filenames []string) (Rendered, bool, error) {
	if strings.TrimSpace(id) == "" {
		return Rendered{}, false, nil
	}
	found, err := s.store.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Rendered{}, false, nil
		}
		return Rendered{}, false, fmt.Errorf("load template: %w", err)
	}
	nodes, err := Parse(found.Body)
	if err != nil {
		return Rendered{}, false, nil
	}
	rendered, err := Render(found.Name, nodes, filenames, s.limits.MaxRepeatExpansion)
	if err != nil {
		return Rendered{}, false, err
	}
	return rendered, true, nil
}

// Directory is this context's published behavior for the post and guideline contexts: the
// ids and names an account owns, which is all either of them needs to validate ownership
// and project a name.
func (s *Service) Directory(ctx context.Context, userID string) ([]Template, error) {
	return s.List(ctx, userID)
}

func (s *Service) validName(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrNameRequired
	}
	if chars := utf8.RuneCountInString(trimmed); chars > s.limits.NameMaxChars {
		return "", &FieldTooLongError{Field: "name", Chars: chars, Max: s.limits.NameMaxChars}
	}
	return trimmed, nil
}

func (s *Service) validDescription(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if chars := utf8.RuneCountInString(trimmed); chars > s.limits.DescriptionMaxChars {
		return "", &FieldTooLongError{Field: "description", Chars: chars, Max: s.limits.DescriptionMaxChars}
	}
	return trimmed, nil
}

// validBody trims, bounds, and PARSES. Parsing is part of validation rather than a later
// concern because a body that does not parse has no meaning: it would reach a prompt as
// prose the model prints back, and the builder could not open it either.
//
// Only the outer whitespace is trimmed. Everything inside is the author's, down to the
// blank lines, because the body is what the prompt receives and what the builder
// re-serializes byte for byte.
func (s *Service) validBody(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrBodyRequired
	}
	if chars := utf8.RuneCountInString(trimmed); chars > s.limits.BodyMaxChars {
		return "", &FieldTooLongError{Field: "body", Chars: chars, Max: s.limits.BodyMaxChars}
	}
	if _, err := Parse(trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("template: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

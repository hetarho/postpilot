package purpose

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
		panic("purpose: field limits must be positive")
	}
	return &Service{store: store, limits: limits, now: time.Now, newID: newID}
}

func (s *Service) Limits() Limits { return s.limits }

func (s *Service) List(ctx context.Context, userID string) ([]Purpose, error) {
	purposes, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list purposes: %w", err)
	}
	return purposes, nil
}

func (s *Service) Create(ctx context.Context, userID, name, description, instructions string) (Purpose, error) {
	name, err := s.validName(name)
	if err != nil {
		return Purpose{}, err
	}
	description, err = s.validDescription(description)
	if err != nil {
		return Purpose{}, err
	}
	instructions, err = s.validInstructions(instructions)
	if err != nil {
		return Purpose{}, err
	}
	now := s.now()
	created := Purpose{
		ID: s.newID(), UserID: userID, Name: name, Description: description,
		Instructions: instructions, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.Insert(ctx, created); err != nil {
		return Purpose{}, err
	}
	return created, nil
}

// Update applies only the fields the request carried. The validation runs per present
// field, so an edit of `instructions` alone can never be refused for a `name` it did not
// send — and can never quietly rewrite one either.
func (s *Service) Update(ctx context.Context, userID, id string, patch Patch) (Purpose, error) {
	if strings.TrimSpace(id) == "" {
		return Purpose{}, ErrNotFound
	}
	if patch.empty() {
		return s.store.Get(ctx, userID, id)
	}
	if patch.Name != nil {
		name, err := s.validName(*patch.Name)
		if err != nil {
			return Purpose{}, err
		}
		patch.Name = &name
	}
	if patch.Description != nil {
		description, err := s.validDescription(*patch.Description)
		if err != nil {
			return Purpose{}, err
		}
		patch.Description = &description
	}
	if patch.Instructions != nil {
		instructions, err := s.validInstructions(*patch.Instructions)
		if err != nil {
			return Purpose{}, err
		}
		patch.Instructions = &instructions
	}
	return s.store.Update(ctx, userID, id, patch, s.now())
}

// Delete removes the purpose and reports how many posts it was detached from. The count is
// what the confirmation named, so it is produced by the same transaction that detaches —
// counting first and deleting after would report a number that was already stale.
func (s *Service) Delete(ctx context.Context, userID, id string) (int, error) {
	if strings.TrimSpace(id) == "" {
		return 0, ErrNotFound
	}
	return s.store.Delete(ctx, userID, id)
}

// BriefFor is this context's published behavior for prompt builders: the frozen text of one
// owned purpose. `ok` false is the ordinary "the post has none, or it was deleted since"
// case — absence is not an error, because a prompt without a purpose is a valid prompt.
func (s *Service) BriefFor(ctx context.Context, userID, id string) (Brief, bool, error) {
	if strings.TrimSpace(id) == "" {
		return Brief{}, false, nil
	}
	found, err := s.store.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Brief{}, false, nil
		}
		return Brief{}, false, fmt.Errorf("load purpose brief: %w", err)
	}
	return found.Brief(), true, nil
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

func (s *Service) validInstructions(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrInstructionsRequired
	}
	if chars := utf8.RuneCountInString(trimmed); chars > s.limits.InstructionsMaxChars {
		return "", &FieldTooLongError{Field: "instructions", Chars: chars, Max: s.limits.InstructionsMaxChars}
	}
	return trimmed, nil
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("purpose: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

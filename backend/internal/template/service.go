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

// parseOptions hands the parser the one configured value it needs. It lives here rather
// than in the parser so the shared fixture can pin its own ceiling and never depend on the
// environment the test runs in.
func (s *Service) parseOptions() ParseOptions {
	return ParseOptions{PhotoRowMax: s.limits.PhotoRowMax, AskMaxPerBody: s.limits.AskMaxPerBody}
}

func (s *Service) List(ctx context.Context, userID string) ([]Template, error) {
	templates, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	return templates, nil
}

// Create validates the two areas together: they are one document with one data-field
// namespace (TMPL-50, TMPL-55), so neither can be judged without the other.
func (s *Service) Create(ctx context.Context, userID string, authored Authored) (Template, error) {
	name, err := s.validName(authored.Name)
	if err != nil {
		return Template{}, err
	}
	description, err := s.validDescription(authored.Description)
	if err != nil {
		return Template{}, err
	}
	body, err := s.validBody(authored.Body)
	if err != nil {
		return Template{}, err
	}
	titleArea, err := s.validTitleArea(authored.TitleArea)
	if err != nil {
		return Template{}, err
	}
	if err := s.validShape(titleArea, body); err != nil {
		return Template{}, err
	}
	numbers := authored.Numbers
	if err := s.validNumbers(numbers); err != nil {
		return Template{}, err
	}
	now := s.now()
	created := Template{
		ID: s.newID(), UserID: userID, Name: name, Description: description,
		Body: body, TitleArea: titleArea, TargetLength: numbers.TargetLength, TagCount: numbers.TagCount,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.Insert(ctx, created, s.limits.MaxPerAccount); err != nil {
		return Template{}, err
	}
	return created, nil
}

// Update applies only the fields the request carried. The validation runs per present
// field, so an edit of `body` alone can never be refused for a `name` it did not send —
// and can never quietly rewrite one either.
//
// The one check that needs both areas runs inside the store's transaction, on the stored
// counterpart of whichever area the patch leaves out: a title that reuses a label the stored
// body holds is refused although the request never sent the body.
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
	if patch.TitleArea != nil {
		titleArea, err := s.validTitleArea(*patch.TitleArea)
		if err != nil {
			return Template{}, err
		}
		patch.TitleArea = &titleArea
	}
	if patch.Numbers != nil {
		if err := s.validNumbers(*patch.Numbers); err != nil {
			return Template{}, err
		}
	}
	// An edit that names neither area parses nothing, as it never has.
	var check func(current Template) error
	if patch.Body != nil || patch.TitleArea != nil {
		body, titleArea := patch.Body, patch.TitleArea
		check = func(current Template) error {
			shapeBody, shapeTitle := current.Body, current.TitleArea
			if body != nil {
				shapeBody = *body
			}
			if titleArea != nil {
				shapeTitle = *titleArea
			}
			return s.validShape(shapeTitle, shapeBody)
		}
	}
	return s.store.Update(ctx, userID, id, patch, s.now(), check)
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
// The photo filenames and the post's answers are arguments rather than something this
// context looks up, because the freeze has to see exactly the attachment set and the exact
// answers the caller froze — resolving either here would let a photo attached or a field
// edited between the two reads change what was frozen.
//
// `ok` false is the ordinary "the post has none, or it was deleted since" case: absence is
// not an error, because a prompt without a template is a valid prompt. A template whose title
// area or body no longer parses is treated the same way rather than failing the run — it can
// only happen if a row was edited outside the service, and refusing to generate would be a
// worse answer than generating without a shape.
func (s *Service) RenderedFor(ctx context.Context, userID, id string, filenames []string, answers []Answer) (Rendered, bool, error) {
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
	title, nodes, err := ParseTemplate(found.TitleArea, found.Body, s.parseOptions())
	if err != nil {
		return Rendered{}, false, nil
	}
	rendered, err := RenderTemplate(found.Name, title, nodes, filenames, s.limits.MaxRepeatExpansion, answers)
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

// validNumbers bounds the two generation numbers against the POST option's own limits. nil
// is valid on both and is what "no opinion" writes (TEMPLATE-47): the assignment then leaves
// the post's value alone rather than clearing it.
func (s *Service) validNumbers(numbers Numbers) error {
	if value := numbers.TargetLength; value != nil {
		if *value < s.limits.TargetLengthMin {
			return &NumberOutOfRangeError{
				Field: "target_length", Value: *value, Min: s.limits.TargetLengthMin,
			}
		}
	}
	if value := numbers.TagCount; value != nil {
		if *value < s.limits.TagCountMin || *value > s.limits.TagCountMax {
			return &NumberOutOfRangeError{
				Field: "tag_count", Value: *value,
				Min: s.limits.TagCountMin, Max: s.limits.TagCountMax,
			}
		}
	}
	return nil
}

// validBody trims and bounds the body. Its parse is validShape's, because the body and the
// title area are one document and neither parses alone.
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
	return trimmed, nil
}

// validTitleArea trims and bounds the title area with the body's edge trim. Unlike the body it
// may be empty: a title form is opted into, not a field every template must answer (TMPL-52).
func (s *Service) validTitleArea(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if chars := utf8.RuneCountInString(trimmed); chars > s.limits.TitleAreaMaxChars {
		return "", &FieldTooLongError{Field: "title_area", Chars: chars, Max: s.limits.TitleAreaMaxChars}
	}
	return trimmed, nil
}

// validShape PARSES the two areas as the one document they are. Parsing is part of validation
// rather than a later concern because text that does not parse has no meaning: it would reach
// a prompt as prose the model prints back, and the builder could not open it either. An error
// names its area, title first, as ParseTemplate reports it.
func (s *Service) validShape(titleArea, body string) error {
	title, nodes, err := ParseTemplate(titleArea, body, s.parseOptions())
	if err != nil {
		return err
	}
	// A data field's TITLE is bounded here rather than by a parse reason: TEMPLATE-20's
	// reason list is the grammar's, and a configured ceiling inside it would make the shared
	// fixture depend on the deployment. It refuses like any other over-long field, which is
	// a message the editor already renders.
	for _, ask := range append(Asks(title), Asks(nodes)...) {
		label := Decode(ask.Label)
		if chars := utf8.RuneCountInString(label); chars > s.limits.AskLabelMaxChars {
			return &FieldTooLongError{Field: "ask_label", Chars: chars, Max: s.limits.AskLabelMaxChars}
		}
	}
	return nil
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("template: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

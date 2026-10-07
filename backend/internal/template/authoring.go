package template

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

var ErrAuthoringConflict = errors.New("template changed since the authoring draft was opened")

type AuthoringKey struct {
	Key, SessionID string
	Revision       uint32
}
type AuthoringPublication struct {
	Key                     AuthoringKey
	TargetID, TargetVersion string
	Draft                   Draft
	// Nil retains captured generation defaults; a present pair is an explicit owner edit.
	Numbers *Numbers
}
type AuthoringStore interface {
	CountAuthoringTargets(context.Context, string) (int, error)
	PublishAuthoring(context.Context, string, AuthoringPublication, string, time.Time, int) (Template, error)
}

// Authoring publishes validated text through an explicit, required transactional port.
// Its reads and field checks reuse the ordinary template service without model work.
type Authoring struct {
	service *Service
	store   AuthoringStore
}

func NewAuthoring(service *Service, store AuthoringStore) *Authoring {
	if service == nil || store == nil {
		panic("template: authoring dependencies required")
	}
	return &Authoring{service: service, store: store}
}
func (a *Authoring) Seed(ctx context.Context, user, id string) (Draft, string, error) {
	draft, _, version, err := a.SeedWithNumbers(ctx, user, id)
	return draft, version, err
}
func (a *Authoring) SeedWithNumbers(ctx context.Context, user, id string) (Draft, Numbers, string, error) {
	current, err := a.service.store.Get(ctx, user, id)
	if err != nil {
		return Draft{}, Numbers{}, "", err
	}
	return Draft{Name: current.Name, Description: current.Description, Body: current.Body, TitleArea: current.TitleArea}, Numbers{TargetLength: current.TargetLength, TagCount: current.TagCount}, AuthoringVersion(current), nil
}
func (a *Authoring) CanStart(ctx context.Context, user, id string) error {
	if id != "" {
		_, err := a.service.store.Get(ctx, user, id)
		return err
	}
	count, err := a.store.CountAuthoringTargets(ctx, user)
	if err != nil {
		return err
	}
	if count >= a.service.limits.MaxPerAccount {
		return ErrTooMany
	}
	return nil
}
func (a *Authoring) Validate(draft Draft) (Draft, error)   { return a.service.validDraft(draft) }
func (a *Authoring) ValidateNumbers(numbers Numbers) error { return a.service.validNumbers(numbers) }

// RenderedForNewWrite validates a private contender through the same grammar and
// required-answer gate as a saved template, before a test may admit paid writing.
func (a *Authoring) RenderedForNewWrite(draft Draft, hasPhotos bool, answers []Answer) (Rendered, error) {
	draft, err := a.Validate(draft)
	if err != nil {
		return Rendered{}, err
	}
	title, body, err := ParseTemplate(draft.TitleArea, draft.Body, a.service.parseOptions())
	if err != nil {
		return Rendered{}, err
	}
	if err := a.service.validateRequiredAnswers(title, body, answers); err != nil {
		return Rendered{}, err
	}
	return RenderTemplate(draft.Name, title, body, hasPhotos, answers), nil
}
func (a *Authoring) Guide() string {
	guide, err := FormatGuide(LanguageKorean, a.service.limits)
	if err != nil {
		panic(err)
	}
	return guide + fmt.Sprintf("\n이름은1~%d자,설명은%d자,본문은%d자,제목형식은%d자까지입니다. name/description/body/title_area로만템플릿구조를표현하세요. target_length와tag_count및저장된연결관계는바꾸지마세요. 편집요청이바꾸지않은부분은유지하세요. 말투나일반작성지침은본문에넣지마세요.", a.service.limits.NameMaxChars, a.service.limits.DescriptionMaxChars, a.service.limits.BodyMaxChars, a.service.limits.TitleAreaMaxChars)
}
func (a *Authoring) Publish(ctx context.Context, user string, in AuthoringPublication) (Template, error) {
	draft, err := a.Validate(in.Draft)
	if err != nil {
		return Template{}, err
	}
	in.Draft = draft
	if in.Numbers != nil {
		if err := a.service.validNumbers(*in.Numbers); err != nil {
			return Template{}, err
		}
	}
	return a.store.PublishAuthoring(ctx, user, in, a.service.newID(), a.service.now(), a.service.limits.MaxPerAccount)
}
func AuthoringVersion(t Template) string {
	optional := func(value *int) string {
		if value == nil {
			return "none"
		}
		return fmt.Sprint(*value)
	}
	fields := []string{t.ID, t.Name, t.Description, t.Body, t.TitleArea, optional(t.TargetLength), optional(t.TagCount), t.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	return fmt.Sprintf("template:%x", sha256.Sum256([]byte(fmt.Sprintf("%q", fields))))
}

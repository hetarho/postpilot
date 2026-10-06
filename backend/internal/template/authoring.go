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
	current, err := a.service.store.Get(ctx, user, id)
	if err != nil {
		return Draft{}, "", err
	}
	return Draft{Name: current.Name, Description: current.Description, Body: current.Body, TitleArea: current.TitleArea}, AuthoringVersion(current), nil
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
func (a *Authoring) Validate(draft Draft) (Draft, error) { return a.service.validDraft(draft) }
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

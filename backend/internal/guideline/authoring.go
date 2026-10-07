package guideline

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"time"
)

var ErrAuthoringConflict = errors.New("guideline changed since the authoring draft was opened")

type AuthoringDraft struct{ Name, Body string }
type AuthoringKey struct {
	Key, SessionID string
	Revision       uint32
}
type AuthoringPublication struct {
	Key                     AuthoringKey
	Kind                    Kind
	TargetID, TargetVersion string
	Draft                   AuthoringDraft
	// Nil retains the saved scope. A present scope is an explicit owner choice.
	Scope *ScopePatch
}
type AuthoringStore interface {
	CountAuthoringTargets(context.Context, string, Kind) (int, error)
	PublishAuthoring(context.Context, string, AuthoringPublication, string, time.Time, int) (Guideline, error)
}
type Authoring struct {
	service *Service
	store   AuthoringStore
}

func NewAuthoring(service *Service, store AuthoringStore) *Authoring {
	if service == nil || store == nil {
		panic("guideline: authoring dependencies required")
	}
	return &Authoring{service: service, store: store}
}
func (a *Authoring) Seed(ctx context.Context, user string, kind Kind, id string) (AuthoringDraft, string, error) {
	draft, _, version, err := a.SeedWithScope(ctx, user, kind, id)
	return draft, version, err
}
func (a *Authoring) SeedWithScope(ctx context.Context, user string, kind Kind, id string) (AuthoringDraft, ScopePatch, string, error) {
	current, err := a.service.store.Get(ctx, user, id)
	if err != nil {
		return AuthoringDraft{}, ScopePatch{}, "", err
	}
	if current.Kind != kind {
		return AuthoringDraft{}, ScopePatch{}, "", ErrNotFound
	}
	return AuthoringDraft{Name: current.Title, Body: current.Text}, ScopePatch{Scope: current.Scope, TemplateIDs: current.TemplateIDs, Fields: current.Fields}, AuthoringVersion(current), nil
}
func (a *Authoring) CanStart(ctx context.Context, user string, kind Kind, id string) error {
	if !kind.Valid() {
		return ErrNotFound
	}
	if id != "" {
		_, _, err := a.Seed(ctx, user, kind, id)
		return err
	}
	count, err := a.store.CountAuthoringTargets(ctx, user, kind)
	if err != nil {
		return err
	}
	if count >= a.service.limits.MaxPerAccount {
		return &AccountCapError{Max: a.service.limits.MaxPerAccount}
	}
	return nil
}
func (a *Authoring) Validate(draft AuthoringDraft) (AuthoringDraft, error) {
	name, err := a.service.validTitle(draft.Name)
	if err != nil {
		return AuthoringDraft{}, err
	}
	body, err := a.service.validText(draft.Body)
	if err != nil {
		return AuthoringDraft{}, err
	}
	return AuthoringDraft{Name: name, Body: body}, nil
}

// ValidateScopeShape checks direct input without reading any account data. Publish
// also checks every normalized link against the owning account's directory.
func (a *Authoring) ValidateScopeShape(kind Kind, patch ScopePatch) error {
	if !kind.Valid() {
		return ErrScopeShape
	}
	_, err := validScopeShape(kind, patch)
	return err
}
func (a *Authoring) Guide(kind Kind) string { return AuthoringGuide(kind, a.service.limits) }

// AuthoringGuide describes only direction fields consumed by this context.
func AuthoringGuide(kind Kind, limits Limits) string {
	topic := "글"
	if kind == KindClip {
		topic = "영상"
	}
	return fmt.Sprintf("%s 작성의 방향을알려주는쉬운지침입니다. name은알아보기쉬운제목(%d자이하),body는구체적인작성지침(%d자이하)으로작성하세요. 템플릿XML/JSON/구조나말투의학습자료를만들지마세요. 기본지침을고치지마세요. 기존지침의종류,적용범위,템플릿및분야연결을바꾸지마세요. 적용범위는사용자가명시적으로선택해저장합니다.", topic, limits.TitleMaxChars, limits.TextMaxChars)
}
func (a *Authoring) Publish(ctx context.Context, user string, in AuthoringPublication) (Guideline, error) {
	if !in.Kind.Valid() {
		return Guideline{}, ErrNotFound
	}
	if in.TargetID == "" && in.Scope == nil {
		return Guideline{}, ErrScopeShape
	}
	draft, err := a.Validate(in.Draft)
	if err != nil {
		return Guideline{}, err
	}
	in.Draft = draft
	if in.Scope != nil {
		scope, err := a.service.validScope(ctx, user, in.Kind, *in.Scope)
		if err != nil {
			return Guideline{}, err
		}
		in.Scope = &scope
	}
	return a.store.PublishAuthoring(ctx, user, in, a.service.newID(), a.service.now(), a.service.limits.MaxPerAccount)
}
func AuthoringVersion(g Guideline) string {
	templates, fields := slices.Clone(g.TemplateIDs), slices.Clone(g.Fields)
	slices.Sort(templates)
	slices.Sort(fields)
	values := []string{g.ID, string(g.Kind), g.Title, g.Text, string(g.Scope), fmt.Sprintf("%q", templates), fmt.Sprintf("%q", fields), g.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	return fmt.Sprintf("guideline:%x", sha256.Sum256([]byte(fmt.Sprintf("%q", values))))
}

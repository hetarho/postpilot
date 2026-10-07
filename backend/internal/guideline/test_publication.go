package guideline

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
)

var ErrTestPublicationConflict = errors.New("guideline winner publication conflicts with its target or receipt")

type TestSnapshot struct {
	TargetID, TargetVersion string
	Kind                    Kind
	Draft                   AuthoringDraft
	Scope                   ScopePatch
}

func EncodeTestSnapshot(snapshot TestSnapshot) ([]byte, error) { return json.Marshal(snapshot) }
func DecodeTestSnapshot(content []byte) (TestSnapshot, error) {
	var snapshot TestSnapshot
	err := json.Unmarshal(content, &snapshot)
	return snapshot, err
}

type TestPublicationStore interface {
	ReadTestPublicationReceipt(context.Context, string, string, string, string) (TestedPublicationReceipt, bool, error)
	TestPublicationReceipt(context.Context, TestedPublication) (TestedPublicationReceipt, bool, error)
	CommitTestPublication(context.Context, TestedPublication, TestSnapshot, ScopePatch, string, time.Time, int) (TestedPublicationReceipt, error)
}

type TestedSettingsService struct {
	service *Service
	store   TestPublicationStore
}

func (p *TestedSettingsService) ValidateTestPublicationChoices(ctx context.Context, in TestedPublication) error {
	if in.MakeDefault || (in.Action != "save_setting" && in.Action != "use_setting") {
		return ErrTestPublicationConflict
	}
	if in.Action == "save_setting" {
		if _, err := p.service.validTitle(in.Name); err != nil {
			return err
		}
	}
	scope := ScopePatch{Scope: Scope(in.Scope)}
	switch scope.Scope {
	case ScopeTemplates:
		scope.TemplateIDs = in.ScopeIDs
	case ScopeFields:
		scope.Fields = in.ScopeIDs
	case ScopeGlobal:
		if len(in.ScopeIDs) != 0 {
			return ErrScopeShape
		}
	}
	_, err := p.service.validScope(ctx, in.UserID, KindPost, scope)
	return err
}

func NewTestedSettings(service *Service, store TestPublicationStore) *TestedSettingsService {
	if service == nil || store == nil {
		panic("guideline: tested settings dependencies required")
	}
	return &TestedSettingsService{service: service, store: store}
}

func (p *TestedSettingsService) PublishTestWinner(ctx context.Context, in TestedPublication) (TestedPublicationReceipt, error) {
	if strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.TestID) == "" || strings.TrimSpace(in.WinnerID) == "" || strings.TrimSpace(in.RequestKey) == "" || strings.TrimSpace(in.Fingerprint) == "" || (in.Action != "save_setting" && in.Action != "use_setting") || in.MakeDefault {
		return TestedPublicationReceipt{}, ErrTestPublicationConflict
	}
	if receipt, found, err := p.store.TestPublicationReceipt(ctx, in); err != nil || found {
		return receipt, err
	}
	snapshot, err := DecodeTestSnapshot(in.FrozenContent)
	if err != nil || !snapshot.Kind.Valid() {
		return TestedPublicationReceipt{}, ErrTestPublicationConflict
	}
	scope := ScopePatch{Scope: Scope(in.Scope)}
	switch scope.Scope {
	case ScopeTemplates:
		scope.TemplateIDs = in.ScopeIDs
	case ScopeFields:
		scope.Fields = in.ScopeIDs
	case ScopeGlobal:
		if len(in.ScopeIDs) != 0 {
			return TestedPublicationReceipt{}, ErrScopeShape
		}
	}
	scope, err = p.service.validScope(ctx, in.UserID, snapshot.Kind, scope)
	if err != nil {
		return TestedPublicationReceipt{}, err
	}
	if in.Action == "save_setting" {
		snapshot.Draft.Name = in.Name
	} else if snapshot.TargetID == "" || snapshot.TargetVersion == "" || !sameTestScope(scope, snapshot.Scope) {
		return TestedPublicationReceipt{}, ErrTestPublicationConflict
	}
	snapshot.Draft.Name, err = p.service.validTitle(snapshot.Draft.Name)
	if err != nil {
		return TestedPublicationReceipt{}, err
	}
	snapshot.Draft.Body, err = p.service.validText(snapshot.Draft.Body)
	if err != nil {
		return TestedPublicationReceipt{}, err
	}
	return p.store.CommitTestPublication(ctx, in, snapshot, scope, p.service.newID(), p.service.now(), p.service.limits.MaxPerAccount)
}

var _ TestedSettings = (*TestedSettingsService)(nil)

func sameTestScope(a, b ScopePatch) bool {
	if a.Scope != b.Scope {
		return false
	}
	equalIDs := func(left, right []string) bool {
		left, right = slices.Clone(left), slices.Clone(right)
		slices.Sort(left)
		slices.Sort(right)
		return slices.Equal(slices.Compact(left), slices.Compact(right))
	}
	return equalIDs(a.TemplateIDs, b.TemplateIDs) && equalIDs(a.Fields, b.Fields)
}

// ReadTestPublicationReceipt reads proof of a committed action without private payload or live target gates.
func (p *TestedSettingsService) ReadTestPublicationReceipt(ctx context.Context, userID, testID, winnerID string, action string) (TestedPublicationReceipt, bool, error) {
	return p.store.ReadTestPublicationReceipt(ctx, userID, testID, winnerID, action)
}

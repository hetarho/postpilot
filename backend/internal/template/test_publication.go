package template

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrTestPublicationConflict = errors.New("template winner publication conflicts with its target or receipt")

// TestSnapshot is the validated owner setting frozen before writing-test admission.
// Target identity is empty for an unpublished contender. Publication never rerenders it.
type TestSnapshot struct {
	TargetID, TargetVersion string
	Draft                   Draft
	Numbers                 Numbers
}

func EncodeTestSnapshot(snapshot TestSnapshot) ([]byte, error) { return json.Marshal(snapshot) }
func DecodeTestSnapshot(content []byte) (TestSnapshot, error) {
	var snapshot TestSnapshot
	err := json.Unmarshal(content, &snapshot)
	return snapshot, err
}

type TestPublicationStore interface {
	TestPublicationReceipt(context.Context, TestedPublication) (TestedPublicationReceipt, bool, error)
	CommitTestPublication(context.Context, TestedPublication, TestSnapshot, string, time.Time, int) (TestedPublicationReceipt, error)
}

type TestedSettingsService struct {
	service *Service
	store   TestPublicationStore
}

func NewTestedSettings(service *Service, store TestPublicationStore) *TestedSettingsService {
	if service == nil || store == nil {
		panic("template: tested settings dependencies required")
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
	if err != nil {
		return TestedPublicationReceipt{}, ErrTestPublicationConflict
	}
	if in.Action == "save_setting" {
		snapshot.Draft.Name = in.Name
	} else if snapshot.TargetID == "" || snapshot.TargetVersion == "" {
		return TestedPublicationReceipt{}, ErrTestPublicationConflict
	}
	snapshot.Draft, err = p.service.validDraft(snapshot.Draft)
	if err != nil {
		return TestedPublicationReceipt{}, err
	}
	if err := p.service.validNumbers(snapshot.Numbers); err != nil {
		return TestedPublicationReceipt{}, err
	}
	return p.store.CommitTestPublication(ctx, in, snapshot, p.service.newID(), p.service.now(), p.service.limits.MaxPerAccount)
}

var _ TestedSettings = (*TestedSettingsService)(nil)

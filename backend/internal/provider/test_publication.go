package provider

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrTestPublicationConflict = errors.New("model adoption action conflicts with its confirmed receipt")

// TestModelAdoptionStore owns the selection and receipt transaction. Receipt lookup
// precedes live eligibility checks, so recovery cannot undo a newer manual choice.
type TestModelAdoptionStore interface {
	TestModelReceipt(context.Context, TestModelAdoption) (TestModelReceipt, bool, error)
	CommitTestModelAdoption(context.Context, TestModelAdoption, time.Time) (TestModelReceipt, error)
}

type TestModelAdoptionsService struct {
	service *Service
	store   TestModelAdoptionStore
}

func NewTestModelAdoptions(service *Service, store TestModelAdoptionStore) *TestModelAdoptionsService {
	if service == nil || store == nil {
		panic("provider: tested model adoption dependencies required")
	}
	return &TestModelAdoptionsService{service: service, store: store}
}

func (a *TestModelAdoptionsService) AdoptTestModel(ctx context.Context, in TestModelAdoption) (TestModelReceipt, error) {
	if strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.TestID) == "" || strings.TrimSpace(in.WinnerID) == "" || strings.TrimSpace(in.RequestKey) == "" || strings.TrimSpace(in.Fingerprint) == "" || (in.Stage != StageObserve && in.Stage != StageWrite) {
		return TestModelReceipt{}, ErrTestPublicationConflict
	}
	if receipt, found, err := a.store.TestModelReceipt(ctx, in); err != nil || found {
		return receipt, err
	}
	if err := a.service.validateRef(ctx, in.UserID, in.Stage, in.Ref); err != nil {
		return TestModelReceipt{}, err
	}
	return a.store.CommitTestModelAdoption(ctx, in, a.service.now())
}

var _ TestedModelAdoptions = (*TestModelAdoptionsService)(nil)

package authoring

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

// Durable source mutations carry the validation verdict supplied by the owning
// domain service. The SQL adapter never decides template/voice semantics.
type durableDrafts interface {
	PatchDraftState(context.Context, DraftMutation, DraftState) (Session, error)
	ResetChat(context.Context, ResetMutation) (Session, error)
	ResetBaseline(context.Context, ResetMutation) (Session, error)
	ListSummaries(context.Context, SummaryQuery) ([]Summary, string, error)
	SelectWithKey(context.Context, ResetMutation, string) (Session, error)
	PrepareSaveWithKey(context.Context, ResetMutation, bool, func(Kind, Artifact) error) (Session, Publication, error)
	PublicationFailure(context.Context, string, string, string, bool) error
}

func currentSource(s Session) *Artifact {
	if s.WorkingSource != nil {
		return s.WorkingSource
	}
	return s.Selected
}
func (s *Service) drafts() (durableDrafts, error) {
	store, ok := s.store.(durableDrafts)
	if !ok {
		return nil, ErrInvalid
	}
	return store, nil
}
func (s *Service) PatchDraft(ctx context.Context, in DraftMutation) (Session, error) {
	if !requestKey(in.OperationKey) || in.UserID == "" {
		return Session{}, ErrInvalid
	}
	a := in.WorkingSource
	if !utf8.ValidString(a.Name+a.Description+a.Body+a.TitleArea) || utf8.RuneCountInString(a.Name+a.Description+a.Body+a.TitleArea) > MaxDocumentChars || utf8.RuneCountInString(a.Name) > 200 || utf8.RuneCountInString(a.Description) > 200 {
		return Session{}, ErrInvalid
	}
	defer s.lock(in.UserID)()
	// Do not reconcile a late job before the explicit manual CAS.
	state, err := s.store.Get(ctx, in.UserID, in.SessionID)
	if err != nil {
		return state, err
	}
	if state.Kind != PostTemplate && a.TitleArea != "" {
		return state, ErrInvalid
	}
	validity := DraftValid
	if strings.TrimSpace(a.Body) == "" || (strings.TrimSpace(a.Name) == "" && state.Kind != PostGuideline && state.Kind != VideoGuideline) {
		validity = DraftIncomplete
	} else if s.targets.Validate(state.Kind, a) != nil {
		validity = DraftInvalid
	}
	store, err := s.drafts()
	if err != nil {
		return state, err
	}
	return store.PatchDraftState(ctx, in, validity)
}
func (s *Service) ResetChat(ctx context.Context, in ResetMutation) (Session, error) {
	if !requestKey(in.OperationKey) {
		return Session{}, ErrInvalid
	}
	defer s.lock(in.UserID)()
	store, err := s.drafts()
	if err != nil {
		return Session{}, err
	}
	return store.ResetChat(ctx, in)
}
func (s *Service) ResetBaseline(ctx context.Context, in ResetMutation) (Session, error) {
	if !requestKey(in.OperationKey) {
		return Session{}, ErrInvalid
	}
	defer s.lock(in.UserID)()
	store, err := s.drafts()
	if err != nil {
		return Session{}, err
	}
	return store.ResetBaseline(ctx, in)
}
func (s *Service) ListSummaries(ctx context.Context, in SummaryQuery) ([]Summary, string, error) {
	if !in.Kind.Valid() {
		return nil, "", ErrInvalidKind
	}
	store, err := s.drafts()
	if err != nil {
		return nil, "", err
	}
	return store.ListSummaries(ctx, in)
}
func (s *Service) SelectWithKey(ctx context.Context, in ResetMutation, candidateID string) (Session, error) {
	if in.OperationKey == "" {
		return s.Select(ctx, in.UserID, in.SessionID, in.ExpectedRevision, candidateID)
	}
	if !requestKey(in.OperationKey) {
		return Session{}, ErrInvalid
	}
	defer s.lock(in.UserID)()
	store, err := s.drafts()
	if err != nil {
		return Session{}, err
	}
	return store.SelectWithKey(ctx, in, candidateID)
}
func (s *Service) SaveWithKey(ctx context.Context, in ResetMutation, makeDefault bool) (Session, error) {
	if in.OperationKey == "" {
		return s.Save(ctx, in.UserID, in.SessionID, in.ExpectedRevision, makeDefault)
	}
	if !requestKey(in.OperationKey) {
		return Session{}, ErrInvalid
	}
	defer s.lock(in.UserID)()
	store, err := s.drafts()
	if err != nil {
		return Session{}, err
	}
	// Receipts are checked before the current draft; a prior save remains retriable
	// after a later invalid edit without replaying publication/default assignment.
	state, p, err := store.PrepareSaveWithKey(ctx, in, makeDefault, s.validatePublication)
	if err != nil {
		return state, err
	}
	if state.Phase == "saved" {
		return state, nil
	}
	if err = s.validatePublication(p.Kind, p.Artifact); err != nil {
		return state, ErrDraftInvalid
	}
	ref, err := s.targets.Publish(ctx, p)
	if err != nil {
		s.publicationFailure(ctx, in.UserID, in.SessionID, err)
		if errors.Is(err, ErrNotFound) {
			return state, ErrTargetConflict
		}
		return state, err
	}
	if ref.Kind != p.Kind || ref.ID == "" {
		return state, ErrPublication
	}
	return s.finalizePublication(ctx, in.UserID, in.SessionID, p, ref)
}
func (s *Service) publicationFailure(ctx context.Context, user, id string, err error) {
	store, e := s.drafts()
	if e != nil {
		return
	}
	reason := "AUTHORING_NOT_READY"
	if errors.Is(err, ErrTargetConflict) || errors.Is(err, ErrNotFound) {
		reason = "AUTHORING_SAVE_CONFLICT"
	}
	_ = store.PublicationFailure(context.WithoutCancel(ctx), user, id, reason, errors.Is(err, ErrNotFound))
}
func (s *Service) FreezeCandidate(ctx context.Context, user string, ref OwnedCandidateRef) (FrozenCandidate, error) {
	state, err := s.store.Get(ctx, user, ref.SessionID)
	if err != nil {
		return FrozenCandidate{}, err
	}
	for _, candidate := range state.Candidates {
		if candidate.ID == ref.CandidateID && candidate.Revision == ref.Revision && candidate.Revision != 0 {
			if err := s.validArtifact(state.Kind, candidate); err != nil {
				return FrozenCandidate{}, ErrDraftInvalid
			}
			contexts, ok := s.store.(interface {
				CandidateContext(context.Context, string, OwnedCandidateRef) (string, string, error)
			})
			if !ok {
				return FrozenCandidate{}, ErrInvalid
			}
			target, version, err := contexts.CandidateContext(ctx, user, ref)
			if err != nil {
				return FrozenCandidate{}, err
			}
			return FrozenCandidate{Kind: state.Kind, Artifact: candidate, TargetID: target, TargetVersion: version, Synthetic: true}, nil
		}
	}
	return FrozenCandidate{}, ErrNotFound
}

var _ WorkingDrafts = (*Service)(nil)
var _ TestCandidates = (*Service)(nil)

func (s *Service) finalizePublication(ctx context.Context, user, id string, p Publication, ref SavedRef) (Session, error) {
	if store, ok := s.store.(interface {
		FinalizeSaveVersion(context.Context, string, string, string, SavedRef, string) (Session, error)
	}); ok {
		seed, err := s.targets.Seed(ctx, user, ref.Kind, ref.ID)
		// Publication is already confirmed even if the target was deleted immediately.
		version := ""
		if err == nil && seed.Artifact != nil && samePublishedContent(*seed.Artifact, p.Artifact) {
			version = seed.TargetVersion
		}
		return store.FinalizeSaveVersion(ctx, user, id, p.Key, ref, version)
	}
	return s.store.FinalizeSave(ctx, user, id, p.Key, ref)
}

func (s *Service) validatePublication(kind Kind, a Artifact) error {
	if utf8.RuneCountInString(a.Name+a.Description+a.Body+a.TitleArea) > MaxDocumentChars || utf8.RuneCountInString(a.Name) > 200 || utf8.RuneCountInString(a.Description) > 200 || strings.TrimSpace(a.Body) == "" || (kind != PostTemplate && a.TitleArea != "") {
		return ErrDraftInvalid
	}
	return s.targets.Validate(kind, a)
}

// A read after publication may already see a later owner edit. That version must
// not authorize writing the old draft over the newer saved content.
func samePublishedContent(a, b Artifact) bool {
	return strings.TrimSpace(a.Name) == strings.TrimSpace(b.Name) && strings.TrimSpace(a.Description) == strings.TrimSpace(b.Description) && strings.TrimSpace(a.Body) == strings.TrimSpace(b.Body) && strings.TrimSpace(a.TitleArea) == strings.TrimSpace(b.TitleArea)
}

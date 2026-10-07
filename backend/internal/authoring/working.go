package authoring

import (
	"context"
	"errors"
	"slices"
	"strconv"
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
		return nil, ErrFeatureUnavailable
	}
	return store, nil
}
func (s *Service) PatchDraft(ctx context.Context, in DraftMutation) (Session, error) {
	if !requestKey(in.OperationKey) || in.UserID == "" {
		return Session{}, ErrInvalid
	}
	a := in.WorkingSource
	if err := validDirectMetadata(a); err != nil {
		return Session{}, err
	}
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
	defer s.lock(in.UserID)()
	store, err := s.drafts()
	if err != nil {
		return nil, "", err
	}
	rows, next, err := store.ListSummaries(ctx, in)
	if err != nil {
		return nil, "", err
	}
	active := false
	for i, row := range rows {
		if row.ActiveJobID == "" {
			continue
		}
		// Reconciliation reads only already-admitted work. It neither issues a
		// provider request nor publishes a setting when a directory is opened.
		state, err := s.get(ctx, in.UserID, row.SessionID)
		if err != nil {
			return nil, "", err
		}
		if in.PageToken != "" {
			// Terminal reconciliation moves UpdatedAt ahead of the supplied cursor.
			// Retain this page's selected identities and original next cursor while
			// projecting the newly settled aggregate rather than filtering it away.
			// Read the committed record for its actual persistence timestamp.
			state, err = s.store.Get(ctx, in.UserID, row.SessionID)
			if err != nil {
				return nil, "", err
			}
			rows[i].Revision, rows[i].UpdatedAt = state.Revision, state.UpdatedAt
			rows[i].SavedAvailable, rows[i].HasUnpublishedChanges = state.SavedAvailable, state.HasUnpublishedChanges
			rows[i].ActiveJobID, rows[i].DraftState = state.ActiveJobID, state.DraftState
			rows[i].PublicationPending = state.Phase == "saving"
			rows[i].TargetConflict = state.FailureReason == "AUTHORING_SAVE_CONFLICT"
			rows[i].LastPublication = state.Saved
			rows[i].DisplayName = ""
			if state.WorkingSource != nil && strings.TrimSpace(state.WorkingSource.Name) != "" {
				rows[i].DisplayName = state.WorkingSource.Name
			} else {
				for _, a := range []*Artifact{state.SavedBaseline, state.Selected, state.WorkingSource} {
					if a == nil {
						continue
					}
					if strings.TrimSpace(a.Name) != "" {
						rows[i].DisplayName = a.Name
						break
					}
					summary := []rune(strings.Join(strings.Fields(a.Body), " "))
					if len(summary) > DisplaySummaryMaxChars {
						summary = summary[:DisplaySummaryMaxChars]
					}
					if len(summary) > 0 {
						rows[i].DisplayName = string(summary)
						break
					}
				}
			}
		}
		active = true
	}
	if active && in.PageToken == "" {
		return store.ListSummaries(ctx, in)
	}
	return rows, next, nil
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
	if user == "" || ref.SessionID == "" || ref.CandidateID == "" || ref.Revision == 0 {
		return FrozenCandidate{}, ErrNotFound
	}
	state, err := s.store.Get(ctx, user, ref.SessionID)
	if err != nil {
		return FrozenCandidate{}, err
	}
	freeze := func(candidate Artifact, target, version string) FrozenCandidate {
		candidate.TemplateIDs = slices.Clone(candidate.TemplateIDs)
		candidate.Fields = slices.Clone(candidate.Fields)
		if candidate.TargetLength != nil {
			value := *candidate.TargetLength
			candidate.TargetLength = &value
		}
		if candidate.TagCount != nil {
			value := *candidate.TagCount
			candidate.TagCount = &value
		}
		if candidate.Scope != nil {
			value := *candidate.Scope
			candidate.Scope = &value
		}
		return FrozenCandidate{Kind: state.Kind, Artifact: candidate, TargetID: target, TargetVersion: version, Synthetic: true}
	}
	// Explicit historical recommendations retain their admitted target context,
	// even after the selected working source or canonical target changes.
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
			return freeze(candidate, target, version), nil
		}
	}
	// Direct/refined work has its own immutable artifact revision. Never resolve
	// an invalid current source through the previous valid preview.
	if candidate := currentSource(state); candidate != nil && candidate.ID == ref.CandidateID && candidate.Revision == ref.Revision {
		if state.DraftState != DraftValid || s.validatePublication(state.Kind, *candidate) != nil {
			return FrozenCandidate{}, ErrDraftInvalid
		}
		target := state.TargetID
		if target == "" && state.Saved != nil {
			target = state.Saved.ID
		}
		return freeze(*candidate, target, state.TargetVersion), nil
	}
	return FrozenCandidate{}, ErrNotFound
}

var _ WorkingDrafts = (*Service)(nil)
var _ TestCandidates = (*Service)(nil)

func (s *Service) finalizePublication(ctx context.Context, user, id string, p Publication, ref SavedRef) (Session, error) {
	ref.Outcome = "updated"
	if p.TargetID == "" || p.Kind == WritingVoice {
		ref.Outcome = "created"
	}
	if store, ok := s.store.(interface {
		FinalizeSaveVersion(context.Context, string, string, string, SavedRef, string) (Session, error)
	}); ok {
		seed, err := s.targets.Seed(ctx, user, ref.Kind, ref.ID)
		// Publication is already confirmed even if the target was deleted immediately.
		version := ""
		if err == nil && seed.Artifact != nil && samePublishedContent(p.Kind, *seed.Artifact, p.Artifact) {
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
func samePublishedContent(kind Kind, a, b Artifact) bool {
	if strings.TrimSpace(a.Name) != strings.TrimSpace(b.Name) || strings.TrimSpace(a.Body) != strings.TrimSpace(b.Body) {
		return false
	}
	// Guidelines and video recipes preserve name/body only. Descriptions are
	// authoring presentation metadata and are absent from their domain seeds.
	if kind == PostTemplate || kind == WritingVoice {
		if strings.TrimSpace(a.Description) != strings.TrimSpace(b.Description) {
			return false
		}
	}
	if kind == PostTemplate {
		if strings.TrimSpace(a.TitleArea) != strings.TrimSpace(b.TitleArea) {
			return false
		}
		if b.TargetLength != nil && !sameOptionalNumber(a.TargetLength, b.TargetLength) || b.TagCount != nil && !sameOptionalNumber(a.TagCount, b.TagCount) {
			return false
		}
	}
	if (kind == PostGuideline || kind == VideoGuideline) && b.Scope != nil {
		if !sameOptionalString(a.Scope, b.Scope) || !sameScopeIDs(a.TemplateIDs, b.TemplateIDs) || !sameScopeIDs(a.Fields, b.Fields) {
			return false
		}
	}
	return true
}

// Direct-only metadata is private working state. It cannot enlarge model input or cross kinds.
func validDirectMetadata(a Artifact) error {
	text := a.BuilderState
	for _, p := range []*string{a.TargetLength, a.TagCount, a.Scope} {
		if p != nil {
			text += *p
		}
	}
	for _, ids := range [][]string{a.TemplateIDs, a.Fields} {
		for _, value := range ids {
			text += value
		}
	}
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > MaxDocumentChars {
		return ErrInvalid
	}
	return nil
}
func sameOptionalString(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func sameScopeIDs(a, b []string) bool {
	left, right := slices.Clone(a), slices.Clone(b)
	for i := range left {
		left[i] = strings.TrimSpace(left[i])
	}
	for i := range right {
		right[i] = strings.TrimSpace(right[i])
	}
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(slices.Compact(left), slices.Compact(right))
}

func sameOptionalNumber(a, b *string) bool {
	if sameOptionalString(a, b) {
		return true
	}
	if a == nil || b == nil || *a == "" || *b == "" {
		return false
	}
	left, le := strconv.Atoi(*a)
	right, re := strconv.Atoi(*b)
	return le == nil && re == nil && left == right
}

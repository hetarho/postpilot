package spoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

type Service struct {
	store    Storage
	profiles ProfileResolver
	objects  AudioObjects
	now      func() time.Time
	newID    func() string
}

func NewService(store Storage, profiles ProfileResolver, objects AudioObjects) *Service {
	if store == nil || profiles == nil || objects == nil {
		panic("spoken library dependencies are required")
	}
	return &Service{store: store, profiles: profiles, objects: objects, now: time.Now, newID: spokenID}
}

func spokenID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("spoken identity unavailable")
	}
	return hex.EncodeToString(id[:])
}

func digest(parts ...string) string {
	h := sha256.New()
	var n [8]byte
	for _, p := range parts {
		binary.BigEndian.PutUint64(n[:], uint64(len(p)))
		h.Write(n[:])
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func request(owner, operation, key string, parts ...string) (RequestIdentity, error) {
	if owner == "" || strings.TrimSpace(key) == "" || len(key) > 200 {
		return RequestIdentity{}, ErrInvalid
	}
	for _, r := range key {
		if r < 33 || r > 126 {
			return RequestIdentity{}, ErrInvalid
		}
	}
	return RequestIdentity{OwnerID: owner, Operation: operation, Key: key, Digest: digest(append([]string{"spoken-mutation-v1", operation}, parts...)...)}, nil
}

func (s *Service) ListDrafts(ctx context.Context, owner string) ([]Draft, error) {
	return s.store.ListDrafts(ctx, owner)
}
func (s *Service) GetDraft(ctx context.Context, owner, id string) (Draft, error) {
	return s.store.GetDraft(ctx, owner, id)
}
func (s *Service) ListVoices(ctx context.Context, owner string, removed bool) ([]Voice, error) {
	return s.store.ListVoices(ctx, owner, removed)
}
func (s *Service) GetVoice(ctx context.Context, owner, id string) (Voice, error) {
	return s.store.GetVoice(ctx, owner, id)
}

func inputDigest(in DraftInput) string {
	return digest(in.Name, in.Description, in.PreviewText, in.ProfileID, strconv.FormatInt(in.ProfileRevision, 10), in.QualificationSessionID)
}

func (s *Service) CreateDraft(ctx context.Context, owner string, tier plan.Plan, key string, in DraftInput) (Draft, error) {
	name, err := validateName(in.Name)
	if err != nil {
		return Draft{}, err
	}
	in.Name = name
	identity, err := request(owner, "create_draft", key, inputDigest(in))
	if err != nil {
		return Draft{}, err
	}
	profile, err := s.profiles.ResolveSpokenProfile(ctx, owner, tier, in.ProfileID, in.ProfileRevision, in.QualificationSessionID)
	if err != nil {
		return Draft{}, err
	}
	if err := validateDraft(in, profile); err != nil {
		return Draft{}, err
	}
	now := s.now()
	draft := Draft{ID: s.newID(), OwnerID: owner, Revision: 1, Name: name, Description: in.Description, PreviewText: in.PreviewText, Profile: profile, QualificationSessionID: in.QualificationSessionID, CreatedAt: now, UpdatedAt: now}
	result, err := s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		return MutationResult{draft.ID, draft.Revision}, tx.InsertDraft(ctx, draft)
	})
	if err != nil {
		return Draft{}, err
	}
	return s.store.GetDraft(ctx, owner, result.ID)
}

func (s *Service) UpdateDraft(ctx context.Context, owner string, tier plan.Plan, id string, revision int64, key string, in DraftInput) (Draft, error) {
	before, err := s.store.GetDraft(ctx, owner, id)
	if err != nil {
		return Draft{}, err
	}
	if before.ConfirmedVoiceID != "" {
		return Draft{}, ErrImmutable
	}
	name, err := validateName(in.Name)
	if err != nil {
		return Draft{}, err
	}
	in.Name = name
	identity, err := request(owner, "update_draft", key, id, strconv.FormatInt(revision, 10), inputDigest(in))
	if err != nil {
		return Draft{}, err
	}
	profile := before.Profile
	if profile.ID != in.ProfileID || profile.Revision != in.ProfileRevision || before.QualificationSessionID != in.QualificationSessionID {
		profile, err = s.profiles.ResolveSpokenProfile(ctx, owner, tier, in.ProfileID, in.ProfileRevision, in.QualificationSessionID)
		if err != nil {
			return Draft{}, err
		}
	}
	if err := validateDraft(in, profile); err != nil {
		return Draft{}, err
	}
	_, err = s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		current, err := tx.GetDraft(ctx, owner, id)
		if err != nil {
			return MutationResult{}, err
		}
		if current.ConfirmedVoiceID != "" {
			return MutationResult{}, ErrImmutable
		}
		if current.Revision != revision {
			return MutationResult{}, ErrConflict
		}
		soundChanged := current.Description != in.Description || current.PreviewText != in.PreviewText || current.Profile.ID != profile.ID || current.Profile.Revision != profile.Revision
		current.Name, current.Description, current.PreviewText = name, in.Description, in.PreviewText
		current.Profile = profile
		current.QualificationSessionID = in.QualificationSessionID
		if soundChanged {
			current.GenerationID, current.SelectedCandidateID = "", ""
			current.Candidates = nil
		}
		current.Revision++
		current.UpdatedAt = s.now()
		return MutationResult{id, current.Revision}, tx.UpdateDraft(ctx, current, revision)
	})
	if err != nil {
		return Draft{}, err
	}
	return s.store.GetDraft(ctx, owner, id)
}

func (s *Service) DeleteDraft(ctx context.Context, owner, id string, revision int64, key string) error {
	identity, err := request(owner, "delete_draft", key, id, strconv.FormatInt(revision, 10))
	if err != nil {
		return err
	}
	_, err = s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		if _, err := tx.GetDraft(ctx, owner, id); err != nil {
			return MutationResult{}, err
		}
		return MutationResult{id, revision}, tx.DeleteDraft(ctx, owner, id, revision)
	})
	return err
}

func (s *Service) RenameVoice(ctx context.Context, owner, id string, revision int64, key, name string) (Voice, error) {
	if _, err := s.store.GetVoice(ctx, owner, id); err != nil {
		return Voice{}, err
	}
	name, err := validateName(name)
	if err != nil {
		return Voice{}, err
	}
	identity, err := request(owner, "rename_voice", key, id, strconv.FormatInt(revision, 10), name)
	if err != nil {
		return Voice{}, err
	}
	_, err = s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		return MutationResult{id, revision + 1}, tx.RenameVoice(ctx, owner, id, revision, name)
	})
	if err != nil {
		return Voice{}, err
	}
	return s.store.GetVoice(ctx, owner, id)
}

func (s *Service) RemoveVoice(ctx context.Context, owner, id string, revision int64, key string) (Voice, error) {
	if _, err := s.store.GetVoice(ctx, owner, id); err != nil {
		return Voice{}, err
	}
	identity, err := request(owner, "remove_voice", key, id, strconv.FormatInt(revision, 10))
	if err != nil {
		return Voice{}, err
	}
	_, err = s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		return MutationResult{id, revision + 1}, tx.RemoveVoice(ctx, owner, id, revision, s.now())
	})
	if err != nil {
		return Voice{}, err
	}
	return s.store.GetVoice(ctx, owner, id)
}

func (s *Service) SelectCandidate(ctx context.Context, owner, id string, revision int64, key, candidateID string) (Draft, error) {
	d, err := s.store.GetDraft(ctx, owner, id)
	if err != nil {
		return Draft{}, err
	}
	if !hasCandidate(d, candidateID) {
		return Draft{}, ErrNotFound
	}
	identity, err := request(owner, "select_candidate", key, id, strconv.FormatInt(revision, 10), candidateID)
	if err != nil {
		return Draft{}, err
	}
	_, err = s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		return MutationResult{id, revision + 1}, tx.SelectCandidate(ctx, owner, id, revision, candidateID)
	})
	if err != nil {
		return Draft{}, err
	}
	return s.store.GetDraft(ctx, owner, id)
}

func hasCandidate(d Draft, id string) bool {
	for _, c := range d.Candidates {
		if c.ID == id && c.GenerationID == d.GenerationID {
			return true
		}
	}
	return false
}

func (s *Service) AcknowledgeCandidate(ctx context.Context, owner, id string, revision int64, key, candidateID, playbackID string) (Draft, error) {
	d, err := s.store.GetDraft(ctx, owner, id)
	if err != nil {
		return Draft{}, err
	}
	if !hasCandidate(d, candidateID) {
		return Draft{}, ErrNotFound
	}
	identity, err := request(owner, "acknowledge_candidate", key, id, strconv.FormatInt(revision, 10), candidateID, playbackID)
	if err != nil {
		return Draft{}, err
	}
	_, err = s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		return MutationResult{id, revision + 1}, tx.AcknowledgeCandidate(ctx, owner, id, revision, candidateID, playbackID, s.now())
	})
	if err != nil {
		return Draft{}, err
	}
	return s.store.GetDraft(ctx, owner, id)
}

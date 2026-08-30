package voice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

type Service struct {
	store                 Store
	models                Models
	jobs                  Jobs
	experiments           Experiments
	now                   func() time.Time
	newID                 func() string
	profileMu             sync.Mutex
	sampleMu              sync.Mutex
	directoryMu           sync.Mutex
	posts                 Posts
	config                PersonalizationConfig
	personalization       PersonalizationStore
	personalizationJobs   PersonalizationJobs
	personalizationModels PersonalizationModels
	personalizationReady  bool
}

func NewService(store Store, models Models, jobs Jobs) *Service {
	svc := &Service{store: store, models: models, jobs: jobs, now: time.Now, newID: newID,
		config: PersonalizationConfig{FewShotTargetCount: 2, FewShotMax: 3, FewShotExcerptTargetChars: 500, FewShotExcerptMaxChars: 800, EmbeddingSwitchPosts: 50, DiffMaxRules: 3, DiffMinPatternEdits: 2, RuleActivationEvidence: 3, RuleRetireAfter: 180 * 24 * time.Hour, ValidationPostCount: 3, EndingMaxConsecutive: 2}}
	if p, ok := store.(PersonalizationStore); ok {
		svc.personalization = p
	}
	return svc
}

func (s *Service) ConfigurePersonalization(posts Posts, config PersonalizationConfig) {
	if posts == nil || config.FewShotMax <= 0 || config.RuleActivationEvidence <= 0 {
		panic("voice: invalid personalization configuration")
	}
	s.posts, s.config = posts, config
	s.personalizationReady = true
	if s.personalization == nil {
		panic("voice: personalization store is not configured")
	}
	if jobs, ok := s.jobs.(PersonalizationJobs); ok {
		s.personalizationJobs = jobs
	} else {
		panic("voice: personalization jobs are not configured")
	}
	if models, ok := s.models.(PersonalizationModels); ok {
		s.personalizationModels = models
	} else {
		panic("voice: personalization model catalog is not configured")
	}
}

// SetExperimentGuard wires the model-experiment context's published guard once both
// services exist in the composition root. Without it, DeleteVoice checks only what this
// context and the queue know about.
func (s *Service) SetExperimentGuard(experiments Experiments) { s.experiments = experiments }

func (s *Service) EndingMaxConsecutive() int {
	return s.config.EndingMaxConsecutive
}

// --- directory ---

func (s *Service) ListVoices(ctx context.Context, userID string) ([]Voice, error) {
	voices, err := s.store.ListVoices(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list voices: %w", err)
	}
	return voices, nil
}

func (s *Service) GetVoice(ctx context.Context, userID, voiceID string) (Voice, error) {
	return s.ownedVoice(ctx, userID, voiceID)
}

// DefaultVoice is the account's one active default. Reads never create it: an account
// without one is a bootstrap failure, surfaced rather than papered over.
func (s *Service) DefaultVoice(ctx context.Context, userID string) (Voice, error) {
	found, ok, err := s.store.DefaultVoice(ctx, userID)
	if err != nil {
		return Voice{}, fmt.Errorf("default voice: %w", err)
	}
	if !ok {
		return Voice{}, ErrVoiceNotFound
	}
	return found, nil
}

// EnsureDefaultVoice is the account bootstrap: idempotent, safe to rerun, and the only
// path that creates a voice without a user asking for one. An account that already has an
// active default is left alone; one whose default is missing gets its oldest active voice
// promoted; one with no active voice at all gets `기본 말투`. The bool reports a new row.
func (s *Service) EnsureDefaultVoice(ctx context.Context, userID string) (Voice, bool, error) {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	if found, ok, err := s.store.DefaultVoice(ctx, userID); err != nil {
		return Voice{}, false, fmt.Errorf("default voice: %w", err)
	} else if ok {
		return found, false, nil
	}
	voices, err := s.store.ListVoices(ctx, userID)
	if err != nil {
		return Voice{}, false, fmt.Errorf("list voices: %w", err)
	}
	var oldest *Voice
	for i := range voices {
		if voices[i].Deleted() {
			continue
		}
		if oldest == nil || voices[i].CreatedAt.Before(oldest.CreatedAt) {
			oldest = &voices[i]
		}
	}
	if oldest != nil {
		if err := s.store.SetDefaultVoice(ctx, userID, oldest.ID, s.now()); err != nil {
			return Voice{}, false, fmt.Errorf("promote default voice: %w", err)
		}
		promoted, err := s.ownedVoice(ctx, userID, oldest.ID)
		return promoted, false, err
	}
	now := s.now()
	created := Voice{ID: s.newID(), UserID: userID, Name: DefaultVoiceName, IsDefault: true, CreatedAt: now, UpdatedAt: now}
	if err := s.store.InsertVoice(ctx, created); err != nil {
		return Voice{}, false, fmt.Errorf("create default voice: %w", err)
	}
	return created, true, nil
}

func (s *Service) CreateVoice(ctx context.Context, userID, name string) (Voice, error) {
	name, err := normalizeVoiceName(name)
	if err != nil {
		return Voice{}, err
	}
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	now := s.now()
	created := Voice{ID: s.newID(), UserID: userID, Name: name, CreatedAt: now, UpdatedAt: now}
	if err := s.store.InsertVoice(ctx, created); err != nil {
		if errors.Is(err, ErrVoiceNameTaken) {
			return Voice{}, err
		}
		return Voice{}, fmt.Errorf("create voice: %w", err)
	}
	return created, nil
}

// RenameVoice changes only the display name — post rows and immutable snapshots never
// carry it. A tombstone may be renamed too: that is how a restore conflict is resolved.
func (s *Service) RenameVoice(ctx context.Context, userID, voiceID, name string) (Voice, error) {
	name, err := normalizeVoiceName(name)
	if err != nil {
		return Voice{}, err
	}
	found, err := s.ownedVoice(ctx, userID, voiceID)
	if err != nil {
		return Voice{}, err
	}
	if found.Name == name {
		return found, nil
	}
	if err := s.store.RenameVoice(ctx, userID, voiceID, name, s.now()); err != nil {
		if errors.Is(err, ErrVoiceNameTaken) || errors.Is(err, ErrVoiceNotFound) {
			return Voice{}, err
		}
		return Voice{}, fmt.Errorf("rename voice: %w", err)
	}
	return s.ownedVoice(ctx, userID, voiceID)
}

// SetDefaultVoice swaps the default in one store transaction and returns the whole
// directory, since two rows changed. No profile or provider work is involved.
func (s *Service) SetDefaultVoice(ctx context.Context, userID, voiceID string) ([]Voice, error) {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	found, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return nil, err
	}
	if !found.IsDefault {
		if err := s.store.SetDefaultVoice(ctx, userID, voiceID, s.now()); err != nil {
			if errors.Is(err, ErrVoiceNotFound) {
				return nil, err
			}
			return nil, fmt.Errorf("set default voice: %w", err)
		}
	}
	return s.ListVoices(ctx, userID)
}

// DeleteVoice is a soft delete. It refuses the default (so the last active voice can never
// go) and anything that could still publish into the voice: a queued/running job frozen to
// it, an undecided comparison or validation, or a publishable analyze experiment. Posts
// and profile history stay exactly as they are.
func (s *Service) DeleteVoice(ctx context.Context, userID, voiceID string) (Voice, error) {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	found, err := s.ownedVoice(ctx, userID, voiceID)
	if err != nil {
		return Voice{}, err
	}
	if found.Deleted() {
		return found, nil
	}
	if found.IsDefault {
		return Voice{}, ErrVoiceIsDefault
	}
	active, err := s.store.CountActiveVoices(ctx, userID)
	if err != nil {
		return Voice{}, fmt.Errorf("count active voices: %w", err)
	}
	if active <= 1 {
		return Voice{}, ErrVoiceIsDefault
	}
	if busy, err := s.jobs.HasActiveForVoice(ctx, voiceID); err != nil {
		return Voice{}, fmt.Errorf("check voice jobs: %w", err)
	} else if busy {
		return Voice{}, ErrVoiceBusy
	}
	if n, err := s.store.CountUndecidedVoiceWork(ctx, voiceID); err != nil {
		return Voice{}, fmt.Errorf("check undecided voice work: %w", err)
	} else if n > 0 {
		return Voice{}, ErrVoiceBusy
	}
	if s.experiments != nil {
		if busy, err := s.experiments.HasPublishableExperimentForVoice(ctx, userID, voiceID); err != nil {
			return Voice{}, fmt.Errorf("check voice experiments: %w", err)
		} else if busy {
			return Voice{}, ErrVoiceBusy
		}
	}
	deleted, err := s.store.SoftDeleteVoice(ctx, userID, voiceID, s.now())
	if err != nil {
		return Voice{}, fmt.Errorf("delete voice: %w", err)
	}
	current, err := s.ownedVoice(ctx, userID, voiceID)
	if err != nil {
		return Voice{}, err
	}
	if !deleted && !current.Deleted() {
		return Voice{}, ErrInvalidLifecycle
	}
	return current, nil
}

// RestoreVoice clears the tombstone and nothing else: no job, no default change. It fails
// while an active voice holds the same name, which a rename of the tombstone resolves.
func (s *Service) RestoreVoice(ctx context.Context, userID, voiceID string) (Voice, error) {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	found, err := s.ownedVoice(ctx, userID, voiceID)
	if err != nil {
		return Voice{}, err
	}
	if !found.Deleted() {
		return found, nil
	}
	if _, err := s.store.RestoreVoice(ctx, userID, voiceID, s.now()); err != nil {
		if errors.Is(err, ErrVoiceNameTaken) {
			return Voice{}, err
		}
		return Voice{}, fmt.Errorf("restore voice: %w", err)
	}
	return s.ownedVoice(ctx, userID, voiceID)
}

func normalizeVoiceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	chars := utf8.RuneCountInString(name)
	if chars == 0 || chars > VoiceNameMaxChars {
		return "", &VoiceNameError{Chars: chars}
	}
	return name, nil
}

// ownedVoice resolves a voice the account owns, tombstone or not. An empty id is a client
// bug, not a lookup miss, so it gets its own error.
func (s *Service) ownedVoice(ctx context.Context, userID, voiceID string) (Voice, error) {
	if strings.TrimSpace(voiceID) == "" {
		return Voice{}, ErrVoiceRequired
	}
	found, err := s.store.GetVoice(ctx, userID, voiceID)
	if err != nil {
		if errors.Is(err, ErrVoiceNotFound) {
			return Voice{}, err
		}
		return Voice{}, fmt.Errorf("get voice: %w", err)
	}
	return found, nil
}

// activeVoice is the gate every mutation and every provider-backed path passes: a deleted
// voice stays readable but never starts or receives AI or profile work.
func (s *Service) activeVoice(ctx context.Context, userID, voiceID string) (Voice, error) {
	found, err := s.ownedVoice(ctx, userID, voiceID)
	if err != nil {
		return Voice{}, err
	}
	if found.Deleted() {
		return Voice{}, ErrVoiceDeleted
	}
	return found, nil
}

// --- profile ---

func (s *Service) Get(ctx context.Context, userID, voiceID string) (Profile, error) {
	found, err := s.ownedVoice(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, err
	}
	profile, err := s.store.GetProfile(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	samples, err := s.store.ListSamples(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, fmt.Errorf("list samples: %w", err)
	}
	active, err := s.jobs.ActiveForVoiceKind(ctx, voiceID, AnalysisJobKind)
	if err != nil {
		return Profile{}, fmt.Errorf("get active analysis: %w", err)
	}
	profile.UserID, profile.VoiceID, profile.Voice = userID, voiceID, found
	profile.Samples = samples
	if s.personalization != nil {
		profile.SourceCount, err = func() (int, error) {
			sources, e := s.personalization.ListAuthoredSources(ctx, userID, voiceID)
			if e == nil {
				profile.Structured.Sources = sources
			}
			return len(sources), e
		}()
		if err != nil {
			return Profile{}, fmt.Errorf("list authored sources: %w", err)
		}
		profile.CanValidate = profile.SourceCount >= s.config.ValidationPostCount
		profile.Structured.SourceCount = profile.SourceCount
		profile.Structured.Rules, err = s.personalization.ListRules(ctx, userID, voiceID)
		if err != nil {
			return Profile{}, fmt.Errorf("list voice rules: %w", err)
		}
		profile.Structured.Feedback, err = s.personalization.ListFeedback(ctx, userID, voiceID)
		if err != nil {
			return Profile{}, fmt.Errorf("list voice feedback: %w", err)
		}
	}
	if active != nil {
		profile.ActiveJobID = active.ID
	}
	return profile, nil
}

func (s *Service) Update(ctx context.Context, userID, voiceID, styleguide, rules string) (Profile, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Profile{}, err
	}
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	if err := s.store.UpsertProfile(ctx, Profile{
		UserID: userID, VoiceID: voiceID, Styleguide: styleguide, Rules: rules, UpdatedAt: s.now(),
	}); err != nil {
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return s.Get(ctx, userID, voiceID)
}

// UpdateStyleguide changes only the generated/hand-edited guide. The SQL operation is
// deliberately field-scoped so a concurrent rules edit cannot be lost.
func (s *Service) UpdateStyleguide(ctx context.Context, userID, voiceID, styleguide string) (Profile, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Profile{}, err
	}
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	if err := s.store.SetStyleguide(ctx, userID, voiceID, styleguide, s.now()); err != nil {
		return Profile{}, fmt.Errorf("update styleguide: %w", err)
	}
	return s.Get(ctx, userID, voiceID)
}

// UpdateRules changes only user-owned rules. In particular, it never writes the
// styleguide value observed by a client while an analysis may be completing.
func (s *Service) UpdateRules(ctx context.Context, userID, voiceID, rules string) (Profile, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Profile{}, err
	}
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	if err := s.store.SetRules(ctx, userID, voiceID, rules, s.now()); err != nil {
		return Profile{}, fmt.Errorf("update rules: %w", err)
	}
	return s.Get(ctx, userID, voiceID)
}

func (s *Service) AddSample(ctx context.Context, userID, voiceID, label, body string, requested llm.ModelRef) (Sample, string, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Sample{}, "", err
	}
	s.sampleMu.Lock()
	defer s.sampleMu.Unlock()
	body = strings.TrimSpace(body)
	chars := utf8.RuneCountInString(body)
	if chars < SampleMinChars {
		return Sample{}, "", &SampleTooShortError{Chars: chars}
	}
	model, err := s.resolveAnalyzeModel(ctx, userID, requested)
	if err != nil {
		return Sample{}, "", err
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = firstRunes(body, LabelFallbackChars)
	}
	sample := Sample{
		ID: s.newID(), UserID: userID, VoiceID: voiceID, Label: label, Body: body, Chars: chars, CreatedAt: s.now(),
	}
	if err := s.store.InsertSample(ctx, sample); err != nil {
		return Sample{}, "", fmt.Errorf("insert sample: %w", err)
	}
	jobID, err := s.enqueueAnalysis(ctx, userID, voiceID, model)
	if err != nil {
		_, cleanupErr := s.store.DeleteSample(ctx, userID, voiceID, sample.ID, s.now())
		return Sample{}, "", errors.Join(ErrSampleMutation, err, cleanupErr)
	}
	return sample, jobID, nil
}

func (s *Service) DeleteSample(ctx context.Context, userID, voiceID, sampleID string) (string, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return "", err
	}
	s.sampleMu.Lock()
	defer s.sampleMu.Unlock()
	sample, err := s.store.GetSampleBody(ctx, userID, voiceID, sampleID)
	if err != nil {
		return "", fmt.Errorf("get sample before delete: %w", err)
	}
	if sample == nil {
		return "", ErrSampleNotFound
	}
	before, err := s.store.CountSamples(ctx, userID, voiceID)
	if err != nil {
		return "", fmt.Errorf("count samples before delete: %w", err)
	}
	var authoredSources []AuthoredSource
	if s.personalization != nil {
		authoredSources, err = s.personalization.ListAuthoredSources(ctx, userID, voiceID)
		if err != nil {
			return "", fmt.Errorf("list finalized sources before delete: %w", err)
		}
	}
	var model llm.ModelRef
	if before > 1 || len(authoredSources) > 0 {
		var ok bool
		model, ok, err = s.models.AnalyzeModel(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("resolve analyze model: %w", err)
		}
		if !ok {
			return "", ErrAnalyzeModelRequired
		}
	}
	deleted, err := s.store.DeleteSample(ctx, userID, voiceID, sampleID, s.now())
	if err != nil {
		return "", fmt.Errorf("delete sample: %w", err)
	}
	if !deleted {
		return "", ErrSampleNotFound
	}
	count, err := s.store.CountSamples(ctx, userID, voiceID)
	if err != nil {
		return "", fmt.Errorf("count samples: %w", err)
	}
	if count == 0 && len(authoredSources) == 0 {
		return "", nil
	}
	if model == (llm.ModelRef{}) {
		var ok bool
		model, ok, err = s.models.AnalyzeModel(ctx, userID)
		if err != nil {
			if restoreErr := s.store.InsertSample(ctx, *sample); restoreErr != nil {
				return "", errors.Join(ErrSampleMutation, err, restoreErr)
			}
			return "", fmt.Errorf("resolve analyze model: %w", err)
		}
		if !ok {
			if restoreErr := s.store.InsertSample(ctx, *sample); restoreErr != nil {
				return "", errors.Join(ErrSampleMutation, ErrAnalyzeModelRequired, restoreErr)
			}
			return "", ErrAnalyzeModelRequired
		}
	}
	jobID, err := s.enqueueAnalysis(ctx, userID, voiceID, model)
	if err == nil {
		return jobID, nil
	}
	if restoreErr := s.store.InsertSample(ctx, *sample); restoreErr != nil {
		return "", errors.Join(ErrSampleMutation, err, restoreErr)
	}
	return "", errors.Join(ErrSampleMutation, err)
}

func (s *Service) resolveAnalyzeModel(ctx context.Context, userID string, requested llm.ModelRef) (llm.ModelRef, error) {
	selected, ok, err := s.models.AnalyzeModel(ctx, userID)
	if err != nil {
		return llm.ModelRef{}, fmt.Errorf("resolve analyze model: %w", err)
	}
	if !ok || requested.ProviderID == "" || requested.ModelID == "" || requested != selected {
		return llm.ModelRef{}, ErrAnalyzeModelRequired
	}
	return selected, nil
}

func (s *Service) enqueueAnalysis(ctx context.Context, userID, voiceID string, model llm.ModelRef) (string, error) {
	id, err := s.jobs.Enqueue(ctx, AnalysisJobRequest{UserID: userID, VoiceID: voiceID, WriteModel: model.String()})
	if err == nil {
		return id, nil
	}
	var active *JobAlreadyInProgressError
	if errors.As(err, &active) {
		return active.ActiveID, nil
	}
	return "", fmt.Errorf("enqueue analysis: %w", err)
}

func firstRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("voice: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

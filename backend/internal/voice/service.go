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
	store     Store
	models    Models
	jobs      Jobs
	now       func() time.Time
	newID     func() string
	profileMu sync.Mutex
	sampleMu  sync.Mutex
}

func NewService(store Store, models Models, jobs Jobs) *Service {
	return &Service{store: store, models: models, jobs: jobs, now: time.Now, newID: newID}
}

func (s *Service) Get(ctx context.Context, userID string) (Profile, error) {
	profile, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	samples, err := s.store.ListSamples(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("list samples: %w", err)
	}
	active, err := s.jobs.ActiveForUserKind(ctx, userID, AnalysisJobKind)
	if err != nil {
		return Profile{}, fmt.Errorf("get active analysis: %w", err)
	}
	profile.UserID = userID
	profile.Samples = samples
	if active != nil {
		profile.ActiveJobID = active.ID
	}
	return profile, nil
}

func (s *Service) Update(ctx context.Context, userID, styleguide, rules string) (Profile, error) {
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	if err := s.store.UpsertProfile(ctx, Profile{
		UserID: userID, Styleguide: styleguide, Rules: rules, UpdatedAt: s.now(),
	}); err != nil {
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return s.Get(ctx, userID)
}

// UpdateStyleguide changes only the generated/hand-edited guide. The SQL operation is
// deliberately field-scoped so a concurrent rules edit cannot be lost.
func (s *Service) UpdateStyleguide(ctx context.Context, userID, styleguide string) (Profile, error) {
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	if err := s.store.SetStyleguide(ctx, userID, styleguide, s.now()); err != nil {
		return Profile{}, fmt.Errorf("update styleguide: %w", err)
	}
	return s.Get(ctx, userID)
}

// UpdateRules changes only user-owned rules. In particular, it never writes the
// styleguide value observed by a client while an analysis may be completing.
func (s *Service) UpdateRules(ctx context.Context, userID, rules string) (Profile, error) {
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	if err := s.store.SetRules(ctx, userID, rules, s.now()); err != nil {
		return Profile{}, fmt.Errorf("update rules: %w", err)
	}
	return s.Get(ctx, userID)
}

func (s *Service) AddSample(ctx context.Context, userID, label, body string, requested llm.ModelRef) (Sample, string, error) {
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
		ID: s.newID(), UserID: userID, Label: label, Body: body, Chars: chars, CreatedAt: s.now(),
	}
	if err := s.store.InsertSample(ctx, sample); err != nil {
		return Sample{}, "", fmt.Errorf("insert sample: %w", err)
	}
	jobID, err := s.enqueueAnalysis(ctx, userID, model)
	if err != nil {
		_, cleanupErr := s.store.DeleteSample(ctx, userID, sample.ID, s.now())
		return Sample{}, "", errors.Join(ErrSampleMutation, err, cleanupErr)
	}
	return sample, jobID, nil
}

func (s *Service) DeleteSample(ctx context.Context, userID, sampleID string) (string, error) {
	s.sampleMu.Lock()
	defer s.sampleMu.Unlock()
	sample, err := s.store.GetSampleBody(ctx, userID, sampleID)
	if err != nil {
		return "", fmt.Errorf("get sample before delete: %w", err)
	}
	if sample == nil {
		return "", ErrSampleNotFound
	}
	before, err := s.store.CountSamples(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("count samples before delete: %w", err)
	}
	var model llm.ModelRef
	if before > 1 {
		var ok bool
		model, ok, err = s.models.AnalyzeModel(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("resolve analyze model: %w", err)
		}
		if !ok {
			return "", ErrAnalyzeModelRequired
		}
	}
	deleted, err := s.store.DeleteSample(ctx, userID, sampleID, s.now())
	if err != nil {
		return "", fmt.Errorf("delete sample: %w", err)
	}
	if !deleted {
		return "", ErrSampleNotFound
	}
	count, err := s.store.CountSamples(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("count samples: %w", err)
	}
	if count == 0 {
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
	jobID, err := s.enqueueAnalysis(ctx, userID, model)
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

func (s *Service) enqueueAnalysis(ctx context.Context, userID string, model llm.ModelRef) (string, error) {
	id, err := s.jobs.Enqueue(ctx, AnalysisJobRequest{UserID: userID, WriteModel: model.String()})
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

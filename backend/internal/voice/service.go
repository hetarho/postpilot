package voice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

type Service struct {
	directory      VoiceDirectoryStore
	profiles       ProfileStore
	samples        SampleStore
	versionSamples VersionSampleStore
	models         Models
	jobs           Jobs
	now            func() time.Time
	newID          func() string
	profileMu      sync.Mutex
	sampleMu       sync.Mutex
	directoryMu    sync.Mutex
	config         PersonalizationConfig
	photoUploads   PhotoUploadStore
	objects        ObjectStore
	photos         PhotoLimits
	// personalization is the versioned profile store as one handle, held only to answer "is
	// it wired at all" — every call goes through one of the narrow ports below it.
	personalization PersonalizationStorage
	versions        ProfileVersionStore
	overrides       ManualOverrideStore
}

func NewService(store Storage, models Models, jobs Jobs) *Service {
	svc := &Service{directory: store, profiles: store, samples: store, versionSamples: store, photoUploads: store, models: models, jobs: jobs, now: time.Now, newID: newID,
		config: PersonalizationThresholds()}
	if p, ok := store.(PersonalizationStorage); ok {
		svc.personalization = p
		svc.versions, svc.overrides = p, p
	}
	return svc
}

// ConfigurePhotos wires the private bucket a photo prompt's photo is stored in (VOICE-60).
func (s *Service) ConfigurePhotos(objects ObjectStore, limits PhotoLimits) {
	if objects == nil || limits.PutTTL <= 0 || limits.GetTTL <= 0 || limits.MaxBytes <= 0 {
		panic("voice: invalid photo storage configuration")
	}
	s.objects, s.photos = objects, limits
}

func (s *Service) EndingMaxConsecutive() int {
	return s.config.EndingMaxConsecutive
}

// --- directory ---

// ListVoices is the directory, each voice not yet made carrying its readiness (VOICE-9).
func (s *Service) ListVoices(ctx context.Context, userID string) ([]Voice, error) {
	voices, err := s.directory.ListVoices(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list voices: %w", err)
	}
	for i := range voices {
		if voices[i].Made {
			continue
		}
		bodies, err := s.samples.ListSampleBodies(ctx, userID, voices[i].ID)
		if err != nil {
			return nil, fmt.Errorf("list sample bodies: %w", err)
		}
		voices[i].ReadinessPercent = ReadinessOf(bodies).Percent
	}
	return voices, nil
}

func (s *Service) GetVoice(ctx context.Context, userID, voiceID string) (Voice, error) {
	return s.ownedVoice(ctx, userID, voiceID)
}

// CreateVoice makes a Korean voice that is not yet made (VOICE-10): the directory row and its
// empty profile, nothing else. It takes no language, description or 분야 and enqueues nothing.
func (s *Service) CreateVoice(ctx context.Context, userID, name string) (Voice, error) {
	name, err := normalizeVoiceName(name)
	if err != nil {
		return Voice{}, err
	}
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	now := s.now()
	created := Voice{ID: s.newID(), UserID: userID, Name: name, CreatedAt: now, UpdatedAt: now}
	if err := s.directory.InsertVoice(ctx, created); err != nil {
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
	if err := s.directory.RenameVoice(ctx, userID, voiceID, name, s.now()); err != nil {
		if errors.Is(err, ErrVoiceNameTaken) || errors.Is(err, ErrVoiceNotFound) {
			return Voice{}, err
		}
		return Voice{}, fmt.Errorf("rename voice: %w", err)
	}
	return s.ownedVoice(ctx, userID, voiceID)
}

// SetDefaultVoice makes one active, made voice the 기본 — clearing the previous one in the
// same store transaction — or, given an empty id, clears the 기본 so the account has none
// (VOICE-12). It returns the whole directory, since two rows may have changed.
func (s *Service) SetDefaultVoice(ctx context.Context, userID, voiceID string) ([]Voice, error) {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	if strings.TrimSpace(voiceID) == "" {
		if err := s.directory.ClearDefaultVoice(ctx, userID, s.now()); err != nil {
			return nil, fmt.Errorf("clear default voice: %w", err)
		}
		return s.ListVoices(ctx, userID)
	}
	found, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return nil, err
	}
	if !found.Made {
		return nil, ErrVoiceNotMade
	}
	if !found.IsDefault {
		if err := s.directory.SetDefaultVoice(ctx, userID, voiceID, s.now()); err != nil {
			if errors.Is(err, ErrVoiceNotFound) {
				return nil, err
			}
			return nil, fmt.Errorf("set default voice: %w", err)
		}
	}
	return s.ListVoices(ctx, userID)
}

// DeleteVoice is a soft delete (VOICE-13). It refuses only a voice with a queued or running
// job frozen to it, which could still publish into the voice; the 기본 and the last voice
// delete like any other and leave the account with none. Posts and profile history stay.
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
	if busy, err := s.jobs.HasActiveForVoice(ctx, voiceID); err != nil {
		return Voice{}, fmt.Errorf("check voice jobs: %w", err)
	} else if busy {
		return Voice{}, ErrVoiceBusy
	}
	deleted, err := s.directory.SoftDeleteVoice(ctx, userID, voiceID, s.now())
	if err != nil {
		if errors.Is(err, ErrVoiceBusy) {
			return Voice{}, err
		}
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
	if _, err := s.directory.RestoreVoice(ctx, userID, voiceID, s.now()); err != nil {
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
	found, err := s.directory.GetVoice(ctx, userID, voiceID)
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
	profile, err := s.profiles.GetProfile(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	samples, err := s.samples.ListSamples(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, fmt.Errorf("list samples: %w", err)
	}
	bodies, err := s.samples.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, fmt.Errorf("list sample bodies: %w", err)
	}
	profile.Readiness = ReadinessOf(bodies)
	active, err := s.jobs.ActiveForVoiceKind(ctx, voiceID, AnalysisJobKind)
	if err != nil {
		return Profile{}, fmt.Errorf("get active analysis: %w", err)
	}
	profile.UserID, profile.VoiceID, profile.Voice = userID, voiceID, found
	profile.Samples = samples
	if active != nil {
		profile.ActiveJobID = active.ID
	}
	return profile, nil
}

// RecordVersionSample copies the raw AI output of a post into the voice's CURRENT head
// version, so that version can be read before it is adopted (VOICE-29). It is called by the
// generation context after a machine baseline is written — the only context that depends on
// both post and voice, and therefore the only one allowed to join them.
//
// Idempotent per (voice, version): a later generation under the same head REPLACES the
// snapshot rather than adding a second one. A voice with no published version yet records
// nothing instead of inventing version 0, and a deleted voice records nothing at all.
//
// The version is the head AT COMPLETION, which is not the same thing as the version the
// prompt was built from: a profile published while the provider was working moves the head,
// and this output is then filed under that newer version. The window is the length of one
// provider call. Closing it would mean freezing the profile version into the generation
// payload and carrying it back out through the write path, which is a contract not opened
// here.
//
// `content` is opaque text. Nothing here parses it.
// RecordVersionSample files a generated post under the profile version its prompt was built
// from, not the head at completion, which may have moved while the provider wrote (VOICE-29).
// Version 0 is a prompt built from no published version, which files nothing.
func (s *Service) RecordVersionSample(ctx context.Context, userID, voiceID string, version int64, content string) error {
	if content == "" || version <= 0 {
		return nil
	}
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return err
	}
	if err := s.versionSamples.UpsertVersionSample(ctx, VersionSample{
		UserID: userID, VoiceID: voiceID, Version: version, Content: content, CreatedAt: s.now(),
	}); err != nil {
		return fmt.Errorf("record version sample: %w", err)
	}
	return nil
}

// VersionSample returns one version's snapshot. Both the voice and the account are named, so
// no cross-voice or cross-account read is expressible ([I4]). A version that never produced a
// post is ErrVersionSampleNotFound, which callers present as "no preview" rather than as a
// failure. A deleted voice's samples stay READABLE, like the rest of its profile.
func (s *Service) VersionSample(ctx context.Context, userID, voiceID string, version int64) (VersionSample, error) {
	if _, err := s.directory.GetVoice(ctx, userID, voiceID); err != nil {
		return VersionSample{}, err
	}
	return s.versionSamples.GetVersionSample(ctx, userID, voiceID, version)
}

// AddSample stores a pasted post as a 학습 글 (VOICE-20). It needs no model and enqueues
// nothing (VOICE-21): before the voice is made it moves the readiness meter, after it the
// analysis stays as it is until the owner presses 다시 분석.
func (s *Service) AddSample(ctx context.Context, userID, voiceID, label, body string) (Sample, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Sample{}, err
	}
	body = strings.TrimSpace(body)
	chars := utf8.RuneCountInString(body)
	if chars < SampleMinChars {
		return Sample{}, &SampleTooShortError{Chars: chars}
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = firstRunes(body, LabelFallbackChars)
	}
	sample := Sample{
		ID: s.newID(), UserID: userID, VoiceID: voiceID, Kind: SampleKindPost, Label: label, Body: body, Chars: chars, CreatedAt: s.now(),
	}
	if err := s.samples.InsertSample(ctx, sample); err != nil {
		return Sample{}, fmt.Errorf("insert sample: %w", err)
	}
	return sample, nil
}

// DeleteSample removes a 학습 글 and enqueues nothing (VOICE-21). The row goes first, then a
// photo answer's object; an object delete that fails is left for the photo sweep (→POST-39).
func (s *Service) DeleteSample(ctx context.Context, userID, voiceID, sampleID string) error {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return err
	}
	photoKey, deleted, err := s.samples.DeleteSample(ctx, userID, voiceID, sampleID, s.now())
	if err != nil {
		return fmt.Errorf("delete sample: %w", err)
	}
	if !deleted {
		return ErrSampleNotFound
	}
	if photoKey != "" && s.objects != nil {
		if err := s.objects.Delete(ctx, photoKey); err != nil {
			slog.WarnContext(ctx, "voice photo delete failed; the sweep reclaims it", "voice_id", voiceID, "err", err)
		}
	}
	return nil
}

// Prompts is the shared prompt set every voice answers (VOICE-60).
func (s *Service) Prompts() []Prompt { return Prompts() }

// CreatePhotoUpload presigns a photo prompt's photo: a private `voices/{voice_id}/{id}.jpg`
// key PUT as `image/jpeg`, with a pending row the sweep can reclaim (VOICE-60, →POST-34).
func (s *Service) CreatePhotoUpload(ctx context.Context, userID, voiceID, promptKey string) (PhotoUpload, string, error) {
	if s.objects == nil {
		return PhotoUpload{}, "", errors.New("voice: photo storage is not configured")
	}
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return PhotoUpload{}, "", err
	}
	prompt, ok := PromptByKey(promptKey)
	if !ok || !prompt.Photo {
		return PhotoUpload{}, "", ErrPromptNotFound
	}
	id := s.newID()
	now := s.now()
	upload := PhotoUpload{
		ID: id, UserID: userID, VoiceID: voiceID, PromptKey: promptKey,
		Key:       photoObjectPrefix + voiceID + "/" + id + ".jpg",
		ExpiresAt: now.Add(s.photos.PutTTL), CreatedAt: now,
	}
	url, err := s.objects.PresignPut(ctx, upload.Key, PhotoContentType, s.photos.PutTTL)
	if err != nil {
		return PhotoUpload{}, "", fmt.Errorf("presign voice photo: %w", err)
	}
	// After the presign: a row with no usable URL would be a reservation nothing can fill.
	if err := s.photoUploads.InsertPhotoUpload(ctx, upload); err != nil {
		return PhotoUpload{}, "", fmt.Errorf("insert voice photo upload: %w", err)
	}
	return upload, url, nil
}

// Answer is one prompt answered in the owner's words, on the photo it uploaded for a photo
// prompt.
type Answer struct {
	PromptKey, Body, UploadID string
	PhotoWidth, PhotoHeight   int
}

// AnswerPrompt stores an answer as a 학습 글 (VOICE-60): trimmed and non-empty, one per prompt,
// and a photo prompt's photo confirmed by a HEAD against the post photo's size and dimension
// limits (→POST-36). It enqueues nothing.
func (s *Service) AnswerPrompt(ctx context.Context, userID, voiceID string, answer Answer) (Sample, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Sample{}, err
	}
	prompt, ok := PromptByKey(answer.PromptKey)
	if !ok {
		return Sample{}, ErrPromptNotFound
	}
	body := strings.TrimSpace(answer.Body)
	if body == "" {
		return Sample{}, ErrAnswerRequired
	}
	sample := Sample{
		ID: s.newID(), UserID: userID, VoiceID: voiceID, Kind: SampleKindAnswer, PromptKey: prompt.Key,
		Body: body, Chars: utf8.RuneCountInString(body), CreatedAt: s.now(),
	}
	uploadID := ""
	if prompt.Photo {
		if answer.UploadID == "" || s.objects == nil {
			return Sample{}, ErrPhotoRequired
		}
		upload, err := s.photoUploads.GetPhotoUpload(ctx, userID, voiceID, answer.UploadID)
		if err != nil {
			return Sample{}, err
		}
		if upload.PromptKey != prompt.Key {
			return Sample{}, ErrPhotoRequired
		}
		if answer.PhotoWidth <= 0 || answer.PhotoHeight <= 0 || answer.PhotoWidth > MaxPhotoDimension || answer.PhotoHeight > MaxPhotoDimension {
			return Sample{}, ErrInvalidPhoto
		}
		head, err := s.objects.Head(ctx, upload.Key)
		if err != nil {
			if errors.Is(err, ErrObjectNotFound) {
				return Sample{}, ErrPhotoRequired
			}
			return Sample{}, fmt.Errorf("head voice photo: %w", err)
		}
		if head.Size <= 0 || head.Size > s.photos.MaxBytes {
			// Dropped at once rather than left for the sweep (→POST-36).
			if err := s.objects.Delete(ctx, upload.Key); err == nil {
				_ = s.photoUploads.DeletePhotoUpload(ctx, upload.ID)
			}
			return Sample{}, ErrInvalidPhoto
		}
		sample.PhotoKey, sample.PhotoWidth, sample.PhotoHeight = upload.Key, answer.PhotoWidth, answer.PhotoHeight
		uploadID = upload.ID
	}
	if err := s.samples.AnswerPrompt(ctx, sample, uploadID); err != nil {
		if errors.Is(err, ErrPromptAnswered) {
			return Sample{}, err
		}
		return Sample{}, fmt.Errorf("answer prompt: %w", err)
	}
	return sample, nil
}

// GetSample opens one 학습 글 for its owner: the full text and, for a photo answer, a view URL
// minted fresh on each read (VOICE-8, VOICE-64). A deleted voice's 학습 글 stay readable.
func (s *Service) GetSample(ctx context.Context, userID, voiceID, sampleID string) (Sample, string, error) {
	if _, err := s.ownedVoice(ctx, userID, voiceID); err != nil {
		return Sample{}, "", err
	}
	sample, err := s.samples.GetSampleBody(ctx, userID, voiceID, sampleID)
	if err != nil {
		return Sample{}, "", fmt.Errorf("get sample: %w", err)
	}
	if sample == nil {
		return Sample{}, "", ErrSampleNotFound
	}
	url := ""
	if sample.HasPhoto() && s.objects != nil {
		if url, err = s.objects.PresignGet(ctx, sample.PhotoKey, s.photos.GetTTL); err != nil {
			return Sample{}, "", fmt.Errorf("presign voice photo view: %w", err)
		}
	}
	return *sample, url, nil
}

// AnalyzeVoice is 말투 만들기 and 다시 분석 (VOICE-23): one durable `analyze_voice` job on the
// model the request names, which must be enabled and registered to the analyze stage. It needs
// the 학습 글 at 100% (VOICE_NOT_READY otherwise); a voice already analysing is refused by the
// per-(voice, kind) guard.
func (s *Service) AnalyzeVoice(ctx context.Context, userID, voiceID string, model llm.ModelRef) (string, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return "", err
	}
	info, found := s.models.Resolve(model)
	if model.ProviderID == "" || model.ModelID == "" || !found || info.Disabled || !info.ServesStage(llm.StageNameAnalyze) {
		return "", ErrAnalyzeModelRequired
	}
	samples, err := s.samples.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return "", fmt.Errorf("list samples: %w", err)
	}
	if !ReadinessOf(samples).Ready() {
		return "", ErrVoiceNotReady
	}
	id, err := s.jobs.Enqueue(ctx, AnalysisJobRequest{UserID: userID, VoiceID: voiceID, WriteModel: model.String()})
	if err != nil {
		var active *JobAlreadyInProgressError
		if errors.As(err, &active) {
			return "", ErrVoiceBusy
		}
		return "", fmt.Errorf("enqueue analysis: %w", err)
	}
	return id, nil
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

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
	directory    VoiceDirectoryStore
	analyses     AnalysisStore
	samples      SampleStore
	photoUploads PhotoUploadStore
	checks       CheckStore
	models       Models
	jobs         Jobs
	now          func() time.Time
	newID        func() string
	directoryMu  sync.Mutex
	objects      ObjectStore
	photos       PhotoLimits
	posts        PostContents
}

func NewService(store Storage, models Models, jobs Jobs) *Service {
	return &Service{directory: store, analyses: store, samples: store, photoUploads: store, checks: store, models: models, jobs: jobs, now: time.Now, newID: newID}
}

// ConfigurePhotos wires the private bucket a photo prompt's photo is stored in (VOICE-60).
func (s *Service) ConfigurePhotos(objects ObjectStore, limits PhotoLimits) {
	if objects == nil || limits.PutTTL <= 0 || limits.GetTTL <= 0 || limits.MaxBytes <= 0 {
		panic("voice: invalid photo storage configuration")
	}
	s.objects, s.photos = objects, limits
}

// --- directory ---

// ListVoices is the directory, each voice not yet made carrying its readiness (VOICE-9). The
// bodies of every such voice are read at once, however many there are.
func (s *Service) ListVoices(ctx context.Context, userID string) ([]Voice, error) {
	voices, err := s.directory.ListVoices(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list voices: %w", err)
	}
	var unmade []string
	for _, found := range voices {
		if !found.Made {
			unmade = append(unmade, found.ID)
		}
	}
	if len(unmade) == 0 {
		return voices, nil
	}
	bodies, err := s.samples.ListSampleBodiesForVoices(ctx, userID, unmade)
	if err != nil {
		return nil, fmt.Errorf("list sample bodies: %w", err)
	}
	for i := range voices {
		if !voices[i].Made {
			voices[i].ReadinessPercent = ReadinessOf(bodies[voices[i].ID]).Percent
		}
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

// Get is a voice as its tabs read it: the 학습 글, the readiness meter, the current analysis with
// the examples whose 학습 글 still exist, whether a previous analysis exists, and the notice. One
// read of the 학습 글 serves both the list and the meter.
func (s *Service) Get(ctx context.Context, userID, voiceID string) (Profile, error) {
	found, err := s.ownedVoice(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, err
	}
	bodies, err := s.samples.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, fmt.Errorf("list sample bodies: %w", err)
	}
	active, err := s.jobs.ActiveForVoiceKind(ctx, voiceID, AnalysisJobKind)
	if err != nil {
		return Profile{}, fmt.Errorf("get active analysis: %w", err)
	}
	samples := make([]Sample, len(bodies))
	for i, sample := range bodies {
		// The list carries no text: a 학습 글 is opened one at a time (GetSample).
		sample.Body = ""
		samples[i] = sample
	}
	profile := Profile{UserID: userID, VoiceID: voiceID, Voice: found, Samples: samples, Readiness: ReadinessOf(bodies)}
	if active != nil {
		profile.ActiveJobID = active.ID
	}
	current, err := s.analyses.CurrentAnalysis(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, fmt.Errorf("current analysis: %w", err)
	}
	if current != nil {
		present := make(map[string]bool, len(samples))
		for _, sample := range samples {
			present[sample.ID] = true
		}
		visible := withoutDeletedExamples(*current, present)
		profile.Analysis = &visible
		profile.Notice = noticeOf(current.MaterialIDs, present)
		if profile.HasPrevious, err = s.analyses.HasPreviousAnalysis(ctx, userID, voiceID); err != nil {
			return Profile{}, fmt.Errorf("previous analysis: %w", err)
		}
	}
	return profile, nil
}

// noticeOf compares the 학습 글 an analysis read with the ones there now (VOICE-21): a read one
// gone is `changed`, else the count of ones added since.
func noticeOf(read []string, present map[string]bool) Notice {
	readSet := make(map[string]bool, len(read))
	for _, id := range read {
		if !present[id] {
			return Notice{Kind: NoticeChanged}
		}
		readSet[id] = true
	}
	added := 0
	for id := range present {
		if !readSet[id] {
			added++
		}
	}
	if added == 0 {
		return Notice{}
	}
	return Notice{Kind: NoticeAdded, Count: added}
}

// withoutDeletedExamples drops every example whose 학습 글 was deleted: the owner took that prose
// back (VOICE-21).
func withoutDeletedExamples(analysis Analysis, present map[string]bool) Analysis {
	keep := func(example Example) Example {
		if example.MaterialID != "" && !present[example.MaterialID] {
			return Example{}
		}
		return example
	}
	counted := analysis.Counted
	counted.Endings.Example = keep(counted.Endings.Example)
	counted.Marks.Example = keep(counted.Marks.Example)
	counted.Emoji.Example = keep(counted.Emoji.Example)
	counted.Shape.Example = keep(counted.Shape.Example)
	counted.OpenClose.Example = keep(counted.OpenClose.Example)
	counted.Adverbs.Example = keep(counted.Adverbs.Example)
	counted.Person.Example = keep(counted.Person.Example)
	counted.Headings.Example = keep(counted.Headings.Example)
	analysis.Counted = counted
	var examples []AIExample
	for _, example := range analysis.AI.Examples {
		if present[example.MaterialID] {
			examples = append(examples, example)
		}
	}
	analysis.AI.Examples = examples
	return analysis
}

// RestorePreviousAnalysis is 이전 분석으로 되돌리기 (VOICE-30): the previous analysis becomes
// current and the one it replaced is discarded. There is no redo, and a tombstone offers none.
func (s *Service) RestorePreviousAnalysis(ctx context.Context, userID, voiceID string) (Profile, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Profile{}, err
	}
	if err := s.analyses.RestorePreviousAnalysis(ctx, userID, voiceID); err != nil {
		if errors.Is(err, ErrNoPreviousAnalysis) {
			return Profile{}, err
		}
		return Profile{}, fmt.Errorf("restore previous analysis: %w", err)
	}
	return s.Get(ctx, userID, voiceID)
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
// limits (→POST-36). Answering a prompt that holds an answer rewrites it: the new answer
// replaces the old one, keeping its photo unless a new one is uploaded; saving the answer the
// prompt already holds, on the photo it already has, changes nothing. It enqueues nothing.
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
	previous, err := s.answerTo(ctx, userID, voiceID, prompt.Key)
	if err != nil {
		return Sample{}, err
	}
	// The same text, and no new photo, is the answer the prompt already holds: it keeps its id,
	// so the profile reports no change and nudges no paid 다시 분석 (VOICE-21).
	if previous != nil && previous.Body == body && answer.UploadID == "" && (!prompt.Photo || previous.HasPhoto()) {
		return *previous, nil
	}
	sample := Sample{
		ID: s.newID(), UserID: userID, VoiceID: voiceID, Kind: SampleKindAnswer, PromptKey: prompt.Key,
		Body: body, Chars: utf8.RuneCountInString(body), CreatedAt: s.now(),
	}
	uploadID := ""
	if prompt.Photo && answer.UploadID == "" && previous != nil && previous.HasPhoto() {
		// A rewrite that picks no new photo stays on the photo the answer was written about.
		sample.PhotoKey, sample.PhotoWidth, sample.PhotoHeight = previous.PhotoKey, previous.PhotoWidth, previous.PhotoHeight
	} else if prompt.Photo {
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
	replaceID := ""
	if previous != nil {
		replaceID = previous.ID
	}
	if err := s.samples.AnswerPrompt(ctx, sample, uploadID, replaceID); err != nil {
		if errors.Is(err, ErrPromptAnswered) {
			return Sample{}, err
		}
		return Sample{}, fmt.Errorf("answer prompt: %w", err)
	}
	// A rewrite on a new photo drops the old object after its row (→POST-39).
	if previous != nil && previous.HasPhoto() && previous.PhotoKey != sample.PhotoKey && s.objects != nil {
		if err := s.objects.Delete(ctx, previous.PhotoKey); err != nil {
			slog.WarnContext(ctx, "voice photo delete failed; the sweep reclaims it", "voice_id", voiceID, "err", err)
		}
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
// per-(voice, kind) guard. The job freezes the 학습 글 it will read (VOICE-22) and declares the
// prompt it will send over them, so its hold covers a large corpus (QUOTA-14).
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
	counted, oldestFirst, ids := analysisSnapshot(samples)
	id, err := s.jobs.Enqueue(ctx, AnalysisJobRequest{
		UserID: userID, VoiceID: voiceID, WriteModel: model.String(),
		MaterialIDs: ids, PromptTokens: promptTokens(analysisRequest(counted, oldestFirst)),
	})
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

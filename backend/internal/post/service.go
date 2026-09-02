package post

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"
)

// Service is the drafting context's behavior. Every method takes the acting user id
// from the caller (the interceptor put it in the context) and never from a payload.
type Service struct {
	store         Store
	blobs         ObjectStore
	putTTL        time.Duration
	getTTL        time.Duration
	maxBytes      int64
	jobs          ActiveJobFinder
	experiments   PendingExperimentFinder
	contentPurger ExperimentContentPurger
	livePublish   LivePublishFinder
	voices        VoiceDirectory
	purposes      PurposeDirectory

	// now and newID are seams for tests in this package, not configuration.
	now   func() time.Time
	newID func() string
}

// SetPendingExperimentFinder wires the experiment projection after both contexts have
// been constructed in the composition root.
func (s *Service) SetPendingExperimentFinder(finder PendingExperimentFinder) {
	s.experiments = finder
}

func (s *Service) SetExperimentContentPurger(purger ExperimentContentPurger) {
	s.contentPurger = purger
}

// SetLivePublishFinder wires the publishing context's in-flight query, which only exists
// once both services have been constructed.
func (s *Service) SetLivePublishFinder(finder LivePublishFinder) {
	s.livePublish = finder
}

// SetVoiceDirectory wires the voice context's published directory. Without it no post can
// be created — failing closed is the point, since a post must name an owned active voice.
func (s *Service) SetVoiceDirectory(directory VoiceDirectory) {
	s.voices = directory
}

// SetPurposeDirectory wires the purpose context's published directory. Unlike the voice
// directory its absence is survivable: a post needs no purpose, so without it assignment is
// simply refused and read models project the stored id with no name.
func (s *Service) SetPurposeDirectory(directory PurposeDirectory) {
	s.purposes = directory
}

// NewService wires the context with its store, its object storage, the presigned URL
// lifetimes, and the largest object it will accept as a photo.
func NewService(store Store, blobs ObjectStore, putTTL, getTTL time.Duration, maxBytes int64, jobs ...ActiveJobFinder) *Service {
	svc := &Service{
		store:    store,
		blobs:    blobs,
		putTTL:   putTTL,
		getTTL:   getTTL,
		maxBytes: maxBytes,
		now:      time.Now,
		newID:    newObjectID,
	}
	if len(jobs) > 0 {
		svc.jobs = jobs[0]
	}
	return svc
}

// SaveDraft creates the post when slug is empty, otherwise updates the caller's own.
//
// It is the autosave endpoint, so it is called every second or so while someone types:
// repeated saves are plain idempotent updates, and only the very first one mints a slug.
//
// voiceID is presence-aware, which is what lets autosave keep patching title and memo:
// required on create, nil preserves the current assignment, and a different present value
// asks for a reassignment. An empty string is never valid — the server substitutes no default.
//
// purposeID is presence-aware too, with one more case, because a post may legitimately have
// none: nil preserves, a present empty string clears, and a present non-empty value assigns.
// It is validated before anything else is written, so a bad id applies nothing at all.
func (s *Service) SaveDraft(ctx context.Context, userID, slug, title, memo string, voiceID, purposeID *string, targetLanguage *Language) (Post, error) {
	if targetLanguage != nil && !targetLanguage.Valid() {
		return Post{}, ErrLanguageRequired
	}
	// Ahead of every write, including the create: a request naming an unknown or foreign
	// purpose must leave the post exactly as it was, title and memo included.
	targetPurpose, err := s.assignablePurpose(ctx, userID, purposeID)
	if err != nil {
		return Post{}, err
	}

	if slug == "" {
		if targetLanguage == nil {
			return Post{}, ErrLanguageRequired
		}
		if voiceID == nil {
			return Post{}, ErrVoiceRequired
		}
		target, err := s.activeVoice(ctx, userID, *voiceID)
		if err != nil {
			return Post{}, err
		}
		return s.createPost(ctx, userID, title, memo, target.ID, targetPurpose, *targetLanguage)
	}

	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return Post{}, err
	}
	if voiceID != nil && *voiceID != found.VoiceID {
		if err := s.reassignVoice(ctx, found, *voiceID); err != nil {
			return Post{}, err
		}
	}
	if purposeID != nil && targetPurpose != found.PurposeID {
		if _, err := s.store.AssignPurpose(ctx, slug, userID, &targetPurpose, s.now()); err != nil {
			return Post{}, fmt.Errorf("assign purpose: %w", err)
		}
	}

	now := s.now()
	updated, err := s.store.UpdateDraft(ctx, slug, userID, title, memo, targetLanguage, now)
	if err != nil {
		return Post{}, fmt.Errorf("update draft: %w", err)
	}
	if !updated {
		// ownedPost just succeeded, so a miss here means the row vanished between the
		// two statements. Report it as gone rather than inventing a post.
		return Post{}, ErrNotFound
	}

	return s.Get(ctx, userID, slug)
}

// reassignVoice moves an idle post to another active owned voice. It is refused while a
// job or an undecided write experiment could still apply output written for the old voice;
// otherwise the store's single UPDATE changes the id and clears the machine baseline's voice
// association, which is what withdraws learn eligibility until a fresh machine result.
func (s *Service) reassignVoice(ctx context.Context, found Post, voiceID string) error {
	target, err := s.activeVoice(ctx, found.UserID, voiceID)
	if err != nil {
		return err
	}
	if s.jobs != nil {
		active, err := s.jobs.ActiveForPost(ctx, found.Slug)
		if err != nil {
			return fmt.Errorf("check active job before reassignment: %w", err)
		}
		if active != nil {
			return ErrPostBusy
		}
	}
	if s.experiments != nil {
		pending, err := s.experiments.PendingForPost(ctx, found.UserID, found.Slug)
		if err != nil {
			return fmt.Errorf("check pending experiment before reassignment: %w", err)
		}
		if pending != "" {
			return ErrPostBusy
		}
	}
	changed, err := s.store.ReassignVoice(ctx, found.Slug, found.UserID, target.ID, s.now())
	if err != nil {
		return fmt.Errorf("reassign voice: %w", err)
	}
	if !changed {
		// Zero rows means another save already moved it; only a vanished post is an error.
		current, err := s.ownedPost(ctx, found.UserID, found.Slug)
		if err != nil {
			return err
		}
		if current.VoiceID != target.ID {
			return ErrNotFound
		}
	}
	return nil
}

// activeVoice resolves an owned, active voice through the directory port. Missing and
// foreign ids read the same, and a tombstone is refused: a new post or a reassignment may
// only target a voice that can still receive AI work.
func (s *Service) activeVoice(ctx context.Context, userID, voiceID string) (VoiceRef, error) {
	if strings.TrimSpace(voiceID) == "" {
		return VoiceRef{}, ErrVoiceRequired
	}
	if s.voices == nil {
		return VoiceRef{}, errors.New("voice directory is not configured")
	}
	voices, err := s.voices.Voices(ctx, userID)
	if err != nil {
		return VoiceRef{}, fmt.Errorf("list voices: %w", err)
	}
	for _, v := range voices {
		if v.ID != voiceID {
			continue
		}
		if v.Deleted {
			return VoiceRef{}, ErrVoiceDeleted
		}
		return v, nil
	}
	return VoiceRef{}, ErrVoiceNotFound
}

// voiceRefs names every voice the account owns, tombstones included, so a read model can
// label a post whose voice was deleted. Without a directory the id alone is projected.
func (s *Service) voiceRefs(ctx context.Context, userID string) (map[string]VoiceRef, error) {
	if s.voices == nil {
		return nil, nil
	}
	voices, err := s.voices.Voices(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list voices: %w", err)
	}
	out := make(map[string]VoiceRef, len(voices))
	for _, v := range voices {
		out[v.ID] = v
	}
	return out, nil
}

// assignablePurpose resolves the presence-aware field to the id that should end up on the
// post: "" for absent-or-cleared, otherwise an owned purpose's id. An unknown or foreign id
// is refused here, before any other part of the request is applied.
func (s *Service) assignablePurpose(ctx context.Context, userID string, purposeID *string) (string, error) {
	if purposeID == nil || strings.TrimSpace(*purposeID) == "" {
		return "", nil
	}
	wanted := strings.TrimSpace(*purposeID)
	if s.purposes == nil {
		return "", errors.New("purpose directory is not configured")
	}
	purposes, err := s.purposes.Purposes(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("list purposes: %w", err)
	}
	for _, p := range purposes {
		if p.ID == wanted {
			return p.ID, nil
		}
	}
	return "", ErrPurposeNotFound
}

// purposeRefs names every purpose the account owns so a read model can label an assignment.
// Without a directory the stored id alone is projected, exactly as voiceRefs does.
func (s *Service) purposeRefs(ctx context.Context, userID string) (map[string]PurposeRef, error) {
	if s.purposes == nil {
		return nil, nil
	}
	purposes, err := s.purposes.Purposes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list purposes: %w", err)
	}
	out := make(map[string]PurposeRef, len(purposes))
	for _, p := range purposes {
		out[p.ID] = p
	}
	return out, nil
}

func projectPurpose(refs map[string]PurposeRef, purposeID string) PurposeRef {
	if purposeID == "" {
		return PurposeRef{}
	}
	if ref, ok := refs[purposeID]; ok {
		return ref
	}
	return PurposeRef{ID: purposeID}
}

func projectVoice(refs map[string]VoiceRef, voiceID string) VoiceRef {
	if ref, ok := refs[voiceID]; ok {
		return ref
	}
	return VoiceRef{ID: voiceID}
}

// slugAttempts bounds the mint-and-insert retry. Minting reads the slugs that exist,
// so a retry only happens when another request took the candidate in between — rare,
// and each attempt sees one more taken slug, so it converges immediately.
const slugAttempts = 5

func (s *Service) createPost(ctx context.Context, userID, title, memo, voiceID, purposeID string, targetLanguage Language) (Post, error) {
	now := s.now()

	// Mint-then-insert is a check-then-act, so the insert is what actually decides:
	// two first-saves of the same title on the same day can mint the same candidate,
	// and the loser must get the next serial rather than an error.
	for attempt := 0; attempt < slugAttempts; attempt++ {
		// exists is consulted per candidate rather than up front: the collision case is
		// rare, and pre-loading every slug for the day to avoid one query would be worse.
		var lookupErr error
		slug := MintSlug(now.UTC().Format("20060102"), title, func(candidate string) bool {
			if lookupErr != nil {
				return false
			}
			taken, err := s.store.SlugExists(ctx, candidate)
			if err != nil {
				lookupErr = err
			}
			return taken
		})
		if lookupErr != nil {
			return Post{}, fmt.Errorf("check slug: %w", lookupErr)
		}

		created := Post{
			Slug:           slug,
			UserID:         userID,
			VoiceID:        voiceID,
			PurposeID:      purposeID,
			Title:          title,
			Memo:           memo,
			Status:         StatusDraft,
			TargetLanguage: targetLanguage,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		err := s.store.CreatePost(ctx, created)
		if err == nil {
			return s.Get(ctx, userID, slug)
		}
		if !errors.Is(err, ErrDuplicateSlug) {
			return Post{}, fmt.Errorf("create post: %w", err)
		}
		// Someone took it between the check and the insert. The next mint sees it.
	}

	return Post{}, fmt.Errorf("create post: could not mint a free slug in %d attempts", slugAttempts)
}

// Get returns the caller's post with a fresh view URL on every image.
//
// The URLs are minted here, per read, rather than stored: the bucket is private and a
// stored URL would either be long-lived (a durable capability to a private object) or
// already expired by the time it is used.
func (s *Service) Get(ctx context.Context, userID, slug string) (Post, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return Post{}, err
	}

	images, err := s.store.ListImages(ctx, slug)
	if err != nil {
		return Post{}, fmt.Errorf("list images: %w", err)
	}
	for i := range images {
		url, err := s.blobs.PresignGet(ctx, images[i].Key, s.getTTL)
		if err != nil {
			return Post{}, fmt.Errorf("presign view url for %s: %w", images[i].Filename, err)
		}
		images[i].ViewURL = url
	}

	found.Images = images
	if s.jobs != nil {
		found.ActiveJob, err = s.jobs.ActiveForPost(ctx, slug)
		if err != nil {
			return Post{}, fmt.Errorf("load active job: %w", err)
		}
	}
	if s.experiments != nil {
		found.PendingExperimentID, err = s.experiments.PendingForPost(ctx, userID, slug)
		if err != nil {
			return Post{}, fmt.Errorf("load pending experiment: %w", err)
		}
	}
	refs, err := s.voiceRefs(ctx, userID)
	if err != nil {
		return Post{}, err
	}
	found.Voice = projectVoice(refs, found.VoiceID)
	purposeRefs, err := s.purposeRefs(ctx, userID)
	if err != nil {
		return Post{}, err
	}
	found.Purpose = projectPurpose(purposeRefs, found.PurposeID)
	return found, nil
}

// List returns the caller's posts, newest first.
func (s *Service) List(ctx context.Context, userID string) ([]Summary, error) {
	summaries, err := s.store.ListPosts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	if s.jobs != nil {
		for i := range summaries {
			summaries[i].ActiveJob, err = s.jobs.ActiveForPost(ctx, summaries[i].Slug)
			if err != nil {
				return nil, fmt.Errorf("load active job for %s: %w", summaries[i].Slug, err)
			}
		}
	}
	if s.experiments != nil {
		for i := range summaries {
			summaries[i].PendingExperimentID, err = s.experiments.PendingForPost(ctx, userID, summaries[i].Slug)
			if err != nil {
				return nil, fmt.Errorf("load pending experiment for %s: %w", summaries[i].Slug, err)
			}
		}
	}
	refs, err := s.voiceRefs(ctx, userID)
	if err != nil {
		return nil, err
	}
	purposeRefs, err := s.purposeRefs(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range summaries {
		summaries[i].Voice = projectVoice(refs, summaries[i].VoiceID)
		summaries[i].Purpose = projectPurpose(purposeRefs, summaries[i].PurposeID)
	}
	return summaries, nil
}

// DeletePost removes one owned post only after sensitive experiment payloads have been
// scrubbed. The durable experiment metadata remains, and its FK is then set to NULL.
func (s *Service) DeletePost(ctx context.Context, userID, slug string) error {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return err
	}
	images, err := s.store.ListImages(ctx, found.Slug)
	if err != nil {
		return fmt.Errorf("list images for post delete: %w", err)
	}
	if s.jobs != nil {
		active, err := s.jobs.ActiveForPost(ctx, slug)
		if err != nil {
			return fmt.Errorf("check active job before post delete: %w", err)
		}
		if active != nil {
			return ErrPostBusy
		}
	}
	// Both remaining preconditions must clear BEFORE the purge: past this point the
	// deletion is destructive and cannot be undone. Unlike the generation-job port
	// above, this one is deliberately not nil-tolerant — a server that cannot ask
	// whether a publication is live must not delete a post out from under an agent.
	//
	// It is a read, not a lock, so a publication started between this answer and the row
	// delete is not prevented. That end state is one the publishing schema was already
	// built for: the frozen manifest and the staged assets are the job's own, under
	// publish_assets.staged_key rather than the post's prefix, and LatestJobForDeletedPost
	// exists to serve exactly a publication whose post is gone. Closing the window would
	// take a transactional gate spanning both contexts.
	if s.livePublish == nil {
		return errors.New("live publish finder is not configured")
	}
	live, err := s.livePublish.LiveForPost(ctx, userID, found.Slug, found.CreatedAt)
	if err != nil {
		return fmt.Errorf("check live publish job before post delete: %w", err)
	}
	if live {
		return ErrPostPublishing
	}
	if s.contentPurger == nil {
		return errors.New("experiment content purger is not configured")
	}
	if err := s.contentPurger.PurgePost(ctx, userID, slug); err != nil {
		return fmt.Errorf("purge experiment content: %w", err)
	}
	for _, image := range images {
		if err := s.blobs.Delete(ctx, image.Key); err != nil {
			return fmt.Errorf("delete post image %s: %w", image.ID, err)
		}
	}
	deleted, err := s.store.DeletePost(ctx, slug, userID)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

// AttachedImages returns an ownership-checked generation snapshot without presigning
// browser URLs. Object keys remain backend-only and are consumed by the generation
// context's storage port. The voice is projected too, since a consumer must refuse to
// write in a deleted voice before it calls a provider.
func (s *Service) AttachedImages(ctx context.Context, userID, slug string) (Post, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return Post{}, err
	}
	found.Images, err = s.store.ListImages(ctx, slug)
	if err != nil {
		return Post{}, fmt.Errorf("list attached images: %w", err)
	}
	refs, err := s.voiceRefs(ctx, userID)
	if err != nil {
		return Post{}, err
	}
	found.Voice = projectVoice(refs, found.VoiceID)
	return found, nil
}

// SetObservations replaces the persisted contact sheet snapshot for one owned post.
func (s *Service) SetObservations(ctx context.Context, userID, slug string, observations []Observation) error {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(found.Observations, observations) {
		return nil
	}
	updated, err := s.store.UpdateObservations(ctx, slug, userID, observations, s.now())
	if err != nil {
		return err
	}
	if !updated {
		return ErrNotFound
	}
	return nil
}

// SetGeneratedContent atomically replaces canonical content and moves the post to review.
func (s *Service) SetGeneratedContent(ctx context.Context, userID, slug string, content PostContent, language Language) error {
	if !language.Valid() {
		return ErrLanguageRequired
	}
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return err
	}
	if found.Status == StatusReview && found.MachineBaselineRevision == found.ContentRevision && found.Content != nil && found.ContentLanguage != nil && *found.ContentLanguage == language && reflect.DeepEqual(*found.Content, content) {
		return nil
	}
	images, err := s.store.ListImages(ctx, slug)
	if err != nil {
		return fmt.Errorf("list images for generated content: %w", err)
	}
	if err := ValidateContent(content, images); err != nil {
		return err
	}
	updated, err := s.store.UpdateGeneratedContent(ctx, slug, userID, content, language, s.now())
	if err != nil {
		return err
	}
	if !updated {
		// The store predicate makes an identical review write atomic under concurrent
		// retries. Re-read after a zero-row update so the loser succeeds without
		// advancing the revision, while deletion/ownership changes remain errors.
		current, loadErr := s.ownedPost(ctx, userID, slug)
		if loadErr == nil && current.Status == StatusReview && current.MachineBaselineRevision == current.ContentRevision && current.Content != nil && current.ContentLanguage != nil && *current.ContentLanguage == language && reflect.DeepEqual(*current.Content, content) {
			return nil
		}
		if loadErr != nil {
			return loadErr
		}
		return ErrNotFound
	}
	return nil
}

// SaveContent optimistically saves only canonical content. The machine baseline is
// intentionally absent from the store operation and remains immutable.
func (s *Service) SaveContent(ctx context.Context, userID, slug string, content PostContent, expectedRevision int64) (Post, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return Post{}, err
	}
	if found.ContentRevision != expectedRevision {
		return Post{}, ErrStaleContentRevision
	}
	if found.Content != nil && reflect.DeepEqual(*found.Content, content) {
		return s.Get(ctx, userID, slug)
	}
	images, err := s.store.ListImages(ctx, slug)
	if err != nil {
		return Post{}, fmt.Errorf("list images for content save: %w", err)
	}
	if err := ValidateContent(content, images); err != nil {
		return Post{}, err
	}
	contentStore, ok := s.store.(ContentStore)
	if !ok {
		return Post{}, errors.New("post content store is not configured")
	}
	updated, err := contentStore.SaveContent(ctx, slug, userID, content, expectedRevision, s.now())
	if err != nil {
		return Post{}, err
	}
	if !updated {
		return Post{}, ErrStaleContentRevision
	}
	return s.Get(ctx, userID, slug)
}

func (s *Service) SaveGenerationOptions(ctx context.Context, userID, slug string, targetLength *int) (Post, error) {
	if targetLength != nil && *targetLength <= 0 {
		return Post{}, &InvalidContentError{Reason: "target length must be positive"}
	}
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return Post{}, err
	}
	if equalOptionalInt(found.TargetLength, targetLength) {
		return s.Get(ctx, userID, slug)
	}
	contentStore, ok := s.store.(ContentStore)
	if !ok {
		return Post{}, errors.New("post content store is not configured")
	}
	updated, err := contentStore.SaveGenerationOptions(ctx, slug, userID, targetLength, s.now())
	if err != nil {
		return Post{}, err
	}
	if !updated {
		return Post{}, ErrNotFound
	}
	return s.Get(ctx, userID, slug)
}

func (s *Service) Finalize(ctx context.Context, userID, slug string, expectedRevision int64) (Post, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return Post{}, err
	}
	if found.ContentRevision != expectedRevision {
		return Post{}, ErrStaleContentRevision
	}
	if found.Content == nil {
		return Post{}, ErrNoMachineBaseline
	}
	if found.Status == StatusFinalized && found.FinalizedRevision == expectedRevision {
		return s.Get(ctx, userID, slug)
	}
	contentStore, ok := s.store.(ContentStore)
	if !ok {
		return Post{}, errors.New("post content store is not configured")
	}
	// The confirmed AI title becomes the post's title (spec/policy/posts.md). An untitled
	// generation leaves the user's working title in place rather than blanking the list row, and
	// the slug is not re-minted: a post keeps the URL it was created with.
	title := strings.TrimSpace(found.Content.Title)
	if title == "" {
		title = found.Title
	}
	updated, err := contentStore.Finalize(ctx, slug, userID, title, expectedRevision, s.now())
	if err != nil {
		return Post{}, err
	}
	if !updated {
		return Post{}, ErrStaleContentRevision
	}
	return s.Get(ctx, userID, slug)
}

func (s *Service) LearningSnapshot(ctx context.Context, userID, slug string) (LearningSnapshot, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return LearningSnapshot{}, err
	}
	if found.Status != StatusFinalized || found.FinalizedRevision != found.ContentRevision {
		return LearningSnapshot{}, ErrPostNotFinalized
	}
	contentStore, ok := s.store.(ContentStore)
	if !ok {
		return LearningSnapshot{}, errors.New("post content store is not configured")
	}
	snapshot, err := contentStore.LearningSnapshot(ctx, slug, userID)
	if err != nil {
		return LearningSnapshot{}, err
	}
	// The post store owns content provenance but does not own the voice directory. Enrich the
	// hand-off through the published directory projection so the voice context can enforce
	// equality without reading either sibling's tables. ownedPost intentionally returns only
	// stored post state, so resolve the reference here just as Get and PublishingSnapshot do.
	refs, err := s.voiceRefs(ctx, userID)
	if err != nil {
		return LearningSnapshot{}, err
	}
	snapshot.VoiceSourceLanguage = projectVoice(refs, found.VoiceID).SourceLanguage
	return snapshot, nil
}

// PublishingSnapshot returns the exact current finalized revision without changing any
// post, voice, generation, experiment, or export state. It deliberately does not call
// Get: that read mints browser view URLs, while the publishing context needs stable
// object identities for server-side copies.
func (s *Service) PublishingSnapshot(ctx context.Context, userID, slug string) (PublishingSnapshot, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return PublishingSnapshot{}, err
	}
	if found.Status != StatusFinalized || found.Content == nil || found.ContentRevision <= 0 ||
		found.FinalizedRevision != found.ContentRevision || found.FinalizedAt == nil {
		return PublishingSnapshot{}, ErrPostNotFinalized
	}
	images, err := s.store.ListImages(ctx, found.Slug)
	if err != nil {
		return PublishingSnapshot{}, fmt.Errorf("list publishing images: %w", err)
	}
	if strings.TrimSpace(found.Content.Title) == "" {
		return PublishingSnapshot{}, ErrInvalidContent
	}
	// Revalidate at the publishing hand-off as well as at current write paths.
	// Posts finalized before a validator was tightened must not bypass the frozen
	// manifest boundary merely because no new save occurs after deployment.
	if err := ValidateContent(*found.Content, images); err != nil {
		return PublishingSnapshot{}, err
	}
	if found.ContentLanguage == nil || !found.ContentLanguage.Valid() || !found.TargetLanguage.Valid() {
		return PublishingSnapshot{}, ErrLanguageRequired
	}
	refs, err := s.voiceRefs(ctx, userID)
	if err != nil {
		return PublishingSnapshot{}, err
	}
	voice := projectVoice(refs, found.VoiceID)
	if !voice.SourceLanguage.Valid() {
		return PublishingSnapshot{}, ErrLanguageRequired
	}
	return PublishingSnapshot{PostSlug: found.Slug, UserID: found.UserID, CreatedAt: found.CreatedAt, Content: clonePostContent(*found.Content),
		ContentRevision: found.ContentRevision, FinalizedRevision: found.FinalizedRevision, Images: append([]Image(nil), images...),
		TargetLanguage: found.TargetLanguage, ContentLanguage: *found.ContentLanguage, VoiceSourceLanguage: voice.SourceLanguage}, nil
}

// PostIdentity returns the immutable incarnation marker used by consumers whose
// history must not attach to a later post that happens to reuse a deleted slug.
func (s *Service) PostIdentity(ctx context.Context, userID, slug string) (time.Time, error) {
	found, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return time.Time{}, err
	}
	return found.CreatedAt, nil
}

func clonePostContent(content PostContent) PostContent {
	cloned := content
	cloned.Tags = append([]string(nil), content.Tags...)
	cloned.Blocks = make([]Block, len(content.Blocks))
	for index, block := range content.Blocks {
		cloned.Blocks[index] = block
		cloned.Blocks[index].Items = append([]string(nil), block.Items...)
	}
	return cloned
}

func equalOptionalInt(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// CreateUpload reserves a filename and hands back a presigned PUT.
//
// The image id is minted now, not at confirm time, because the object key contains it:
// the browser has to PUT to the final key, and the server has to be able to find that
// object again from an upload_id alone after a restart.
func (s *Service) CreateUpload(ctx context.Context, userID, postSlug, filename string) (Upload, string, string, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return Upload{}, "", "", fmt.Errorf("filename is required")
	}
	if _, err := s.ownedPost(ctx, userID, postSlug); err != nil {
		return Upload{}, "", "", err
	}

	// A CONFIRMED photo with this name is a real conflict — the filename is how the
	// model and the exporters address a photo, so two cannot share one.
	taken, err := s.store.ImageFilenameTaken(ctx, postSlug, filename)
	if err != nil {
		return Upload{}, "", "", fmt.Errorf("check filename: %w", err)
	}
	if taken {
		return Upload{}, "", "", ErrDuplicateFilename
	}
	// A PENDING upload with this name is not a conflict, it is a retry: the plan gives
	// every photo a retry button that restarts from here with a fresh id. Refusing
	// would strand the user until the sweep ran, up to an hour later.
	if err := s.replacePendingUpload(ctx, postSlug, filename); err != nil {
		return Upload{}, "", "", err
	}

	now := s.now()
	upload := Upload{
		ID:        s.newID(),
		PostSlug:  postSlug,
		Filename:  filename,
		ExpiresAt: now.Add(s.putTTL),
		CreatedAt: now,
	}
	upload.Key = ObjectKey(postSlug, upload.ID)

	url, err := s.blobs.PresignPut(ctx, upload.Key, uploadContentType, s.putTTL)
	if err != nil {
		return Upload{}, "", "", fmt.Errorf("presign upload url: %w", err)
	}
	// The row is written after the presign succeeds: a row with no usable URL would be
	// swept later as an orphan for no reason.
	if err := s.store.CreateUpload(ctx, upload); err != nil {
		// The UNIQUE(post_slug, filename) constraint fired, which means another request
		// claimed the name between the checks above and this insert.
		if errors.Is(err, ErrDuplicateFilename) {
			return Upload{}, "", "", ErrDuplicateFilename
		}
		return Upload{}, "", "", fmt.Errorf("create upload: %w", err)
	}

	return upload, url, uploadContentType, nil
}

// replacePendingUpload clears an unconfirmed upload holding this filename so a retry
// can take it. Its object goes too — nothing will ever reference that key again.
func (s *Service) replacePendingUpload(ctx context.Context, postSlug, filename string) error {
	pending, err := s.store.GetUploadByFilename(ctx, postSlug, filename)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check pending upload: %w", err)
	}

	// Row first, then object. The reverse order could delete the bytes and then fail,
	// leaving a row that promises an object which is not there.
	if err := s.store.DeleteUpload(ctx, pending.ID); err != nil {
		return fmt.Errorf("clear pending upload: %w", err)
	}
	if err := s.blobs.Delete(ctx, pending.Key); err != nil {
		// Not fatal: the object is now unreferenced, which is exactly what the sweep
		// collects. Failing the retry over it would be the worse outcome.
		slog.Warn("could not delete a replaced upload's object", "key", pending.Key, "err", err)
	}
	return nil
}

// ConfirmUpload turns a landed object into a photo.
//
// The size comes from storage, never from the client: the browser reports width and
// height because only it decoded the image ([I6]), but bytes is something the server
// can check, so it does — and it refuses an object too large to be one of our photos.
//
// It is idempotent. A client that never saw the response retries, and a retry has to
// return the photo rather than a primary-key failure.
func (s *Service) ConfirmUpload(ctx context.Context, userID, uploadID string, width, height int32) (Image, error) {
	if width <= 0 || height <= 0 || width > maxImageDimension || height > maxImageDimension {
		return Image{}, fmt.Errorf("%w: dimensions %dx%d", ErrInvalidImage, width, height)
	}

	upload, err := s.store.GetUpload(ctx, uploadID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// No upload row: either this id never existed, or it was already confirmed
			// and the client is retrying.
			return s.alreadyConfirmed(ctx, userID, uploadID)
		}
		return Image{}, fmt.Errorf("load upload: %w", err)
	}
	if _, err := s.ownedPost(ctx, userID, upload.PostSlug); err != nil {
		return Image{}, err
	}

	size, err := s.blobs.Head(ctx, upload.Key)
	if err != nil {
		if errors.Is(err, ErrObjectNotFound) {
			// The PUT never landed, or landed elsewhere. Leaving the uploads row in
			// place lets the client retry with the same id, and the sweep collects it
			// if the client gives up.
			return Image{}, ErrObjectMissing
		}
		return Image{}, fmt.Errorf("head uploaded object: %w", err)
	}
	// The browser caps size before uploading, but a presigned PUT is a URL an
	// authenticated client can use however it likes — so the cap is enforced where it
	// can actually be trusted. The object is dropped rather than left for the sweep,
	// which would keep paying for it for an hour.
	if size <= 0 || size > s.maxBytes {
		if err := s.blobs.Delete(ctx, upload.Key); err != nil {
			slog.Warn("could not delete a rejected upload's object", "key", upload.Key, "err", err)
		}
		if err := s.store.DeleteUpload(ctx, upload.ID); err != nil {
			slog.Warn("could not delete a rejected upload row", "upload_id", upload.ID, "err", err)
		}
		return Image{}, fmt.Errorf("%w: %d bytes", ErrInvalidImage, size)
	}

	image := Image{
		ID:        upload.ID,
		PostSlug:  upload.PostSlug,
		Filename:  upload.Filename,
		Key:       upload.Key,
		Width:     width,
		Height:    height,
		Bytes:     size,
		CreatedAt: s.now(),
	}
	// One transaction: see the note on Store.ConfirmUpload for why the two writes must
	// not be separable.
	if err := s.store.ConfirmUpload(ctx, image, upload.ID); err != nil {
		if errors.Is(err, ErrDuplicateFilename) {
			return Image{}, ErrDuplicateFilename
		}
		return Image{}, fmt.Errorf("confirm upload: %w", err)
	}

	return image, nil
}

// alreadyConfirmed answers a retry whose upload row is gone because the first attempt
// succeeded. Anything else is a genuinely unknown id.
func (s *Service) alreadyConfirmed(ctx context.Context, userID, uploadID string) (Image, error) {
	image, err := s.store.GetImage(ctx, uploadID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Image{}, ErrNotFound
		}
		return Image{}, fmt.Errorf("load image: %w", err)
	}
	if _, err := s.ownedPost(ctx, userID, image.PostSlug); err != nil {
		return Image{}, err
	}
	return image, nil
}

// DeleteImage removes the photo and its object.
//
// Storage first, then the row: the reverse order would drop the only reference to the
// object if the delete failed, leaving bytes nobody can name. This way a failure leaves
// a row whose object is gone, which the user can retry.
func (s *Service) DeleteImage(ctx context.Context, userID, imageID string) error {
	image, err := s.store.GetImage(ctx, imageID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("load image: %w", err)
	}
	if _, err := s.ownedPost(ctx, userID, image.PostSlug); err != nil {
		return err
	}

	if err := s.blobs.Delete(ctx, image.Key); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	if err := s.store.DeleteImage(ctx, imageID); err != nil {
		return fmt.Errorf("delete image row: %w", err)
	}
	return nil
}

// ownedPost resolves a slug to the caller's post, or says why not.
//
// Unknown and foreign are distinguished on purpose (PRD §7 specifies 403 for a foreign
// slug). At two users there is nothing to enumerate.
func (s *Service) ownedPost(ctx context.Context, userID, slug string) (Post, error) {
	found, err := s.store.GetPost(ctx, slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Post{}, ErrNotFound
		}
		return Post{}, fmt.Errorf("load post: %w", err)
	}
	if found.UserID != userID {
		return Post{}, ErrForbidden
	}
	return found, nil
}

// newObjectID mints an image/upload id. 16 random bytes rather than a counter or a
// timestamp: the id lands in an object key, so it must not be guessable from another
// user's id or reveal how many photos exist.
func newObjectID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("post: cannot read random bytes for an object id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

package post

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeStore is an in-memory post.Storage. These tests are about the context’s rules —
// ownership, the upload handshake, what the sweep may delete — not about SQL.
type fakeStore struct {
	mu      sync.Mutex
	posts   map[string]Post
	images  map[string]Image
	videos  map[string]Video
	uploads map[string]Upload
	// answers is keyed by slug then label, the way the table is keyed.
	answers map[string]map[string]TemplateAnswer

	// slugTaken lets a test simulate another request claiming a slug between the
	// existence check and the insert — the race the retry loop exists for.
	slugTaken func(slug string)
	// beforeGuardedWrite runs first in every write the published lock guards, outside the
	// lock, so a test can publish the post after the service's check and before the write:
	// the race the statements' own predicates and guards exist for.
	beforeGuardedWrite func(slug string)
	// beforeSnapshotRead runs first in LearningSnapshot, outside the lock, so a test can change
	// the post between the service's own read and the snapshot's.
	beforeSnapshotRead func(slug string)
	// fieldAssignments counts AssignField calls, so a test can say a save named no 분야 write.
	fieldAssignments int
	// optionWrites counts SaveGenerationOptions calls, so a test can say one save was one write.
	optionWrites int
}

// guarded runs the race hook for one guarded write.
func (f *fakeStore) guarded(slug string) {
	f.mu.Lock()
	hook := f.beforeGuardedWrite
	f.mu.Unlock()
	if hook != nil {
		hook(slug)
	}
}

// publishedLocked mirrors `status <> 'published'` and the insert guard: it reads the stored
// row, so a post published by the race hook is refused like the real statement refuses it.
func (f *fakeStore) publishedLocked(slug string) bool {
	return f.posts[slug].Status == StatusPublished
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		posts:   map[string]Post{},
		images:  map[string]Image{},
		videos:  map[string]Video{},
		uploads: map[string]Upload{},
		answers: map[string]map[string]TemplateAnswer{},
	}
}

func (f *fakeStore) CreatePost(_ context.Context, p Post) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.posts[p.Slug]; exists {
		return ErrDuplicateSlug
	}
	// The real store reads a NULL tag_count as the default (POST-63); the fake mirrors that
	// at insert so a freshly created post reads the same way through both.
	if p.TagCount == 0 {
		p.TagCount = TagCountRange.Default
	}
	f.posts[p.Slug] = p
	return nil
}

func (f *fakeStore) UpdateDraft(_ context.Context, slug, userID, title, memo string, targetLanguage *Language, updatedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || f.publishedLocked(slug) {
		return false, nil
	}
	existing.Title = title
	existing.Memo = memo
	if targetLanguage != nil {
		existing.TargetLanguage = *targetLanguage
	}
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

func (f *fakeStore) UpdateObservations(_ context.Context, slug, userID string, observations []Observation, updatedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || f.publishedLocked(slug) {
		return false, nil
	}
	existing.Observations = append([]Observation(nil), observations...)
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

func (f *fakeStore) UpdateGeneratedContent(_ context.Context, slug, userID string, content PostContent, language Language, annotations WriteAnnotations, updatedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || f.publishedLocked(slug) {
		return false, nil
	}
	// The statement's idempotence group: an identical content with identical annotations is
	// no write; different nouns or candidates are.
	if generatedAlready(existing, content, language, annotations) {
		return false, nil
	}
	existing.Content = &content
	existing.ContentLanguage = &language
	// NULL for none, as the columns store it.
	existing.ContentNouns = nilIfEmpty(annotations.Nouns)
	existing.ReplacementCandidates = nil
	if len(annotations.Candidates) > 0 {
		existing.ReplacementCandidates = append([]ReplacementCandidate(nil), annotations.Candidates...)
	}
	existing.ContentRevision++
	existing.MachineBaselineRevision = existing.ContentRevision
	existing.MachineBaselineVoiceID = existing.VoiceID
	existing.Status = StatusReview
	existing.FinalizedRevision = 0
	existing.FinalizedAt = nil
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

// ReassignVoice mirrors the real single UPDATE: the id moves and the machine baseline is
// withdrawn; canonical content, its revision, and finalization state stay.
func (f *fakeStore) ReassignVoice(_ context.Context, slug, userID, voiceID string, updatedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || existing.VoiceID == voiceID || f.publishedLocked(slug) {
		return false, nil
	}
	existing.VoiceID = voiceID
	existing.MachineBaselineRevision = 0
	existing.MachineBaselineVoiceID = ""
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

// UpsertTemplateAnswers mirrors the real upsert: one row per label, nothing ever deleted, so
// a label the current template no longer declares stays where it is.
func (f *fakeStore) UpsertTemplateAnswers(_ context.Context, slug string, answers []TemplateAnswer, _ time.Time) error {
	// An empty set writes nothing and opens no transaction, so it meets no guard either.
	if len(answers) == 0 {
		return nil
	}
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishedLocked(slug) {
		return ErrPostPublished
	}
	if f.answers[slug] == nil {
		f.answers[slug] = map[string]TemplateAnswer{}
	}
	for _, answer := range answers {
		f.answers[slug][answer.Label] = answer
	}
	return nil
}

// ListTemplateAnswers returns them ordered by label, like the query's ORDER BY.
func (f *fakeStore) ListTemplateAnswers(_ context.Context, slug string) ([]TemplateAnswer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	labels := make([]string, 0, len(f.answers[slug]))
	for label := range f.answers[slug] {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	out := make([]TemplateAnswer, 0, len(labels))
	for _, label := range labels {
		out = append(out, f.answers[slug][label])
	}
	return out, nil
}

// AssignTemplate mirrors the real single UPDATE: the assignment, the seeded numbers and
// updated_at move together. Everything the voice reassignment above withdraws is deliberately
// left alone here — a template is never learned from, so assigning one may not cost a post its
// learn eligibility.
func (f *fakeStore) AssignTemplate(_ context.Context, slug, userID string, templateID *string, seed TemplateNumbers, updatedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || f.publishedLocked(slug) {
		return false, nil
	}
	if templateID == nil {
		existing.TemplateID = ""
	} else {
		existing.TemplateID = *templateID
	}
	// COALESCE, like the SQL: a number the template has no opinion about keeps the post's own.
	if seed.TargetLength != nil {
		existing.TargetLength = seed.TargetLength
	}
	if seed.TagCount != nil {
		existing.TagCount = *seed.TagCount
	}
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

// AssignField mirrors AssignPostField: it refuses a published row, and an equal field matches
// no row, NULL-safely.
func (f *fakeStore) AssignField(_ context.Context, slug, userID string, field *string, updatedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fieldAssignments++
	value := ""
	if field != nil {
		value = *field
	}
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || f.publishedLocked(slug) || existing.Field == value {
		return false, nil
	}
	existing.Field = value
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

func (f *fakeStore) SaveContent(_ context.Context, slug, userID string, content PostContent, expectedRevision int64, candidates *[]ReplacementCandidate, updatedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || existing.ContentRevision != expectedRevision || f.publishedLocked(slug) {
		return false, nil
	}
	// The statement's CASE: a save that took candidates stores what is left, NULL for none.
	if candidates != nil {
		existing.ReplacementCandidates = nil
		if len(*candidates) > 0 {
			existing.ReplacementCandidates = slices.Clone(*candidates)
		}
	}
	existing.Content = &content
	existing.ContentRevision++
	existing.Status = StatusReview
	existing.FinalizedRevision = 0
	existing.FinalizedAt = nil
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

func (f *fakeStore) SaveGenerationOptions(_ context.Context, slug, userID string, set GenerationOptionsSet, updatedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.optionWrites++
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || f.publishedLocked(slug) {
		return false, nil
	}
	existing.TargetLength = set.TargetLength
	existing.TagCount = set.TagCount
	existing.UseMemory = set.UseMemory
	existing.Field = set.Field
	// NULL for none, like the column: an empty set reads back as nil.
	existing.QualityRules = nil
	if len(set.QualityRules) > 0 {
		existing.QualityRules = append([]string(nil), set.QualityRules...)
	}
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

func (f *fakeStore) Finalize(_ context.Context, slug, userID, title string, expectedRevision int64, finalizedAt time.Time) (bool, error) {
	f.guarded(slug)
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || existing.ContentRevision != expectedRevision || existing.Content == nil || f.publishedLocked(slug) {
		return false, nil
	}
	existing.Status = StatusFinalized
	existing.FinalizedRevision = existing.ContentRevision
	// The one statement writes the title with the finalization, so the fake does too — the slug
	// and the content revision are untouched by the copy.
	existing.Title = title
	existing.FinalizedAt = &finalizedAt
	existing.UpdatedAt = finalizedAt
	f.posts[slug] = existing
	return true, nil
}

// LearningSnapshot mirrors the store: it maps the row and judges nothing, so the service's own
// finalization rule is what refuses an unfinalized one.
func (f *fakeStore) LearningSnapshot(_ context.Context, slug, userID string) (LearningSnapshot, error) {
	f.mu.Lock()
	hook := f.beforeSnapshotRead
	f.mu.Unlock()
	if hook != nil {
		hook(slug)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok {
		return LearningSnapshot{}, ErrNotFound
	}
	if existing.UserID != userID {
		return LearningSnapshot{}, ErrForbidden
	}
	if existing.Content == nil || existing.MachineBaselineRevision <= 0 {
		return LearningSnapshot{}, ErrNoMachineBaseline
	}
	snapshot := LearningSnapshot{
		PostSlug: slug, UserID: userID, VoiceID: existing.VoiceID, MachineBaselineVoiceID: existing.MachineBaselineVoiceID,
		Status: existing.Status, Current: *existing.Content,
		ContentRevision: existing.ContentRevision, FinalizedRevision: existing.FinalizedRevision, MachineBaseline: *existing.Content,
		BaselineRevision: existing.MachineBaselineRevision, TargetLength: existing.TargetLength,
		UpdatedAt: existing.UpdatedAt, ContentLanguage: valueLanguage(existing.ContentLanguage),
	}
	if existing.FinalizedAt != nil {
		snapshot.FinalizedAt = *existing.FinalizedAt
	}
	return snapshot, nil
}

// PublishPost mirrors the store's guarded statement: a post whose current revision is its
// finalized one, or one already published.
func (f *fakeStore) PublishPost(_ context.Context, slug, userID, url string, publishedAt time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID {
		return false, nil
	}
	if !existing.FinalizedAtCurrentRevision() && existing.Status != StatusPublished {
		return false, nil
	}
	existing.Status = StatusPublished
	existing.PublishedURL = url
	existing.PublishedAt = &publishedAt
	existing.UpdatedAt = publishedAt
	f.posts[slug] = existing
	return true, nil
}

func (f *fakeStore) UnpublishPost(_ context.Context, slug, userID string, updatedAt time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID || existing.Status != StatusPublished {
		return false, nil
	}
	existing.Status = StatusFinalized
	existing.PublishedURL = ""
	existing.PublishedAt = nil
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

func (f *fakeStore) ListPublishedPosts(_ context.Context, userID string, limit int) ([]PublishedPost, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []PublishedPost
	for _, p := range f.posts {
		if p.UserID != userID || p.Status != StatusPublished || p.Content == nil || p.PublishedAt == nil {
			continue
		}
		out = append(out, PublishedPost{
			Slug: p.Slug, ContentRevision: p.ContentRevision, Content: *p.Content,
			ContentLanguage: p.ContentLanguage, Nouns: p.ContentNouns, PublishedAt: *p.PublishedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].PublishedAt.Equal(out[j].PublishedAt) {
			return out[i].PublishedAt.After(out[j].PublishedAt)
		}
		return out[i].Slug > out[j].Slug
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) GetPost(_ context.Context, slug string) (Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.posts[slug]
	if !ok {
		return Post{}, ErrNotFound
	}
	return p, nil
}

func (f *fakeStore) SlugExists(_ context.Context, slug string) (bool, error) {
	f.mu.Lock()
	_, ok := f.posts[slug]
	hook := f.slugTaken
	f.mu.Unlock()

	if !ok && hook != nil {
		hook(slug)
	}
	return ok, nil
}

// ListPosts honours the filter the way the SQLite store does — the stored-string order, the
// status, the keyset cursor and the limit — so service tests run against the real contract.
func (f *fakeStore) ListPosts(_ context.Context, userID string, filter ListFilter) ([]Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Summary
	for _, p := range f.posts {
		if p.UserID != userID {
			continue
		}
		if filter.Status != "" && p.Status != filter.Status {
			continue
		}
		title := p.Title
		var tags []string
		if p.Content != nil {
			if strings.TrimSpace(title) == "" {
				title = p.Content.Title
			}
			tags = p.Content.Tags
		}
		cursor := ListCursor{UpdatedAt: p.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"), Slug: p.Slug}
		if filter.After != nil && !listCursorBefore(*filter.After, cursor) {
			continue
		}
		out = append(out, Summary{Slug: p.Slug, VoiceID: p.VoiceID, TemplateID: p.TemplateID, Title: title, Status: p.Status, UpdatedAt: p.UpdatedAt, TargetLanguage: p.TargetLanguage, ContentLanguage: p.ContentLanguage, Tags: tags, Cursor: cursor})
	}
	sort.Slice(out, func(i, j int) bool { return listCursorBefore(out[i].Cursor, out[j].Cursor) })
	if filter.Limit >= 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

// listCursorBefore reports whether b comes after a in the list order (updated_at DESC, slug DESC).
func listCursorBefore(a, b ListCursor) bool {
	if a.UpdatedAt != b.UpdatedAt {
		return b.UpdatedAt < a.UpdatedAt
	}
	return b.Slug < a.Slug
}

func valueLanguage(value *Language) Language {
	if value == nil {
		return ""
	}
	return *value
}

func (f *fakeStore) DeletePost(_ context.Context, slug, userID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	found, ok := f.posts[slug]
	if !ok || found.UserID != userID {
		return false, nil
	}
	delete(f.posts, slug)
	for id, image := range f.images {
		if image.PostSlug == slug {
			delete(f.images, id)
		}
	}
	for id, video := range f.videos {
		if video.PostSlug == slug {
			delete(f.videos, id)
		}
	}
	for id, upload := range f.uploads {
		if upload.PostSlug == slug {
			delete(f.uploads, id)
		}
	}
	return true, nil
}

func (f *fakeStore) CreateImage(_ context.Context, img Image) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createImageLocked(img)
}

// createImageLocked mirrors the real schema's constraints, so a test sees the same
// failures production would.
func (f *fakeStore) createImageLocked(img Image) error {
	if _, exists := f.images[img.ID]; exists {
		return ErrDuplicateFilename
	}
	for _, existing := range f.images {
		if existing.PostSlug == img.PostSlug && existing.Filename == img.Filename {
			return ErrDuplicateFilename
		}
	}
	f.images[img.ID] = img
	return nil
}

// ConfirmUpload is atomic here too — a fake that let the two writes come apart would
// hide the very bug the real transaction exists to prevent.
func (f *fakeStore) ConfirmUpload(_ context.Context, img Image, uploadID string) error {
	f.guarded(img.PostSlug)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishedLocked(img.PostSlug) {
		return ErrPostPublished
	}
	if err := f.createImageLocked(img); err != nil {
		return err
	}
	delete(f.uploads, uploadID)
	return nil
}

func (f *fakeStore) ListImages(_ context.Context, postSlug string) ([]Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Image
	for _, img := range f.images {
		if img.PostSlug == postSlug {
			out = append(out, img)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) GetImage(_ context.Context, id string) (Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	img, ok := f.images[id]
	if !ok {
		return Image{}, ErrNotFound
	}
	return img, nil
}

// DeleteImage mirrors the statement's subquery: a published post's photo stays, and zero
// rows says so or says the row was already gone.
func (f *fakeStore) DeleteImage(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	image, ok := f.images[id]
	f.mu.Unlock()
	if ok {
		f.guarded(image.PostSlug)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	image, ok = f.images[id]
	if !ok || f.publishedLocked(image.PostSlug) {
		return false, nil
	}
	delete(f.images, id)
	return true, nil
}

func (f *fakeStore) ImageFilenameTaken(_ context.Context, postSlug, filename string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, img := range f.images {
		if img.PostSlug == postSlug && img.Filename == filename {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) ImageKeyInUse(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, img := range f.images {
		if img.Key == key {
			return true, nil
		}
	}
	return false, nil
}

// createVideoLocked mirrors the real schema's constraints, as createImageLocked does.
func (f *fakeStore) createVideoLocked(video Video) error {
	if _, exists := f.videos[video.ID]; exists {
		return ErrDuplicateFilename
	}
	for _, existing := range f.videos {
		if existing.PostSlug == video.PostSlug && existing.Filename == video.Filename {
			return ErrDuplicateFilename
		}
	}
	f.videos[video.ID] = video
	return nil
}

func (f *fakeStore) ConfirmVideoUpload(_ context.Context, video Video, uploadID string) error {
	f.guarded(video.PostSlug)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishedLocked(video.PostSlug) {
		return ErrPostPublished
	}
	if err := f.createVideoLocked(video); err != nil {
		return err
	}
	delete(f.uploads, uploadID)
	return nil
}

func (f *fakeStore) ListVideos(_ context.Context, postSlug string) ([]Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Video
	for _, video := range f.videos {
		if video.PostSlug == postSlug {
			out = append(out, video)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) GetVideo(_ context.Context, id string) (Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	video, ok := f.videos[id]
	if !ok {
		return Video{}, ErrNotFound
	}
	return video, nil
}

// DeleteVideo mirrors the statement's subquery, as DeleteImage does.
func (f *fakeStore) DeleteVideo(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	video, ok := f.videos[id]
	f.mu.Unlock()
	if ok {
		f.guarded(video.PostSlug)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	video, ok = f.videos[id]
	if !ok || f.publishedLocked(video.PostSlug) {
		return false, nil
	}
	delete(f.videos, id)
	return true, nil
}

func (f *fakeStore) VideoFilenameTaken(_ context.Context, postSlug, filename string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, video := range f.videos {
		if video.PostSlug == postSlug && video.Filename == filename {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) CountVideos(_ context.Context, postSlug string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, video := range f.videos {
		if video.PostSlug == postSlug {
			count++
		}
	}
	return count, nil
}

func (f *fakeStore) VideoKeyInUse(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, video := range f.videos {
		if video.Key == key {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) CreateUpload(_ context.Context, u Upload) error {
	f.guarded(u.PostSlug)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishedLocked(u.PostSlug) {
		return ErrPostPublished
	}
	// UNIQUE(post_slug, filename), as in the schema.
	for _, existing := range f.uploads {
		if existing.PostSlug == u.PostSlug && existing.Filename == u.Filename {
			return ErrDuplicateFilename
		}
	}
	f.uploads[u.ID] = u
	return nil
}

func (f *fakeStore) GetUploadByFilename(_ context.Context, postSlug, filename string) (Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.uploads {
		if u.PostSlug == postSlug && u.Filename == filename {
			return u, nil
		}
	}
	return Upload{}, ErrNotFound
}

func (f *fakeStore) GetUpload(_ context.Context, id string) (Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[id]
	if !ok {
		return Upload{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) DeleteUpload(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.uploads, id)
	return nil
}

func (f *fakeStore) ListUploadsExpiredBefore(_ context.Context, t time.Time) ([]Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Upload
	for _, u := range f.uploads {
		if u.ExpiresAt.Before(t) {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) AllReferencedKeys(_ context.Context) (map[string]struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := map[string]struct{}{}
	for _, img := range f.images {
		keys[img.Key] = struct{}{}
	}
	for _, video := range f.videos {
		keys[video.Key] = struct{}{}
	}
	for _, u := range f.uploads {
		keys[u.Key] = struct{}{}
	}
	return keys, nil
}

// fakeBlobs is an in-memory ObjectStore. It records deletes so a test can assert that
// storage was reached, which is the half of DeleteImage a database cannot show.
type fakeBlobs struct {
	mu      sync.Mutex
	objects map[string]fakeObject
	deleted []string

	// failDelete makes every Delete fail, for the "storage is down" paths.
	failDelete bool
	// beforeDelete runs first in every Delete, outside the blobs' lock, so a test can read the
	// store at the moment storage is reached.
	beforeDelete func(key string)
	// failList makes List fail, so the sweep can be shown to delete nothing.
	failList bool
	// failPresign makes PresignGet fail, so a read can be shown to presign nothing.
	failPresign bool
}

type fakeObject struct {
	size         int64
	contentType  string
	lastModified time.Time
}

func newFakeBlobs() *fakeBlobs {
	return &fakeBlobs{objects: map[string]fakeObject{}}
}

func (f *fakeBlobs) put(key string, size int64, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = fakeObject{size: size, contentType: uploadContentType, lastModified: at}
}

// putTyped is the video case: the object reports back the Content-Type its PUT was signed
// for, which is what the confirm checks against the reservation.
func (f *fakeBlobs) putTyped(key string, size int64, contentType string, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = fakeObject{size: size, contentType: contentType, lastModified: at}
}

func (f *fakeBlobs) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func (f *fakeBlobs) PresignPut(_ context.Context, key, contentType string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("https://storage.example/%s?put&ct=%s&ttl=%s", key, contentType, ttl), nil
}

func (f *fakeBlobs) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	if f.failPresign {
		return "", fmt.Errorf("storage unavailable")
	}
	return fmt.Sprintf("https://storage.example/%s?get&ttl=%s", key, ttl), nil
}

func (f *fakeBlobs) Head(_ context.Context, key string) (ObjectHead, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.objects[key]
	if !ok {
		return ObjectHead{}, ErrObjectNotFound
	}
	return ObjectHead{Size: obj.size, ContentType: obj.contentType}, nil
}

func (f *fakeBlobs) Delete(_ context.Context, key string) error {
	if f.beforeDelete != nil {
		f.beforeDelete(key)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failDelete {
		return fmt.Errorf("storage unavailable")
	}
	f.deleted = append(f.deleted, key)
	delete(f.objects, key)
	return nil
}

func (f *fakeBlobs) List(_ context.Context, prefix string) ([]Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failList {
		return nil, fmt.Errorf("storage unavailable")
	}
	var out []Object
	for key, obj := range f.objects {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, Object{Key: key, Size: obj.size, LastModified: obj.lastModified})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func nilIfEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

// optionsSet is the post's stored run options with change applied: a whole-set save overwrites
// all five, so a test that means one member sends the rest as they stand (POST-89).
func optionsSet(t *testing.T, svc *Service, userID, slug string, change func(*GenerationOptionsSet)) GenerationOptionsSet {
	t.Helper()
	found, err := svc.Get(context.Background(), userID, slug)
	if err != nil {
		t.Fatal(err)
	}
	set := found.GenerationOptions()
	change(&set)
	return set
}

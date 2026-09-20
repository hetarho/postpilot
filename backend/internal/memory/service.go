package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	store  Store
	limits Limits
	now    func() time.Time
	newID  func() string
	// The extraction collaborators. They are set after construction, like the guideline
	// context's template directory, because the memory store stands alone and everything
	// below belongs to the ONE path that calls a provider (MEM-13).
	models      Models
	posts       Posts
	extractions ExtractionJobs
}

func NewService(store Store, limits Limits) *Service {
	if !limits.valid() {
		panic("memory: limits must be positive")
	}
	return &Service{store: store, limits: limits, now: time.Now, newID: newID}
}

// ConfigureExtraction wires the one path that reads a post and calls a provider. Without
// it the directory still works completely: a memory written by hand needs none of this.
func (s *Service) ConfigureExtraction(models Models, posts Posts, jobs ExtractionJobs) {
	s.models, s.posts, s.extractions = models, posts, jobs
}

func (s *Service) Limits() Limits { return s.limits }

// StartExtraction is 기억으로 저장 (MEM-13): it resolves the account's analyze selection,
// reads the finished post ONCE and enqueues a durable job with both frozen onto it. The
// credit gate lives at the queue's enqueue seam, so an account without the balance is
// refused there and no job row survives (QUOTA-13).
func (s *Service) StartExtraction(ctx context.Context, userID, postSlug string) (string, error) {
	if s.models == nil || s.posts == nil || s.extractions == nil {
		return "", fmt.Errorf("memory: extraction is not wired")
	}
	slug := strings.TrimSpace(postSlug)
	if slug == "" {
		return "", ErrNotFound
	}
	model, ok, err := s.models.AnalyzeModel(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("resolve analyze model: %w", err)
	}
	if !ok {
		return "", ErrAnalyzeModelRequired
	}
	source, err := s.posts.ExtractionSource(ctx, userID, slug)
	if err != nil {
		return "", err
	}
	return s.extractions.Enqueue(ctx, ExtractionRequest{
		UserID: userID, PostSlug: slug, Model: model.String(), Source: source,
	})
}

// List returns the account's memories in injection order, so the management screen shows
// exactly what a post that opted in would be given, in that order.
func (s *Service) List(ctx context.Context, userID string) ([]Memory, error) {
	memories, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	return memories, nil
}

// Create is the ONE way a memory comes into being: approving an extracted candidate
// (MEM-15) and writing one by hand (MEM-25) are the same call. There is deliberately no
// approve procedure — every field rule, the text bound, the tag ceiling and the account cap
// already live here, and a second entry point would have to restate all of them to stay in
// agreement.
//
// sourcePostSlug is the post the fact was approved from, empty when it was typed by hand.
// A duplicate text stores nothing new: it links the post, advances the existing memory's
// last use, and answers that memory with deduplicated true.
func (s *Service) Create(ctx context.Context, userID, text string, kind Kind, tags []string, sourcePostSlug string) (Memory, bool, error) {
	text, err := s.validText(text)
	if err != nil {
		return Memory{}, false, err
	}
	if !kind.Valid() {
		return Memory{}, false, ErrInvalidKind
	}
	clean, err := s.validTags(tags)
	if err != nil {
		return Memory{}, false, err
	}
	now := s.now()
	candidate := Memory{
		ID: s.newID(), UserID: userID, Text: text, Kind: kind, Tags: clean,
		CreatedAt: now, UpdatedAt: now, LastSeenAt: now,
	}
	stored, deduplicated, err := s.store.Insert(ctx, candidate, strings.TrimSpace(sourcePostSlug), s.limits.MaxPerAccount)
	if err != nil {
		return Memory{}, false, err
	}
	return stored, deduplicated, nil
}

// Update applies only what the request carried. The validation runs per present part, so a
// text edit can never be refused for tags it did not send — and never rewrites them either.
func (s *Service) Update(ctx context.Context, userID, id string, patch Patch) (Memory, error) {
	if strings.TrimSpace(id) == "" {
		return Memory{}, ErrNotFound
	}
	if patch.empty() {
		return s.store.Get(ctx, userID, id)
	}
	if patch.Text != nil {
		text, err := s.validText(*patch.Text)
		if err != nil {
			return Memory{}, err
		}
		patch.Text = &text
	}
	if patch.Kind != nil && !patch.Kind.Valid() {
		return Memory{}, ErrInvalidKind
	}
	if patch.Tags != nil {
		clean, err := s.validTags(*patch.Tags)
		if err != nil {
			return Memory{}, err
		}
		patch.Tags = &clean
	}
	return s.store.Update(ctx, userID, id, patch, s.now())
}

// Delete removes one memory with no undo. Nothing references it — a generation froze the
// texts at enqueue (MEM-19) — so nothing in flight changes.
func (s *Service) Delete(ctx context.Context, userID, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrNotFound
	}
	return s.store.Delete(ctx, userID, id)
}

// DetachPost is MEM-17, called by the post context when a post is deleted: every link that
// named the post is dropped, and a memory goes only when that link was its last. A fact
// re-confirmed across several posts outlives any one of them; a fact that existed only
// inside the deleted post leaves no orphan.
func (s *Service) DetachPost(ctx context.Context, userID, postSlug string) error {
	if strings.TrimSpace(postSlug) == "" {
		return nil
	}
	return s.store.DropPostSources(ctx, userID, postSlug)
}

func (s *Service) validText(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrInvalidText
	}
	if chars := utf8.RuneCountInString(trimmed); chars > s.limits.TextMaxChars {
		return "", &TextTooLongError{Chars: chars, Max: s.limits.TextMaxChars}
	}
	return trimmed, nil
}

// validTags trims, collapses duplicates and then counts. Collapsing before counting is what
// keeps a user who typed one tag twice from being told they used too many; an EMPTY tag is
// refused rather than dropped, because silently saving a smaller set than the one sent is a
// repair nobody asked for.
func (s *Service) validTags(tags []string) ([]string, error) {
	unique := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			return nil, ErrInvalidTag
		}
		if _, duplicate := seen[tag]; duplicate {
			continue
		}
		seen[tag] = struct{}{}
		unique = append(unique, tag)
	}
	if len(unique) > s.limits.TagsMax {
		return nil, &TooManyTagsError{Count: len(unique), Max: s.limits.TagsMax}
	}
	return unique, nil
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("memory: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

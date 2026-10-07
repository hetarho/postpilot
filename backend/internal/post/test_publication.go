package post

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"time"
)

var ErrTestPublicationConflict = errors.New("writing test publication target changed")

// TestOutputPublication is the frozen winner handed to the post owner. The caller
// maps its own test contract here; this context never reads test-owned tables.
type TestOutputPublication struct {
	UserID, TestID, WinnerID, RequestKey, PostSlug string
	AssignmentsHash                                string
	InputRevision, ContentRevision                 int64
	Content, Baseline                              PostContent
	ContentLanguage                                Language
	Storyline                                      *Storyline
	Nouns                                          []string
}

// TestOutputReceipt is durable provenance and proof of one committed publication.
// It survives changes and deletion of the target, so replay cannot undo later edits.
type TestOutputReceipt struct {
	UserID, TestID, WinnerID, Action, RequestKey, TargetID string
	ResultingRevision                                      int64
}

type TestResultStore interface {
	ApplyTestResult(context.Context, TestOutputPublication, time.Time) (TestOutputReceipt, error)
}

// TestResultService exposes the post's publication port separately from autosave.
// Its store must atomically check ordinary jobs along with all post-owned guards.
type TestResultService struct {
	store TestResultStore
	now   func() time.Time
}

func NewTestResultService(store TestResultStore) *TestResultService {
	if store == nil {
		panic("post: test result store is required")
	}
	return &TestResultService{store: store, now: time.Now}
}

func (s *TestResultService) ApplyTestResult(ctx context.Context, publication TestOutputPublication) (TestOutputReceipt, error) {
	return s.store.ApplyTestResult(ctx, publication, s.now())
}

// TestAssignmentsHash is shared by frozen-input and publication adapters. It covers
// the assignments and generation options owned by the post; the input revision also
// fences all material, attachment and storyline changes.
func TestAssignmentsHash(p Post) string {
	rules := slices.Clone(p.QualityRules)
	slices.Sort(rules)
	if len(rules) == 0 {
		rules = nil
	}
	data, _ := json.Marshal(struct {
		Voice, Template, Field string
		Language               Language
		TargetLength           *int
		TagCount               int
		UseMemory              bool
		QualityRules           []string
	}{p.VoiceID, p.TemplateID, p.Field, p.TargetLanguage, p.TargetLength, p.TagCount, p.UseMemory, rules})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// ValidateTestStoryline checks a winner's plan against the same current attachment
// snapshot as its content. A test is a machine result, so it cannot claim owner edits.
func ValidateTestStoryline(storyline *Storyline, images []Image, videos []Video) error {
	if storyline == nil {
		return nil
	}
	if storyline.EditedByHand {
		return ErrStorylineInvalid
	}
	if err := (StorylineEdit{Paragraphs: storyline.Paragraphs}).validShape(); err != nil {
		return err
	}
	attached := make(map[string]bool, len(images)+len(videos))
	for _, image := range images {
		attached[image.Filename] = true
	}
	for _, video := range videos {
		attached[video.Filename] = true
	}
	for _, name := range storyline.MadeWith {
		if !attached[name] {
			return &StorylineFileUnknownError{File: name}
		}
	}
	for _, paragraph := range storyline.Paragraphs {
		for _, name := range paragraph.Files {
			if !attached[name] || !slices.Contains(storyline.MadeWith, name) {
				return &StorylineFileUnknownError{File: name}
			}
		}
	}
	return nil
}

package post

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"unicode/utf8"
)

// StorylineTextMaxChars bounds one paragraph's text, the owner's edits included; it mirrors the
// generation parser's bound on a machine one (GEN-67, POST-96).
const StorylineTextMaxChars = 1000

var (
	// ErrStorylineMissing: an edit of a storyline the post does not hold (POST-96).
	ErrStorylineMissing = errors.New("the post holds no storyline")
	// ErrStorylineInvalid: an edit that changes the paragraph count or puts one attachment in
	// two paragraphs. The count is the storyline job's (POST-96), so another count is a client bug.
	ErrStorylineInvalid = errors.New("the storyline edit is invalid")
)

// StorylineTextTooLongError is a paragraph past StorylineTextMaxChars.
type StorylineTextTooLongError struct{ Max int }

func (e *StorylineTextTooLongError) Error() string {
	return fmt.Sprintf("a storyline paragraph exceeds %d characters", e.Max)
}

func (e *StorylineTextTooLongError) Unwrap() error { return ErrStorylineInvalid }

// StorylineFileUnknownError names an attachment an edit may not place: one no longer attached,
// or one attached after the storyline was made (POST-96, POST-99).
type StorylineFileUnknownError struct{ File string }

func (e *StorylineFileUnknownError) Error() string {
	return fmt.Sprintf("the storyline cannot hold %q", e.File)
}

// StorylineEdit is the owner's own edit of the storyline (POST-96): the paragraphs' texts and
// files as one whole value.
type StorylineEdit struct {
	Paragraphs []StorylineParagraph
}

// validShape is what the edit can be refused for without reading the post: a paragraph past the
// ceiling, or a file in two places.
func (e StorylineEdit) validShape() error {
	seen := make(map[string]bool)
	for _, paragraph := range e.Paragraphs {
		if utf8.RuneCountInString(paragraph.Text) > StorylineTextMaxChars {
			return &StorylineTextTooLongError{Max: StorylineTextMaxChars}
		}
		for _, file := range paragraph.Files {
			if seen[file] {
				return ErrStorylineInvalid
			}
			seen[file] = true
		}
	}
	return nil
}

// checkStorylineEdit refuses an edit the stored storyline cannot take: while a job targets the
// post (its answer would overwrite the edit or be written from the old one), when the post holds
// no storyline, with another paragraph count, or naming a file that is not attached or that the
// storyline was not made with. It returns the storyline to store, nil when the edit changes
// nothing — an identical save is no edit and sets no mark.
func (s *Service) checkStorylineEdit(ctx context.Context, found Post, edit StorylineEdit) (*Storyline, error) {
	if s.jobs != nil {
		active, err := s.ordinaryForPost(ctx, found.UserID, found.Slug)
		if err != nil {
			return nil, fmt.Errorf("check active job before a storyline edit: %w", err)
		}
		if blocksFutureSettings(active) {
			return nil, ErrPostBusy
		}
	}
	stored := found.Storyline
	if stored == nil {
		return nil, ErrStorylineMissing
	}
	if len(edit.Paragraphs) != len(stored.Paragraphs) {
		return nil, ErrStorylineInvalid
	}
	attached, err := s.attachedNames(ctx, found.Slug)
	if err != nil {
		return nil, err
	}
	for _, paragraph := range edit.Paragraphs {
		for _, file := range paragraph.Files {
			if !slices.Contains(attached, file) || !slices.Contains(stored.MadeWith, file) {
				return nil, &StorylineFileUnknownError{File: file}
			}
		}
	}
	next := Storyline{Paragraphs: edit.Paragraphs, MadeWith: stored.MadeWith}.normalized()
	if reflect.DeepEqual(next.Paragraphs, stored.Paragraphs) {
		return nil, nil
	}
	next.EditedByHand = true
	return &next, nil
}

// attachedNames is every confirmed photo and video name, the one filename namespace.
func (s *Service) attachedNames(ctx context.Context, slug string) ([]string, error) {
	images, err := s.images.ListImages(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("list images for a storyline edit: %w", err)
	}
	videos, err := s.videos.ListVideos(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("list videos for a storyline edit: %w", err)
	}
	names := make([]string, 0, len(images)+len(videos))
	for _, image := range images {
		names = append(names, image.Filename)
	}
	for _, video := range videos {
		names = append(names, video.Filename)
	}
	return names, nil
}

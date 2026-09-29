package voice

import (
	"context"
	"errors"
	"fmt"
)

// PostFingerprint is ②'s reading of a post against its voice (POST-102): the content revision
// it counted and one comparison per item. Applicable is false, with no items, for a post with
// 말투 없음, with no content yet, or whose voice is deleted or not made.
type PostFingerprint struct {
	Applicable bool
	Revision   int64
	Items      []ItemComparison
}

// ConfigurePosts wires the post reader PostFingerprint counts from.
func (s *Service) ConfigurePosts(posts PostContents) {
	if posts == nil {
		panic("voice: nil post contents")
	}
	s.posts = posts
}

// PostFingerprint counts the caller's post from its blocks and compares it with its voice's
// current analysis (VOICE-62). It is computed per request, never stored, and calls no model.
func (s *Service) PostFingerprint(ctx context.Context, userID, slug string) (PostFingerprint, error) {
	if s.posts == nil {
		return PostFingerprint{}, errors.New("voice: post contents not configured")
	}
	voiceID, revision, blocks, err := s.posts.PostForFingerprint(ctx, userID, slug)
	if err != nil {
		return PostFingerprint{}, err
	}
	out := PostFingerprint{Revision: revision}
	if voiceID == "" || len(blocks) == 0 {
		return out, nil
	}
	found, err := s.directory.GetVoice(ctx, userID, voiceID)
	if errors.Is(err, ErrVoiceNotFound) {
		return out, nil
	}
	if err != nil {
		return PostFingerprint{}, fmt.Errorf("get voice: %w", err)
	}
	if found.DeletedAt != nil {
		return out, nil
	}
	analysis, err := s.analyses.CurrentAnalysis(ctx, userID, voiceID)
	if err != nil {
		return PostFingerprint{}, fmt.Errorf("current analysis: %w", err)
	}
	if analysis == nil {
		return out, nil
	}
	out.Applicable = true
	out.Items = Compare(analysis.Counted, MeasureBlocks(blocks))
	return out, nil
}

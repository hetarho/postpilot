package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

// ReflectionPromptVersion names the prompt a 말투 반영 비교 freezes: 검증's, which both candidates
// receive unchanged (MODEL-30, MODEL-67).
const ReflectionPromptVersion = "voice-reflection-v1"

// ReflectionInput is what a 말투 반영 비교 froze from the voice: the snapshot both candidates read
// and what the comparison records beside it.
type ReflectionInput struct {
	Content    []byte
	PromptKey  string
	MaterialID string
	// Photo is a photo prompt, which needs both write models to read images.
	Photo bool
}

// reflectionSnapshot is the frozen input on the wire. The answer's text and the photo key are
// private content and leave with the snapshot when the comparison's content is purged.
type reflectionSnapshot struct {
	VoiceID    string `json:"voice_id"`
	PromptKey  string `json:"prompt_key"`
	MaterialID string `json:"material_id"`
	Projection string `json:"projection"`
	Answer     string `json:"answer"`
	PhotoKey   string `json:"photo_key,omitempty"`
}

// SnapshotReflectionInput freezes a 말투 반영 비교 (MODEL-67): the voice's projection with the
// prompt's answer withheld, the prompt, the answer's text and a photo prompt's photo key. It
// refuses what 검증 refuses — a tombstone, a voice not made, an unknown or unanswered prompt —
// and calls nothing.
func (s *Service) SnapshotReflectionInput(ctx context.Context, userID, voiceID, promptKey string) (ReflectionInput, error) {
	input, err := s.prepareCheck(ctx, userID, voiceID, promptKey)
	if err != nil {
		return ReflectionInput{}, err
	}
	content, err := json.Marshal(reflectionSnapshot{
		VoiceID: voiceID, PromptKey: input.prompt.Key, MaterialID: input.answer.ID, Projection: input.projection,
		Answer: input.answer.Body, PhotoKey: input.answer.PhotoKey,
	})
	if err != nil {
		return ReflectionInput{}, fmt.Errorf("encode reflection snapshot: %w", err)
	}
	return ReflectionInput{Content: content, PromptKey: input.prompt.Key, MaterialID: input.answer.ID, Photo: input.prompt.Photo}, nil
}

// ReflectionResult is one candidate's piece and what its call reported.
type ReflectionResult struct {
	Piece string
	Usage llm.Usage
}

// RunReflectionCandidate is one candidate's call: 검증's prompt over the frozen snapshot, on the
// candidate's model. A retry reads the same snapshot, never the voice as it is now (MODEL-35).
func (s *Service) RunReflectionCandidate(ctx context.Context, content []byte, model llm.ModelRef) (ReflectionResult, error) {
	snapshot, err := decodeReflection(content)
	if err != nil {
		return ReflectionResult{}, err
	}
	prompt, ok := PromptByKey(snapshot.PromptKey)
	if !ok {
		return ReflectionResult{}, ErrPromptNotFound
	}
	piece, usage, err := s.writePiece(ctx, model, snapshot.Projection, prompt, snapshot.PhotoKey)
	return ReflectionResult{Piece: piece, Usage: usage}, err
}

// ReflectionView is what a review shows of the frozen input: the prompt and the owner's answer.
func ReflectionView(content []byte) (prompt Prompt, answer string, err error) {
	snapshot, err := decodeReflection(content)
	if err != nil {
		return Prompt{}, "", err
	}
	prompt, _ = PromptByKey(snapshot.PromptKey)
	return prompt, snapshot.Answer, nil
}

// CompareText measures a text against the voice's current analysis (VOICE-62), readable on a
// deleted voice too; a voice with no analysis answers nothing.
func (s *Service) CompareText(ctx context.Context, userID, voiceID, text string) ([]ItemComparison, error) {
	if _, err := s.ownedVoice(ctx, userID, voiceID); err != nil {
		return nil, err
	}
	analysis, err := s.analyses.CurrentAnalysis(ctx, userID, voiceID)
	if err != nil {
		return nil, fmt.Errorf("current analysis: %w", err)
	}
	if analysis == nil {
		return nil, nil
	}
	return Compare(analysis.Counted, MeasureText(text)), nil
}

func decodeReflection(content []byte) (reflectionSnapshot, error) {
	var snapshot reflectionSnapshot
	if len(content) == 0 {
		return snapshot, errors.New("the reflection snapshot is gone")
	}
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return snapshot, fmt.Errorf("decode reflection snapshot: %w", err)
	}
	return snapshot, nil
}

package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// checkRequest is the task after the frozen projection (VOICE-43).
var checkRequest = fmt.Sprintf("위 말투로, 이 문항의 주제에 대해 블로그 글의 한 부분을 %d~%d문장으로 써 주세요. "+
	"첫인사로 시작해서 끝인사로 마무리하세요. 사진이 있으면 사진에 보이는 것을 소재로 쓰되, 사진에 없는 사실은 지어내지 마세요. "+
	"제목·목록·설명 없이 본문만 쓰세요.", CheckSentencesMin, CheckSentencesMax)

// CheckVoice is the `check_voice` job (VOICE-43): one write call with the frozen projection, the
// prompt and, for a photo prompt, the answer's photo, storing the trimmed piece or the failure.
// Nothing calls a second time.
func (s *Service) CheckVoice(ctx context.Context, found CheckJob, progress Progress) error {
	ref, err := parseModelRef(found.WriteModel)
	if err != nil {
		return err
	}
	check, err := s.checks.GetCheck(ctx, found.UserID, found.CheckID)
	if err != nil {
		return err
	}
	running, err := s.checks.MarkCheckRunning(ctx, found.UserID, check.ID, s.now())
	if err != nil {
		return fmt.Errorf("mark check running: %w", err)
	}
	if !running {
		return fmt.Errorf("voice check %s is no longer waiting", check.ID)
	}
	piece, err := s.writeCheck(ctx, found, check, ref, progress)
	if err != nil {
		if _, failErr := s.checks.FailCheck(context.WithoutCancel(ctx), found.UserID, check.ID, normalizeFailure(err), s.now()); failErr != nil {
			return errors.Join(err, fmt.Errorf("record check failure: %w", failErr))
		}
		return err
	}
	if _, err := s.checks.FinishCheck(ctx, found.UserID, check.ID, piece, s.now()); err != nil {
		return fmt.Errorf("검증 결과를 저장하지 못했어요: %w", err)
	}
	progress("write", 1, 1)
	return nil
}

func (s *Service) writeCheck(ctx context.Context, found CheckJob, check Check, ref llm.ModelRef, progress Progress) (string, error) {
	// The job froze its voice at enqueue; recheck before the provider call so a voice deleted
	// while the check waited is never written in.
	if _, err := s.activeVoice(ctx, found.UserID, found.VoiceID); err != nil {
		return "", voiceUnavailableError(err)
	}
	prompt, ok := PromptByKey(check.PromptKey)
	if !ok {
		return "", ErrPromptNotFound
	}
	photoKey := ""
	if prompt.Photo {
		answer, err := s.samples.GetSampleBody(ctx, found.UserID, found.VoiceID, check.MaterialID)
		if err != nil {
			return "", fmt.Errorf("get answer: %w", err)
		}
		if answer == nil || !answer.HasPhoto() {
			return "", ErrPhotoRequired
		}
		photoKey = answer.PhotoKey
	}
	progress("write", 0, 1)
	piece, _, err := s.writePiece(ctx, ref, check.Projection, prompt, photoKey)
	return piece, err
}

// writePiece is 검증's one call, shared with 말투 반영 비교 (MODEL-67): the frozen projection as
// the system prompt, the prompt and the request as the task, and a photo prompt's photo.
func (s *Service) writePiece(ctx context.Context, ref llm.ModelRef, projection string, prompt Prompt, photoKey string) (string, llm.Usage, error) {
	parts := []llm.Part{{Text: "[문항]\n" + prompt.Text + "\n[요청]\n" + checkRequest}}
	if prompt.Photo {
		if photoKey == "" {
			return "", llm.Usage{}, ErrPhotoRequired
		}
		if s.objects == nil {
			return "", llm.Usage{}, errors.New("voice: photo storage not configured")
		}
		image, err := s.objects.Read(ctx, photoKey)
		if err != nil {
			return "", llm.Usage{}, fmt.Errorf("사진을 불러오지 못했어요: %w", err)
		}
		parts = append(parts, llm.Part{Image: image, MIME: PhotoContentType})
	}
	// No MaxTokens of its own: the registry's default is the write stage's floor.
	response, err := s.models.Complete(ctx, ref, llm.Request{
		System:   projection,
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: parts}},
	})
	if err != nil {
		return "", response.Usage, err
	}
	piece := strings.TrimSpace(response.Text)
	if piece == "" {
		return "", response.Usage, fmt.Errorf("the piece came back empty: %w", llm.ErrBadOutput)
	}
	return piece, response.Usage, nil
}

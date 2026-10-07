package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// UpdateVoiceSample edits the same owned source identity. Only explicit Save
// changes it; validation and the receipt path perform no model or queue work.
func (s *Service) UpdateVoiceSample(ctx context.Context, in SampleMutation) (Sample, error) {
	in.Validated, in.ValidatedUploadID = nil, ""
	if _, err := s.activeVoice(ctx, in.UserID, in.VoiceID); err != nil {
		return Sample{}, err
	}
	if in.SampleID == "" || strings.TrimSpace(in.OperationKey) == "" || in.ExpectedContentRevision <= 0 {
		return Sample{}, ErrSampleUpdateInvalid
	}
	if prior, err := s.materialReceipts.VoiceSampleMutationReceipt(ctx, in); err != nil {
		return Sample{}, err
	} else if prior != nil {
		return *prior, nil
	}
	current, err := s.samples.GetSampleBody(ctx, in.UserID, in.VoiceID, in.SampleID)
	if err != nil {
		return Sample{}, fmt.Errorf("read material for edit: %w", err)
	}
	if current == nil {
		return Sample{}, ErrSampleNotFound
	}
	if current.ContentRevision != in.ExpectedContentRevision {
		return Sample{}, ErrSampleRevisionConflict
	}
	updated := *current
	if in.Body != nil {
		updated.Body = strings.TrimSpace(*in.Body)
	}
	if !utf8.ValidString(updated.Body) {
		return Sample{}, ErrSampleUpdateInvalid
	}
	updated.Chars = utf8.RuneCountInString(updated.Body)
	if updated.Kind == SampleKindPost {
		if updated.Chars < SampleMinChars {
			return Sample{}, &SampleTooShortError{Chars: updated.Chars}
		}
		if in.Label != nil {
			updated.Label = strings.TrimSpace(*in.Label)
			if updated.Label == "" {
				updated.Label = firstRunes(updated.Body, LabelFallbackChars)
			}
		}
		if in.PhotoUploadID != nil || in.PhotoWidth != nil || in.PhotoHeight != nil {
			return Sample{}, ErrSampleUpdateInvalid
		}
	} else if updated.Kind == SampleKindAnswer {
		prompt, ok := PromptByKey(updated.PromptKey)
		if !ok {
			return Sample{}, ErrPromptNotFound
		}
		if updated.Body == "" || !containsKoreanProse(ProseSentences(updated.Body)) {
			return Sample{}, ErrAnswerRequired
		}
		if in.Label != nil && *in.Label != "" {
			return Sample{}, ErrSampleUpdateInvalid
		}
		updated.Label = ""
		if in.PhotoUploadID == nil {
			if in.PhotoWidth != nil || in.PhotoHeight != nil {
				return Sample{}, ErrSampleUpdateInvalid
			}
		} else if *in.PhotoUploadID == "" {
			if prompt.Photo {
				return Sample{}, ErrPhotoRequired
			}
			if (in.PhotoWidth != nil && *in.PhotoWidth != 0) || (in.PhotoHeight != nil && *in.PhotoHeight != 0) {
				return Sample{}, ErrInvalidPhoto
			}
			updated.PhotoKey, updated.PhotoWidth, updated.PhotoHeight = "", 0, 0
		} else {
			if !prompt.Photo || s.objects == nil || in.PhotoWidth == nil || in.PhotoHeight == nil {
				return Sample{}, ErrPhotoRequired
			}
			upload, err := s.photoUploads.GetPhotoUpload(ctx, in.UserID, in.VoiceID, *in.PhotoUploadID)
			if err != nil {
				return Sample{}, err
			}
			if upload.PromptKey != updated.PromptKey || !upload.ExpiresAt.After(s.now()) {
				return Sample{}, ErrPhotoRequired
			}
			if *in.PhotoWidth <= 0 || *in.PhotoHeight <= 0 || *in.PhotoWidth > MaxPhotoDimension || *in.PhotoHeight > MaxPhotoDimension {
				return Sample{}, ErrInvalidPhoto
			}
			head, err := s.objects.Head(ctx, upload.Key)
			if err != nil {
				if errors.Is(err, ErrObjectNotFound) {
					return Sample{}, ErrPhotoRequired
				}
				return Sample{}, fmt.Errorf("head edited material photo: %w", err)
			}
			if head.Size <= 0 || head.Size > s.photos.MaxBytes || head.ContentType != PhotoContentType {
				if s.objects.Delete(ctx, upload.Key) == nil {
					_ = s.photoUploads.DeletePhotoUpload(ctx, upload.ID)
				}
				return Sample{}, ErrInvalidPhoto
			}
			updated.PhotoKey, updated.PhotoWidth, updated.PhotoHeight = upload.Key, *in.PhotoWidth, *in.PhotoHeight
			in.ValidatedUploadID = upload.ID
		}
		if prompt.Photo && updated.PhotoKey == "" {
			return Sample{}, ErrPhotoRequired
		}
	} else {
		return Sample{}, ErrSampleUpdateInvalid
	}
	if !utf8.ValidString(updated.Label) {
		return Sample{}, ErrSampleUpdateInvalid
	}
	in.Validated = &updated
	result, err := s.edits.UpdateVoiceSample(ctx, in)
	if err != nil {
		return Sample{}, err
	}
	// The row/receipt committed first. A failed object delete is reclaimed by the
	// existing photo sweep, and replay never adopts or deletes a photo again.
	if current.PhotoKey != "" && current.PhotoKey != result.PhotoKey && s.objects != nil {
		_ = s.objects.Delete(ctx, current.PhotoKey)
	}
	return result, nil
}

package post

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// WritingTestAttachment is authorized media identity and bounded stored metadata.
// It contains no source memo, canonical content, captions or signed URL.
type WritingTestAttachment struct {
	ID, Filename, Key, Fingerprint string
	Kind                           AttachmentKind
	ContentType                    string
	Bytes, DurationMs              int64
	Width, Height, Rotation        int
	RotationByOwner                bool
}
type WritingTestMaterial struct {
	Language     Language
	TargetLength *int
	TagCount     int
	Answers      []TemplateAnswer
	QualityRules []string
}
type WritingTestAttachmentStore interface {
	WritingTestAttachments(context.Context, string, []string) ([]WritingTestAttachment, error)
}
type WritingTestMaterials struct {
	service     *Service
	attachments WritingTestAttachmentStore
}

func NewWritingTestMaterials(service *Service, attachments WritingTestAttachmentStore) *WritingTestMaterials {
	if service == nil || attachments == nil {
		panic("post: writing-test material and attachment owners are required")
	}
	return &WritingTestMaterials{service: service, attachments: attachments}
}
func (s *WritingTestMaterials) AttachedImages(ctx context.Context, user, slug string) (Post, error) {
	return s.service.AttachedImages(ctx, user, slug)
}
func (s *WritingTestMaterials) ValidateWritingTestMaterial(in WritingTestMaterial) error {
	if !in.Language.Valid() {
		return ErrLanguageRequired
	}
	if in.TargetLength != nil && (*in.TargetLength < TargetLengthMin || *in.TargetLength > TargetLengthMax) {
		return &TargetLengthError{Min: TargetLengthMin, Max: TargetLengthMax}
	}
	if !TagCountRange.Allows(in.TagCount) {
		return ErrInvalidTagCount
	}
	if _, err := s.service.validTemplateAnswers(in.Answers); err != nil {
		return err
	}
	if _, err := NormalizeQualityRules(in.QualityRules); err != nil {
		return err
	}
	return nil
}
func (s *WritingTestMaterials) WritingTestAttachments(ctx context.Context, user string, ids []string) ([]WritingTestAttachment, error) {
	if user == "" {
		return nil, ErrNotFound
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			return nil, ErrNotFound
		}
		seen[id] = true
	}
	if len(ids) == 0 {
		return nil, nil
	}
	found, err := s.attachments.WritingTestAttachments(ctx, user, ids)
	if err != nil {
		return nil, err
	}
	if len(found) != len(ids) {
		return nil, ErrNotFound
	}
	photos, videos := 0, 0
	byID := make(map[string]WritingTestAttachment, len(found))
	for i := range found {
		if !seen[found[i].ID] {
			return nil, ErrNotFound
		}
		delete(seen, found[i].ID)
		switch found[i].Kind {
		case AttachmentPhoto:
			photos++
		case AttachmentVideo:
			videos++
		default:
			return nil, ErrNotFound
		}
		found[i].Fingerprint = WritingTestAttachmentFingerprint(found[i])
		byID[found[i].ID] = found[i]
	}
	if photos > s.service.maxPhotos {
		return nil, ErrTooManyPhotos
	}
	if videos > s.service.maxVideos {
		return nil, ErrTooManyVideos
	}
	ordered := make([]WritingTestAttachment, len(ids))
	for index, id := range ids {
		ordered[index] = byID[id]
	}
	return ordered, nil
}
func WritingTestAttachmentFingerprint(value WritingTestAttachment) string {
	value.Fingerprint = ""
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

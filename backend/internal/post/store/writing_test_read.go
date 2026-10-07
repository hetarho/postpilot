package store

import (
	"context"
	"encoding/json"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store/sqlc"
)

func (s *Store) WritingTestAttachments(ctx context.Context, user string, ids []string) ([]post.WritingTestAttachment, error) {
	raw, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	images, err := s.read.WritingTestOwnedImages(ctx, sqlc.WritingTestOwnedImagesParams{UserID: user, JsonEach: string(raw)})
	if err != nil {
		return nil, err
	}
	videos, err := s.read.WritingTestOwnedVideos(ctx, sqlc.WritingTestOwnedVideosParams{UserID: user, JsonEach: string(raw)})
	if err != nil {
		return nil, err
	}
	result := make([]post.WritingTestAttachment, 0, len(images)+len(videos))
	for _, i := range images {
		result = append(result, post.WritingTestAttachment{ID: i.ID, Filename: i.Filename, Key: i.R2Key, Kind: post.AttachmentPhoto, ContentType: "image/jpeg", Bytes: i.Bytes, Width: int(i.Width), Height: int(i.Height), Rotation: int(i.Rotation), RotationByOwner: i.RotationByOwner != 0})
	}
	for _, v := range videos {
		result = append(result, post.WritingTestAttachment{ID: v.ID, Filename: v.Filename, Key: v.R2Key, Kind: post.AttachmentVideo, ContentType: v.ContentType, Bytes: v.Bytes, DurationMs: v.DurationMs, Width: int(v.Width), Height: int(v.Height)})
	}
	return result, nil
}

var _ post.WritingTestAttachmentStore = (*Store)(nil)

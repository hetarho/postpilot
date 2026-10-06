package storage

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/clip"
)

// One bounded page per sweep, with a rotating opaque cursor. Unlike the photo
// full-list contract, orphan cleanup checks each found key against current
// owned rows and can safely process an incomplete page.
func (b *Bucket) ListAnalysisCopies(ctx context.Context) ([]clip.StoredObject, error) {
	b.analysisListMu.Lock()
	defer b.analysisListMu.Unlock()
	in := &s3.ListObjectsV2Input{Bucket: aws.String(b.name), Prefix: aws.String(clip.AnalysisPreparationPrefix), MaxKeys: aws.Int32(1000)}
	if b.analysisListCursor != "" {
		in.ContinuationToken = aws.String(b.analysisListCursor)
	}
	page, e := b.ops.ListObjectsV2(ctx, in)
	if e != nil {
		return nil, errors.New("analysis copy listing failed")
	}
	if len(page.Contents) > 1000 {
		return nil, errors.New("analysis copy listing exceeded its bound")
	}
	b.analysisListCursor = ""
	if aws.ToBool(page.IsTruncated) {
		if page.NextContinuationToken == nil || *page.NextContinuationToken == "" {
			return nil, errors.New("analysis copy listing omitted cursor")
		}
		b.analysisListCursor = *page.NextContinuationToken
	}
	out := make([]clip.StoredObject, 0, len(page.Contents))
	for _, o := range page.Contents {
		out = append(out, clip.StoredObject{Key: aws.ToString(o.Key), Modified: aws.ToTime(o.LastModified)})
	}
	return out, nil
}

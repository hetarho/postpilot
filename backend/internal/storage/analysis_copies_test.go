package storage

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/postpilot/backend/internal/clip"
)

type boundedAnalysisList struct {
	*fakeS3
	inputs []*s3.ListObjectsV2Input
}

func (f *boundedAnalysisList) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, options ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.inputs = append(f.inputs, input)
	return f.fakeS3.ListObjectsV2(ctx, input, options...)
}

func TestAnalysisOrphanListingRotatesOneBoundedPage(t *testing.T) {
	now := time.Now()
	f := &boundedAnalysisList{fakeS3: &fakeS3{listPages: []*s3.ListObjectsV2Output{
		{Contents: []types.Object{{Key: aws.String("clip-analysis/alice/session/a.mp4"), LastModified: &now}}, IsTruncated: aws.Bool(true), NextContinuationToken: aws.String("page-two")},
		{Contents: []types.Object{{Key: aws.String("clip-analysis/alice/session/b.mp4"), LastModified: &now}}},
		{},
	}}}
	b := &Bucket{ops: f, name: "private"}
	for range 3 {
		before := len(f.inputs)
		if _, e := b.ListAnalysisCopies(t.Context()); e != nil {
			t.Fatal(e)
		}
		if len(f.inputs) != before+1 {
			t.Fatal("sweep listed an unbounded number of pages")
		}
	}
	for i, cursor := range []string{"", "page-two", ""} {
		in := f.inputs[i]
		if aws.ToString(in.ContinuationToken) != cursor || aws.ToInt32(in.MaxKeys) != 1000 || aws.ToString(in.Prefix) != clip.AnalysisPreparationPrefix {
			t.Fatal("orphan listing lost its namespace/page bound", i)
		}
	}
}

func TestAnalysisOrphanListingRejectsMalformedPage(t *testing.T) {
	for _, page := range []*s3.ListObjectsV2Output{{Contents: make([]types.Object, 1001)}, {IsTruncated: aws.Bool(true)}} {
		b := &Bucket{ops: &fakeS3{listPages: []*s3.ListObjectsV2Output{page}}, name: "private"}
		if _, e := b.ListAnalysisCopies(t.Context()); e == nil {
			t.Fatal("malformed bounded listing accepted")
		}
	}
}

// Package storage is the object-storage adapter. It is a supporting wrapper behind the
// port the post context declares (ARCHITECTURE §2.1): the SDK's types and errors stop
// here, and nothing above it knows whether the bucket is Cloudflare R2, MinIO, or S3.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/publishing"
)

// R2 authenticates per-bucket with a static key pair and has no notion of regions, but
// SigV4 requires one in the signature. "auto" is what Cloudflare specifies; MinIO
// ignores it.
const signingRegion = "auto"

// Bucket is the object store for photos.
//
// It holds two clients on purpose. Presigned URLs are consumed by the BROWSER, and a
// SigV4 signature covers the Host header, so a URL signed against the endpoint the API
// dials is rejected when the browser sends it to a different name. In production both
// endpoints are the same R2 host and the two clients are equivalent; in local dev the
// API reaches MinIO by its compose service name while the browser reaches the published
// port.
type Bucket struct {
	// ops issues the calls this process makes itself.
	ops s3API
	// presign only ever builds URLs — it never dials anything.
	presign      *s3.PresignClient
	name         string
	maxReadBytes int64
}

type s3API interface {
	CopyObject(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// Config is what cmd/api reads out of the process configuration.
type Config struct {
	Endpoint        string
	PublicEndpoint  string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	MaxReadBytes    int64
}

// New builds the adapter. It performs no I/O: a bad endpoint surfaces on the first real
// call, and failing to start over an unreachable bucket would make the whole app
// hostage to object storage it does not need to serve a login.
func New(ctx context.Context, cfg Config) (*Bucket, error) {
	if cfg.MaxReadBytes <= 0 {
		return nil, fmt.Errorf("max read bytes must be positive")
	}
	ops, err := newClient(ctx, cfg, cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	publicEndpoint := cfg.PublicEndpoint
	if publicEndpoint == "" {
		publicEndpoint = cfg.Endpoint
	}
	signer, err := newClient(ctx, cfg, publicEndpoint)
	if err != nil {
		return nil, err
	}

	return &Bucket{
		ops: ops, presign: s3.NewPresignClient(signer), name: cfg.Bucket,
		maxReadBytes: cfg.MaxReadBytes,
	}, nil
}

func newClient(ctx context.Context, cfg Config, endpoint string) (*s3.Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(signingRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		// Path style addressing. R2 accepts either; MinIO only works this way, because
		// virtual-hosted style would make the SDK dial `<bucket>.<host>`, which does not
		// resolve on a compose network.
		o.UsePathStyle = true
		// The SDK otherwise adds a CRC32 checksum header to every PUT and bakes
		// `x-amz-checksum-mode=ENABLED` into presigned GET URLs. R2 does not implement
		// the full-object CRC32 form, and a signed query parameter cannot be stripped by
		// the client — so both are turned off rather than debugged later.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}), nil
}

// PresignPut returns a URL the browser may PUT the photo to.
//
// contentType is signed, which means the browser MUST send exactly this value: a
// different one fails as a signature mismatch, and omitting it fails as an unsigned
// header. That is the point — it stops a presigned photo slot from being used to host
// arbitrary content types.
func (b *Bucket) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	req, err := b.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.name),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign put %s: %w", key, err)
	}
	return req.URL, nil
}

// PresignGet returns a short-lived read URL. The bucket is private, so this is the only
// way to read an object, and it is issued only after the caller's ownership is checked.
func (b *Bucket) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := b.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign get %s: %w", key, err)
	}
	return req.URL, nil
}

// Copy performs an S3/R2 server-side copy. The API sends only object identities and
// verifies the copied size; JPEG bytes never enter this process ([I6]).
func (b *Bucket) Copy(ctx context.Context, sourceKey, targetKey string) (int64, error) {
	_, err := b.ops.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(b.name),
		Key:               aws.String(targetKey),
		CopySource:        aws.String(copySourceHeader(b.name, sourceKey)),
		MetadataDirective: types.MetadataDirectiveCopy,
	})
	if err != nil {
		if isNotFound(err) {
			return 0, publishing.ErrInvalid
		}
		return 0, fmt.Errorf("copy %s to %s: %w", sourceKey, targetKey, err)
	}
	out, err := b.ops.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(b.name), Key: aws.String(targetKey)})
	if err != nil {
		return 0, fmt.Errorf("verify copied object %s: %w", targetKey, err)
	}
	return aws.ToInt64(out.ContentLength), nil
}

// CopySource is a URL-encoded bucket/key path, but its slash separators are part
// of the S3 wire format and must remain separators. Escaping the whole string
// turns them into %2F and no longer identifies the documented bucket/key shape.
func copySourceHeader(bucket, key string) string {
	segments := strings.Split(key, "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	return url.PathEscape(bucket) + "/" + strings.Join(segments, "/")
}

func (b *Bucket) SignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return b.PresignGet(ctx, key, ttl)
}

func (b *Bucket) ListStaged(ctx context.Context, prefix string) ([]publishing.StagedObject, error) {
	objects, err := b.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]publishing.StagedObject, 0, len(objects))
	for _, object := range objects {
		out = append(out, publishing.StagedObject{Key: object.Key, LastModified: object.LastModified})
	}
	return out, nil
}

// ReadObject reads one already-normalized JPEG for a model call. Upload bytes still
// travel browser-to-bucket; this is the generation-side read path only.
func (b *Bucket) ReadObject(ctx context.Context, key string) ([]byte, error) {
	out, err := b.ops.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(b.name), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	defer out.Body.Close()
	if size := aws.ToInt64(out.ContentLength); size > b.maxReadBytes {
		return nil, fmt.Errorf("read %s: object is %d bytes, limit is %d", key, size, b.maxReadBytes)
	}
	data, err := readAtMost(out.Body, b.maxReadBytes)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", key, err)
	}
	return data, nil
}

func readAtMost(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("object exceeds %d-byte read limit", limit)
	}
	return data, nil
}

// Head returns the stored size, or post.ErrObjectNotFound.
func (b *Bucket) Head(ctx context.Context, key string) (int64, error) {
	out, err := b.ops.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return 0, post.ErrObjectNotFound
		}
		return 0, fmt.Errorf("head %s: %w", key, err)
	}
	return aws.ToInt64(out.ContentLength), nil
}

// Delete removes the object. S3-compatible deletes are idempotent — a key that is not
// there returns success — so callers do not have to special-case it.
func (b *Bucket) Delete(ctx context.Context, key string) error {
	_, err := b.ops.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

// List returns every object under a prefix, following pagination to the end. The sweep
// deletes what is missing from this listing, so a short read would delete live photos —
// hence a page error aborts rather than returning what it has.
func (b *Bucket) List(ctx context.Context, prefix string) ([]post.Object, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(b.name),
		Prefix: aws.String(prefix),
	}

	var objects []post.Object
	for {
		page, err := b.ops.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, err)
		}
		for _, o := range page.Contents {
			objects = append(objects, post.Object{
				Key:          aws.ToString(o.Key),
				Size:         aws.ToInt64(o.Size),
				LastModified: aws.ToTime(o.LastModified),
			})
		}
		if !aws.ToBool(page.IsTruncated) {
			break
		}
		if page.NextContinuationToken == nil || *page.NextContinuationToken == "" {
			return nil, fmt.Errorf("list %s: truncated response omitted continuation token", prefix)
		}
		input.ContinuationToken = page.NextContinuationToken
	}
	return objects, nil
}

// isNotFound recognizes a missing object across S3, R2 and MinIO.
//
// HeadObject and GetObject report it differently — HEAD has no response body, so the
// SDK derives the code from the 404 status line and yields types.NotFound, while GET
// parses an XML body and yields types.NoSuchKey. Both arrive wrapped in a
// smithy.OperationError, so this must be errors.As and not a type assertion. The
// APIError fallback covers S3-compatible servers that word it differently.
//
// Deliberately NOT "any 404": a 404 can also mean the bucket is missing or the endpoint
// path is wrong, and a 403 (no permission) must stay an error rather than silently
// becoming "this photo does not exist".
func isNotFound(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return true
		}
	}
	return false
}

var _ post.ObjectStore = (*Bucket)(nil)
var _ publishing.ObjectStaging = (*Bucket)(nil)

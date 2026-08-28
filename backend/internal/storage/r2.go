// Package storage is the object-storage adapter. It is a supporting wrapper behind the
// port the post context declares (ARCHITECTURE §2.1): the SDK's types and errors stop
// here, and nothing above it knows whether the bucket is Cloudflare R2, MinIO, or S3.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/postpilot/backend/internal/post"
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
	ops *s3.Client
	// presign only ever builds URLs — it never dials anything.
	presign *s3.PresignClient
	name    string
}

// Config is what cmd/api reads out of the process configuration.
type Config struct {
	Endpoint        string
	PublicEndpoint  string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

// New builds the adapter. It performs no I/O: a bad endpoint surfaces on the first real
// call, and failing to start over an unreachable bucket would make the whole app
// hostage to object storage it does not need to serve a login.
func New(ctx context.Context, cfg Config) (*Bucket, error) {
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

	return &Bucket{ops: ops, presign: s3.NewPresignClient(signer), name: cfg.Bucket}, nil
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
	paginator := s3.NewListObjectsV2Paginator(b.ops, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.name),
		Prefix: aws.String(prefix),
	})

	var objects []post.Object
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
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

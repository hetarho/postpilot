package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// CheckCORS checks the response a browser receives, rather than only the upload
// preflight. Its disposable object is outside every owner-media sweep prefix.
func (b *Bucket) CheckCORS(ctx context.Context, origin string) error {
	if err := b.probeCORS(ctx, origin); err != nil {
		return fmt.Errorf("storage CORS check failed; apply deploy/r2-cors.json: %w", err)
	}
	return nil
}

func (b *Bucket) probeCORS(ctx context.Context, origin string) error {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("expected a web origin without a path, query or credentials")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("create probe key: %w", err)
	}
	key := "cors-probe/" + hex.EncodeToString(nonce[:]) + ".bin"
	// Delete even if PUT loses its response after the object has been stored.
	defer func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		if err := b.Delete(cleanup, key); err != nil {
			slog.Warn("storage CORS probe cleanup failed", "err", err)
		}
	}()
	probe := []byte("ok")
	if _, err := b.ops.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.name), Key: aws.String(key),
		Body: bytes.NewReader(probe), ContentLength: aws.Int64(int64(len(probe))),
		ContentType: aws.String("application/octet-stream"),
	}); err != nil {
		return fmt.Errorf("write probe object: %w", err)
	}
	signed, err := b.PresignSourcePlayback(ctx, key, "application/octet-stream", time.Minute)
	if err != nil {
		return fmt.Errorf("presign probe GET: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
	if err != nil {
		return errors.New("invalid probe GET URL")
	}
	request.Header.Set("Origin", origin)
	request.Header.Set("Range", "bytes=0-0")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		// A url.Error contains the presigned URL. Preserve its cause without logging
		// the short-lived signature in a failed deployment's output.
		var requestError *url.Error
		if errors.As(err, &requestError) {
			err = requestError.Err
		}
		return fmt.Errorf("ranged probe GET: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("ranged probe GET returned %d, expected 206", response.StatusCode)
	}
	allow := strings.Join(response.Header.Values("Access-Control-Allow-Origin"), ",")
	if allow != origin && allow != "*" {
		return errors.New("ranged probe GET does not allow the web origin")
	}
	if !exposesContentRange(response.Header.Values("Access-Control-Expose-Headers")) {
		return errors.New("ranged probe GET hides Content-Range from the browser")
	}
	if strings.Join(response.Header.Values("Content-Range"), ",") != "bytes 0-0/2" || response.ContentLength != 1 {
		return errors.New("ranged probe GET has invalid Content-Range or Content-Length")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2))
	if err != nil {
		return fmt.Errorf("read ranged probe body: %w", err)
	}
	if !bytes.Equal(body, probe[:1]) {
		return errors.New("ranged probe GET did not return the requested byte")
	}
	return nil
}

func exposesContentRange(values []string) bool {
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			if token = strings.TrimSpace(token); token == "*" || strings.EqualFold(token, "content-range") {
				return true
			}
		}
	}
	return false
}

package workerclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// Transfers sends no worker token to storage, follows no redirects, buffers no
// media file and refreshes signed access separately from encoding.
type Transfers struct {
	control *Client
	http    *http.Client
}

func NewTransfers(c *Client) *Transfers {
	return &Transfers{control: c, http: &http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 10 * time.Second, IdleConnTimeout: 30 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (t *Transfers) Download(ctx context.Context, lease clip.MediaLeaseCredentials, slot string, dst io.Writer, size int64) (int64, error) {
	for i := 0; i < 3; i++ {
		a, err := retryAccess(ctx, func() (clip.MediaArtifactAccess, error) { return t.control.Read(ctx, lease, slot) })
		if err != nil {
			return 0, err
		}
		if a.Bytes != size || a.Slot != slot {
			return 0, clip.ErrInvalidMedia
		}
		n, again, err := t.get(ctx, a, dst, size)
		// A partially streamed original is discarded by the workspace owner; do not
		// append a fresh request to the partially written original.
		if !again || n != 0 || i == 2 {
			return n, err
		}
		if err = backoff(ctx, i); err != nil {
			return n, err
		}
	}
	return 0, clip.ErrMediaUnavailable
}
func (t *Transfers) get(ctx context.Context, a clip.MediaArtifactAccess, dst io.Writer, size int64) (int64, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, a.ExpiresAfter)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return 0, false, clip.ErrInvalid
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return 0, true, clip.ErrMediaUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, retryStatus(resp.StatusCode), clip.ErrMediaUnavailable
	}
	if resp.ContentLength >= 0 && resp.ContentLength != size {
		return 0, false, clip.ErrInvalidMedia
	}
	n, err := io.Copy(dst, io.LimitReader(resp.Body, size+1))
	if err != nil {
		if ctx.Err() != nil {
			return n, false, ctx.Err()
		}
		if errors.Is(err, clip.ErrInvalidMedia) || errors.Is(err, clip.ErrWorkspaceLimit) {
			return n, false, err
		}
		return n, false, clip.ErrMediaUnavailable
	}
	if n != size {
		return n, false, clip.ErrInvalidMedia
	}
	return n, false, nil
}
func retryStatus(n int) bool {
	return n == http.StatusForbidden || n == http.StatusUnauthorized || n == http.StatusRequestTimeout || n == http.StatusTooManyRequests || n >= 500
}
func backoff(ctx context.Context, i int) error {
	timer := time.NewTimer(time.Duration(i+1) * 200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (t *Transfers) Upload(ctx context.Context, lease clip.MediaLeaseCredentials, out clip.MediaOutput, path string) error {
	for i := 0; i < 3; i++ {
		accesses, err := retryAccess(ctx, func() ([]clip.MediaArtifactAccess, error) {
			return t.control.Reserve(ctx, lease, []clip.MediaOutput{out})
		})
		if err != nil {
			return err
		}
		if len(accesses) != 1 {
			return clip.ErrInvalid
		}
		status, err := t.put(ctx, accesses[0], path)
		if err == nil && status >= 200 && status < 300 {
			return nil
		}
		if status == http.StatusPreconditionFailed {
			// The previous PUT may have succeeded before its response was lost. Compare
			// the entire existing candidate with the immutable local digest, never ETag.
			h := sha256.New()
			n, read := t.Download(ctx, lease, "output/"+out.Slot, h, out.Bytes)
			if read != nil {
				return read
			}
			if n != out.Bytes || hex.EncodeToString(h.Sum(nil)) != out.Digest {
				return clip.ErrMediaConflict
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && !errors.Is(err, clip.ErrMediaUnavailable) {
			return err
		}
		if err == nil && !retryStatus(status) {
			return clip.ErrMediaUnavailable
		}
		if i < 2 {
			if err = backoff(ctx, i); err != nil {
				return err
			}
		}
	}
	return clip.ErrMediaUnavailable
}
func (t *Transfers) put(ctx context.Context, a clip.MediaArtifactAccess, path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, clip.ErrInvalidMedia
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() != a.Bytes {
		return 0, clip.ErrInvalidMedia
	}
	ctx, cancel := context.WithTimeout(ctx, a.ExpiresAfter)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, a.URL, f)
	if err != nil {
		return 0, clip.ErrInvalid
	}
	req.ContentLength = a.Bytes
	for k, v := range a.Headers {
		req.Header.Set(k, v)
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return 0, clip.ErrMediaUnavailable
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// Access issuance itself is replayable: a lost reservation response must not
// force a completed encode to run again.
func retryAccess[T any](ctx context.Context, call func() (T, error)) (out T, err error) {
	for i := 0; i < 3; i++ {
		if err = ctx.Err(); err != nil {
			return out, err
		}
		out, err = call()
		if !errors.Is(err, clip.ErrMediaUnavailable) && !errors.Is(err, context.DeadlineExceeded) {
			return out, err
		}
		if i < 2 {
			if err = backoff(ctx, i); err != nil {
				return out, err
			}
		}
	}
	return out, err
}

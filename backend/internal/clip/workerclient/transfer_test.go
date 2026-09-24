package workerclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

type transferControl struct {
	postpilotv1connect.UnimplementedClipMediaWorkerServiceHandler
	url             string
	size            int64
	reads, reserves atomic.Int32
}

func (c *transferControl) GetMediaArtifactAccess(_ context.Context, r *connect.Request[pb.GetMediaArtifactAccessRequest]) (*connect.Response[pb.GetMediaArtifactAccessResponse], error) {
	c.reads.Add(1)
	return connect.NewResponse(&pb.GetMediaArtifactAccessResponse{Access: &pb.MediaArtifactAccess{Slot: r.Msg.Slot, Url: c.url, MaxBytes: c.size, ExpiresAfterMs: 60000}}), nil
}
func (c *transferControl) ReserveMediaOutputs(_ context.Context, r *connect.Request[pb.ReserveMediaOutputsRequest]) (*connect.Response[pb.ReserveMediaOutputsResponse], error) {
	c.reserves.Add(1)
	out := r.Msg.Outputs[0]
	return connect.NewResponse(&pb.ReserveMediaOutputsResponse{Outputs: []*pb.MediaArtifactAccess{{Slot: out.Slot, Url: c.url, MaxBytes: out.Bytes, ContentType: out.ContentType, ExpiresAfterMs: 60000, Headers: map[string]string{"Content-Type": out.ContentType, "If-None-Match": "*"}}}}), nil
}
func transfers(t *testing.T, h http.Handler, size int64) (*Transfers, *transferControl) {
	t.Helper()
	object := httptest.NewServer(h)
	t.Cleanup(object.Close)
	control := &transferControl{url: object.URL, size: size}
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewClipMediaWorkerServiceHandler(control))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	return NewTransfers(New(api.URL, "worker", "private-token")), control
}
func TestUploadRefreshesAccessWithoutReencodingAndVerifiesLostReply(t *testing.T) {
	data := []byte("immutable candidate")
	sum := sha256.Sum256(data)
	for _, mode := range []string{"expired", "lost", "collision", "failed"} {
		t.Run(mode, func(t *testing.T) {
			var puts atomic.Int32
			tr, c := transfers(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("worker token leaked to storage")
				}
				if r.Method == http.MethodGet {
					if mode == "collision" {
						w.Write(bytes.Repeat([]byte("x"), len(data)))
					} else {
						w.Write(data)
					}
					return
				}
				if r.ContentLength != int64(len(data)) || r.Header.Get("If-None-Match") != "*" {
					t.Error("unsigned upload")
				}
				got, _ := io.ReadAll(r.Body)
				if !bytes.Equal(got, data) {
					t.Error("different retry bytes")
				}
				n := puts.Add(1)
				switch mode {
				case "expired":
					if n == 1 {
						w.WriteHeader(403)
						return
					}
				case "lost", "collision":
					if n == 1 {
						conn, _, _ := w.(http.Hijacker).Hijack()
						conn.Close()
						return
					}
					w.WriteHeader(412)
					return
				case "failed":
					w.WriteHeader(503)
					return
				}
				w.WriteHeader(200)
			}), int64(len(data)))
			path := filepath.Join(t.TempDir(), "out.mp4")
			os.WriteFile(path, data, 0600)
			err := tr.Upload(t.Context(), clip.MediaLeaseCredentials{}, clip.MediaOutput{Slot: "result", Bytes: int64(len(data)), ContentType: "video/mp4", Digest: hex.EncodeToString(sum[:])}, path)
			switch mode {
			case "expired", "lost":
				if err != nil {
					t.Fatal(err)
				}
			case "collision":
				if !errors.Is(err, clip.ErrMediaConflict) {
					t.Fatal(err)
				}
			case "failed":
				if !errors.Is(err, clip.ErrMediaUnavailable) {
					t.Fatal(err)
				}
			}
			if c.reserves.Load() != puts.Load() || puts.Load() > 3 {
				t.Fatal("unbounded or stale signed retry")
			}
		})
	}
}
func TestDownloadRejectsOversizeAndRefreshesExpiredAccess(t *testing.T) {
	for _, mode := range []string{"expired", "oversize", "short", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			var gets atomic.Int32
			tr, c := transfers(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := gets.Add(1)
				switch mode {
				case "expired":
					if n == 1 {
						w.WriteHeader(403)
						return
					}
				case "oversize":
					w.Write([]byte("too many"))
					return
				case "short":
					w.Write([]byte("no"))
					return
				case "redirect":
					http.Redirect(w, r, "http://invalid.invalid/", 307)
					return
				}
				w.Write([]byte("data"))
			}), 4)
			var dst bytes.Buffer
			n, err := tr.Download(t.Context(), clip.MediaLeaseCredentials{}, "source/s", &dst, 4)
			if mode == "expired" {
				if err != nil || n != 4 || c.reads.Load() != 2 {
					t.Fatal(n, err, c.reads.Load())
				}
			} else if err == nil {
				t.Fatal("invalid body accepted")
			}
			if dst.Len() > 5 {
				t.Fatal("unbounded download")
			}
		})
	}
}

func TestAccessIssuanceRetriesOnlyTransientFailures(t *testing.T) {
	for _, failure := range []error{clip.ErrMediaUnavailable, clip.ErrMediaLeaseLost, clip.ErrInvalid} {
		calls := 0
		out, err := retryAccess(t.Context(), func() (string, error) {
			calls++
			if calls < 2 {
				return "", failure
			}
			return "same-reservation", nil
		})
		if errors.Is(failure, clip.ErrMediaUnavailable) {
			if err != nil || out != "same-reservation" || calls != 2 {
				t.Fatal(out, err, calls)
			}
		} else if !errors.Is(err, failure) || calls != 1 {
			t.Fatal("retried fenced/invalid access", err, calls)
		}
	}
}

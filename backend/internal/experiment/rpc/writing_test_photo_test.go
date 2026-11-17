package rpc

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// photoActionsFake records what the handler passed and answers with canned results.
type photoActionsFake struct {
	user, filename, confirmedID, viewKey string
	width, height                        int64
	upload                               experiment.TestPhotoUpload
	photo                                experiment.TestPhoto
	err                                  error
}

func (f *photoActionsFake) Create(_ context.Context, user, filename string) (experiment.TestPhotoUpload, error) {
	f.user, f.filename = user, filename
	return f.upload, f.err
}
func (f *photoActionsFake) Confirm(_ context.Context, user, id string, width, height int64) (experiment.TestPhoto, error) {
	f.user, f.confirmedID, f.width, f.height = user, id, width, height
	return f.photo, f.err
}
func (f *photoActionsFake) ViewURL(_ context.Context, key string) (string, time.Time, error) {
	f.viewKey = key
	return "https://storage.test/view", time.Date(2026, 11, 17, 9, 10, 0, 0, time.UTC), nil
}

func TestCreateTestPhotoUploadDerivesTheOwnerAndReturnsTheHandshake(t *testing.T) {
	f := &photoActionsFake{upload: experiment.TestPhotoUpload{
		Photo:  experiment.TestPhoto{ID: "photo-1"},
		PutURL: "https://storage.test/put", ContentType: "image/jpeg", PutExpires: time.Date(2026, 11, 17, 9, 10, 0, 0, time.UTC),
	}}
	h := NewWritingTestHandler(&testActionsFake{}, &testActionsFake{}, f)
	resp, err := h.CreateTestPhotoUpload(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.CreateTestPhotoUploadRequest{Filename: "lunch.jpg"}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg
	if f.user != "alice" || f.filename != "lunch.jpg" {
		t.Fatalf("service saw user=%q filename=%q", f.user, f.filename)
	}
	if got.GetUploadId() != "photo-1" || got.GetPutUrl() != "https://storage.test/put" || got.GetContentType() != "image/jpeg" || got.GetExpiresAt() != "2026-11-17T09:10:00Z" {
		t.Fatalf("response = %+v", got)
	}
}

func TestConfirmTestPhotoUploadReturnsThePhotoWithAViewURLAndNeverItsKey(t *testing.T) {
	f := &photoActionsFake{photo: experiment.TestPhoto{ID: "photo-1", Filename: "lunch.jpg", Key: "tests/alice/photo-1.jpg", Width: 800, Height: 600}}
	h := NewWritingTestHandler(&testActionsFake{}, &testActionsFake{}, f)
	resp, err := h.ConfirmTestPhotoUpload(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.ConfirmTestPhotoUploadRequest{UploadId: "photo-1", Width: 800, Height: 600}))
	if err != nil {
		t.Fatal(err)
	}
	if f.user != "alice" || f.confirmedID != "photo-1" || f.width != 800 || f.height != 600 || f.viewKey != "tests/alice/photo-1.jpg" {
		t.Fatalf("service saw %+v", f)
	}
	photo := resp.Msg.GetPhoto()
	if photo.GetId() != "photo-1" || photo.GetFilename() != "lunch.jpg" || photo.GetWidth() != 800 || photo.GetHeight() != 600 || photo.GetViewUrl() != "https://storage.test/view" || photo.GetViewExpiresAt() != "2026-11-17T09:10:00Z" {
		t.Fatalf("photo = %+v", photo)
	}
}

func TestTestPhotoUploadRefusalsMapToTheirWireReasons(t *testing.T) {
	cases := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{experiment.ErrTestNotFound, connect.CodeNotFound, "WRITING_TEST_NOT_FOUND"},
		{experiment.ErrTestMaterialInvalid, connect.CodeInvalidArgument, "WRITING_TEST_MATERIAL_INVALID"},
		{experiment.ErrTestPhotoLimit, connect.CodeFailedPrecondition, "POST_PHOTO_LIMIT"},
	}
	for _, c := range cases {
		h := NewWritingTestHandler(&testActionsFake{}, &testActionsFake{}, &photoActionsFake{err: c.err})
		ctx := auth.WithUser(context.Background(), "alice")
		_, create := h.CreateTestPhotoUpload(ctx, connect.NewRequest(&v1.CreateTestPhotoUploadRequest{Filename: "a.jpg"}))
		_, confirm := h.ConfirmTestPhotoUpload(ctx, connect.NewRequest(&v1.ConfirmTestPhotoUploadRequest{UploadId: "x", Width: 1, Height: 1}))
		for name, err := range map[string]error{"create": create, "confirm": confirm} {
			if connect.CodeOf(err) != c.code || experimentAppErrorDetail(t, err).GetReason() != c.reason {
				t.Fatalf("%s of %v: code=%v detail=%+v", name, c.err, connect.CodeOf(err), experimentAppErrorDetail(t, err))
			}
		}
	}
}

func TestTestPhotoUploadNeedsAnAuthenticatedOwner(t *testing.T) {
	f := &photoActionsFake{}
	h := NewWritingTestHandler(&testActionsFake{}, &testActionsFake{}, f)
	if _, err := h.CreateTestPhotoUpload(context.Background(), connect.NewRequest(&v1.CreateTestPhotoUploadRequest{Filename: "a.jpg"})); err == nil || f.filename != "" {
		t.Fatalf("an unauthenticated create reached the service: %v", err)
	}
	if _, err := h.ConfirmTestPhotoUpload(context.Background(), connect.NewRequest(&v1.ConfirmTestPhotoUploadRequest{UploadId: "x"})); err == nil || f.confirmedID != "" {
		t.Fatalf("an unauthenticated confirm reached the service: %v", err)
	}
}

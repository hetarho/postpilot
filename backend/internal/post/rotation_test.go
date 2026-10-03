package post

import (
	"context"
	"errors"
	"testing"
)

// POST-107: the owner turns a photo by a quarter turn; the turn is theirs from then on, and the
// observation that follows leaves it; a value that is not a quarter turn, another account, an
// unknown photo and a published post are refused.
func TestRotateImage(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")
	upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg", AttachmentPhoto)
	blobs.put(upload.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1024, 768, 0); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RotateImage(ctx, alice, upload.ID, 45); !errors.Is(err, ErrInvalidRotation) {
		t.Fatalf("45 degrees = %v, want ErrInvalidRotation", err)
	}
	if _, err := svc.RotateImage(ctx, bob, upload.ID, 90); !errors.Is(err, ErrForbidden) {
		t.Fatalf("another account = %v, want ErrForbidden", err)
	}
	if _, err := svc.RotateImage(ctx, alice, "nope", 90); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown photo = %v, want ErrNotFound", err)
	}
	turned, err := svc.RotateImage(ctx, alice, upload.ID, 90)
	if err != nil || turned.Rotation != 90 || !turned.RotationByOwner || turned.ViewURL == "" {
		t.Fatalf("RotateImage = %+v, %v", turned, err)
	}
	// An observation run afterwards says 270; the owner's 90 stands.
	if err := svc.SetObservations(ctx, alice, p.Slug, []Observation{{File: "IMG_1.jpg", Scene: "접시", Rotation: 270}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetImage(ctx, upload.ID); got.Rotation != 90 {
		t.Fatalf("an observation overrode the owner's turn: %+v", got)
	}
}

// GEN-79: a photo the owner never turned follows its observation.
func TestSetObservationsTurnsAPhotoTheOwnerNeverTurned(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")
	upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg", AttachmentPhoto)
	blobs.put(upload.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1024, 768, 0); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetObservations(ctx, alice, p.Slug, []Observation{{File: "IMG_1.jpg", Scene: "접시", Rotation: 90}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetImage(ctx, upload.ID); got.Rotation != 90 || got.RotationByOwner {
		t.Fatalf("observed photo = %+v", got)
	}
}

// POST-74: turning a photo is a write, so a published post refuses it.
func TestRotateImageOnAPublishedPostIsLocked(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	upload, _, _, _ := svc.CreateUpload(ctx, alice, finalized.Slug, "IMG_9.jpg", AttachmentPhoto)
	blobs.put(upload.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1024, 768, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RotateImage(ctx, alice, upload.ID, 90); !errors.Is(err, ErrPostPublished) {
		t.Fatalf("published turn = %v, want ErrPostPublished", err)
	}
	if got, _ := store.GetImage(ctx, upload.ID); got.Rotation != 0 {
		t.Fatalf("a published photo turned: %+v", got)
	}
}

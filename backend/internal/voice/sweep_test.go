package voice_test

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/voice"
)

// VOICE-60, →POST-40: the photo sweep reclaims an expired pending upload with its object and a
// `voices/` object nothing names, and never an answered photo, a pending upload still in its
// window, or anything when the listing fails.
func TestThePhotoSweepReclaimsOnlyUnnamedPhotos(t *testing.T) {
	h := newVoiceHarness(t)
	objects := h.withPhotos()
	ctx := context.Background()
	alice := h.voice("alice")

	answeredUpload, _, err := h.svc.CreatePhotoUpload(ctx, "alice", alice, "photo_food")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[answeredUpload.Key] = 100
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_food", Body: "맛있어요.", UploadID: answeredUpload.ID, PhotoWidth: 10, PhotoHeight: 10}); err != nil {
		t.Fatal(err)
	}
	pending, _, err := h.svc.CreatePhotoUpload(ctx, "alice", alice, "photo_space")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[pending.Key] = 100
	expired := voice.PhotoUpload{
		ID: "expired", UserID: "alice", VoiceID: alice, PromptKey: "photo_item", Key: "voices/" + alice + "/expired.jpg",
		ExpiresAt: time.Now().Add(-48 * time.Hour), CreatedAt: time.Now().Add(-49 * time.Hour),
	}
	if err := h.store.InsertPhotoUpload(ctx, expired); err != nil {
		t.Fatal(err)
	}
	objects.objects[expired.Key] = 100
	objects.objects["voices/"+alice+"/stray.jpg"] = 100

	voice.NewPhotoSweeper(h.store, objects, time.Hour).SweepOnce(ctx)

	for key, want := range map[string]bool{answeredUpload.Key: true, pending.Key: true, expired.Key: false, "voices/" + alice + "/stray.jpg": false} {
		if _, kept := objects.objects[key]; kept != want {
			t.Errorf("%s kept = %v, want %v", key, kept, want)
		}
	}
	if left, err := h.store.ListPhotoUploadsExpiredBefore(ctx, time.Now()); err != nil || len(left) != 0 {
		t.Fatalf("expired rows left = %+v err=%v", left, err)
	}
}

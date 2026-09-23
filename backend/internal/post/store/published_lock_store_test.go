package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store"
)

// publishedRow seeds a post with an answer, a confirmed photo and clip, and a pending photo and
// clip upload, then publishes it through the real statements.
func publishedRow(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)
	if err := s.UpsertTemplateAnswers(ctx, "p", []post.TemplateAnswer{{Label: "총평", Text: "맑았다", Enabled: true}}, testNow); err != nil {
		t.Fatal(err)
	}
	photoKey, clipKey := post.ObjectKey("p", "i1"), post.VideoObjectKey("p", "v1", "mp4")
	for _, upload := range []post.Upload{
		{ID: "i1", PostSlug: "p", Filename: "IMG_1.jpg", Key: photoKey, Kind: post.AttachmentPhoto, ContentType: "image/jpeg"},
		{ID: "v1", PostSlug: "p", Filename: "clip.mp4", Key: clipKey, Kind: post.AttachmentVideo, ContentType: "video/mp4"},
		{ID: "u-photo", PostSlug: "p", Filename: "IMG_2.jpg", Key: post.ObjectKey("p", "u-photo"), Kind: post.AttachmentPhoto, ContentType: "image/jpeg"},
		{ID: "u-clip", PostSlug: "p", Filename: "clip2.mp4", Key: post.VideoObjectKey("p", "u-clip", "mp4"), Kind: post.AttachmentVideo, ContentType: "video/mp4"},
	} {
		upload.ExpiresAt, upload.CreatedAt = testNow.Add(time.Hour), testNow
		if err := s.CreateUpload(ctx, upload); err != nil {
			t.Fatalf("CreateUpload(%s): %v", upload.ID, err)
		}
	}
	if err := s.ConfirmUpload(ctx, post.Image{ID: "i1", PostSlug: "p", Filename: "IMG_1.jpg", Key: photoKey, Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}, "i1"); err != nil {
		t.Fatal(err)
	}
	clip := testVideo("v1", "clip.mp4")
	if err := s.ConfirmVideoUpload(ctx, clip, "v1"); err != nil {
		t.Fatal(err)
	}
	content := post.PostContent{Title: "제주 3일", Blocks: []post.Block{{Type: post.BlockText, Content: "협재 해변은 물빛이 맑았다."}}}
	if updated, err := s.UpdateGeneratedContent(ctx, "p", "alice", content, post.LanguageKorean, testNow); err != nil || !updated {
		t.Fatalf("machine save: %v, %v", updated, err)
	}
	if updated, err := s.Finalize(ctx, "p", "alice", "제주 3일", 1, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("finalize: %v, %v", updated, err)
	}
	if published, err := s.PublishPost(ctx, "p", "alice", firstAddress, testNow.Add(2*time.Minute)); err != nil || !published {
		t.Fatalf("publish: %v, %v", published, err)
	}
	return s
}

// lockedRows is everything a refused statement must leave as it was.
type lockedRows struct {
	post    post.Post
	answers []post.TemplateAnswer
	images  []post.Image
	videos  []post.Video
	keys    map[string]struct{}
}

func readLockedRows(t *testing.T, s *store.Store) lockedRows {
	t.Helper()
	ctx := context.Background()
	var rows lockedRows
	var err error
	if rows.post, err = s.GetPost(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	if rows.answers, err = s.ListTemplateAnswers(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	if rows.images, err = s.ListImages(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	if rows.videos, err = s.ListVideos(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	// Every key the three tables name, so a lost or added upload row shows too.
	if rows.keys, err = s.AllReferencedKeys(ctx); err != nil {
		t.Fatal(err)
	}
	return rows
}

// POST-74: every statement that writes to a published post, or under one, refuses it — an
// update matches no row, a delete deletes nothing, an insert is refused by its guard — so a
// write that passed the service's check and lost the race to a publish changes nothing.
func TestEveryGuardedStatementRefusesAPublishedPost(t *testing.T) {
	later := testNow.Add(time.Hour)
	other := post.PostContent{Title: "고친 글", Blocks: []post.Block{{Type: post.BlockText, Content: "고친 문장"}}}
	length := 1200
	updates := map[string]func(*store.Store) (bool, error){
		"UpdatePostDraft": func(s *store.Store) (bool, error) {
			return s.UpdateDraft(context.Background(), "p", "alice", "새 제목", "새 메모", nil, later)
		},
		"UpdatePostObservations": func(s *store.Store) (bool, error) {
			return s.UpdateObservations(context.Background(), "p", "alice", []post.Observation{{File: "IMG_1.jpg", Scene: "바다"}}, later)
		},
		"UpdateGeneratedContent": func(s *store.Store) (bool, error) {
			return s.UpdateGeneratedContent(context.Background(), "p", "alice", other, post.LanguageKorean, later)
		},
		"SavePostContent": func(s *store.Store) (bool, error) {
			return s.SaveContent(context.Background(), "p", "alice", other, 1, later)
		},
		"SavePostGenerationOptions": func(s *store.Store) (bool, error) {
			return s.SaveGenerationOptions(context.Background(), "p", "alice", &length, 5, true, later)
		},
		"FinalizePost": func(s *store.Store) (bool, error) {
			return s.Finalize(context.Background(), "p", "alice", "제주 3일", 1, later)
		},
		"ReassignPostVoice": func(s *store.Store) (bool, error) {
			return s.ReassignVoice(context.Background(), "p", "alice", voiceIDFor("alice", 1), later)
		},
		"AssignPostTemplate": func(s *store.Store) (bool, error) {
			return s.AssignTemplate(context.Background(), "p", "alice", nil, post.TemplateNumbers{}, later)
		},
		"DeleteImage": func(s *store.Store) (bool, error) { return s.DeleteImage(context.Background(), "i1") },
		"DeleteVideo": func(s *store.Store) (bool, error) { return s.DeleteVideo(context.Background(), "v1") },
	}
	for name, update := range updates {
		t.Run(name, func(t *testing.T) {
			s := publishedRow(t)
			before := readLockedRows(t, s)
			if changed, err := update(s); err != nil || changed {
				t.Fatalf("changed = %v, err = %v; want no row", changed, err)
			}
			if after := readLockedRows(t, s); !reflect.DeepEqual(after, before) {
				t.Fatalf("a refused statement changed something:\nbefore %+v\nafter  %+v", before, after)
			}
		})
	}

	inserts := map[string]func(*store.Store) error{
		"UpsertTemplateAnswers": func(s *store.Store) error {
			return s.UpsertTemplateAnswers(context.Background(), "p", []post.TemplateAnswer{{Label: "총평", Text: "흐렸다", Enabled: true}}, later)
		},
		"CreateUpload": func(s *store.Store) error {
			return s.CreateUpload(context.Background(), post.Upload{
				ID: "u-new", PostSlug: "p", Filename: "IMG_3.jpg", Key: post.ObjectKey("p", "u-new"), Kind: post.AttachmentPhoto,
				ContentType: "image/jpeg", ExpiresAt: later, CreatedAt: later,
			})
		},
		"ConfirmUpload": func(s *store.Store) error {
			return s.ConfirmUpload(context.Background(), post.Image{
				ID: "u-photo", PostSlug: "p", Filename: "IMG_2.jpg", Key: post.ObjectKey("p", "u-photo"), Width: 1, Height: 1, Bytes: 1, CreatedAt: later,
			}, "u-photo")
		},
		"ConfirmVideoUpload": func(s *store.Store) error {
			clip := testVideo("u-clip", "clip2.mp4")
			return s.ConfirmVideoUpload(context.Background(), clip, "u-clip")
		},
	}
	for name, insert := range inserts {
		t.Run(name, func(t *testing.T) {
			s := publishedRow(t)
			before := readLockedRows(t, s)
			if err := insert(s); !errors.Is(err, post.ErrPostPublished) {
				t.Fatalf("err = %v, want post.ErrPostPublished", err)
			}
			if after := readLockedRows(t, s); !reflect.DeepEqual(after, before) {
				t.Fatalf("a refused insert changed something:\nbefore %+v\nafter  %+v", before, after)
			}
		})
	}

	// The guard is the published status and nothing else: once the address is cleared, the
	// same statements write again.
	t.Run("a cleared address unlocks the post", func(t *testing.T) {
		s := publishedRow(t)
		if cleared, err := s.UnpublishPost(context.Background(), "p", "alice", later); err != nil || !cleared {
			t.Fatalf("clear: %v, %v", cleared, err)
		}
		if changed, err := s.UpdateDraft(context.Background(), "p", "alice", "새 제목", "", nil, later); err != nil || !changed {
			t.Fatalf("draft after the clear: %v, %v", changed, err)
		}
		if deleted, err := s.DeleteImage(context.Background(), "i1"); err != nil || !deleted {
			t.Fatalf("photo delete after the clear: %v, %v", deleted, err)
		}
		if err := s.ConfirmUpload(context.Background(), post.Image{
			ID: "u-photo", PostSlug: "p", Filename: "IMG_2.jpg", Key: post.ObjectKey("p", "u-photo"), Width: 1, Height: 1, Bytes: 1, CreatedAt: later,
		}, "u-photo"); err != nil {
			t.Fatalf("confirm after the clear: %v", err)
		}
	})
}

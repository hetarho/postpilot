package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store"
)

var testNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// newStore opens a throwaway SQLite database, applies the embedded migrations, and
// seeds a user — posts reference users, so the foreign key needs one to exist.
func newStore(t *testing.T) *store.Store {
	t.Helper()

	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	users := authstore.New(handle.Writer, handle.Reader)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(context.Background(), auth.User{ID: id, PasswordHash: "hash", Plan: plan.Free, CreatedAt: testNow}); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
		// A post names its voice, so each account gets its default plus one more.
		stamp := testNow.UTC().Format(time.RFC3339Nano)
		for i, name := range []string{"기본 말투", "리뷰"} {
			if _, err := handle.Writer.Exec("INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,?,?,?,?)",
				voiceIDFor(id, i), id, name, boolInt(i == 0), stamp, stamp); err != nil {
				t.Fatalf("seed voice %s: %v", id, err)
			}
		}
	}

	return store.New(handle.Writer, handle.Reader)
}

func voiceIDFor(userID string, index int) string {
	if index == 0 {
		return "voice-" + userID
	}
	return "voice-" + userID + "-review"
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func seedPost(t *testing.T, s *store.Store, slug, userID string, updatedAt time.Time) post.Post {
	t.Helper()
	p := post.Post{
		Slug: slug, UserID: userID, VoiceID: voiceIDFor(userID, 0), Title: slug, Memo: "",
		Status: post.StatusDraft, TargetLanguage: post.LanguageKorean, CreatedAt: testNow, UpdatedAt: updatedAt,
	}
	if err := s.CreatePost(context.Background(), p); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	return p
}

func TestPostRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// A non-UTC, sub-second timestamp: the store must normalize it and return the same
	// instant.
	created := time.Date(2026, 3, 1, 21, 30, 45, 123456789, time.FixedZone("KST", 9*3600))
	want := post.Post{Slug: "20260301-jeju", UserID: "alice", VoiceID: "voice-alice", Title: "Jeju", Memo: "went",
		Status: post.StatusDraft, TargetLanguage: post.LanguageKorean, CreatedAt: created, UpdatedAt: created}
	if err := s.CreatePost(ctx, want); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	got, err := s.GetPost(ctx, want.Slug)
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if got.UserID != "alice" || got.VoiceID != "voice-alice" || got.Title != "Jeju" || got.Memo != "went" || got.Status != post.StatusDraft || got.MachineBaselineVoiceID != "" {
		t.Errorf("post = %+v", got)
	}
	// The schema, not the service, is the last line: a post cannot name another account's
	// voice or a voice that does not exist.
	if err := s.CreatePost(ctx, post.Post{Slug: "foreign-voice", UserID: "alice", VoiceID: "voice-bob", Status: post.StatusDraft, TargetLanguage: post.LanguageKorean, CreatedAt: created, UpdatedAt: created}); err == nil {
		t.Error("a post naming another account's voice was accepted")
	}
	if err := s.CreatePost(ctx, post.Post{Slug: "no-voice", UserID: "alice", Status: post.StatusDraft, TargetLanguage: post.LanguageKorean, CreatedAt: created, UpdatedAt: created}); err == nil {
		t.Error("a post without a voice was accepted")
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("created_at = %v, want the same instant as %v", got.CreatedAt, created)
	}
}

func TestGeneratedContentAndObservationsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)
	observations := []post.Observation{{
		File: "IMG_1.jpg", Scene: "바다", Mood: "차분", VisibleText: "표지판",
		Objects: []string{"파도"}, PeoplePresent: true,
	}}
	updated, err := s.UpdateObservations(ctx, "p", "alice", observations, testNow.Add(time.Minute))
	if err != nil || !updated {
		t.Fatalf("UpdateObservations: updated=%v err=%v", updated, err)
	}
	content := post.PostContent{Title: "완성", Summary: "요약", Tags: []string{"여행"}, Blocks: []post.Block{
		{Type: post.BlockText, Content: "본문"},
		{Type: post.BlockImage, File: "IMG_1.jpg", Caption: "바다"},
	}}
	updated, err = s.UpdateGeneratedContent(ctx, "p", "alice", content, post.LanguageKorean, testNow.Add(2*time.Minute))
	if err != nil || !updated {
		t.Fatalf("UpdateGeneratedContent: updated=%v err=%v", updated, err)
	}
	updated, err = s.UpdateGeneratedContent(ctx, "p", "alice", content, post.LanguageKorean, testNow.Add(3*time.Minute))
	if err != nil || updated {
		t.Fatalf("identical machine retry: updated=%v err=%v", updated, err)
	}
	got, err := s.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != post.StatusReview || got.Content == nil || got.Content.Title != "완성" || len(got.Content.Blocks) != 2 {
		t.Fatalf("post = %+v", got)
	}
	if got.ContentRevision != 1 || !got.UpdatedAt.Equal(testNow.Add(2*time.Minute)) {
		t.Fatalf("identical retry advanced post: revision=%d updated_at=%v", got.ContentRevision, got.UpdatedAt)
	}
	if len(got.Observations) != 1 || got.Observations[0].VisibleText != "표지판" || !got.Observations[0].PeoplePresent {
		t.Fatalf("observations = %+v", got.Observations)
	}
	updated, err = s.UpdateGeneratedContent(ctx, "p", "bob", post.PostContent{Title: "hijack"}, post.LanguageKorean, testNow)
	if err != nil || updated {
		t.Fatalf("foreign update: updated=%v err=%v", updated, err)
	}
}

func TestTargetChangePreservesPostStateAndFrozenMachineWriteKeepsItsOwnLanguage(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "language-transition", "alice", testNow)
	observations := []post.Observation{{File: "one.jpg", Scene: "raw-scene", Objects: []string{"raw-object"}}}
	if updated, err := s.UpdateObservations(ctx, "language-transition", "alice", observations, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("observations: updated=%v err=%v", updated, err)
	}
	content := post.PostContent{Title: "동일 본문", Summary: "raw-summary", Tags: []string{"raw-tag"}, Blocks: []post.Block{{Type: post.BlockText, Content: "raw-content"}}}
	if updated, err := s.UpdateGeneratedContent(ctx, "language-transition", "alice", content, post.LanguageKorean, testNow.Add(2*time.Minute)); err != nil || !updated {
		t.Fatalf("machine write: updated=%v err=%v", updated, err)
	}
	before, err := s.GetPost(ctx, "language-transition")
	if err != nil {
		t.Fatal(err)
	}
	english := post.LanguageEnglish
	changedAt := testNow.Add(3 * time.Minute)
	if updated, err := s.UpdateDraft(ctx, before.Slug, before.UserID, before.Title, before.Memo, &english, changedAt); err != nil || !updated {
		t.Fatalf("target update: updated=%v err=%v", updated, err)
	}
	after, err := s.GetPost(ctx, before.Slug)
	if err != nil {
		t.Fatal(err)
	}
	want := before
	want.TargetLanguage = post.LanguageEnglish
	want.UpdatedAt = changedAt
	if !reflect.DeepEqual(after, want) {
		t.Fatalf("target update changed unrelated state\n got: %#v\nwant: %#v", after, want)
	}
	if after.ContentLanguage == nil || *after.ContentLanguage != post.LanguageKorean {
		t.Fatalf("target update changed provenance = %v", after.ContentLanguage)
	}

	// A Korean run frozen before the target changed may still complete afterwards. Its
	// content provenance follows the frozen run, not the post's newer English target.
	oldFrozenResult := post.PostContent{Title: "늦게 도착한 결과", Blocks: []post.Block{{Type: post.BlockText, Content: "한국어 결과"}}}
	if updated, err := s.UpdateGeneratedContent(ctx, after.Slug, after.UserID, oldFrozenResult, post.LanguageKorean, testNow.Add(4*time.Minute)); err != nil || !updated {
		t.Fatalf("old frozen write: updated=%v err=%v", updated, err)
	}
	landed, err := s.GetPost(ctx, after.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if landed.TargetLanguage != post.LanguageEnglish || landed.ContentLanguage == nil || *landed.ContentLanguage != post.LanguageKorean {
		t.Fatalf("frozen result was retargeted: target=%q content=%v", landed.TargetLanguage, landed.ContentLanguage)
	}

	// Provenance is part of idempotence: identical bytes with a different canonical
	// language still require one atomic machine write.
	if updated, err := s.UpdateGeneratedContent(ctx, after.Slug, after.UserID, oldFrozenResult, post.LanguageEnglish, testNow.Add(5*time.Minute)); err != nil || !updated {
		t.Fatalf("provenance-only machine write: updated=%v err=%v", updated, err)
	}
	remapped, err := s.GetPost(ctx, after.Slug)
	if err != nil || remapped.ContentLanguage == nil || *remapped.ContentLanguage != post.LanguageEnglish || remapped.ContentRevision != landed.ContentRevision+1 {
		t.Fatalf("provenance-only write = %+v err=%v", remapped, err)
	}
}

func TestContentSavePreservesFrozenMachineBaseline(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "editable", "alice", testNow)
	baseline := post.PostContent{Title: "machine", Blocks: []post.Block{{Type: post.BlockText, Content: "생성 문장입니다."}}}
	target1400 := 1400
	if updated, err := s.SaveGenerationOptions(ctx, "editable", "alice", &target1400, testNow); err != nil || !updated {
		t.Fatalf("option save: updated=%v err=%v", updated, err)
	}
	if updated, err := s.UpdateGeneratedContent(ctx, "editable", "alice", baseline, post.LanguageKorean, testNow); err != nil || !updated {
		t.Fatalf("machine save: updated=%v err=%v", updated, err)
	}
	final := post.PostContent{Title: "mine", Blocks: []post.Block{{Type: post.BlockText, Content: "제가 고친 문장이에요."}}}
	if updated, err := s.SaveContent(ctx, "editable", "alice", final, 1, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("manual save: updated=%v err=%v", updated, err)
	}
	if updated, err := s.SaveContent(ctx, "editable", "alice", baseline, 1, testNow); err != nil || updated {
		t.Fatalf("stale save: updated=%v err=%v", updated, err)
	}
	target1500 := 1500
	if updated, err := s.SaveGenerationOptions(ctx, "editable", "alice", &target1500, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("option update: updated=%v err=%v", updated, err)
	}
	if updated, err := s.Finalize(ctx, "editable", "alice", "machine", 2, testNow.Add(2*time.Minute)); err != nil || !updated {
		t.Fatalf("finalize: updated=%v err=%v", updated, err)
	}
	snapshot, err := s.LearningSnapshot(ctx, "editable", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.MachineBaseline.Title != "machine" || snapshot.Current.Title != "mine" || snapshot.BaselineRevision != 1 || snapshot.ContentRevision != 2 || snapshot.TargetLength == nil || *snapshot.TargetLength != 1500 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if snapshot.VoiceID != "voice-alice" || snapshot.MachineBaselineVoiceID != "voice-alice" {
		t.Fatalf("snapshot voices = %q / %q", snapshot.VoiceID, snapshot.MachineBaselineVoiceID)
	}
	nextBaseline := post.PostContent{Title: "machine 2", Blocks: []post.Block{{Type: post.BlockText, Content: "새 기준 문장입니다."}}}
	if updated, err := s.UpdateGeneratedContent(ctx, "editable", "alice", nextBaseline, post.LanguageKorean, testNow.Add(3*time.Minute)); err != nil || !updated {
		t.Fatalf("second machine save: updated=%v err=%v", updated, err)
	}
	if _, err = s.LearningSnapshot(ctx, "editable", "alice"); !errors.Is(err, post.ErrPostNotFinalized) {
		t.Fatalf("unfinalized snapshot err=%v", err)
	}
	if updated, err := s.Finalize(ctx, "editable", "alice", "machine 2", 3, testNow.Add(4*time.Minute)); err != nil || !updated {
		t.Fatalf("second finalize: updated=%v err=%v", updated, err)
	}
	snapshot, err = s.LearningSnapshot(ctx, "editable", "alice")
	if err != nil || snapshot.BaselineRevision != 3 || snapshot.ContentRevision != 3 || snapshot.MachineBaseline.Title != "machine 2" {
		t.Fatalf("second snapshot = %+v err=%v", snapshot, err)
	}
	if _, err := s.LearningSnapshot(ctx, "editable", "bob"); !errors.Is(err, post.ErrNotFound) {
		t.Fatalf("foreign snapshot err=%v", err)
	}
}

func TestGetPostUnknown(t *testing.T) {
	if _, err := newStore(t).GetPost(context.Background(), "nope"); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound (sql.ErrNoRows must not escape)", err)
	}
}

// UpdateDraft carries the ownership check in the WHERE clause, so a foreign slug
// updates nothing rather than relying on the caller having checked first.
func TestUpdateDraftIsScopedToTheOwner(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	updated, err := s.UpdateDraft(ctx, "p", "bob", "hijacked", "hijacked", nil, testNow)
	if err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}
	if updated {
		t.Fatal("another user's post was updated")
	}

	got, _ := s.GetPost(ctx, "p")
	if got.Title == "hijacked" {
		t.Error("the row was modified")
	}

	updated, err = s.UpdateDraft(ctx, "p", "alice", "mine", "memo", nil, testNow.Add(time.Hour))
	if err != nil || !updated {
		t.Fatalf("owner update: updated=%v err=%v", updated, err)
	}
}

func TestListPostsNewestFirstAndScoped(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "old", "alice", testNow.Add(-2*time.Hour))
	seedPost(t, s, "new", "alice", testNow)
	seedPost(t, s, "theirs", "bob", testNow.Add(time.Hour))

	got, err := s.ListPosts(ctx, "alice")
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d posts, want 2", len(got))
	}
	if got[0].Slug != "new" || got[1].Slug != "old" {
		t.Errorf("order = %s, %s; want new, old", got[0].Slug, got[1].Slug)
	}
}

func TestListPostsFallsBackToGeneratedTitle(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	p := seedPost(t, s, "untitled", "alice", testNow)
	p.Title = ""
	if updated, err := s.UpdateDraft(ctx, p.Slug, p.UserID, "", "memo", nil, testNow); err != nil || !updated {
		t.Fatalf("clear title: updated=%v err=%v", updated, err)
	}
	if updated, err := s.UpdateGeneratedContent(ctx, p.Slug, p.UserID, post.PostContent{Title: "Generated title"}, post.LanguageKorean, testNow); err != nil || !updated {
		t.Fatalf("generated content: updated=%v err=%v", updated, err)
	}
	got, err := s.ListPosts(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Generated title" {
		t.Fatalf("summaries = %+v", got)
	}
}

// The list orders on a plain string comparison of updated_at, so the stored format has
// to be fixed-width — a trimmed fraction would sort "…08.5Z" after "…08.51Z".
func TestListPostsOrdersSubSecondTimestamps(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	base := time.Date(2026, 3, 1, 12, 0, 8, 0, time.UTC)
	seedPost(t, s, "earlier", "alice", base.Add(500*time.Millisecond))
	seedPost(t, s, "later", "alice", base.Add(513110616*time.Nanosecond))

	got, err := s.ListPosts(ctx, "alice")
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if got[0].Slug != "later" {
		t.Errorf("newest = %q, want later", got[0].Slug)
	}
}

func TestImageRoundTripAndDelete(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	img := post.Image{ID: "i1", PostSlug: "p", Filename: "IMG_1.jpg", Key: post.ObjectKey("p", "i1"),
		Width: 1024, Height: 768, Bytes: 204800, CreatedAt: testNow}
	if err := s.CreateImage(ctx, img); err != nil {
		t.Fatalf("CreateImage: %v", err)
	}

	got, err := s.GetImage(ctx, "i1")
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if got.Filename != img.Filename || got.Key != img.Key || got.Width != 1024 || got.Bytes != 204800 {
		t.Errorf("image = %+v", got)
	}

	if err := s.DeleteImage(ctx, "i1"); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	if _, err := s.GetImage(ctx, "i1"); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ImageFilenameTaken reports CONFIRMED photos only: a pending upload is a retry, which
// CreateUpload replaces rather than refuses.
func TestImageFilenameTaken(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)
	seedPost(t, s, "other", "alice", testNow)

	taken, err := s.ImageFilenameTaken(ctx, "p", "IMG_1.jpg")
	if err != nil || taken {
		t.Fatalf("fresh: taken=%v err=%v", taken, err)
	}

	if err := s.CreateUpload(ctx, post.Upload{ID: "u1", PostSlug: "p", Filename: "IMG_1.jpg",
		Key: post.ObjectKey("p", "u1"), ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if taken, _ := s.ImageFilenameTaken(ctx, "p", "IMG_1.jpg"); taken {
		t.Error("a pending upload was reported as a confirmed photo")
	}

	if err := s.ConfirmUpload(ctx, post.Image{ID: "u1", PostSlug: "p", Filename: "IMG_1.jpg",
		Key: post.ObjectKey("p", "u1"), Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}, "u1"); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	if taken, _ := s.ImageFilenameTaken(ctx, "p", "IMG_1.jpg"); !taken {
		t.Error("a confirmed image did not reserve the filename")
	}
	// The same name under another post is free.
	if taken, _ := s.ImageFilenameTaken(ctx, "other", "IMG_1.jpg"); taken {
		t.Error("a filename was reserved across posts")
	}
}

// The schema, not the service precheck, is what actually stops two concurrent
// CreateUpload calls for one filename.
func TestUploadFilenameIsUniquePerPost(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	first := post.Upload{ID: "u1", PostSlug: "p", Filename: "IMG_1.jpg",
		Key: post.ObjectKey("p", "u1"), ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}
	if err := s.CreateUpload(ctx, first); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	second := first
	second.ID, second.Key = "u2", post.ObjectKey("p", "u2")
	if err := s.CreateUpload(ctx, second); !errors.Is(err, post.ErrDuplicateFilename) {
		t.Errorf("err = %v, want ErrDuplicateFilename", err)
	}
}

// ConfirmUpload must move the key from uploads to images in one step: a surviving
// upload row would still name the live photo's object, and the sweep would delete it.
func TestConfirmUploadIsAtomic(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	key := post.ObjectKey("p", "u1")
	if err := s.CreateUpload(ctx, post.Upload{ID: "u1", PostSlug: "p", Filename: "IMG_1.jpg",
		Key: key, ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	if err := s.ConfirmUpload(ctx, post.Image{ID: "u1", PostSlug: "p", Filename: "IMG_1.jpg",
		Key: key, Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}, "u1"); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	if _, err := s.GetImage(ctx, "u1"); err != nil {
		t.Errorf("the image was not written: %v", err)
	}
	if _, err := s.GetUpload(ctx, "u1"); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("the upload row survived the confirm: %v", err)
	}
	// Exactly one table names the key.
	inUse, err := s.ImageKeyInUse(ctx, key)
	if err != nil || !inUse {
		t.Errorf("ImageKeyInUse = %v, %v; want true", inUse, err)
	}
	keys, _ := s.AllReferencedKeys(ctx)
	if len(keys) != 1 {
		t.Errorf("referenced keys = %v, want exactly one", keys)
	}
}

// A failed confirm must leave BOTH sides untouched, or the upload becomes unretryable.
func TestConfirmUploadRollsBack(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	// A confirmed photo already holds the filename.
	if err := s.CreateImage(ctx, post.Image{ID: "existing", PostSlug: "p", Filename: "IMG_1.jpg",
		Key: "posts/p/existing.jpg", Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	if err := s.CreateUpload(ctx, post.Upload{ID: "u1", PostSlug: "p", Filename: "OTHER.jpg",
		Key: post.ObjectKey("p", "u1"), ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	err := s.ConfirmUpload(ctx, post.Image{ID: "u1", PostSlug: "p", Filename: "IMG_1.jpg",
		Key: post.ObjectKey("p", "u1"), Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}, "u1")
	if !errors.Is(err, post.ErrDuplicateFilename) {
		t.Fatalf("err = %v, want ErrDuplicateFilename", err)
	}
	if _, err := s.GetUpload(ctx, "u1"); err != nil {
		t.Errorf("the upload row was dropped by a failed confirm: %v", err)
	}
}

func TestGetUploadByFilename(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	if _, err := s.GetUploadByFilename(ctx, "p", "IMG_1.jpg"); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if err := s.CreateUpload(ctx, post.Upload{ID: "u1", PostSlug: "p", Filename: "IMG_1.jpg",
		Key: post.ObjectKey("p", "u1"), ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	got, err := s.GetUploadByFilename(ctx, "p", "IMG_1.jpg")
	if err != nil || got.ID != "u1" {
		t.Errorf("got %+v, %v", got, err)
	}
}

// A duplicate slug must arrive as the domain error createPost retries on, not as a raw
// constraint failure.
func TestCreatePostDuplicateSlug(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	err := s.CreatePost(ctx, post.Post{Slug: "p", UserID: "bob", VoiceID: "voice-bob", Status: post.StatusDraft,
		TargetLanguage: post.LanguageKorean, CreatedAt: testNow, UpdatedAt: testNow})
	if !errors.Is(err, post.ErrDuplicateSlug) {
		t.Errorf("err = %v, want ErrDuplicateSlug", err)
	}
}

// Plan 10 A8 at the SQL level: one UPDATE moves the post and withdraws the old voice's
// baseline/eligibility, and it is owner-scoped and a no-op for the same voice.
func TestReassignVoiceIsOneOwnedWriteThatKeepsContent(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "moving", "alice", testNow)
	baseline := post.PostContent{Title: "machine", Blocks: []post.Block{{Type: post.BlockText, Content: "생성 문장입니다."}}}
	if updated, err := s.UpdateGeneratedContent(ctx, "moving", "alice", baseline, post.LanguageKorean, testNow); err != nil || !updated {
		t.Fatalf("machine save: updated=%v err=%v", updated, err)
	}
	if updated, err := s.Finalize(ctx, "moving", "alice", "machine", 1, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("finalize: updated=%v err=%v", updated, err)
	}
	if moved, err := s.ReassignVoice(ctx, "moving", "bob", "voice-bob", testNow.Add(2*time.Minute)); err != nil || moved {
		t.Fatalf("foreign reassign: moved=%v err=%v", moved, err)
	}
	if moved, err := s.ReassignVoice(ctx, "moving", "alice", "voice-alice", testNow.Add(2*time.Minute)); err != nil || moved {
		t.Fatalf("same-voice reassign: moved=%v err=%v", moved, err)
	}
	if _, err := s.ReassignVoice(ctx, "moving", "alice", "voice-bob", testNow.Add(2*time.Minute)); !errors.Is(err, post.ErrVoiceNotFound) {
		t.Fatalf("reassign to another account's voice = %v", err)
	}
	moved, err := s.ReassignVoice(ctx, "moving", "alice", "voice-alice-review", testNow.Add(2*time.Minute))
	if err != nil || !moved {
		t.Fatalf("reassign: moved=%v err=%v", moved, err)
	}
	got, err := s.GetPost(ctx, "moving")
	if err != nil {
		t.Fatal(err)
	}
	if got.VoiceID != "voice-alice-review" || got.MachineBaselineVoiceID != "" {
		t.Fatalf("voices after reassign = %q / %q", got.VoiceID, got.MachineBaselineVoiceID)
	}
	if got.Content == nil || got.Content.Title != "machine" || got.ContentRevision != 1 || got.MachineBaselineRevision != 0 || got.Status != post.StatusFinalized || got.FinalizedRevision != 1 {
		t.Fatalf("reassign changed content or lifecycle: %+v", got)
	}
	// A later machine result under the new voice establishes a fresh baseline and restores
	// learn eligibility there.
	if updated, err := s.UpdateGeneratedContent(ctx, "moving", "alice", post.PostContent{Title: "again", Blocks: []post.Block{{Type: post.BlockText, Content: "새 문장입니다."}}}, post.LanguageKorean, testNow.Add(3*time.Minute)); err != nil || !updated {
		t.Fatalf("second machine save: updated=%v err=%v", updated, err)
	}
	if got, _ = s.GetPost(ctx, "moving"); got.MachineBaselineVoiceID != "voice-alice-review" {
		t.Fatalf("new baseline voice = %q", got.MachineBaselineVoiceID)
	}
}

func TestFinalizeAfterReassignmentDoesNotRequireALearningBaseline(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "moving-review", "alice", testNow)
	content := post.PostContent{Title: "machine", Blocks: []post.Block{{Type: post.BlockText, Content: "본문"}}}
	if updated, err := s.UpdateGeneratedContent(ctx, "moving-review", "alice", content, post.LanguageKorean, testNow); err != nil || !updated {
		t.Fatalf("machine save: updated=%v err=%v", updated, err)
	}
	if moved, err := s.ReassignVoice(ctx, "moving-review", "alice", "voice-alice-review", testNow.Add(time.Minute)); err != nil || !moved {
		t.Fatalf("reassign: moved=%v err=%v", moved, err)
	}
	if updated, err := s.Finalize(ctx, "moving-review", "alice", "machine", 1, testNow.Add(2*time.Minute)); err != nil || !updated {
		t.Fatalf("finalize without baseline: updated=%v err=%v", updated, err)
	}
	got, err := s.GetPost(ctx, "moving-review")
	if err != nil || got.Status != post.StatusFinalized || got.MachineBaselineRevision != 0 {
		t.Fatalf("finalized reassigned post = %+v err=%v", got, err)
	}
}

// The UNIQUE(post_slug, filename) constraint is the last line of defence behind the
// FilenameTaken check.
func TestDuplicateFilenameIsRefusedByTheSchema(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	img := post.Image{ID: "i1", PostSlug: "p", Filename: "IMG_1.jpg", Key: "k1",
		Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}
	if err := s.CreateImage(ctx, img); err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	img.ID, img.Key = "i2", "k2"
	if err := s.CreateImage(ctx, img); err == nil {
		t.Error("a duplicate filename was accepted")
	}
}

func TestListUploadsExpiredBefore(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	for id, expiry := range map[string]time.Time{
		"dead": testNow.Add(-2 * time.Hour),
		"live": testNow.Add(2 * time.Hour),
	} {
		if err := s.CreateUpload(ctx, post.Upload{ID: id, PostSlug: "p", Filename: id + ".jpg",
			Key: post.ObjectKey("p", id), ExpiresAt: expiry, CreatedAt: testNow}); err != nil {
			t.Fatalf("CreateUpload(%s): %v", id, err)
		}
	}

	got, err := s.ListUploadsExpiredBefore(ctx, testNow)
	if err != nil {
		t.Fatalf("ListUploadsExpiredBefore: %v", err)
	}
	if len(got) != 1 || got[0].ID != "dead" {
		t.Errorf("got %+v, want just the expired one", got)
	}
}

// The sweep deletes objects missing from this set, so it has to cover BOTH tables — a
// key omitted here is a live photo deleted from storage.
func TestAllReferencedKeys(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	if err := s.CreateImage(ctx, post.Image{ID: "i1", PostSlug: "p", Filename: "a.jpg",
		Key: "posts/p/i1.jpg", Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	if err := s.CreateUpload(ctx, post.Upload{ID: "u1", PostSlug: "p", Filename: "b.jpg",
		Key: "posts/p/u1.jpg", ExpiresAt: testNow, CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	keys, err := s.AllReferencedKeys(ctx)
	if err != nil {
		t.Fatalf("AllReferencedKeys: %v", err)
	}
	for _, key := range []string{"posts/p/i1.jpg", "posts/p/u1.jpg"} {
		if _, ok := keys[key]; !ok {
			t.Errorf("%s is missing — the sweep would delete a referenced object", key)
		}
	}
	if len(keys) != 2 {
		t.Errorf("got %d keys, want 2", len(keys))
	}
}

// A photo can only exist under a real post. This is the foreign key doing its job; the
// ON DELETE CASCADE half has no exercise yet because nothing deletes a post (the PRD
// does not define post deletion).
func TestImageRequiresItsPost(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	// A photo for a post that does not exist must be refused outright.
	err := s.CreateImage(ctx, post.Image{ID: "orphan", PostSlug: "ghost", Filename: "a.jpg",
		Key: "k", Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow})
	if err == nil {
		t.Error("an image for an unknown post was accepted — foreign keys are off")
	}
}

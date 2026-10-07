package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
)

func TestInputRevisionAdvancesOnlyOnActualMaterialOrSettingChanges(t *testing.T) {
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)
	ctx := context.Background()
	readRevision := func() int64 {
		t.Helper()
		p, err := s.GetPost(ctx, "p")
		if err != nil {
			t.Fatal(err)
		}
		return p.InputRevision
	}
	if readRevision() != 1 {
		t.Fatal("a new post needs a concrete input revision")
	}
	check := func(name string, change bool, mutate func() error) {
		t.Helper()
		before := readRevision()
		if err := mutate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := before
		if change {
			want++
		}
		if got := readRevision(); got != want {
			t.Fatalf("%s revision = %d, want %d", name, got, want)
		}
	}
	draft := func(title, memo string, language *post.Language) func() error {
		return func() error { _, err := s.UpdateDraft(ctx, "p", "alice", title, memo, language, testNow); return err }
	}
	check("title", true, draft("New title", "", nil))
	check("memo", true, draft("New title", "New memo", nil))
	check("same draft", false, draft("New title", "New memo", nil))
	en := post.LanguageEnglish
	check("language", true, draft("New title", "New memo", &en))
	check("same language", false, draft("New title", "New memo", &en))
	answer := []post.TemplateAnswer{{Label: "Topic", Text: "First", Enabled: true}}
	check("answer", true, func() error { return s.UpsertTemplateAnswers(ctx, "p", answer, testNow) })
	check("same answer", false, func() error { return s.UpsertTemplateAnswers(ctx, "p", answer, testNow.Add(time.Hour)) })
	answer[0].Enabled = false
	check("answer enabled", true, func() error { return s.UpsertTemplateAnswers(ctx, "p", answer, testNow) })
	check("voice", true, func() error { _, err := s.ReassignVoice(ctx, "p", "alice", "", testNow); return err })
	check("same voice", false, func() error { _, err := s.ReassignVoice(ctx, "p", "alice", "", testNow); return err })
	field := "cafe"
	check("field", true, func() error { _, err := s.AssignField(ctx, "p", "alice", &field, testNow); return err })
	check("same field", false, func() error { _, err := s.AssignField(ctx, "p", "alice", &field, testNow); return err })
	options := post.GenerationOptionsSet{TagCount: post.TagCountRange.Default, Field: "cafe"}
	check("equivalent default options", false, func() error { _, err := s.SaveGenerationOptions(ctx, "p", "alice", options, testNow); return err })
	options.UseMemory = true
	check("generation options", true, func() error { _, err := s.SaveGenerationOptions(ctx, "p", "alice", options, testNow); return err })
	check("same generation options", false, func() error { _, err := s.SaveGenerationOptions(ctx, "p", "alice", options, testNow); return err })
	story := &post.Storyline{Paragraphs: []post.StorylineParagraph{{Text: "Plan"}}}
	check("storyline", true, func() error { _, err := s.UpdateStoryline(ctx, "p", "alice", story, testNow); return err })
	check("same storyline", false, func() error { _, err := s.UpdateStoryline(ctx, "p", "alice", story, testNow); return err })
	content := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Generated"}}}
	check("content without storyline change", false, func() error {
		_, err := s.UpdateGeneratedContent(ctx, "p", "alice", content, post.LanguageEnglish, post.WriteAnnotations{Storyline: story}, testNow)
		return err
	})
	check("confirmed title", true, func() error { _, err := s.Finalize(ctx, "p", "alice", "Confirmed title", 1, testNow); return err })
	check("same confirmed title", false, func() error { _, err := s.Finalize(ctx, "p", "alice", "Confirmed title", 1, testNow); return err })
}

func TestConfirmedAttachmentsAndActualTurnsAdvanceInputRevision(t *testing.T) {
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)
	ctx := context.Background()
	image := post.Image{ID: "photo", PostSlug: "p", Filename: "photo.jpg", Key: "photo", Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}
	upload := post.Upload{ID: "photo", PostSlug: "p", Filename: "photo.jpg", Key: "photo", Kind: post.AttachmentPhoto, ContentType: "image/jpeg", CreatedAt: testNow, ExpiresAt: testNow.Add(time.Hour)}
	if err := s.CreateUpload(ctx, upload); err != nil {
		t.Fatal(err)
	}
	assertRevision := func(want int64) {
		t.Helper()
		p, err := s.GetPost(ctx, "p")
		if err != nil || p.InputRevision != want {
			t.Fatalf("revision = %d, want %d: %v", p.InputRevision, want, err)
		}
	}
	assertRevision(1) // an unconfirmed reservation is not material
	if err := s.ConfirmUpload(ctx, image, upload.ID); err != nil {
		t.Fatal(err)
	}
	assertRevision(2)
	if _, err := s.SetImageRotation(ctx, image.ID, 90); err != nil {
		t.Fatal(err)
	}
	assertRevision(3)
	if _, err := s.SetImageRotation(ctx, image.ID, 90); err != nil {
		t.Fatal(err)
	}
	assertRevision(3)
	if _, err := s.DeleteImage(ctx, image.ID); err != nil {
		t.Fatal(err)
	}
	assertRevision(4)
	if _, err := s.DeleteImage(ctx, image.ID); err != nil {
		t.Fatal(err)
	}
	assertRevision(4)
	video := testVideo("clip", "clip.mp4")
	if err := s.CreateUpload(ctx, post.Upload{ID: video.ID, PostSlug: "p", Filename: video.Filename, Key: video.Key, Kind: post.AttachmentVideo, ContentType: video.ContentType, CreatedAt: testNow, ExpiresAt: testNow.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmVideoUpload(ctx, video, video.ID); err != nil {
		t.Fatal(err)
	}
	assertRevision(5)
	if _, err := s.DeleteVideo(ctx, video.ID); err != nil {
		t.Fatal(err)
	}
	assertRevision(6)
	if _, err := s.DeleteVideo(ctx, video.ID); err != nil {
		t.Fatal(err)
	}
	assertRevision(6)
}

func TestHistoryReadinessUsesActualCanonicalContentAndLanguage(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	for _, slug := range []string{"draft", "review", "empty", "title-only", "unknown-language", "published"} {
		seedPost(t, s, slug, "alice", testNow)
	}
	for _, statement := range []string{
		`UPDATE posts SET status='finalized' WHERE slug='draft'`,
		`UPDATE posts SET status='review',content='{"blocks":[]}',content_language='ko' WHERE slug='empty'`,
		`UPDATE posts SET content='{"title":"Legacy heading"}',content_language='ko' WHERE slug='title-only'`,
		`UPDATE posts SET content='{"blocks":[{"type":"TEXT","content":"Real"}]}',content_language='ko' WHERE slug IN ('review','published')`,
		`UPDATE posts SET content='{"blocks":[{"type":"TEXT","content":"Legacy"}]}' WHERE slug='unknown-language'`,
		`UPDATE posts SET status='published',published_url='https://blog.naver.com/alice/1',published_at='2026-03-01T12:00:00Z' WHERE slug='published'`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.ListPosts(context.Background(), "alice", post.ListFilter{Limit: -1})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		wantContent := row.Slug == "review" || row.Slug == "unknown-language" || row.Slug == "published"
		wantExport := row.Slug == "review" || row.Slug == "published"
		if row.ContentReady != wantContent || row.ExportReady != wantExport || row.InputRevision != 1 {
			t.Fatalf("readiness for %s = %+v", row.Slug, row)
		}
		if row.Slug == "published" && row.PublishedURL != "https://blog.naver.com/alice/1" {
			t.Fatal("lost owner-entered address")
		}
	}
}

func TestTemplateSeedsAndObservedTurnsHaveRevisionNoops(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	if _, err := handle.Writer.Exec("INSERT INTO templates(id,user_id,name,body,created_at,updated_at) VALUES('template','alice','Template','','2026-03-01T12:00:00Z','2026-03-01T12:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	assertRevision := func(want int64) {
		t.Helper()
		p, err := s.GetPost(ctx, "p")
		if err != nil || p.InputRevision != want {
			t.Fatalf("revision = %d, want %d: %v", p.InputRevision, want, err)
		}
	}
	templateID, length, tags := "template", 1700, 5
	seeds := post.TemplateNumbers{TargetLength: &length, TagCount: &tags}
	if _, err := s.AssignTemplate(ctx, "p", "alice", &templateID, seeds, testNow); err != nil {
		t.Fatal(err)
	}
	assertRevision(2)
	if _, err := s.AssignTemplate(ctx, "p", "alice", &templateID, seeds, testNow); err != nil {
		t.Fatal(err)
	}
	assertRevision(2)
	if _, err := s.AssignTemplate(ctx, "p", "alice", nil, post.TemplateNumbers{}, testNow); err != nil {
		t.Fatal(err)
	}
	assertRevision(3)
	if _, err := s.AssignTemplate(ctx, "p", "alice", nil, post.TemplateNumbers{}, testNow); err != nil {
		t.Fatal(err)
	}
	assertRevision(3)
	image := post.Image{ID: "photo", PostSlug: "p", Filename: "photo.jpg", Key: "photo", Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}
	if err := s.CreateImage(ctx, image); err != nil {
		t.Fatal(err)
	} // seed helper intentionally bypasses confirmation
	observations := []post.Observation{{File: image.Filename, Rotation: 90}}
	if _, err := s.UpdateObservations(ctx, "p", "alice", observations, testNow); err != nil {
		t.Fatal(err)
	}
	assertRevision(4)
	if _, err := s.UpdateObservations(ctx, "p", "alice", observations, testNow); err != nil {
		t.Fatal(err)
	}
	assertRevision(4)
}

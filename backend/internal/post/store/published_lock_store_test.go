package store_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/post/store/sqlc"
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
	if updated, err := s.UpdateGeneratedContent(ctx, "p", "alice", content, post.LanguageKorean, post.WriteAnnotations{}, testNow); err != nil || !updated {
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

var (
	lockLater   = testNow.Add(time.Hour)
	lockContent = post.PostContent{Title: "고친 글", Blocks: []post.Block{{Type: post.BlockText, Content: "고친 문장"}}}
	lockLength  = 1200
)

// predicateGuarded holds every write statement whose own WHERE carries the published predicate,
// keyed by its sqlc name, with a call that runs it against the published fixture. On a
// published post each matches no row.
var predicateGuarded = map[string]func(*store.Store) (bool, error){
	"UpdatePostDraft": func(s *store.Store) (bool, error) {
		return s.UpdateDraft(context.Background(), "p", "alice", "새 제목", "새 메모", nil, lockLater)
	},
	"UpdatePostObservations": func(s *store.Store) (bool, error) {
		// A turn rides the entry, so the refusal also proves the photo kept its rotation (GEN-79).
		return s.UpdateObservations(context.Background(), "p", "alice", []post.Observation{{File: "IMG_1.jpg", Scene: "바다", Rotation: 90}}, lockLater)
	},
	"UpdatePostStoryline": func(s *store.Store) (bool, error) {
		return s.UpdateStoryline(context.Background(), "p", "alice", &post.Storyline{Paragraphs: []post.StorylineParagraph{{Text: "가게 앞"}}}, lockLater)
	},
	"UpdatePostAttachmentTraces": func(s *store.Store) (bool, error) {
		return s.UpdateAttachmentTraces(context.Background(), "p", "alice", "a.jpg", "old-photo", lockLater)
	},
	"UpdateGeneratedContent": func(s *store.Store) (bool, error) {
		return s.UpdateGeneratedContent(context.Background(), "p", "alice", lockContent, post.LanguageKorean, post.WriteAnnotations{}, lockLater)
	},
	"SavePostContent": func(s *store.Store) (bool, error) {
		return s.SaveContent(context.Background(), "p", "alice", lockContent, 1, lockLater)
	},
	"SavePostGenerationOptions": func(s *store.Store) (bool, error) {
		return s.SaveGenerationOptions(context.Background(), "p", "alice", post.GenerationOptionsSet{TargetLength: &lockLength, TagCount: 5, UseMemory: true, Field: "cafe"}, lockLater)
	},
	"FinalizePost": func(s *store.Store) (bool, error) {
		return s.Finalize(context.Background(), "p", "alice", "제주 3일", 1, lockLater)
	},
	"ReassignPostVoice": func(s *store.Store) (bool, error) {
		return s.ReassignVoice(context.Background(), "p", "alice", voiceIDFor("alice", 1), lockLater)
	},
	"AssignPostTemplate": func(s *store.Store) (bool, error) {
		return s.AssignTemplate(context.Background(), "p", "alice", nil, post.TemplateNumbers{}, lockLater)
	},
	// SaveDraft still applies a 분야 an older tab sends (POST-89).
	"AssignPostField": func(s *store.Store) (bool, error) {
		field := "cafe"
		return s.AssignField(context.Background(), "p", "alice", &field, lockLater)
	},
	"DeleteImage": func(s *store.Store) (bool, error) { return s.DeleteImage(context.Background(), "i1") },
	"SetImageRotation": func(s *store.Store) (bool, error) {
		return s.SetImageRotation(context.Background(), "i1", 90)
	},
	"DeleteVideo": func(s *store.Store) (bool, error) { return s.DeleteVideo(context.Background(), "v1") },
}

// transactionGuarded holds every insert that has no row to predicate on, keyed by its sqlc name,
// with the store method that runs it: each checks PostIsPublished first in its own write
// transaction, and refuses a published post with ErrPostPublished.
var transactionGuarded = map[string]func(*store.Store) error{
	"DeleteOtherPostStageCaptures": writePublishedRequestCapture,
	"WritePostRequestCapture":      writePublishedRequestCapture,
	"DeletePostStageCaptures":      finishPublishedRequestCapture,
	"BindPostRequestCapture":       finishPublishedRequestCapture,
	"UpsertPostTemplateAnswer": func(s *store.Store) error {
		return s.UpsertTemplateAnswers(context.Background(), "p", []post.TemplateAnswer{{Label: "총평", Text: "흐렸다", Enabled: true}}, lockLater)
	},
	"CreateUpload": func(s *store.Store) error {
		return s.CreateUpload(context.Background(), post.Upload{
			ID: "u-new", PostSlug: "p", Filename: "IMG_3.jpg", Key: post.ObjectKey("p", "u-new"), Kind: post.AttachmentPhoto,
			ContentType: "image/jpeg", ExpiresAt: lockLater, CreatedAt: lockLater,
		})
	},
	"CreateImage": func(s *store.Store) error {
		return s.ConfirmUpload(context.Background(), post.Image{
			ID: "u-photo", PostSlug: "p", Filename: "IMG_2.jpg", Key: post.ObjectKey("p", "u-photo"), Width: 1, Height: 1, Bytes: 1, CreatedAt: lockLater,
		}, "u-photo")
	},
	"CreateVideo": func(s *store.Store) error {
		clip := testVideo("u-clip", "clip2.mp4")
		return s.ConfirmVideoUpload(context.Background(), clip, "u-clip")
	},
}

func publishedRequestCaptureRun(s *store.Store) (post.RequestCaptureRun, error) {
	ctx := context.Background()
	found, err := s.GetPost(ctx, "p")
	if err != nil {
		return post.RequestCaptureRun{}, err
	}
	found.Images, err = s.ListImages(ctx, "p")
	if err != nil {
		return post.RequestCaptureRun{}, err
	}
	found.Videos, err = s.ListVideos(ctx, "p")
	if err != nil {
		return post.RequestCaptureRun{}, err
	}
	found.TemplateAnswers, err = s.ListTemplateAnswers(ctx, "p")
	if err != nil {
		return post.RequestCaptureRun{}, err
	}
	return post.RequestCaptureRun{JobID: "capture-job", UserID: "alice", PostSlug: "p", InputRevision: found.InputRevision, ContentRevision: found.ContentRevision, SourceFingerprint: post.RequestCaptureSourceFingerprint(found), PlanFingerprint: post.StorylineFingerprint(found.Storyline)}, nil
}

func writePublishedRequestCapture(s *store.Store) error {
	run, err := publishedRequestCaptureRun(s)
	if err != nil {
		return err
	}
	return s.WritePostRequestCapture(context.Background(), run, post.RequestCaptureCall{ID: "call"}, requestCaptureFixture())
}

func finishPublishedRequestCapture(s *store.Store) error {
	run, err := publishedRequestCaptureRun(s)
	if err != nil {
		return err
	}
	return s.FinishPostRequestCapture(context.Background(), run, &post.RequestCaptureCompletion{UnavailableStages: []string{"post-writing"}, UnavailableReason: "capture_persistence_failed"})
}

// publishedLockExemptStatements are the writes the lock deliberately lets through, with why.
var publishedLockExemptStatements = map[string]string{
	"PurgePostRequestCaptures":          "privacy erasure removes private request payload even when canonical published content is locked",
	"FencePostRequestCapturePurge":      "private purge tombstone prevents restoring erased payload; it changes no canonical content or input",
	"PurgeWithdrawnPostRequestCaptures": "privacy erasure within guarded attachment deletion; preserves the published canonical result",
	"CreatePostTestPublication":         "durable receipt only, inserted after the owned publication guard in the same transaction",
	"CreatePost":                        "the create: no post exists to be locked",
	"PublishPost":                       "records or replaces the address (POST-73, POST-75)",
	"UnpublishPost":                     "clears the address",
	"DeletePost":                        "POST-74 lets a published post be deleted",
	"DeleteUpload":                      "a pending upload is the sweep's ledger, not the post: confirm runs it inside its guarded transaction, and the sweep and a retry run it by design",
	"SetObservedImageRotation":          "runs only inside UpdateObservations' transaction, after UpdatePostObservations' published predicate matched a row (GEN-79)",
}

// Internal revision and publication statements are checked directly because neither
// belongs to the ordinary draft store's public API.
var publicationPredicateGuarded = map[string]func(*sqlc.Queries, post.Post) (int64, error){
	"PublishPostOriginContent": func(q *sqlc.Queries, p post.Post) (int64, error) {
		return q.PublishPostOriginContent(context.Background(), sqlc.PublishPostOriginContentParams{Slug: p.Slug, UserID: p.UserID, ExpectedContentRevision: p.ContentRevision, Content: sql.NullString{String: "{}", Valid: true}, UpdatedAt: lockLater.Format(time.RFC3339Nano)})
	},
	"PublishPostOriginStoryline": func(q *sqlc.Queries, p post.Post) (int64, error) {
		return q.PublishPostOriginStoryline(context.Background(), sqlc.PublishPostOriginStorylineParams{Slug: p.Slug, UserID: p.UserID, ExpectedContentRevision: p.ContentRevision, ExpectedInputRevision: p.InputRevision, Storyline: sql.NullString{String: "{}", Valid: true}, UpdatedAt: lockLater.Format(time.RFC3339Nano)})
	},
	"SetPostOriginAvailability": func(q *sqlc.Queries, p post.Post) (int64, error) {
		return q.SetPostOriginAvailability(context.Background(), sqlc.SetPostOriginAvailabilityParams{Slug: p.Slug, UserID: p.UserID, ContentOrigins: sql.NullString{String: "{}", Valid: true}})
	},
	"AdvancePostInputRevision": func(q *sqlc.Queries, p post.Post) (int64, error) {
		return q.AdvancePostInputRevision(context.Background(), sqlc.AdvancePostInputRevisionParams{Slug: p.Slug, UpdatedAt: lockLater.Format(time.RFC3339Nano)})
	},
	"ApplyPostTestOutput": func(q *sqlc.Queries, p post.Post) (int64, error) {
		return q.ApplyPostTestOutput(context.Background(), sqlc.ApplyPostTestOutputParams{Slug: p.Slug, UserID: p.UserID,
			ExpectedInputRevision: p.InputRevision, ExpectedContentRevision: p.ContentRevision,
			Content:         sql.NullString{String: `{"blocks":[{"type":"TEXT","content":"new"}]}`, Valid: true},
			MachineBaseline: sql.NullString{String: `{"blocks":[{"type":"TEXT","content":"new"}]}`, Valid: true},
			ContentLanguage: sql.NullString{String: "ko", Valid: true}, UpdatedAt: lockLater.Format(time.RFC3339Nano)})
	},
}

func TestPublicationBookkeepingStatementsRefusePublishedPosts(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	if _, err := handle.Writer.Exec("UPDATE posts SET status='published' WHERE slug='p'"); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetPost(context.Background(), "p")
	for name, mutate := range publicationPredicateGuarded {
		if n, err := mutate(sqlc.New(handle.Writer), before); err != nil || n != 0 {
			t.Fatalf("%s matched %d: %v", name, n, err)
		}
	}
	after, _ := s.GetPost(context.Background(), "p")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("published bookkeeping changed the post")
	}
}

// namedStatement is one `-- name:` statement from the queries directory.
type namedStatement struct {
	name, kind, body string
}

// readStatements parses every `-- name:` statement in queries/*.sql: the name line opens a
// statement, other comment lines are dropped, and the rest is its body.
func readStatements(t *testing.T) []namedStatement {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("queries: %v (%d files)", err, len(files))
	}
	var statements []namedStatement
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "-- name:") {
				fields := strings.Fields(trimmed)
				if len(fields) < 4 {
					t.Fatalf("%s: malformed name line %q", file, trimmed)
				}
				statements = append(statements, namedStatement{name: fields[2], kind: fields[3]})
				continue
			}
			if strings.HasPrefix(trimmed, "--") || len(statements) == 0 {
				continue
			}
			last := &statements[len(statements)-1]
			last.body += line + "\n"
		}
	}
	return statements
}

var (
	writeWords         = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|REPLACE)\b`)
	publishedPredicate = regexp.MustCompile(`(?i)status\s*<>\s*'published'`)
)

func (st namedStatement) isWrite() bool {
	return strings.HasPrefix(st.kind, ":exec") || writeWords.MatchString(st.body)
}

// POST-74, default-deny: a write statement nobody classified fails here, so a new one cannot
// reach a published post by being forgotten.
func TestEveryPostWriteStatementIsClassifiedForThePublishedLock(t *testing.T) {
	seen := map[string]bool{}
	for _, st := range readStatements(t) {
		seen[st.name] = true
		if !st.isWrite() {
			continue
		}
		_, predicate := predicateGuarded[st.name]
		_, publication := publicationPredicateGuarded[st.name]
		predicate = predicate || publication
		_, transaction := transactionGuarded[st.name]
		_, exempt := publishedLockExemptStatements[st.name]
		switch n := boolCount(predicate, transaction, exempt); {
		case n == 0:
			t.Errorf("%s is a write statement the published lock does not classify: add it to predicateGuarded (with status <> 'published') or transactionGuarded with a published-post case, or to publishedLockExemptStatements with its reason (POST-74)", st.name)
		case n > 1:
			t.Errorf("%s is classified more than once", st.name)
		}
		if predicate && !publishedPredicate.MatchString(st.body) {
			t.Errorf("%s is predicate-guarded but its body carries no status <> 'published':\n%s", st.name, st.body)
		}
	}
	for _, table := range []map[string]bool{keys(predicateGuarded), keys(publicationPredicateGuarded), keys(transactionGuarded), keys(publishedLockExemptStatements)} {
		for name := range table {
			if !seen[name] {
				t.Errorf("%s is classified for the published lock but no longer exists", name)
			}
		}
	}
}

func boolCount(values ...bool) int {
	n := 0
	for _, v := range values {
		if v {
			n++
		}
	}
	return n
}

func keys[V any](table map[string]V) map[string]bool {
	out := make(map[string]bool, len(table))
	for name := range table {
		out[name] = true
	}
	return out
}

// F15: the delete finds the one post by its key rather than listing every unpublished one.
// Only SQLite's SEARCH/SCAN word is pinned: an index name or the plan's nesting is not stable.
func TestPhotoAndVideoDeletesSearchPostsByKey(t *testing.T) {
	_, handle := newStoreWithHandle(t)
	bodies := map[string]string{}
	for _, st := range readStatements(t) {
		if st.name == "DeleteImage" || st.name == "DeleteVideo" {
			bodies[st.name] = st.body
		}
	}
	if len(bodies) != 2 {
		t.Fatalf("found %d of the two deletes", len(bodies))
	}
	for name, body := range bodies {
		rows, err := handle.Writer.Query("EXPLAIN QUERY PLAN "+body, "x")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var details []string
		for rows.Next() {
			var id, parent, notUsed int
			var detail string
			if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
				t.Fatal(err)
			}
			details = append(details, detail)
		}
		rows.Close()
		searched := false
		for _, detail := range details {
			if strings.HasPrefix(detail, "SCAN posts") {
				t.Errorf("%s scans posts: %q", name, details)
			}
			searched = searched || strings.HasPrefix(detail, "SEARCH posts")
		}
		if !searched {
			t.Errorf("%s never searches posts by key: %q", name, details)
		}
	}
}

// POST-74: every statement that writes to a published post, or under one, refuses it — an
// update matches no row, a delete deletes nothing, an insert is refused by its guard — so a
// write that passed the service's check and lost the race to a publish changes nothing. The
// tables are the ones the default-deny test checks, so a classified statement has a case.
func TestEveryGuardedStatementRefusesAPublishedPost(t *testing.T) {
	for name, update := range predicateGuarded {
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

	for name, insert := range transactionGuarded {
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
		if cleared, err := s.UnpublishPost(context.Background(), "p", "alice", lockLater); err != nil || !cleared {
			t.Fatalf("clear: %v, %v", cleared, err)
		}
		if changed, err := s.UpdateDraft(context.Background(), "p", "alice", "새 제목", "", nil, lockLater); err != nil || !changed {
			t.Fatalf("draft after the clear: %v, %v", changed, err)
		}
		if deleted, err := s.DeleteImage(context.Background(), "i1"); err != nil || !deleted {
			t.Fatalf("photo delete after the clear: %v, %v", deleted, err)
		}
		if err := s.ConfirmUpload(context.Background(), post.Image{
			ID: "u-photo", PostSlug: "p", Filename: "IMG_2.jpg", Key: post.ObjectKey("p", "u-photo"), Width: 1, Height: 1, Bytes: 1, CreatedAt: lockLater,
		}, "u-photo"); err != nil {
			t.Fatalf("confirm after the clear: %v", err)
		}
	})
}

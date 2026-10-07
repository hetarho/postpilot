package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	postrpc "github.com/postpilot/backend/internal/post/rpc"
	poststore "github.com/postpilot/backend/internal/post/store"
)

func originStorageHarness(t *testing.T, models *recordingModels) *drainHarness {
	t.Helper()
	return newDrainHarness(t, models, func(service *generation.Service, _ *post.Service, handle *db.DB) *generation.Service {
		publisher := generationOriginPublisher{service: post.NewOriginResultService(poststore.New(handle.Writer, handle.Reader))}
		return generation.NewOriginService(service, publisher, publisher)
	})
}

func TestOriginStorageActualPlanAllowsItsOwnUprightObservation(t *testing.T) {
	models := &recordingModels{
		observeAnswer: `{"observations":[{"file":"turn.jpg","scene":"컵","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":90}],"origins":[{"file":"turn.jpg","field":{"kind":"observation_scene"},"quote":"컵","category":"photo_interpretation","source_refs":["media.0"]}]}`,
		answer:        `{"storyline":[{"text":"컵을 봅니다.","files":["turn.jpg"]}],"origins":[{"field":{"kind":"storyline_paragraph","paragraph_index":0},"quote":"컵","category":"photo_interpretation","source_refs":["current.visual.0.0"]}]}`,
	}
	h := originStorageHarness(t, models)
	draft := h.draft(t, "")
	if _, err := h.handle.Writer.Exec(`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('turn-photo',?,'turn.jpg','private/turn.jpg',120,180,4,?)`, draft.Slug, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	before, err := h.posts.Get(h.ctx, "alice", draft.Slug)
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.generation.StartStoryline(h.ctx, generation.StartStorylineRequest{UserID: "alice", PostSlug: draft.Slug, ObserveModel: "p/observer", WriteModel: "p/writer"})
	if err != nil {
		t.Fatal(err)
	}
	h.waitDone(id)
	stored, err := h.posts.Get(h.ctx, "alice", draft.Slug)
	if err != nil || stored.Storyline == nil || stored.Storyline.Origins == nil || len(stored.Storyline.Origins.Spans) != 1 || stored.Images[0].Rotation != 90 || stored.InputRevision <= before.InputRevision {
		t.Fatalf("own observer progress invalidated admitted plan: %+v %v", stored, err)
	}
	if len(stored.Observations) != 1 || stored.Observations[0].Origins == nil || stored.Observations[0].Origins.Sources[0].AttachmentID != "turn-photo" {
		t.Fatal("actual media incarnation/observation was not retained")
	}
	models.mu.Lock()
	calls := len(models.requests)
	models.mu.Unlock()
	if calls != 2 || len(h.admitter.holds) != 1 || len(h.admitter.holds[0].Calls) != 2 {
		t.Fatal("rotation fence added calls or a second hold")
	}
}

func TestOriginStorageActualQueuePublishesCurrentOwnerReviewAndCleanBaseline(t *testing.T) {
	core := `{"storyline":[{"text":"방문을 보여줍니다.","files":[]}],"title":"노포","summary":"직접 알려준 방문","tags":[],"blocks":[{"type":"TEXT","content":"노포🙂 식감 제안"}],"nouns":[]`
	for _, test := range []struct {
		name, tail, finish string
		spans              int
	}{
		{"mixed", `,"origins":[{"field":{"kind":"block_content","block_index":0},"quote":"노포","category":"owner_input","source_refs":["current.memo"]},{"field":{"kind":"block_content","block_index":0},"quote":"🙂 식감 제안","category":"ai_added","source_refs":[]}]}`, "", 2},
		{"missing", `}`, "", 0},
		{"malformed", `,"origins":"invalid"}`, "", 0},
		{"tail only", `,"origins":[{"quote":`, "length", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			models := &recordingModels{answer: core + test.tail, finishReason: test.finish}
			h := originStorageHarness(t, models)
			draft := h.draft(t, "")
			id, err := h.generation.Start(h.ctx, generation.StartRequest{UserID: "alice", PostSlug: draft.Slug, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}.String()})
			if err != nil {
				t.Fatal(err)
			}
			h.waitDone(id)
			stored, err := h.posts.Get(h.ctx, "alice", draft.Slug)
			if err != nil || stored.Content == nil || stored.ContentOrigins == nil || len(stored.ContentOrigins.Spans) != test.spans || stored.ContentRevision != 1 || stored.MachineBaselineRevision != 1 {
				t.Fatalf("atomic canonical/origin/baseline publication lost: %+v %v", stored, err)
			}
			identity := post.ContentOriginIdentity(*stored.Content, stored.ContentRevision)
			if stored.ContentOrigins.Result != identity {
				t.Fatalf("origins describe another stored result: %+v %+v", stored.ContentOrigins.Result, identity)
			}
			var canonical, baseline string
			if err := h.handle.Reader.QueryRow("SELECT content, machine_baseline FROM posts WHERE slug = ?", draft.Slug).Scan(&canonical, &baseline); err != nil || canonical != baseline || strings.Contains(canonical, `"origins"`) || strings.Contains(canonical, "current.memo") {
				t.Fatal("presentation metadata entered canonical/baseline", err)
			}
			handler := postrpc.NewHandler(h.posts)
			read, err := handler.GetPost(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.GetPostRequest{Slug: draft.Slug}))
			if err != nil || read.Msg.Post.ContentHash != identity.ContentHash || read.Msg.Post.ContentOrigins == nil || read.Msg.Post.ContentOrigins.Result.ContentRevision != 1 || len(read.Msg.Post.ContentOrigins.Spans) != test.spans {
				t.Fatalf("owner read lost aligned result: %+v %v", read, err)
			}
			if _, err := handler.GetPost(auth.WithUser(context.Background(), "bob"), connect.NewRequest(&postpilotv1.GetPostRequest{Slug: draft.Slug})); connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("another owner read private origins: %v", err)
			}
			models.mu.Lock()
			calls := len(models.requests)
			models.mu.Unlock()
			if calls != 1 || len(h.admitter.holds) != 1 {
				t.Fatal("storage added provider/admission work")
			}
			if test.spans == 0 {
				return
			}
			content := proto.Clone(read.Msg.Post.Content).(*postpilotv1.PostContent)
			content.Blocks[0].Content = "새 문장입니다. 🙂 식감 제안"
			saved, err := handler.SavePostContent(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.SavePostContentRequest{Slug: draft.Slug, Content: content, ExpectedRevision: 1}))
			if err != nil || saved.Msg.Post.ContentRevision != 2 || saved.Msg.Post.MachineBaselineRevision != 1 || saved.Msg.Post.ContentOrigins == nil {
				t.Fatalf("manual save lost aligned review/baseline: %+v %v", saved, err)
			}
			var ai bool
			for _, span := range saved.Msg.Post.ContentOrigins.Spans {
				if span.Quote == "🙂 식감 제안" {
					ai = span.Category == postpilotv1.SemanticOriginCategory_SEMANTIC_ORIGIN_CATEGORY_AI_ADDED
				}
			}
			if !ai {
				t.Fatal("manual replacement promoted/lost unchanged AI meaning")
			}
			noop, err := handler.SavePostContent(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.SavePostContentRequest{Slug: draft.Slug, Content: content, ExpectedRevision: 2}))
			if err != nil || noop.Msg.Post.ContentRevision != 2 || noop.Msg.Post.ContentHash != saved.Msg.Post.ContentHash {
				t.Fatalf("identical save changed result identity: %+v %v", noop, err)
			}
			if err := h.posts.DeletePost(h.ctx, "alice", draft.Slug); err != nil {
				t.Fatal(err)
			}
			publisher := generationOriginPublisher{service: post.NewOriginResultService(poststore.New(h.handle.Writer, h.handle.Reader))}
			_, err = publisher.PublishGeneratedResult(h.ctx, "alice", draft.Slug, generation.OriginPostCompletion{Content: generation.PostContent{Title: "late", Blocks: []generation.Block{{Type: generation.BlockText, Content: "late"}}}, Language: generation.LanguageKorean, ExpectedContentRevision: 0})
			if !errors.Is(err, generation.ErrNotFound) {
				t.Fatalf("late callback restored purged result: %v", err)
			}
		})
	}
}

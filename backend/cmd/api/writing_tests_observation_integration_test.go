package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/storage"
)

func writingPhotoBucket(t *testing.T) *storage.Bucket {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xd9})
	}))
	t.Cleanup(server.Close)
	bucket, err := storage.New(t.Context(), storage.Config{Endpoint: server.URL, AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "photos", MaxReadBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	return bucket
}

func TestWritingTestsObserverPipelinesAndSharedObservationUseActualOwnedAttachments(t *testing.T) {
	for _, stage := range []v1.WritingTestStage{v1.WritingTestStage_WRITING_TEST_STAGE_OBSERVE, v1.WritingTestStage_WRITING_TEST_STAGE_WRITE} {
		t.Run(stage.String(), func(t *testing.T) {
			h := newWritingIntegrationHarness(t, func(p *platform) { p.bucket = writingPhotoBucket(t) })
			language := post.LanguageEnglish
			voice := ""
			source, err := h.app.post.SaveDraft(t.Context(), "alice", post.DraftSave{Title: "Owned material", Memo: "Source memo must remain unchanged", VoiceID: &voice, TargetLanguage: &language})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.platform.db.Writer.Exec(`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('owned-photo',?,'owned.jpg','owned/photo.jpg',100,100,4,?)`, source.Slug, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				t.Fatal(err)
			}
			plan := h.plan(2)
			plan.ModelStage = stage
			plan.Context.SourcePostSlug = source.Slug
			plan.Context.ExpectedInputRevision = source.InputRevision
			plan.Context.ExpectedContentRevision = source.ContentRevision
			plan.Context.Material.AttachmentIds = []string{"owned-photo"}
			plan.Context.WriteModel = &v1.ModelRef{ProviderId: "fixture", ModelId: "writer-0"}
			plan.Context.ObserveModel = &v1.ModelRef{ProviderId: "fixture", ModelId: "writer-0"}
			quote, err := h.client.EstimateWritingTest(t.Context(), writingRPCRequest(h, &v1.EstimateWritingTestRequest{Plan: plan}))
			if err != nil {
				t.Fatal(err)
			}
			started, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h, &v1.StartWritingTestRequest{Plan: plan, RequestKey: "observe-start", QuoteKey: quote.Msg.QuoteKey}))
			if err != nil {
				t.Fatal(err)
			}
			current := started.Msg.Test
			for deadline := time.Now().Add(30 * time.Second); current.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW && time.Now().Before(deadline); {
				time.Sleep(10 * time.Millisecond)
				current = h.get(t, current.Id)
				if current.Status == v1.WritingTestStatus_WRITING_TEST_STATUS_FAILED || current.Status == v1.WritingTestStatus_WRITING_TEST_STATUS_PARTIAL {
					t.Fatalf("pipeline failed: %+v", current.Failure)
				}
			}
			if current.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW {
				t.Fatal("all-success barrier not reached")
			}
			h.provider.mu.Lock()
			requests := append([]llm.Request(nil), h.provider.requests...)
			h.provider.mu.Unlock()
			observe, writes := 0, 0
			for _, request := range requests {
				switch request.Stage {
				case "observe":
					observe++
					if !request.HasImages() {
						t.Fatal("observation lost owned image")
					}
				case "write":
					writes++
					if stage == v1.WritingTestStage_WRITING_TEST_STAGE_OBSERVE && request.Model != "writer-0" {
						t.Fatal("observer test changed fixed writer")
					}
				}
			}
			wantObserve := 1
			if stage == v1.WritingTestStage_WRITING_TEST_STAGE_OBSERVE {
				wantObserve = 2
			}
			if observe != wantObserve || writes != 2 {
				t.Fatalf("observe=%d writes=%d", observe, writes)
			}
			var observations, content string
			if err := h.platform.db.Reader.QueryRow(`SELECT COALESCE(observations,''),COALESCE(content,'') FROM posts WHERE slug=?`, source.Slug).Scan(&observations, &content); err != nil {
				t.Fatal(err)
			}
			if observations != "" && observations != "[]" || content != "" {
				t.Fatalf("test mutated canonical source observations/content")
			}
		})
	}
}

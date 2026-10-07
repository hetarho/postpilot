package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/usage"
)

// This consumer records the producer's result contract in the integration test.
// Durable origin storage and its atomic publisher are implemented by T637.
type originResultConsumer struct {
	posts   *post.Service
	mu      sync.Mutex
	results []generation.OriginPostCompletion
}

func (p *originResultConsumer) PublishGeneratedResult(ctx context.Context, user, slug string, result generation.OriginPostCompletion) (post.OriginResultIdentity, error) {
	if err := (generationPosts{service: p.posts}).SetGeneratedContent(ctx, user, slug, result.Content, result.Language, result.Annotations); err != nil {
		return post.OriginResultIdentity{}, err
	}
	p.mu.Lock()
	p.results = append(p.results, result)
	p.mu.Unlock()
	return generation.OriginContentIdentity(result.Content), nil
}
func (p *originResultConsumer) PublishStorylineResult(ctx context.Context, user, slug string, result generation.OriginStorylineCompletion) error {
	return (generationPosts{service: p.posts}).SetStoryline(ctx, user, slug, result.Storyline)
}

func TestOriginGenerationPublishesValidCanonicalOutputAndIndependentSidecarWithOneAdmittedCall(t *testing.T) {
	core := `{"storyline":[{"text":"제공한 방문을 보여줍니다.","files":[]}],"title":"노포","summary":"직접 알려준 방문","tags":[],"blocks":[{"type":"TEXT","content":"노포🙂 식감 제안"}],"nouns":["노포"]`
	valid := core + `,"origins":[{"field":{"kind":"block_content","block_index":0},"quote":"노포","category":"owner_input","source_refs":["current.memo"]},{"field":{"kind":"block_content","block_index":0},"quote":"🙂 식감 제안","category":"ai_added","source_refs":[]}]} `
	for _, tc := range []struct {
		name, answer, finish string
		spans                int
	}{
		{"valid mixed phrase", valid, "", 2},
		{"legacy absent sidecar", core + `}`, "", 0},
		{"origin tail exhausted only", core + `,"origins":[{"field":`, "length", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			models := &recordingModels{answer: tc.answer, finishReason: tc.finish}
			var consumer *originResultConsumer
			h := newDrainHarness(t, models, func(service *generation.Service, posts *post.Service, _ *db.DB) *generation.Service {
				consumer = &originResultConsumer{posts: posts}
				return generation.NewOriginService(service, consumer, consumer)
			})
			source := h.draft(t, "")
			id, err := h.generation.Start(h.ctx, generation.StartRequest{UserID: "alice", PostSlug: source.Slug, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}.String()})
			if err != nil {
				t.Fatal(err)
			}
			h.waitDone(id)
			consumer.mu.Lock()
			if len(consumer.results) != 1 {
				consumer.mu.Unlock()
				t.Fatal("canonical result was not published exactly once")
			}
			result := consumer.results[0]
			consumer.mu.Unlock()
			if result.Origins == nil || len(result.Origins.Spans) != tc.spans || result.Content.Blocks[0].Content != "노포🙂 식감 제안" {
				t.Fatalf("valid canonical output discarded or sidecar guessed: %+v", result)
			}
			if tc.spans == 2 && (result.Origins.Spans[0].Start != 0 || result.Origins.Spans[0].End != 2 || result.Origins.Spans[1].Start != 2 || result.Origins.Spans[1].End != 9) {
				t.Fatalf("emoji quote offsets are not Unicode scalars: %+v", result.Origins.Spans)
			}
			stored, err := h.posts.Get(h.ctx, "alice", source.Slug)
			if err != nil || stored.Content == nil || stored.Content.Blocks[0].Content != result.Content.Blocks[0].Content || len(stored.Content.Tags) != 0 {
				t.Fatal("canonical publication or sparse tags failed", err)
			}
			models.mu.Lock()
			calls := len(models.requests)
			sent := models.requests[0]
			models.mu.Unlock()
			if calls != 1 || len(h.admitter.holds) != 1 || h.admitter.holds[0].Calls[0].Count != 1 || h.admitter.holds[0].Calls[0].CompletionTokens != sent.MaxTokens || h.admitter.holds[0].Calls[0].PromptTokens != int(usage.HoldPromptTokenBound(0))+generation.OriginPromptTokenOverhead {
				t.Fatal("origin production changed call count or dispatched an unreserved cap")
			}
			var supplied strings.Builder
			for _, message := range sent.Messages {
				for _, part := range message.Parts {
					supplied.WriteString(part.Text)
				}
			}
			if !strings.Contains(supplied.String(), "current.memo") || !strings.Contains(sent.System, "origins") {
				t.Fatal("catalog/origin output contract did not reach the actual writer")
			}
		})
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

func t674Answer(t *testing.T, tags []string, body string) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{"title": "을지로 기록", "summary": "직접 알려준 경험", "tags": tags, "blocks": []map[string]string{{"type": "TEXT", "content": body}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
func t674SetMinimum(t *testing.T, h *drainHarness, slug string, minimum int) post.Post {
	t.Helper()
	current, err := h.posts.Get(h.ctx, "alice", slug)
	if err != nil {
		t.Fatal(err)
	}
	options := current.GenerationOptions()
	options.TagCount = minimum
	changed, err := h.posts.SaveGenerationOptions(h.ctx, "alice", slug, options)
	if err != nil {
		t.Fatal(err)
	}
	return changed
}
func t674AssertOneCallAndHold(t *testing.T, h *drainHarness, models *recordingModels, kind string) {
	t.Helper()
	models.mu.Lock()
	calls := len(models.requests)
	models.mu.Unlock()
	if calls != 1 {
		t.Fatalf("provider calls=%d, want one without retries", calls)
	}
	if len(h.admitter.holds) != 1 {
		t.Fatalf("holds=%d, want one", len(h.admitter.holds))
	}
	var rows int
	if err := h.handle.Reader.QueryRowContext(h.ctx, `SELECT count(*) FROM generation_jobs WHERE user_id='alice' AND kind=?`, kind).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("durable jobs=%d %v", rows, err)
	}
}

// GEN-46: tag_count is a minimum the prompt asks for. The stored post keeps every returned
// tag up to post.TagOutputMax, a shorter list never fails or retries the paid write, and the
// server pads nothing.
func TestT674OrdinaryGenerationAsksForTheMinimumAndKeepsEveryReturnedTag(t *testing.T) {
	long := make([]string, post.TagOutputMax+2)
	for index := range long {
		long[index] = fmt.Sprintf("태그%d", index)
	}
	cases := []struct {
		name           string
		minimum        int
		returned, want []string
	}{
		{"fewer than four accepted", 4, []string{"을지로", "노포"}, []string{"을지로", "노포"}},
		{"zero below ten accepted", 10, []string{}, []string{}},
		{"exactly the minimum", 1, []string{"을지로"}, []string{"을지로"}},
		{"past the minimum kept", 1, []string{"을지로", "노포", "식당"}, []string{"을지로", "노포", "식당"}},
		{"cut at the output max", 10, long, long[:post.TagOutputMax]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			models := &recordingModels{answer: t674Answer(t, tc.returned, "노포에서 직접 경험한 일을 기록했습니다.")}
			h := newDrainHarness(t, models)
			source := h.draft(t, "")
			t674SetMinimum(t, h, source.Slug, tc.minimum)
			id, err := h.generation.Start(h.ctx, generation.StartRequest{UserID: "alice", PostSlug: source.Slug, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}.String()})
			if err != nil {
				t.Fatal(err)
			}
			h.waitDone(id)
			result, err := h.posts.Get(h.ctx, "alice", source.Slug)
			if err != nil || result.Content == nil {
				t.Fatalf("canonical output=%+v %v", result, err)
			}
			if !slices.Equal(result.Content.Tags, tc.want) {
				t.Fatalf("tags=%q, want %q", result.Content.Tags, tc.want)
			}
			if result.TagCount != tc.minimum || result.Status != post.StatusReview {
				t.Fatal("generation changed the saved option or readiness")
			}
			models.mu.Lock()
			system := models.requests[0].System
			models.mu.Unlock()
			if !strings.Contains(system, fmt.Sprintf("%d개 이상의 tags", tc.minimum)) {
				t.Fatalf("write prompt does not ask for the minimum %d", tc.minimum)
			}
			t674AssertOneCallAndHold(t, h, models, "generate")
		})
	}
}
func TestT674RevisionPreservesUnrequestedTagsAndKeepsEveryRequestedTag(t *testing.T) {
	oldTags := []string{" #을지로 ", "노포", "기록\u00a0중간", "最後"}
	cases := []struct {
		name, instruction string
		returned, want    []string
		body              string
	}{
		{"unrelated prose keeps exact four", "첫 문장만 차분하게 바꾸고 제목·요약·태그는 그대로 두세요.", oldTags, oldTags, "첫 문장을 차분하게 바꿨습니다."},
		{"requested tag replacement keeps all three", "태그를 재료로 확인되는 새 태그로 바꿔 주세요.", []string{"새장소", "새기록", "새경험"}, []string{"새장소", "새기록", "새경험"}, "첫 문장입니다."},
		{"requested tag removal accepts zero", "태그는 모두 지우고 본문을 그대로 두세요.", []string{}, []string{}, "첫 문장입니다."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			models := &recordingModels{answer: t674Answer(t, tc.returned, tc.body)}
			h := newDrainHarness(t, models)
			source := h.draft(t, "")
			original := post.PostContent{Title: "을지로 기록", Summary: "직접 알려준 경험", Tags: oldTags, Blocks: []post.Block{{Type: post.BlockText, Content: "첫 문장입니다."}}}
			if err := h.posts.SetGeneratedContent(h.ctx, "alice", source.Slug, original, post.LanguageKorean, nil); err != nil {
				t.Fatal(err)
			}
			before := t674SetMinimum(t, h, source.Slug, 1)
			if !reflect.DeepEqual(before.Content.Tags, oldTags) {
				t.Fatal("option save rewrote current tags")
			}
			id, err := h.generation.StartRevision(h.ctx, generation.StartRevisionRequest{UserID: "alice", PostSlug: source.Slug, Instruction: tc.instruction, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}.String()})
			if err != nil {
				t.Fatal(err)
			}
			h.waitDone(id)
			result, err := h.posts.Get(h.ctx, "alice", source.Slug)
			if err != nil || result.Content == nil {
				t.Fatalf("revision canonical=%+v %v", result, err)
			}
			if !slices.Equal(result.Content.Tags, tc.want) {
				t.Fatalf("returned tags=%q, want byte-exact %q", result.Content.Tags, tc.want)
			}
			if result.Content.Blocks[0].Content != tc.body || result.Content.Title != original.Title || result.Content.Summary != original.Summary || result.ContentRevision != before.ContentRevision+1 || result.TagCount != 1 {
				t.Fatal("replacement/minimum/revision mismatch")
			}
			var payload map[string]json.RawMessage
			var raw []byte
			if err = h.handle.Reader.QueryRowContext(h.ctx, `SELECT payload FROM generation_jobs WHERE id=?`, id).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, &payload); err != nil || string(payload["tag_count"]) != "1" {
				t.Fatalf("frozen minimum=%s %v", raw, err)
			}
			t674AssertOneCallAndHold(t, h, models, "revise")
		})
	}
}

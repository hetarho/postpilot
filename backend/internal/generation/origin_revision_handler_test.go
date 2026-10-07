package generation

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

func originRevisionRequestText(request llm.Request) string {
	var text strings.Builder
	text.WriteString(request.System)
	for _, message := range request.Messages {
		for _, part := range message.Parts {
			text.WriteString(part.Text)
		}
	}
	return text.String()
}

func originRevisionQueuedJob(jobs *fakeJobs) RevisionJob {
	start := jobs.revisions[0]
	return RevisionJob{UserID: start.UserID, PostSlug: start.PostSlug, VoiceID: start.VoiceID, WriteModel: start.WriteModel, Payload: jobs.payloads[0]}
}

func TestOriginReviseActualFrozenCapKeepsAITailAndExcludesOldMemory(t *testing.T) {
	for _, structured := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "structured"}[structured], func(t *testing.T) {
			language := LanguageKorean
			oldPrefix, newPrefix := "오래된 첫 문장입니다. ", "새 사실입니다. "
			tail := "달콤한 향🙂은 AI가 보탠 제안입니다." + strings.Repeat(" 변경하지 않는 문장입니다.", 160)
			content := PostContent{Title: "제목", Summary: "요약", Tags: []string{" kept ", "second"}, Blocks: []Block{{Type: BlockText, Content: oldPrefix + tail}}}
			field := post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}
			privateMemory := "PRIVATE OLD MEMORY MUST NEVER ENTER THE REVISION REQUEST"
			prior := ResolveWriteOriginCandidates(content, []post.OriginSource{{ID: "old-memory", Kind: post.OriginSourceMemory, Text: privateMemory, Available: true}}, []post.OriginCandidate{originCandidate(field, "달콤한 향🙂", post.OriginAIAdded, "old-memory")}).Review
			identity := prior.Result
			identity.ContentRevision = 9
			prior.Result = identity
			prior.Spans[0].ReviewState = post.OriginConfirmed
			target := 2200
			posts := &fakePosts{input: PostInput{UserID: "alice", Slug: "post", Content: &content, ContentLanguage: &language, TargetLanguage: language, ContentRevision: 9,
				ContentOrigins: &prior, ContentOriginIdentity: &identity, TargetLength: &target, TagCount: 1, UseMemory: true, Memories: []string{"UNRELATED CURRENT MEMORY"}}}
			jobs, models, publisher := &fakeJobs{id: "revision"}, newFakeModels(), &originFixturePublisher{}
			info := models.infos[writeRef]
			info.StructuredOutput, info.ReasoningNativeEffort = structured, true
			models.infos[writeRef] = info
			guidelines := &fakeGuidelines{stock: []StockGuideline{{Key: "memory", Text: "STOCK MEMORY RULE MUST NOT ENTER REVISION", Applicability: []StockRuleApplicability{{Stage: "write", Outputs: []string{"prose"}}, {Stage: "storyline", Outputs: []string{"plan"}}}}, {Key: "revision", Text: "ADMITTED REVISION RULE", Applicability: []StockRuleApplicability{{Stage: "revise", Outputs: []string{"prose"}}}}}}
			memories := &recordingMemories{texts: []string{"RETRIEVED MEMORY MUST NOT ENTER REVISION"}}
			deps := testDeps()
			deps.Guidelines, deps.Memories = guidelines, memories
			service := NewOriginService(NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps), publisher, publisher)
			if _, err := service.StartRevision(context.Background(), StartRevisionRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Instruction: "첫 문장만 새 사실입니다.로 바꾸고 나머지는 그대로 두세요."}); err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 0 || len(jobs.revisions) != 1 || memories.calls != 0 || guidelines.askedMemories {
				t.Fatal("admission executed work or retrieved revision memories")
			}
			frozen, err := parseRevisionPayload(jobs.payloads[0])
			if err != nil {
				t.Fatal(err)
			}
			expectedCap := testBudget.Revise(contentChars(&content)+OriginCompletionExtraChars, &target, true)
			if frozen.OriginProtocolVersion != OriginProtocolVersion || frozen.CompletionTokens != expectedCap || jobs.revisions[0].CompletionTokens != expectedCap {
				t.Fatal("new-protocol demand was not frozen before enqueue")
			}
			if expectedCap == testBudget.Revise(contentChars(&content), originInt(10000), true) {
				t.Fatal("fixture cannot detect recomputed live cap")
			}
			posts.input.TargetLength = originInt(10000)
			guidelines.stock[1].Text = "LATER MUTATED RULE"
			after := content
			after.Blocks = []Block{{Type: BlockText, Content: newPrefix + tail}}
			models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
				whole := originRevisionRequestText(request)
				for _, forbidden := range []string{privateMemory, "UNRELATED CURRENT MEMORY", "RETRIEVED MEMORY", "STOCK MEMORY RULE", "LATER MUTATED RULE", "[기억]"} {
					if strings.Contains(whole, forbidden) {
						t.Fatalf("revision widened frozen material: %q", forbidden)
					}
				}
				if !strings.Contains(whole, "ADMITTED REVISION RULE") || !strings.Contains(whole, "달콤한 향🙂") || request.MaxTokens != expectedCap || structured != (request.JSONSchema != nil) {
					t.Fatal("frozen request conditions or prior phrase missing")
				}
				if structured && !bytes.Equal(request.JSONSchema, PostContentSchema()) {
					t.Fatal("new protocol did not select its actual schema")
				}
				core := strings.TrimSuffix(marshalPromptJSON(contentForPrompt(after)), "}")
				return llm.Response{Text: core + `,"origins":[{"field":{"kind":"block_content","block_index":0},"quote":"새 사실입니다.","category":"owner_input","source_refs":["current.edit"]},{"field":{"kind":"block_content","block_index":0},"quote":"달콤한 향🙂","category":"owner_input","source_refs":["current.edit"]}]}`, Usage: llm.Usage{PromptTokens: 27, CompletionTokens: 29}}, nil
			}
			if err := service.Revise(context.Background(), originRevisionQueuedJob(jobs), func(string, int, int) {}); err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 1 || len(publisher.results) != 1 || len(posts.contents) != 0 || memories.calls != 0 {
				t.Fatal("revision added another call, legacy write or memory retrieval")
			}
			result := publisher.results[0]
			if result.ExpectedContentRevision != 9 || result.Language != language || result.Content.Title != content.Title || result.Content.Summary != content.Summary || !slices.Equal(result.Content.Tags, content.Tags) || result.Content.Blocks[0].Content != after.Blocks[0].Content || len(result.Origins.Spans) != 2 {
				t.Fatalf("canonical result/CAS or meaningful annotations lost: %+v", result)
			}
			var retained *post.OriginSpan
			for i := range result.Origins.Spans {
				if result.Origins.Spans[i].Quote == "달콤한 향🙂" {
					retained = &result.Origins.Spans[i]
				}
			}
			if retained == nil || retained.Category != post.OriginAIAdded || retained.ReviewState != post.OriginConfirmed || retained.Start != utf8.RuneCountInString(newPrefix) || retained.End-retained.Start != utf8.RuneCountInString(retained.Quote) || !slices.Equal(retained.SourceRefs, []string{"old-memory"}) {
				t.Fatalf("unchanged AI tail recolored or offset/state/evidence lost: %+v", retained)
			}
		})
	}
}

func TestOriginReviseActualMissingMalformedSidecarStillPublishesCanonicalResult(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, mode := range []string{"missing", "malformed", "truncated-tail"} {
			t.Run(map[bool]string{false: "plain", true: "structured"}[structured]+"/"+mode, func(t *testing.T) {
				language := LanguageEnglish
				content := PostContent{Title: "Title", Summary: "Summary", Tags: []string{}, Blocks: []Block{{Type: BlockText, Content: "old canonical"}}}
				posts := &fakePosts{input: PostInput{UserID: "alice", Slug: "post", Content: &content, ContentLanguage: &language, TargetLanguage: language, ContentRevision: 4}}
				jobs, models, publisher := &fakeJobs{id: "revision"}, newFakeModels(), &originFixturePublisher{}
				info := models.infos[writeRef]
				info.StructuredOutput = structured
				models.infos[writeRef] = info
				service := NewOriginService(NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps()), publisher, publisher)
				if _, err := service.StartRevision(context.Background(), StartRevisionRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Instruction: "Replace only the body with new canonical."}); err != nil {
					t.Fatal(err)
				}
				models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
					core := `{"title":"Title","summary":"Summary","tags":[],"blocks":[{"type":"TEXT","content":"new canonical"}]}`
					response := llm.Response{Text: core, Usage: llm.Usage{PromptTokens: 3, CompletionTokens: 5}}
					if mode == "malformed" {
						response.Text = strings.TrimSuffix(core, "}") + `,"origins":"malformed"}`
					}
					if mode == "truncated-tail" {
						response.Text = strings.TrimSuffix(core, "}") + `,"origins":[{"field":`
						response.FinishReason = "length"
					}
					if request.MaxTokens != jobs.revisions[0].CompletionTokens || structured != (request.JSONSchema != nil) {
						t.Fatal("drain changed admitted request")
					}
					return response, nil
				}
				if err := service.Revise(context.Background(), originRevisionQueuedJob(jobs), func(string, int, int) {}); err != nil {
					t.Fatal(err)
				}
				if len(models.calls) != 1 || len(publisher.results) != 1 || len(posts.contents) != 0 || publisher.results[0].ExpectedContentRevision != 4 || publisher.results[0].Content.Blocks[0].Content != "new canonical" || len(publisher.results[0].Origins.Spans) != 0 {
					t.Fatal("origin-only failure discarded, guessed or replayed canonical result")
				}
			})
		}
	}
}

func TestOriginReviseActualFreshAttachmentDeletionWithdrawsEvidence(t *testing.T) {
	language := LanguageEnglish
	content := PostContent{Title: "Title", Summary: "Summary", Blocks: []Block{{Type: BlockText, Content: "Blue wall."}, {Type: BlockImage, File: "one.jpg", Alt: "Blue wall", Caption: "Wall"}}}
	sources := []post.OriginSource{{ID: "photo", Kind: post.OriginSourceVisualObservation, Text: "Blue wall", AttachmentFilename: "one.jpg", Available: true}}
	prior := ResolveWriteOriginCandidates(content, sources, []post.OriginCandidate{originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "Blue wall", post.OriginPhotoInterpretation, "photo")}).Review
	identity := prior.Result
	identity.ContentRevision = 8
	prior.Result = identity
	posts := &fakePosts{input: PostInput{UserID: "alice", Slug: "post", Content: &content, ContentLanguage: &language, TargetLanguage: language, ContentRevision: 8, ContentOrigins: &prior, ContentOriginIdentity: &identity, Images: []Image{{Filename: "one.jpg"}}}}
	jobs, models, publisher := &fakeJobs{id: "revision"}, newFakeModels(), &originFixturePublisher{}
	service := NewOriginService(NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps()), publisher, publisher)
	if _, err := service.StartRevision(context.Background(), StartRevisionRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Instruction: "Rephrase only the prose without changing photo placement."}); err != nil {
		t.Fatal(err)
	}
	models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
		posts.input.Images = nil
		return llm.Response{Text: `{"title":"Title","summary":"Summary","tags":[],"blocks":[{"type":"TEXT","content":"A blue-painted wall."},{"type":"IMAGE","file":"one.jpg","alt":"Blue wall","caption":"Wall"}],"origins":[{"field":{"kind":"block_content","block_index":0},"quote":"A blue-painted wall","category":"photo_interpretation","source_refs":["prior.content.0"]}]}`}, nil
	}
	if err := service.Revise(context.Background(), originRevisionQueuedJob(jobs), func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || len(publisher.results) != 1 || len(posts.contents) != 0 {
		t.Fatal("withdrawal added work or used legacy publication")
	}
	result := publisher.results[0]
	if result.ExpectedContentRevision != 8 || len(result.Content.Blocks) != 1 || result.Content.Blocks[0].Content != "A blue-painted wall." || len(result.Origins.Spans) != 0 {
		t.Fatalf("withdrawn photo evidence survived or usable prose was lost: %+v", result)
	}
	for _, source := range result.Origins.Sources {
		if source.Kind == post.OriginSourceVisualObservation && source.AttachmentFilename == "one.jpg" && source.Available {
			t.Fatal("withdrawn result source stayed available")
		}
	}
}

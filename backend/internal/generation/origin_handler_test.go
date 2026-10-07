package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

type originFixturePublisher struct {
	results []OriginPostCompletion
	plans   []OriginStorylineCompletion
}

func TestOriginServicePublishesMixedMeaningThroughOneFrozenCall(t *testing.T) {
	for _, structured := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "declared-structured"}[structured], func(t *testing.T) {
			posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", TargetLanguage: LanguageEnglish, Memo: "It tasted good", ContentRevision: 7}}
			jobs, models := &fakeJobs{id: "job"}, newFakeModels()
			info := models.infos[writeRef]
			info.StructuredOutput = structured
			models.infos[writeRef] = info
			models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
				if !strings.Contains(request.System, "origin") || strings.Contains(request.System, "[글 예시 발췌]") {
					t.Fatal("new contract or no-voice boundary missing")
				}
				if structured != (request.JSONSchema != nil) {
					t.Fatal("structured capability changed")
				}
				if structured && !bytes.Equal(request.JSONSchema, WriteAnswerSchema()) {
					t.Fatal("new schema not selected")
				}
				return llm.Response{Text: `{"storyline":[{"text":"A proposed arrangement","files":[]}],"title":"Title","summary":"Summary","tags":[],"blocks":[{"type":"TEXT","content":"It tasted good, with a fragrant note 🍰."}],"nouns":[],"origins":[{"field":{"kind":"block_content","block_index":0},"quote":"It tasted good","category":"owner_input","source_refs":["current.memo"]},{"field":{"kind":"block_content","block_index":0},"quote":"fragrant note 🍰","category":"ai_added","source_refs":[]}]}`, Usage: llm.Usage{PromptTokens: 11, CompletionTokens: 13}}, nil
			}
			pub := &originFixturePublisher{}
			service := NewOriginService(NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps()), pub, pub)
			target := 1600
			if _, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), TargetLength: &target}); err != nil {
				t.Fatal(err)
			}
			frozen := jobs.frozen(t, 0)
			if frozen.OriginProtocolVersion != OriginProtocolVersion || frozen.CompletionTokens != testBudget.Write(OriginBudgetTarget(&target), false) {
				t.Fatal("metadata demand was not admitted", frozen)
			}
			if err := service.Generate(context.Background(), jobs.queued(0), func(string, int, int) {}); err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 1 || len(pub.results) != 1 || len(posts.contents) != 0 {
				t.Fatal("origin publication added/fell back to work", len(models.calls), len(pub.results), len(posts.contents))
			}
			if models.calls[0].request.MaxTokens != frozen.CompletionTokens {
				t.Fatal("execution changed admitted cap")
			}
			result := pub.results[0]
			if result.ExpectedContentRevision != 7 || result.Origins == nil || len(result.Origins.Spans) != 2 {
				t.Fatal("canonical/review CAS result lost", result)
			}
			for _, span := range result.Origins.Spans {
				if span.End-span.Start != utf8.RuneCountInString(span.Quote) {
					t.Fatal("span used UTF8 byte offsets", span)
				}
			}
			inspection := assertCompositionPrompt(t, models.calls[0].request)
			if inspection.PromptVersion != "post-origin-contracts-v1" || inspection.Output.Version != schemaInspection("WriteAnswer", WriteAnswerSchema()).Version {
				t.Fatal("false captured schema/prompt version", inspection)
			}
		})
	}
}

func TestOriginSourceCatalogRejectsUnprovedPlanContextsAndNeverInjectsMemoryTextIntoRevision(t *testing.T) {
	paragraphs := []StorylineParagraph{{Text: "Known meaning"}}
	valid := originFixturePlan(paragraphs)
	if len(catalogWithPriorPlan(nil, paragraphs, valid, nil, nil)) != 1 {
		t.Fatal("valid prior meaning lost")
	}
	for _, mutate := range []func(*PlanOriginReview){func(r *PlanOriginReview) { r.Result.ContentRevision = 9 }, func(r *PlanOriginReview) { r.Spans[0].End++ }, func(r *PlanOriginReview) { r.Sources[0].Available = false }, func(r *PlanOriginReview) {
		r.Spans = append(r.Spans, r.Spans[0])
		r.Spans[1].Category = post.OriginAIAdded
	}} {
		broken := clonePlanOrigins(valid)
		mutate(broken)
		if got := catalogWithPriorPlan(nil, paragraphs, broken, nil, nil); len(got) != 0 {
			t.Fatal("unproved prior restored as factual source", got)
		}
	}
	content := PostContent{Title: "Current phrase"}
	prior := originFixtureContent(content)
	prior.Sources[0].Kind, prior.Sources[0].Text = post.OriginSourceMemory, "PRIVATE OLD MEMORY TEXT WHICH IS NOT CURRENT PROSE"
	sources := catalogWithPriorContent(nil, content, prior, nil, nil, nil)
	projection := priorOriginProjection(validatedContentOriginContext(content, prior, nil, nil, nil))
	raw, _ := json.Marshal(struct {
		Sources []post.OriginSource
		Prior   any
	}{sources, projection})
	if strings.Contains(string(raw), prior.Sources[0].Text) || !strings.Contains(string(raw), content.Title) {
		t.Fatal("prose revision widened its supplied memory material", string(raw))
	}
	stale := cloneOriginReview(prior)
	stale.Result.ContentRevision = 3
	if validatedContentOriginContext(content, stale, nil, nil, nil) != nil || len(catalogWithPriorContent(nil, content, stale, nil, nil, nil)) != 0 {
		t.Fatal("missing current identity restored stale evidence")
	}
}

func TestLegacyPaidWritingTestCheckpointReplaysWithoutNewSchemaBudgetOrCall(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 2, nil)
	common, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	common.PromptVersion, common.SchemaVersion, common.Post.OriginProtocolVersion = legacyWritingTestPromptVersion, legacyWritingTestSchemaVersion(), 0
	for i := range variants {
		variants[i].Snapshot.Post.OriginProtocolVersion = 0
		variants[i].Snapshot.Post.OriginCompletionTokens = 0
	}
	snapshot = encodeRunTestSnapshot(t, common, variants)
	stored, save := checkpointStore()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if !bytes.Equal(request.JSONSchema, LegacyWriteAnswerSchema()) || strings.Contains(request.System, originResponseContract) {
			t.Fatal("old paid work got new schema/contract")
		}
		inspection := assertCompositionPrompt(t, request)
		if inspection.Output.Version != schemaInspection("WriteAnswer", LegacyWriteAnswerSchema()).Version || inspection.PromptVersion != postPromptCompositionVersion {
			t.Fatal("legacy history claimed new schema", inspection)
		}
		return runTestAnswer(), nil
	}
	first, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: save})
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || first.Checkpoint.Answer == nil {
		t.Fatal("paid result missing")
	}
	checkpoint := stored[0]
	plan, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0})
	if err != nil || len(plan) != 0 {
		t.Fatal("completed paid checkpoint needs another hold", plan, err)
	}
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		t.Fatal("completed paid checkpoint replayed provider")
		return llm.Response{}, nil
	}
	second, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &checkpoint, SaveCheckpoint: save, RetryFailed: true})
	if err != nil || !second.Replayed || len(models.calls) != 1 || !reflect.DeepEqual(first.Answer, second.Answer) {
		t.Fatal("old paid answer was not reused exactly", second, err)
	}
	common.SchemaVersion = "unknown"
	bad := encodeRunTestSnapshot(t, common, variants)
	if _, err := factory.RunWritingTestCandidate(context.Background(), bad, 0, nil, WritingTestRunOptions{SaveCheckpoint: save}); err == nil || len(models.calls) != 1 {
		t.Fatal("unknown schema admitted")
	}
}

func TestOriginObservePlanRewriteUseOnlyTheirExistingBoundedCalls(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			language := LanguageEnglish
			posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", TargetLanguage: language, ContentLanguage: &language, Memo: "It tasted good", Images: []Image{{Filename: "one.jpg", Key: "synthetic-private-key"}}, ContentRevision: 7}}
			jobs, models, pub := &fakeJobs{id: "job"}, newFakeModels(), &originFixturePublisher{}
			service := NewOriginService(NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps()), pub, pub)
			withOrigins := func(core, origins string) string {
				switch mode {
				case "valid":
					return strings.TrimSuffix(core, "}") + `,"origins":` + origins + `}`
				case "malformed":
					return strings.TrimSuffix(core, "}") + `,"origins":[{"field":`
				default:
					return core
				}
			}
			models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
				assertCompositionPrompt(t, request)
				if strings.Contains(request.System, "synthetic-private-key") {
					t.Fatal("media storage key leaked")
				}
				if request.Stage == llm.StageNameObserve {
					if !bytes.Equal(request.JSONSchema, ObservationsSchema()) || !strings.Contains(request.System, "observation_scene") {
						t.Fatal("new observation contract missing")
					}
					core := `{"observations":[{"file":"one.jpg","scene":"Blue wall","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":0}]}`
					return llm.Response{Text: withOrigins(core, `[{"file":"one.jpg","field":{"kind":"observation_scene"},"quote":"Blue wall","category":"photo_interpretation","source_refs":["media.0"]}]`)}, nil
				}
				if !bytes.Equal(request.JSONSchema, StorylineAnswerSchema()) || strings.Contains(request.System, "field.kind is title") {
					t.Fatal("planning consumes unused final-post contract")
				}
				core := `{"storyline":[{"text":"It tasted good. Blue wall","files":["one.jpg"]}]}`
				return llm.Response{Text: withOrigins(core, `[{"field":{"kind":"storyline_paragraph","paragraph_index":0},"quote":"It tasted good","category":"owner_input","source_refs":["current.memo"]},{"field":{"kind":"storyline_paragraph","paragraph_index":0},"quote":"Blue wall","category":"photo_interpretation","source_refs":["current.visual.0.0"]}]`)}, nil
			}
			if _, err := service.StartStoryline(context.Background(), StartStorylineRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String()}); err != nil {
				t.Fatal(err)
			}
			if err := service.WriteStoryline(context.Background(), StorylineJob{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(), Payload: jobs.storylinePayloads[0]}, func(string, int, int) {}); err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 2 || len(pub.plans) != 1 || len(posts.storylines) != 0 {
				t.Fatal("plan added/fell back to a call or publication")
			}
			if models.calls[1].request.MaxTokens != jobs.storylineStarts[0].CompletionTokens {
				t.Fatal("plan cap changed after admission")
			}
			if pub.plans[0].ExpectedContentRevision != 7 {
				t.Fatal("plan source CAS lost")
			}
			plan := pub.plans[0].Storyline
			if mode == "valid" && (plan.Origins == nil || len(plan.Origins.Spans) != 2) {
				t.Fatal("actual observed/owner plan origins lost", plan)
			}
			posts.input.Storyline = &plan
			posts.input.Observations = posts.observationWrites[len(posts.observationWrites)-1]
			if _, err := service.StartStorylineRevision(context.Background(), StartStorylineRevisionRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Request: "Keep the current proposed arrangement"}); err != nil {
				t.Fatal(err)
			}
			if err := service.ReviseStoryline(context.Background(), StorylineRevisionJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: jobs.storylineRequestPayloads[0]}, func(string, int, int) {}); err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 3 || len(pub.plans) != 2 || models.calls[2].request.MaxTokens != jobs.storylineRequests[0].CompletionTokens {
				t.Fatal("rewrite added observation or changed admitted cap")
			}
			if mode == "valid" && !reflect.DeepEqual(pub.plans[1].Storyline.Origins.Spans, plan.Origins.Spans) {
				t.Fatal("approval changed existing plan origins")
			}
		})
	}
}

func (p *originFixturePublisher) PublishGeneratedResult(_ context.Context, _, _ string, result OriginPostCompletion) (post.OriginResultIdentity, error) {
	p.results = append(p.results, result)
	return OriginContentIdentity(result.Content), nil
}
func (p *originFixturePublisher) PublishStorylineResult(_ context.Context, _, _ string, result OriginStorylineCompletion) error {
	p.plans = append(p.plans, result)
	return nil
}

func originFixturePlan(paragraphs []StorylineParagraph) *PlanOriginReview {
	if len(paragraphs) == 0 {
		return nil
	}
	result := ValidatePlanOrigins(paragraphs, []post.OriginSource{{ID: "memo", Kind: post.OriginSourceMemo, Text: paragraphs[0].Text, Available: true}}, []PlanOriginCandidate{{ParagraphIndex: 0, Quote: paragraphs[0].Text, Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}}})
	return &result
}

func originFixtureObservation(observation Observation) *ObservationOriginReview {
	observation.OriginCandidates = []ObservationOriginCandidate{{Field: "scene", Quote: observation.Scene, Category: post.OriginPhotoInterpretation, SourceRefs: []string{"media.0"}}}
	return ValidateObservationOrigins(observation, observationSources([]string{observation.File}, false, map[string]string{observation.File: "fixture-attachment"}))
}

func originFixtureContent(content PostContent) *post.OriginReview {
	result := ResolveWriteOriginCandidates(content, []post.OriginSource{{ID: "memo", Kind: post.OriginSourceMemo, Text: content.Title, Available: true}}, []post.OriginCandidate{{Field: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, Quote: content.Title, Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}}}).Review
	return &result
}

func TestOriginServiceRequiresBothResultPublishers(t *testing.T) {
	for _, test := range []struct {
		service *Service
		posts   OriginPostPublisher
		plans   OriginStorylinePublisher
	}{{nil, &originFixturePublisher{}, &originFixturePublisher{}}, {&Service{}, nil, &originFixturePublisher{}}, {&Service{}, &originFixturePublisher{}, nil}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("required origin publisher was optional")
				}
			}()
			NewOriginService(test.service, test.posts, test.plans)
		}()
	}
}

package generation

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// These are fixed synthetic responses, not evidence of live source accuracy.
// The fixture deliberately orders interior3, food3, interior1. Arrangement has
// no supplied movement or event chronology; all seven attachments stay intact.
func TestOriginQualificationSevenPhotoArrangementWithoutInventedMovement(t *testing.T) {
	for _, protocol := range []int{0, OriginProtocolVersion} {
		for _, language := range []Language{LanguageKorean, LanguageEnglish} {
			t.Run(fmt.Sprintf("protocol-%d/%s", protocol, language), func(t *testing.T) {
				input := PostInput{Slug: "synthetic-seven", UserID: "synthetic-owner", TargetLanguage: language, ContentLanguage: &language, TagCount: 4, OriginProtocolVersion: protocol, OriginCompletionTokens: 8192,
					Memo: "진아분식에서 맛있게 먹었어요. 이동이나 사건 순서는 제공하지 않았어요.", StockGuidelines: []StockGuideline{}}
				var observations []Observation
				var names []string
				for i, subject := range []string{"interior", "interior", "interior", "food", "food", "food", "interior"} {
					filename := fmt.Sprintf("%02d-%s.jpg", i+1, subject)
					names = append(names, filename)
					input.Images = append(input.Images, Image{ID: fmt.Sprintf("synthetic-photo-%d", i), Filename: filename, Key: fmt.Sprintf("synthetic-key-%d", i), Kind: AttachmentPhoto, Width: 1200, Height: 800})
					observations = append(observations, Observation{File: filename, Scene: map[string]string{"interior": "벽과 테이블이 보임", "food": "접시에 떡볶이가 보임"}[subject], Objects: []string{}})
				}
				posts, models := &fakePosts{input: input}, newFakeModels()
				service := NewService(posts, fakeProfiles{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
				models.complete = func(ref llm.ModelRef, request llm.Request) (llm.Response, error) {
					if ref != observeRef || !strings.Contains(request.System, "사진 업로드·저장 배열·파일명·촬영 순서로") {
						t.Fatal("observation lost its unconditional chronology boundary")
					}
					var files []string
					for _, part := range request.Messages[0].Parts {
						if strings.HasPrefix(part.Text, "files: ") {
							files = strings.Split(strings.TrimPrefix(part.Text, "files: "), ", ")
						}
					}
					var rows []Observation
					for _, file := range files {
						for _, observation := range observations {
							if observation.File == file {
								rows = append(rows, observation)
							}
						}
					}
					return llm.Response{Text: marshalPromptJSON(map[string]any{"observations": observationsForPrompt(rows)})}, nil
				}
				actual, err := service.observe(context.Background(), input, input.Images, nil, observeRef, func(string, int, int) {})
				if err != nil || len(models.calls) != 2 || len(actual) != 7 {
					t.Fatalf("seven-photo bounded observation failed: %d calls, %d observations, %v", len(models.calls), len(actual), err)
				}
				for i, observation := range actual {
					if observation.File != names[i] || observation.Scene != observations[i].Scene || len(observation.Events) != 0 {
						t.Fatal("filename order generated movement or lost a scene", observation)
					}
				}
				plan := []StorylineParagraph{{Text: "실내 사진 세 장을 소개합니다.", Files: names[:3]}, {Text: "음식 사진 세 장과 제공한 감상을 소개합니다.", Files: names[3:6]}, {Text: "다른 실내 사진의 모습을 보여줍니다.", Files: names[6:]}}
				planInput := StorylinePromptInput{Language: language, Memo: input.Memo, Photos: names, Observations: actual, StockGuidelines: []StockGuideline{}}
				planRequest, _ := preparePlanRequest(planInput, protocol, 4096, originAttachmentIDs(input.Images), nil)
				parsedPlan, _, err := ParseStorylineAnswerWithOrigins(marshalPromptJSON(storylineForPrompt(plan)), names)
				if err != nil || !reflect.DeepEqual(parsedPlan, plan) {
					t.Fatal("actual plan parser changed the subject-only arrangement", err)
				}
				contract, revisionContract := koreanSourceHonestyContract, koreanRevisionHonestyContract
				if language == LanguageEnglish {
					contract, revisionContract = englishSourceHonestyContract, englishRevisionHonestyContract
				}
				if !strings.Contains(planRequest.System, contract) {
					t.Fatal("plan lost the unconditional evidence/chronology contract")
				}
				// Run the actual writer seam twice, once with no tags and once with
				// one supported tag, well below the frozen maximum of four.
				for _, tags := range [][]string{{}, {"진아분식"}} {
					text := "실내 사진에는 벽과 테이블이 보여요. 음식 사진에는 떡볶이가 보여요. 맛있게 먹었어요."
					if language == LanguageEnglish {
						text = "The interior photos show walls and tables. The food photos show tteokbokki. I enjoyed it."
					}
					content := PostContent{Title: "진아분식 기록", Summary: "사진과 제공한 감상", Tags: tags, Blocks: []Block{{Type: BlockText, Content: text}}}
					for _, name := range names {
						content.Blocks = append(content.Blocks, Block{Type: BlockImage, File: name, Alt: "사진에 보이는 모습", Caption: "사진에 보이는 모습"})
					}
					models.complete = func(ref llm.ModelRef, request llm.Request) (llm.Response, error) {
						if ref != writeRef || !strings.Contains(request.System, contract) || strings.Contains(request.System, "[작문 지침]") {
							t.Fatal("writer lost unconditional evidence rules or reenabled preferences")
						}
						wire := evaluationContentWire(content)
						wire["storyline"], wire["nouns"] = storylineForPrompt(plan)["storyline"], []string{}
						return llm.Response{Text: marshalPromptJSON(wire)}, nil
					}
					before := len(models.calls)
					answer, _, err := service.writeCandidate(context.Background(), input, Profile{NoVoice: true}, actual, writeRef)
					if err != nil || len(models.calls) != before+1 || !slices.Equal(answer.Content.Tags, tags) || len(answer.Content.Blocks) != 8 || answer.Storyline == nil || !reflect.DeepEqual(answer.Storyline.Paragraphs, plan) {
						t.Fatal("writer lost zero/sparse tags, attachment coverage or bounded calls", err)
					}
					if answer.Content.Blocks[0].Content != text {
						t.Fatal("fixed subject-only prose changed")
					}
					// A newer cap of one must not touch unrequested tag bytes.
					current := answer.Content
					current.Tags = []string{" 진아분식 ", " 음식 사진 ", "실내"}
					input.Content = &current
					payload := revisionPayloadJSON{ContentLanguage: language, Instruction: "제목만 진아분식 사진 기록으로 바꿔 주세요.", TagCount: 1, OriginProtocolVersion: protocol, CompletionTokens: 8192}
					revision, _ := service.prepareRevisionRequest(input, Profile{NoVoice: true}, payload, writeRef)
					if !strings.Contains(revision.System, revisionContract) {
						t.Fatal("revision lost unconditional chronology/evidence contract")
					}
					expected := current
					expected.Title = "진아분식 사진 기록"
					got, _, err := ParseRevisionContentWithOrigins(marshalPromptJSON(evaluationContentWire(expected)), 1, current)
					if err != nil || !reflect.DeepEqual(evaluationContentWire(*got), evaluationContentWire(expected)) {
						t.Fatal("requested title-only revision changed unrelated canonical tag/prose/attachment bytes", err)
					}
				}
			})
		}
	}
}

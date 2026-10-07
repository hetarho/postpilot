package generation

import (
	"fmt"
	"strings"
	"testing"
)

func TestPostStageContractsKeepSuppliedMeaningVisualInterpretationAndFurtherProposalsDistinct(t *testing.T) {
	for _, language := range []Language{LanguageKorean, LanguageEnglish} {
		photos := []string{"interior-first.jpg", "food.jpg", "interior-last.jpg"}
		observations := []Observation{{File: photos[0], Scene: "interior"}, {File: photos[1], Scene: "food"}, {File: photos[2], Scene: "interior"}}
		direct, _ := BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: Profile{NoVoice: true}, Memo: "Owner: 맛있었다; this is not a supplied event sequence.", Photos: photos, Observations: observations, TagCount: 4, StockGuidelines: []StockGuideline{}})
		frozen, _ := BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: Profile{NoVoice: true}, Photos: photos, Observations: observations, TagCount: 4, StockGuidelines: []StockGuideline{}, FollowStoryline: []StorylineParagraph{{Text: "Proposed interior-food-interior arrangement", Files: photos}}})
		create, _ := BuildStorylinePromptForLanguage(StorylinePromptInput{Language: language, Photos: photos, Observations: observations, StockGuidelines: []StockGuideline{}})
		rewrite, _ := BuildStorylinePromptForLanguage(StorylinePromptInput{Language: language, Photos: photos, StockGuidelines: []StockGuideline{}, Current: []StorylineParagraph{{Text: "AI plan claims an action", Files: photos}}, Request: "Keep the arrangement; I am not supplying a new event."})
		revise, _ := BuildRevisePromptForLanguage(language, Profile{NoVoice: true}, goldenContent(), photos, "Rewrite the full structure using the new fact: 12500 KRW.", nil, 2, nil, FrozenGuidelines{Stock: []StockGuideline{}})
		contract, revisionContract := koreanSourceHonestyContract, koreanRevisionHonestyContract
		if language == LanguageEnglish {
			contract, revisionContract = englishSourceHonestyContract, englishRevisionHonestyContract
		}
		for name, system := range map[string]string{"direct": direct, "frozen": frozen, "plan": create, "plan-rewrite": rewrite} {
			if !strings.Contains(system, contract) || strings.Contains(system, "[작문 지침]") {
				t.Fatal("mandatory interpretation or disabled optional behavior drifted", name, system)
			}
		}
		if !strings.Contains(revise, revisionContract) {
			t.Fatal("revision lost supplied edit facts or original-evidence boundary")
		}
		if !strings.Contains(direct, `"storyline":[`) || strings.Contains(frozen, `"storyline":[`) {
			t.Fatal("direct/frozen plan result fields do not match consumers")
		}
		for _, system := range []string{create, rewrite} {
			if strings.Contains(system, `"title":`) || strings.Contains(system, `"tags":`) || strings.Contains(system, `"blocks":`) {
				t.Fatal("plan-only stage requested final result fields")
			}
		}
	}
	for name, prompt := range map[string]string{"photo": ObservePrompt, "video": ObserveVideoPrompt} {
		for _, required := range []string{"시각", "사용자가 준", "파일명", "시간", "사진", "행동"} {
			if !strings.Contains(prompt, required) {
				t.Fatal("observation evidence contract absent", name, required)
			}
		}
		if strings.Contains(prompt, "[작문 지침]") || strings.Contains(prompt, "[말투]") {
			t.Fatal("context-free observation gained writing inputs")
		}
	}
	if !strings.Contains(ObserveVideoPrompt, "영상 안에서 실제 관찰한 시간 순서는 별도의 근거") {
		t.Fatal("real video chronology was conflated with photo order")
	}
}

func TestPostPromptCanonicalBlockFieldsAndConditionalVideoVocabulary(t *testing.T) {
	for _, language := range []Language{LanguageKorean, LanguageEnglish} {
		write, _ := BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: Profile{NoVoice: true}, TagCount: 4, Videos: []string{"clip.mp4"}})
		revise, _ := buildRevisePrompt(language, Profile{NoVoice: true}, PostContent{Blocks: []Block{{Type: BlockVideo, File: "clip.mp4"}}}, []string{"photo.jpg", "clip.mp4"}, []string{"photo.jpg"}, nil, "adjust", nil, 4, nil, FrozenGuidelines{})
		for name, prompt := range map[string]string{"write": write, "revise": revise} {
			for _, required := range []string{"TEXT", "QUOTE", "content", "LIST", "items", "IMAGE", "file", "alt", "caption", "GALLERY", "files", "layout", "VIDEO"} {
				if !strings.Contains(prompt, required) {
					t.Fatal("canonical field missing", name, required)
				}
			}
		}
		without, _ := BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: Profile{NoVoice: true}, TagCount: 4, Photos: []string{"photo.jpg"}})
		if strings.Contains(without, "VIDEO") {
			t.Fatal("photo-only prompt invites unbound video blocks")
		}
	}
}

func TestPostTagUpperBoundsAllowEmptyAndNeverOverrideUnrelatedRevisionTags(t *testing.T) {
	for maximum := 1; maximum <= 10; maximum++ {
		for _, language := range []Language{LanguageKorean, LanguageEnglish} {
			write, _ := BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: Profile{NoVoice: true}, TagCount: maximum, StockGuidelines: []StockGuideline{}})
			revise, _ := BuildRevisePromptForLanguage(language, Profile{NoVoice: true}, PostContent{Tags: []string{"old1", "old2", " old3 "}}, nil, "Fix the body only", nil, maximum, nil, FrozenGuidelines{Stock: []StockGuideline{}})
			writeBound, reviseBound := fmt.Sprintf("최대 %d개의 tags", maximum), fmt.Sprintf("경우에만 최대 %d개", maximum)
			empty, unchanged := "0개", "내용·순서·공백"
			if language == LanguageEnglish {
				writeBound, reviseBound = fmt.Sprintf("at most %d tags", maximum), fmt.Sprintf("at most %d tags", maximum)
				empty, unchanged = "Zero or fewer", "whitespace byte-for-byte"
			}
			if !strings.Contains(write, writeBound) || !strings.Contains(write, empty) || !strings.Contains(revise, reviseBound) || !strings.Contains(revise, unchanged) {
				t.Fatal("upper bound or unchanged tag scope lost", maximum, language)
			}
			if strings.Contains(write, "[작문 지침]") {
				t.Fatal("static tag cap reenabled tag-quality preference")
			}
		}
	}
}

package ai

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"strings"
	"testing"
)

func TestStorylinePromptCarriesSectionsAndNeighbors(t *testing.T) {
	post := &clip.LinkedPostText{Slug: "owned", Title: "Owner", Sections: []clip.LinkedPostSection{{Heading: "외관", Photos: []clip.LinkedPhoto{{File: "31134.jpg"}}}, {Heading: "주문", Text: "내가 주문한 음식"}}, CaptureNeighbors: map[string][]int{"source": {0, 1}}}
	for _, c := range linkedPromptCases(post) {
		if c.name != "storyline" && c.name != "flow" && c.name != "flow-measured" && c.name != "spoken" {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			var payload map[string]json.RawMessage
			if err := json.Unmarshal([]byte(c.user), &payload); err != nil {
				t.Fatal(err)
			}
			var got clip.LinkedPostText
			if err := json.Unmarshal(payload["linked_post"], &got); err != nil || !reflect.DeepEqual(&got, post) {
				t.Fatal("lost sections/photos/candidates", err, got)
			}
			for _, rule := range []string{"paragraphs follow linked_post.sections", "leave out a section with no footage", "whichever candidate section they fit", "section whose text they match", "matching no section stays unused", "Never write capture order as time passing", "outranks project_instruction ordering", "GUIDE-42"} {
				if !strings.Contains(c.system, rule) {
					t.Fatal("missing rule", rule)
				}
			}
			if strings.Contains(c.system, "31134.jpg") {
				t.Fatal("photo data gained system authority")
			}
		})
	}
	for _, c := range linkedPromptCases(post) {
		if c.name == "flow-following" && strings.Contains(c.system, linkedPostOrderRule) {
			t.Fatal("linked order overrode reviewed storyline")
		}
	}
}
func TestEmptyStageRuleInStorylineAndFlowPrompts(t *testing.T) {
	for _, post := range []*clip.LinkedPostText{nil, {Slug: "owned"}} {
		for _, c := range linkedPromptCases(post) {
			if !strings.HasPrefix(c.name, "flow") && !strings.HasPrefix(c.name, "storyline") && c.name != "spoken" && c.name != "spoken-following" {
				continue
			}
			if !strings.Contains(c.system, emptyStageRule) {
				t.Fatal("missing empty-stage rule", c.name, post)
			}
		}
	}
}

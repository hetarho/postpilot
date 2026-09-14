package ai

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestObservePromptUsesTheProjectLanguageAndPreservesSpeech(t *testing.T) {
	ko, _ := BuildObservePrompt(clip.ChunkInput{Language: "ko"})
	en, _ := BuildObservePrompt(clip.ChunkInput{Language: "en"})
	if ko == en || !strings.Contains(ko, "Required output language: Korean (ko)") || !strings.Contains(en, "Required output language: English (en)") {
		t.Fatal("project language lost")
	}
	for _, prompt := range []string{ko, en} {
		if !strings.Contains(prompt, "event, action, motion, subjects and quality") || !strings.Contains(prompt, "speech stays in the language spoken, without translation") {
			t.Fatal("incomplete language contract")
		}
	}
}

package voice

import (
	"strings"
	"testing"
	"unicode"
)

// VOICE-79: every catalog question carries three example replies in three registers, each a
// short Hangul reply that keeps a published review's frame without its content words.
func TestPromptExamplesCoverEveryQuestion(t *testing.T) {
	frames := map[string]exampleFrame{}
	for _, frame := range exampleFrames {
		if _, dup := frames[frame.ID]; dup {
			t.Fatalf("frame %q listed twice", frame.ID)
		}
		frames[frame.ID] = frame
	}
	familyText := map[string]string{}
	used := map[string]bool{}
	for _, prompt := range Prompts() {
		family := promptFamily(prompt.Key)
		used[family] = true
		if text, seen := familyText[family]; seen && text != prompt.Text {
			t.Fatalf("%s: family %s holds two question texts", prompt.Key, family)
		}
		familyText[family] = prompt.Text
		if len(prompt.Examples) != PromptExamples {
			t.Fatalf("%s: %d examples, want %d", prompt.Key, len(prompt.Examples), PromptExamples)
		}
		registers := map[Register]bool{}
		for _, example := range prompt.Examples {
			registers[example.Register] = true
			frame, ok := frames[example.Frame]
			if !ok {
				t.Fatalf("%s: unknown frame %q", prompt.Key, example.Frame)
			}
			if frame.Register != example.Register {
				t.Fatalf("%s: %q is %s but frame %s is %s", prompt.Key, example.Text, example.Register, frame.ID, frame.Register)
			}
			if n := len(ProseSentences(example.Text)); n < 1 || n > 3 {
				t.Fatalf("%s: %q has %d sentences", prompt.Key, example.Text, n)
			}
			if !strings.ContainsFunc(example.Text, func(r rune) bool { return unicode.Is(unicode.Hangul, r) }) {
				t.Fatalf("%s: %q holds no Hangul", prompt.Key, example.Text)
			}
			if strings.Contains(example.Text, "#") {
				t.Fatalf("%s: %q carries a hashtag", prompt.Key, example.Text)
			}
			for _, word := range frame.Avoid {
				if strings.Contains(example.Text, word) {
					t.Fatalf("%s: %q reuses %q from %s", prompt.Key, example.Text, word, frame.Source)
				}
			}
		}
		if len(registers) != PromptExamples {
			t.Fatalf("%s: registers %v are not distinct", prompt.Key, registers)
		}
	}
	for family := range promptExamples {
		if !used[family] {
			t.Fatalf("examples filed under %q match no question", family)
		}
	}
	if len(promptExamples) != len(familyText) {
		t.Fatalf("%d example sets for %d questions", len(promptExamples), len(familyText))
	}
}

func TestPromptByKeyCarriesExamples(t *testing.T) {
	prompt, ok := PromptByKey("everyday_food_03")
	if !ok || len(prompt.Examples) != PromptExamples {
		t.Fatalf("everyday_food_03 = %+v ok=%v", prompt.Examples, ok)
	}
	prompt.Examples[0].Text = "changed"
	again, _ := PromptByKey("everyday_food_03")
	if again.Examples[0].Text == "changed" {
		t.Fatal("a caller's edit reached the catalog")
	}
}

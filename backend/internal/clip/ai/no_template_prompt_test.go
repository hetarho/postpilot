package ai_test

import (
	"os"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// noTemplateInput is a project generated from its own settings alone (CLIP-5):
// no recipe, and the server's own empty document in place of a frozen template.
func noTemplateInput(in clip.PlanningInput) clip.PlanningInput {
	in.Template = clip.Recipe{}
	in.Composition = &clip.ProjectComposition{
		Snapshot: clip.CompositionSnapshot{Version: 1, Body: clip.EmptyCompositionBody()},
		Inputs:   clip.CompositionInputs{Values: map[string]string{}},
	}
	return in
}

// The template guide section is omitted WHOLE rather than sent empty, so a clip
// with no template adds no bytes to either writing call and the prefix every
// revision re-injects stays byte-identical (CLIP-5, TMPL-12).
func TestNoTemplateAddsNoBytesToTheFlowCall(t *testing.T) {
	_, payload, system := flowRequest(t, noTemplateInput(flowInput()), defaultFlow())
	if _, present := payload["template_guide"]; present {
		t.Fatal("an empty template guide was sent as a key of its own", payload["template_guide"])
	}
	// The same request WITH a template carries the key, and the prefix both
	// calls are sent under is the same bytes either way.
	_, withTemplate, templateSystem := flowRequest(t, flowInput(), defaultFlow())
	if _, present := withTemplate["template_guide"]; !present {
		t.Fatal("the fixture template carries no guide, so this proves nothing")
	}
	if system != templateSystem {
		t.Fatal("the prefix changed with the template attached")
	}
	golden(t, "testdata/flow-prompt-prefix.txt", system)
	// A template that simply says nothing is omitted the same way: an empty
	// guide is a section that is not there, not a section that is empty.
	silent := flowInput()
	body := strings.Replace(flowBody, "<guide>음식을 차분하게 보여준다.</guide>", "", 1)
	silent.Template.CompositionBody = body
	silent.Composition.Snapshot.Body = body
	if _, quiet, _ := flowRequest(t, silent, defaultFlow()); hasKey(quiet, "template_guide") {
		t.Fatal("a template with nothing to say still sent the section", quiet["template_guide"])
	}
}

func hasKey(payload map[string]any, key string) bool {
	_, present := payload[key]
	return present
}

func TestNoTemplateAddsNoBytesToTheNarrationCall(t *testing.T) {
	// The flow is written from the same settings, so the narration reads a flow
	// that was itself produced without a template.
	planning := noTemplateInput(flowInput())
	s, _, _ := newService(t, defaultFlow(), true)
	flow, _, err := s.Flow(t.Context(), testRef(), planning)
	if err != nil {
		t.Fatal(err)
	}
	in := clip.NarrationInput{PlanningInput: planning, Flow: flow}
	_, payload, system := narrate(t, in, narrationResponse(narrationCaption("고기를 올렸어요", 1000, 4000)))
	if _, present := payload["template_guide"]; present {
		t.Fatal("an empty template guide was sent as a key of its own", payload["template_guide"])
	}
	golden(t, "testdata/narration-prompt-prefix.txt", system)
}

// golden pins the recorded prefix, so a prompt edit is a deliberate act:
// re-record with UPDATE_PROMPT_GOLDEN=1.
func golden(t *testing.T, path, got string) {
	t.Helper()
	if os.Getenv("UPDATE_PROMPT_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(string(want), "\n") != strings.TrimRight(got, "\n") {
		t.Fatalf("%s no longer matches the prompt this call sends", path)
	}
}

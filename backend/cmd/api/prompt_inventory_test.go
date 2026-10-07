package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/llm"
)

func TestPromptInventoryEnumeratesCurrentOwnersWithRealSourcesAndNoIssuedClaim(t *testing.T) {
	var output bytes.Buffer
	if err := runPromptInventory(nil, &output); err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Kind         string                   `json:"kind"`
		Material     string                   `json:"material"`
		SourceRoot   string                   `json:"source_root"`
		Compositions []llm.RequestComposition `json:"compositions"`
	}
	if err := json.Unmarshal(output.Bytes(), &inventory); err != nil || inventory.Kind != "code" || inventory.Material != "synthetic" || inventory.SourceRoot != "backend" {
		t.Fatalf("not a declarative code inventory: %+v %v", inventory, err)
	}
	seen, owners := map[string]bool{}, map[string]bool{}
	for _, composition := range inventory.Compositions {
		key := composition.Composer + "/" + composition.Stage + "/" + composition.Mode
		if seen[key] || composition.Composer == "" || composition.Parser == "" || composition.Consumer == "" || composition.Activation == "" || len(composition.SourceFiles) == 0 {
			t.Fatalf("ambiguous or incomplete real composer: %+v", composition)
		}
		seen[key] = true
		for _, source := range composition.SourceFiles {
			if filepath.IsAbs(source) || strings.Contains(source, "..") {
				t.Fatal("inventory source is not repository relative", source)
			}
			if _, err := os.Stat(filepath.Join("../..", source)); err != nil {
				t.Fatal("inventory claims a fictional source", source, err)
			}
			for _, owner := range []string{"generation", "authoring", "voice", "memory", "template", "clip/ai", "voice/spoken/app", "clip/app"} {
				if strings.HasPrefix(source, "internal/"+owner+"/") {
					owners[owner] = true
				}
			}
		}
		prepared, err := llm.PreparedCompositionInspection(&composition)
		if err != nil || prepared.Status != llm.InspectionPrepared || prepared.IssuedAt != nil || prepared.Measures.ProviderPromptTokens != nil {
			t.Fatalf("code inventory claims actual dispatch or invalid output: %+v %v", prepared, err)
		}
	}
	if len(owners) != 8 || len(seen) == 0 {
		t.Fatalf("actual model-request owners missing: %+v", owners)
	}
	output.Reset()
	if err := runPromptInventory([]string{"--private-account"}, &output); err == nil || output.Len() != 0 {
		t.Fatal("code inventory accepted customer-read input")
	}
}

func TestPromptInventoryRunsBeforePlatformBoot(t *testing.T) {
	if os.Getenv("POSTPILOT_PROMPT_INVENTORY_PROCESS") == "1" {
		if !runCommand([]string{"prompt-inventory"}) {
			t.Fatal("inventory command was not discovered")
		}
		return
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPromptInventoryRunsBeforePlatformBoot$")
	command.Env = append(os.Environ(), "POSTPILOT_PROMPT_INVENTORY_PROCESS=1", "PORT=invalid-before-boot")
	output, err := command.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte(`"kind": "code"`)) || bytes.Contains(output, []byte("migration applied")) {
		t.Fatalf("code inspection depended on platform/customer boot: %s %v", output, err)
	}
}

func TestAuthoringInventoryUsesActualKindGrammarAndModeOutput(t *testing.T) {
	guides := authoringInventoryGuides()
	seen := 0
	for _, c := range productRequestCompositions() {
		if c.Stage != "setting-authoring" {
			continue
		}
		seen++
		kind, mode, _ := strings.Cut(c.Mode, "/")
		var guide, user, system string
		for _, f := range c.Fragments {
			if f.ID == "guide" {
				guide = f.Text
			}
			if f.Role == llm.InspectionRoleUser {
				user += f.Text
			} else {
				system += f.Text
			}
		}
		var decoded string
		if err := json.Unmarshal([]byte(guide), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != guides[authoring.Kind(kind)] || strings.Contains(decoded, "Synthetic kind") {
			t.Fatal("inventory missed real owning grammar", kind)
		}
		if mode == "refine" && (strings.Contains(user, "candidate_count") || strings.Contains(system, "candidate_count")) {
			t.Fatal("batch rule reached refinement")
		}
		if strings.Contains(kind, "guideline") && (strings.Contains(c.Output.Schema, "description") || strings.Contains(c.Output.Schema, "title_area")) {
			t.Fatal("unconsumed guideline fields requested")
		}
		if kind == "post_template" && (!strings.Contains(decoded, "&lt;") || !strings.Contains(decoded, "ask") || strings.Contains(decoded, "[답변 방식]")) {
			t.Fatal("post role grammar drifted")
		}
		if kind == "video_template" && !strings.Contains(decoded, `<clip version="1">`) {
			t.Fatal("video grammar absent")
		}
		if kind == "writing_voice" && !strings.Contains(decoded, "사용자의 실제 경험이나 입력 사실이 아니며") {
			t.Fatal("fictional style role absent")
		}
	}
	if seen != 10 {
		t.Fatal("kind/mode inventory incomplete", seen)
	}
}

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
		hash, err := promptJSONHash(composition)
		if err != nil {
			t.Fatal(err)
		}
		key := composition.Composer + "/" + promptCompositionKey(composition) + "/" + hash
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

func TestPromptInventoryPreparedRequestsAndSourceIdentity(t *testing.T) {
	report, err := buildPromptInventory(productRequestCompositions())
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != 2 || report.Provenance.SourceStatus != "available" || report.Provenance.SourceSHA256 == "" || len(report.Entries) != len(report.Compositions) || report.Execution != (promptOfflineExecution{Mode: "offline"}) {
		t.Fatalf("missing offline source-bound inventory: %+v", report.Provenance)
	}
	root, err := promptBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range report.Provenance.Sources {
		raw, err := os.ReadFile(filepath.Join(root, strings.TrimPrefix(source.Path, "backend/")))
		if err != nil || source.SHA256 != promptHash(raw) {
			t.Fatalf("source identity is not its actual bytes: %+v %v", source, err)
		}
	}
	for i, entry := range report.Entries {
		composition := report.Compositions[i]
		hash, err := promptJSONHash(composition)
		if err != nil || entry.CompositionSHA256 != hash || entry.SchemaSHA256 != promptHash([]byte(composition.Output.Schema)) {
			t.Fatalf("composition/schema identity changed: %s %v", entry.Key, err)
		}
		if entry.Prepared.Status != llm.InspectionPrepared || entry.Prepared.IssuedAt != nil || entry.Prepared.Conditions != nil || entry.Prepared.Measures.ProviderPromptTokens != nil || len(entry.Links.Composer) == 0 || len(entry.Links.Parser) == 0 || len(entry.Links.Schema) == 0 || len(entry.Links.Consumer) == 0 {
			t.Fatalf("entry lacks actual code links or invents runtime conditions: %s %+v", entry.Key, entry.Links)
		}
		if err := entry.Prepared.Validate(); err != nil {
			t.Fatal(err)
		}
		if entry.Operation == "native-speech" && (len(entry.ApplicationText.Messages) != 0 || entry.ApplicationText.System != "" || len(entry.ApplicationText.Native) == 0 || entry.Prepared.Measures.ReferenceTokenEstimate != nil) {
			t.Fatalf("native operation acquired chat/token vocabulary: %s", entry.Key)
		}
		for _, links := range [][]promptSourceLink{entry.Links.Composer, entry.Links.Parser, entry.Links.Schema, entry.Links.Consumer} {
			for _, link := range links {
				if !strings.HasPrefix(link.Path, "backend/internal/") || link.SHA256 == "" {
					t.Fatalf("non-code or unhashed inspection source: %+v", link)
				}
				if link.Symbol != "" {
					raw, err := os.ReadFile(filepath.Join(root, strings.TrimPrefix(link.Path, "backend/")))
					lines := strings.Split(string(raw), "\n")
					if err != nil || link.Line < 1 || link.Line > len(lines) || !strings.Contains(lines[link.Line-1], link.Symbol) || !strings.HasPrefix(lines[link.Line-1], "func ") {
						t.Fatalf("invented declaration: %+v %v", link, err)
					}
				}
			}
		}
	}
}

func TestPromptSourceDigestChangesWithActualCodeAndIgnoresPrivateFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "composer.go")
	if err := os.WriteFile(source, []byte("package fixture\nfunc Compose() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first, _, err := loadPromptSources(root, []string{"composer.go"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("private fixture config"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "customer.sqlite"), []byte("private customer fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	second, _, err := loadPromptSources(root, []string{"composer.go"})
	if err != nil || first.SourceSHA256 != second.SourceSHA256 || len(second.Sources) != 1 {
		t.Fatalf("unrelated private files influenced code provenance: %+v %v", second, err)
	}
	if err := os.WriteFile(source, []byte("package fixture\nfunc Compose() { _ = 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	third, _, err := loadPromptSources(root, []string{"composer.go"})
	if err != nil || first.SourceSHA256 == third.SourceSHA256 || first.Sources[0].SHA256 == third.Sources[0].SHA256 {
		t.Fatal("source edits did not change exact source identities", err)
	}
	for _, invalid := range []string{"../private.go", "/private.go", "sub/../composer.go"} {
		if _, _, err := loadPromptSources(root, []string{invalid}); err == nil {
			t.Fatal("source manifest accepted a non-repository path", invalid)
		}
	}
}

func TestPromptInventoryStillRunsWithoutDeployedSourceTree(t *testing.T) {
	if os.Getenv("POSTPILOT_PROMPT_INVENTORY_PROCESS") == "1" {
		return // the existing subprocess entrypoint handles the command
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPromptInventoryRunsBeforePlatformBoot$")
	command.Dir = t.TempDir()
	command.Env = append(os.Environ(), "POSTPILOT_PROMPT_INVENTORY_PROCESS=1", "PORT=invalid-before-boot")
	output, err := command.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte(`"source_status": "unavailable"`)) || !bytes.Contains(output, []byte(`"source_sha256": "unavailable"`)) || !bytes.Contains(output, []byte(`"composition_sha256": "sha256:`)) {
		t.Fatalf("deployed inventory lost safe code output or fabricated source identity: %s %v", output, err)
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

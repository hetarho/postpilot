package generation

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestPromptEvaluationUsesActualCurrentAndRetainedRequestsForEveryMode(t *testing.T) {
	fixtures := PromptInventoryFixtures()
	cases, err := PromptEvaluationCases()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 24 || len(cases) != 48 {
		t.Fatal("missing admitted mode, protocol or instruction condition", len(fixtures), len(cases))
	}
	modes := map[string]int{}
	ids := map[string]bool{}
	for i, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			if ids[fixture.ID] {
				t.Fatal("ambiguous fixture identity")
			}
			ids[fixture.ID] = true
			baseline, variant := cases[2*i], cases[2*i+1]
			if !reflect.DeepEqual(baseline.Request, fixture.Request) {
				t.Fatal("developer baseline changed actual Korean request bytes or conditions")
			}
			if baseline.InstructionLanguage != PromptInstructionsKorean || variant.InstructionLanguage != PromptInstructionsEnglishCommon || baseline.OutputLanguage != LanguageKorean || variant.OutputLanguage != LanguageKorean || fixture.ModelRef != "synthetic/prompt-evaluation" {
				t.Fatal("instruction condition became an output language or changed model identity")
			}
			if fixture.Request.MaxTokens <= 0 || fixture.Request.Reasoning != llm.ReasoningLow || len(fixture.Request.JSONSchema) == 0 {
				t.Fatal("not a prepared application request with explicit synthetic conditions")
			}
			if bytes.Equal(fixture.Request.JSONSchema, LegacyWriteAnswerSchema()) != (fixture.OriginProtocolVersion == 0) && fixture.Scenario == "direct" {
				t.Fatal("origin protocol did not select its real schema")
			}
			if strings.Contains(fixture.Request.System, observationOriginContract) || strings.Contains(fixture.Request.System, "Semantic-origin metadata is optional") {
				if fixture.OriginProtocolVersion != OriginProtocolVersion {
					t.Fatal("retained admitted path acquired origin instructions")
				}
			} else if fixture.OriginProtocolVersion == OriginProtocolVersion {
				t.Fatal("current runtime origin path was omitted")
			}
			if variant.Request.System == baseline.Request.System || !reflect.DeepEqual(variant.Request.Messages, baseline.Request.Messages) || !bytes.Equal(variant.Request.JSONSchema, baseline.Request.JSONSchema) || variant.Request.MaxTokens != baseline.Request.MaxTokens || variant.Request.Reasoning != baseline.Request.Reasoning {
				t.Fatal("language condition changed fixed material/schema/cap/effort or did not change common instructions")
			}
			before, after := baseline.Request.Composition.Fragments, variant.Request.Composition.Fragments
			if len(before) != len(after) {
				t.Fatal("condition changed material topology")
			}
			for j, fragment := range before {
				if fragment.ID != after[j].ID || fragment.MaterialRole != after[j].MaterialRole || fragment.Role != after[j].Role || fragment.Authorship != after[j].Authorship {
					t.Fatal("instruction compilation changed source roles")
				}
				if !evaluationContractFragment(fragment.ID) && !reflect.DeepEqual(fragment, after[j]) {
					t.Fatal("condition changed policy, voice, template, memory, source catalog or examples", fragment.ID)
				}
			}
			assertCompositionPrompt(t, baseline.Request)
			assertCompositionPrompt(t, variant.Request)
			modes[fixture.Request.Composition.Mode]++
		})
	}
	for _, mode := range []string{"direct", "frozen-storyline", "full-test-direct", "photo-observation", "video-observation", "full-test-photo-observation", "full-test-video-observation", "storyline-create", "storyline-rewrite", "revision"} {
		if modes[mode] < 2 {
			t.Fatal("missing current or retained admitted mode", mode)
		}
	}
}

func TestInstructionCompilerCannotTranslateOwnerValuesThatResembleCode(t *testing.T) {
	lookalike := koreanWriteTask + "\n" + koreanParagraphFormat + "\n" + koreanBlockFields + "\n" + koreanBlockFieldContract
	request := ComposeWriteRequest(WritePromptInput{
		Language: LanguageKorean, TagCount: 6,
		Profile: Profile{Text: lookalike, Excerpts: []string{lookalike}}, Title: koreanWriteTask, Memo: lookalike,
		Template:   &TemplateBrief{Name: "경계", BodyParts: []TemplateMaterialPart{{Kind: "literal", Text: lookalike}}, TitleParts: []TemplateMaterialPart{}},
		Guidelines: []string{lookalike}, Memories: []string{lookalike}, QualityRules: []string{lookalike},
	})
	beforeSystem := request.System
	variant, err := compilePromptEvaluationRequest(request, PromptInstructionsEnglishCommon)
	if err != nil {
		t.Fatal(err)
	}
	if request.System != beforeSystem || !reflect.DeepEqual(request.Messages, variant.Messages) {
		t.Fatal("compiler mutated input or translated owner User material")
	}
	for i, fragment := range request.Composition.Fragments {
		if fragment.Authorship == llm.FragmentAuthorshipAccount && !reflect.DeepEqual(fragment, variant.Composition.Fragments[i]) {
			t.Fatal("owner lookalike text was translated", fragment.ID)
		}
	}
	for _, fixed := range []string{koreanSourceHonestyContract, koreanNounsRule, templatePrecedence, typedTemplateLegend, memoryPrecedence} {
		if !strings.Contains(variant.System+actualEvaluationUserText(variant), fixed) {
			t.Fatal("language condition changed a noncommon policy", fixed)
		}
	}
}

func TestInstructionCompilerRefusesUnknownOrDetachedContract(t *testing.T) {
	request := PromptInventoryFixtures()[0].Request
	for _, condition := range []PromptInstructionLanguage{"", "en", "ko", "full-material-translation"} {
		if _, err := compilePromptEvaluationRequest(request, condition); err == nil {
			t.Fatal("accepted output-language selection or unknown condition", condition)
		}
	}
	request.System += "detached bytes"
	if _, err := compilePromptEvaluationRequest(request, PromptInstructionsEnglishCommon); err == nil {
		t.Fatal("compiled a reconstruction that differs from the actual request")
	}
	request = PromptInventoryFixtures()[0].Request
	request.Composition.Fragments[0].Authorship = llm.FragmentAuthorshipAccount
	if _, err := compilePromptEvaluationRequest(request, PromptInstructionsEnglishCommon); err == nil {
		t.Fatal("translated an account-owned contract")
	}
}

func TestPromptEvaluationDeterministicChecksUseActualParsersWithoutSemanticClaims(t *testing.T) {
	cases, err := PromptEvaluationCases()
	if err != nil {
		t.Fatal(err)
	}
	checks := CheckPromptEvaluationCases(cases)
	seen := map[string]int{}
	for _, check := range checks {
		if !check.Passed {
			t.Errorf("%s/%s: %s", check.CaseID, check.ID, check.Detail)
		}
		seen[check.ID]++
	}
	for _, id := range []string{"actual-response-parser", "zero-tags-under-cap", "sparse-tags-under-cap", "mixed-source-unicode-occurrences", "invalid-reference-ambiguous-quote-rejection", "compatible-label-does-not-certify-truth", "requested-only-revision-fixture", "identified-observation-reference-range", "plan-reference-range"} {
		if seen[id] == 0 {
			t.Fatal("missing actual structural contract check", id)
		}
	}
	for _, item := range cases {
		if item.ResponseProvenance != "fixed-synthetic-output-not-model-response" || item.FixtureVersion != PromptEvaluationFixtureVersion || item.CompilerVersion != PromptEvaluationCompilerVersion {
			t.Fatal("synthetic response/version provenance missing")
		}
		for _, expectation := range item.Expectations {
			if expectation.Assessment != "human-semantic" || expectation.ExpectedSource == "" || expectation.Note == "" {
				t.Fatal("source/prose review became an automatic semantic verdict")
			}
		}
	}
}

func TestPromptEvaluationChecksDetectMaterialConditionAndSyntheticOutputChanges(t *testing.T) {
	for _, mutation := range []struct {
		name string
		edit func(*PromptEvaluationCase)
		want string
	}{
		{"material", func(item *PromptEvaluationCase) {
			item.Request.Messages = []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("Translated material")}}}
		}, "instruction-condition-parity"},
		{"model", func(item *PromptEvaluationCase) { item.ModelRef = "other/model" }, "instruction-condition-parity"},
		{"budget", func(item *PromptEvaluationCase) { item.Request.MaxTokens++ }, "instruction-condition-parity"},
		{"target", func(item *PromptEvaluationCase) { item.OutputLanguage = LanguageEnglish }, "instruction-condition-parity"},
		{"synthetic-response", func(item *PromptEvaluationCase) { item.SyntheticResponse = `{"title":"unfinished"` }, "actual-response-parser"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			cases, err := PromptEvaluationCases()
			if err != nil {
				t.Fatal(err)
			}
			mutation.edit(&cases[1])
			failed := false
			for _, check := range CheckPromptEvaluationCases(cases[:2]) {
				failed = failed || check.CaseID == cases[1].ID && check.ID == mutation.want && !check.Passed
			}
			if !failed {
				t.Fatal("check certified changed condition or incomplete response", mutation.want)
			}
		})
	}
}

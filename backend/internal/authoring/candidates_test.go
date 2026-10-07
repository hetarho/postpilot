package authoring

import (
	"encoding/json"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

func countedBatch(kind Kind, count int) string {
	var values []artifactWire
	for i := 0; i < count; i++ {
		body := fmt.Sprintf("<write>方向%d</write>", i)
		if kind == WritingVoice {
			body = strings.Repeat("가", 250) + fmt.Sprint(i)
		}
		values = append(values, artifactWire{Name: fmt.Sprintf("후보%d", i), Description: "서로 다른 방향", Body: body})
	}
	raw, _ := json.Marshal(map[string]any{"candidates": values})
	return string(raw)
}
func TestExactCandidateCountsAcrossEveryDomain(t *testing.T) {
	svc := &Service{targets: testTargetGuide{}}
	for _, kind := range []Kind{PostTemplate, VideoTemplate, PostGuideline, VideoGuideline, WritingVoice} {
		for count := 1; count <= MaxCandidateCount; count++ {
			t.Run(fmt.Sprintf("%s/%d", kind, count), func(t *testing.T) {
				in := operationInput{Kind: kind, Mode: Recommend, OperationID: "op", CandidateCount: count}
				out, err := svc.parseResponse(in, countedBatch(kind, count))
				if err != nil || len(out.Candidates) != count {
					t.Fatalf("exact batch %v", err)
				}
				for _, wrong := range []int{count - 1, count + 1} {
					if _, err = svc.parseResponse(in, countedBatch(kind, wrong)); err == nil {
						t.Fatal("partial/oversized output accepted")
					}
				}
				var parsed map[string]any
				_ = json.Unmarshal(responseSchema(Recommend, count), &parsed)
				array := parsed["properties"].(map[string]any)["candidates"].(map[string]any)
				if array["minItems"] != float64(count) || array["maxItems"] != float64(count) {
					t.Fatal("schema did not freeze exact count")
				}
				if count == 1 {
					return
				}
				out.Candidates[1].Body = out.Candidates[0].Body
				var malformed []artifactWire
				for _, a := range out.Candidates {
					a.ID = ""
					malformed = append(malformed, a)
				}
				raw, _ := json.Marshal(map[string]any{"candidates": malformed})
				if _, err = svc.parseResponse(in, string(raw)); err == nil {
					t.Fatal("duplicate body accepted")
				}
			})
		}
	}
}
func TestCandidateBudgetsUseExactBatchSizeAndConfiguredCeiling(t *testing.T) {
	budget := &countBudget{}
	svc := &Service{targets: testTargetGuide{}, budget: budget}
	info := llm.ModelInfo{ContextTokens: 131072}
	for count := 1; count <= MaxCandidateCount; count++ {
		in, err := svc.freezeInput(Session{Kind: PostGuideline}, Operation{}, Start{Mode: Recommend, RequestedCandidateCount: count}, info)
		expectedChars := RecommendationOutputChars(PostGuideline) / CandidateCount * count
		expectedCap := min(max(8192, expectedChars*2), 32768)
		if err != nil || in.CompletionTokens != expectedCap || in.CandidateCount != count || budget.chars != expectedChars {
			t.Fatalf("count%d budget %+v %v", count, in, err)
		}
		if !strings.Contains(modelMessage(in), fmt.Sprintf(`"candidate_count":%d`, count)) {
			t.Fatal("count missing from provider prompt")
		}
	}
	for count := CandidateCount + 1; count <= MaxCandidateCount; count++ {
		for _, contextTokens := range []int64{12000, 0} {
			info.ContextTokens = contextTokens
			if _, err := svc.freezeInput(Session{Kind: PostGuideline}, Operation{}, Start{Mode: Recommend, RequestedCandidateCount: count}, info); err != ErrModel {
				t.Fatalf("unsafe count%d with context%d was admitted or shrank: %v", count, contextTokens, err)
			}
		}
	}
}

func TestCandidateCountBoundsPreserveTheOrdinaryDefault(t *testing.T) {
	if count, err := NormalizeCandidateCount(0); err != nil || count != CandidateCount {
		t.Fatal("ordinary default changed", count, err)
	}
	for count := 1; count <= MaxCandidateCount; count++ {
		if actual, err := NormalizeCandidateCount(count); err != nil || actual != count {
			t.Fatal("explicit count changed", count, actual, err)
		}
	}
	for _, count := range []int{-1, MaxCandidateCount + 1, int(^uint(0) >> 1)} {
		if _, err := NormalizeCandidateCount(count); err != ErrCandidateCount {
			t.Fatal("unbounded count admitted", count, err)
		}
	}
}
func TestRefineUsesInvalidCurrentSourceWithoutClippingOrOldPreview(t *testing.T) {
	svc := &Service{targets: testTargetGuide{}, budget: testBudget{}}
	state := Session{Kind: PostTemplate, Selected: &Artifact{ID: "draft", Name: "valid", Body: "OLD-PREVIEW"}, WorkingSource: &Artifact{ID: "draft", Name: "current", Body: "<write>UNFINISHED"}, DraftState: DraftInvalid}
	in, err := svc.freezeInput(state, Operation{}, Start{Mode: Refine, Prompt: "완성해 주세요"}, llm.ModelInfo{ContextTokens: 131072})
	if err != nil {
		t.Fatal(err)
	}
	if in.Selected.Body != state.WorkingSource.Body || strings.Contains(modelMessage(in), "OLD-PREVIEW") {
		t.Fatal("AI ignored incomplete current source")
	}
}

type countBudget struct{ chars int }

func (b *countBudget) CompletionCap(_ Kind, _ Mode, chars int, native bool) int {
	b.chars = chars
	cap := max(8192, chars*2)
	if native {
		cap *= 2
	}
	return min(cap, 32768)
}

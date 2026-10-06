package authoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

// Core tests use the real SQLite store through test fixtures in store/store_test.go.
// These narrow behavior fakes isolate provider and cancellation execution boundaries.
type testModels struct {
	mu      sync.Mutex
	calls   []llm.Request
	text    string
	failure error
	after   func()
}

func (m *testModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Ref: ref, Stages: []string{"write"}, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0", ContextTokens: 32768, StructuredOutput: true}, ref.ProviderID != "" && ref.ModelID != ""
}
func (m *testModels) Complete(ctx context.Context, _ llm.ModelRef, r llm.Request) (llm.Response, error) {
	m.mu.Lock()
	m.calls = append(m.calls, r)
	text, failure, after := m.text, m.failure, m.after
	m.mu.Unlock()
	if after != nil {
		after()
	}
	return llm.Response{Text: text}, failure
}

type testBudget struct{}

func (testBudget) CompletionCap(_ Kind, _ Mode, chars int, _ bool) int {
	if chars > 4000 {
		return 32768
	}
	return 8192
}

type testEstimator struct{}

func (testEstimator) CallCredits(context.Context, llm.ModelInfo, int64, int64) (int, bool) {
	return 12, true
}
func TestFrozenContextNeverClipsCurrentDraftOrLatestRequest(t *testing.T) {
	targets := testTargetGuide{}
	svc := &Service{targets: targets, budget: testBudget{}}
	request := strings.Repeat("한", MaxPromptChars)
	selected := Artifact{ID: "private-artifact-id", Name: "스타일", Description: "설명", Body: strings.Repeat("가", 700)}
	state := Session{ID: "private-session-id", UserID: "secret-owner-id", Kind: WritingVoice, Selected: &selected, SourceContext: "counted style projection"}
	for n := 0; n < 20; n++ {
		state.Turns = append(state.Turns, Turn{Status: "done", Request: strings.Repeat("나", 800), Reply: strings.Repeat("다", 200)})
	}
	info, _ := (&testModels{}).Resolve(llm.ModelRef{ProviderID: "p", ModelID: "m"})
	input, e := svc.freezeInput(state, Operation{ID: "private-operation-id"}, Start{Mode: Refine, Prompt: request}, info)
	if e != nil {
		t.Fatal(e)
	}
	text := modelMessage(input)
	if !strings.Contains(text, request) || !strings.Contains(text, selected.Body) {
		t.Fatal("current material was clipped")
	}
	if len(input.History) >= 20 {
		t.Fatal("unbounded history reached provider")
	}
	for _, private := range []string{state.ID, state.UserID, selected.ID, "private-operation-id"} {
		if strings.Contains(text, private) {
			t.Fatalf("private identifier %s reached provider", private)
		}
	}
	if input.PromptTokens+input.CompletionTokens > int(info.ContextTokens) {
		t.Fatal("request exceeds model context")
	}
}

type testTargetGuide struct{}

func (testTargetGuide) Seed(context.Context, string, Kind, string) (Seed, error) { return Seed{}, nil }
func (testTargetGuide) CanStart(context.Context, string, Kind, string) error     { return nil }
func (testTargetGuide) Validate(k Kind, a Artifact) error {
	if a.Body == "" {
		return ErrOutput
	}
	if k == WritingVoice && (utf8.RuneCountInString(a.Body) < 200 || utf8.RuneCountInString(a.Body) > 700) {
		return ErrOutput
	}
	return nil
}
func (testTargetGuide) Guide(Kind) string {
	return "문법 안내: 본문을 바꾸고 저장하기 전에는 초안입니다."
}
func (testTargetGuide) Publish(context.Context, Publication) (SavedRef, error) {
	return SavedRef{}, errors.New("not used")
}
func recommendationText(kind Kind) string {
	values := []map[string]string{}
	for n := 0; n < 8; n++ {
		body := fmt.Sprintf("<write>주제 %d</write>", n)
		if kind == WritingVoice {
			body = strings.Repeat("가", 250) + fmt.Sprint(n)
		}
		values = append(values, map[string]string{"name": fmt.Sprintf("후보%d", n), "description": "이런 느낌이에요", "body": body, "title_area": ""})
	}
	b, _ := json.Marshal(map[string]any{"candidates": values})
	return string(b)
}
func TestRecommendationParsingRequiresAllEightDistinctDomainValidArtifacts(t *testing.T) {
	svc := &Service{targets: testTargetGuide{}}
	for _, kind := range []Kind{PostTemplate, VideoTemplate, PostGuideline, VideoGuideline, WritingVoice} {
		in := operationInput{Version: 1, Kind: kind, Mode: Recommend, OperationID: "op"}
		out, e := svc.parseResponse(in, recommendationText(kind))
		if e != nil || len(out.Candidates) != 8 {
			t.Fatalf("%s: %+v %v", kind, out, e)
		}
		if out.Candidates[0].ID == out.Candidates[1].ID {
			t.Fatal("server identifiers collided")
		}
	}
	var data map[string][]map[string]string
	_ = json.Unmarshal([]byte(recommendationText(PostTemplate)), &data)
	data["candidates"] = data["candidates"][:7]
	raw, _ := json.Marshal(data)
	if _, e := svc.parseResponse(operationInput{Kind: PostTemplate, Mode: Recommend}, string(raw)); e == nil {
		t.Fatal("partial result accepted")
	}
	_ = json.Unmarshal([]byte(recommendationText(PostTemplate)), &data)
	data["candidates"][1]["name"] = data["candidates"][0]["name"]
	raw, _ = json.Marshal(data)
	if _, e := svc.parseResponse(operationInput{Kind: PostTemplate, Mode: Recommend}, string(raw)); e == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestRefinementRejectsModelIdentifiersAndKeepsSelectedIdentity(t *testing.T) {
	svc := &Service{targets: testTargetGuide{}}
	selected := artifactToWire(Artifact{ID: "owner-chosen", Body: "old"})
	in := operationInput{Kind: PostGuideline, Mode: Refine, Selected: &selected}
	response := `{"artifact":{"name":"지침","description":"설명","body":"짧게 써요","title_area":""},"reply":"더 간결하게 바꿨어요."}`
	out, e := svc.parseResponse(in, response)
	if e != nil || out.Selected.ID != "owner-chosen" {
		t.Fatalf("identity %+v %v", out, e)
	}
	bad := strings.Replace(response, `"name":"지침"`, `"id":"model-chosen","name":"지침"`, 1)
	if _, e := svc.parseResponse(in, bad); e == nil {
		t.Fatal("model-supplied id accepted")
	}
}
func TestSmallContextFitsCompletionCapAndRejectsImpossibleInput(t *testing.T) {
	svc := &Service{targets: testTargetGuide{}, budget: testBudget{}}
	info, _ := (&testModels{}).Resolve(llm.ModelRef{ProviderID: "p", ModelID: "m"})
	state := Session{Kind: PostTemplate}
	info.ContextTokens = 10000
	in, e := svc.freezeInput(state, Operation{}, Start{Mode: Recommend}, info)
	if e != nil || in.CompletionTokens+in.PromptTokens > 10000 {
		t.Fatalf("small context %+v %v", in, e)
	}
	state.Selected = &Artifact{Body: strings.Repeat("가", 20000)}
	if _, e = svc.freezeInput(state, Operation{}, Start{Mode: Refine}, info); e == nil {
		t.Fatal("impossible document was admitted")
	}
}

func TestEscapedRecentHistoryFitsActualSerializedContext(t *testing.T) {
	svc := &Service{targets: testTargetGuide{}, budget: testBudget{}}
	info, _ := (&testModels{}).Resolve(llm.ModelRef{ProviderID: "p", ModelID: "m"})
	info.ContextTokens = 11500
	state := Session{Kind: PostGuideline, Selected: &Artifact{Name: "선택한 지침", Body: "현재 문서는 유지해요."}}
	for range 20 {
		state.Turns = append(state.Turns, Turn{Status: "done", Request: strings.Repeat("\x01", 400), Reply: strings.Repeat("\x02", 100)})
	}
	in, e := svc.freezeInput(state, Operation{}, Start{Mode: Refine, Prompt: "이 요청도 유지해요."}, info)
	if e != nil {
		t.Fatal(e)
	}
	if in.PromptTokens+in.CompletionTokens > int(info.ContextTokens) || in.PromptTokens > MaxModelPromptChars {
		t.Fatal("escaped history exceeded actual serialized model context")
	}
	if in.Selected.Body != state.Selected.Body || in.Prompt != "이 요청도 유지해요." {
		t.Fatal("latest material changed")
	}
}

func TestTerminalRecoveryRejectsChangedSelectedIdentity(t *testing.T) {
	svc := &Service{targets: testTargetGuide{}}
	a := artifactToWire(Artifact{ID: "selected", Name: "지침", Body: "원래 방향"})
	in := operationInput{Version: 1, SessionID: "session", OperationID: "operation", Kind: PostGuideline, Mode: Refine, Selected: &a}
	payload, _ := encodeInput(in)
	op := Operation{ID: "operation", SessionID: "session", Mode: Refine, Payload: payload}
	out, e := svc.parseResponse(in, `{"artifact":{"name":"지침","description":"설명","body":"새 방향","title_area":""},"reply":"방향을 바꿨어요."}`)
	if e != nil {
		t.Fatal(e)
	}
	out.Selected.ID = "foreign-identity"
	raw, _ := json.Marshal(out)
	if _, e = svc.readResult(PostGuideline, op, Job{Payload: raw}); e == nil {
		t.Fatal("terminal result changed the chosen artifact identity")
	}
}

var _ Models = (*testModels)(nil)
var _ Budget = testBudget{}
var _ Targets = testTargetGuide{}

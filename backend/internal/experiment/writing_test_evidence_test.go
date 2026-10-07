package experiment

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

func TestWritingTestPrivateEvidenceRoundTripAndPublicOutputIsolation(t *testing.T) {
	index := 0
	value := TestOutput{ContentLanguage: "ko", Content: TestOutputContent{Title: "결과", Blocks: []TestOutputBlock{{Type: "TEXT", Content: "직접 입력"}}}, Storyline: &TestOutputStoryline{Paragraphs: []TestOutputParagraph{{Text: "계획"}}}, RequestInspections: []llm.RequestInspection{privateInspectionFixture()}}
	value.Origins = &post.OriginReview{Version: 1, Result: post.ContentOriginIdentity(post.PostContent{Title: "결과", Blocks: []post.Block{{Type: post.BlockText, Content: "직접 입력"}}}, 0), Sources: []post.OriginSource{{ID: "memo", Kind: post.OriginSourceMemo, Text: "private memo evidence", Available: true}}, Spans: []post.OriginSpan{{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index}, Start: 0, End: 5, Quote: "직접 입력", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}, ReviewState: post.OriginUnreviewed}}}
	value.Storyline.Origins = &post.PlanOriginReview{Version: 1, Result: post.PlanOriginIdentity([]post.StorylineParagraph{{Text: "계획"}}), Sources: []post.OriginSource{{ID: "proposal", Kind: post.OriginSourceAIProposal, Text: "private plan evidence", Available: true}}, Spans: []post.PlanOriginSpan{{ParagraphIndex: 0, Start: 0, End: 2, Quote: "계획", Category: post.OriginAIAdded, SourceRefs: []string{"proposal"}, ReviewState: post.OriginUnreviewed}}}
	raw, err := EncodeTestOutput(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeTestOutput(raw)
	if err != nil || !reflect.DeepEqual(decoded, value) {
		t.Fatalf("evidence changed: %v, %#v", err, decoded)
	}
	decoded.Origins.Sources[0].Text = "changed"
	decoded.RequestInspections[0].Fragments[0].Text = "changed"
	check, _ := DecodeTestOutput(raw)
	if check.Origins.Sources[0].Text != "private memo evidence" || check.RequestInspections[0].Fragments[0].Text != "private model instructions" {
		t.Fatal("private payload alias")
	}
	public, err := DecodeTestOutput(PublicTestOutput(raw))
	if err != nil || public.Origins != nil || len(public.RequestInspections) != 0 || public.Storyline.Origins != nil || !reflect.DeepEqual(public.Content, value.Content) {
		t.Fatalf("public output: %v, %#v", err, public)
	}
	if bytes.Contains(PublicTestOutput(raw), []byte("private")) {
		t.Fatal("hidden output evidence leaked")
	}
}

func TestWritingTestMalformedOptionalEvidenceKeepsCanonicalOutput(t *testing.T) {
	value := TestOutput{ContentLanguage: "en", Content: TestOutputContent{Title: "accepted"}, Storyline: &TestOutputStoryline{Paragraphs: []TestOutputParagraph{{Text: "accepted plan"}}}}
	raw, _ := EncodeTestOutput(value)
	for _, corruption := range []string{`"not an origin"`, `{"cost_microusd":123,"api_key":"private"}`, `[]`, `{"Version":"broken"}`} {
		var wire map[string]json.RawMessage
		_ = json.Unmarshal(raw, &wire)
		wire["origins"], wire["request_inspections"] = json.RawMessage(corruption), json.RawMessage(corruption)
		corrupt, _ := json.Marshal(wire)
		got, err := DecodeTestOutput(corrupt)
		if err != nil || !reflect.DeepEqual(got, value) {
			t.Fatalf("canonical rejected for %s: %v, %#v", corruption, err, got)
		}
	}
	if output := PublicTestOutput([]byte(`{"version":1,"cost_microusd":999}`)); output != nil {
		t.Fatal("opaque corrupt private payload forwarded")
	}
}

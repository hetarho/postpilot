package ai_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
)

type fakeModels struct {
	info     llm.ModelInfo
	response llm.Response
	err      error
	calls    []llm.Request
	refs     []llm.ModelRef
}

func (f *fakeModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) { return f.info, true }
func (f *fakeModels) Complete(_ context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	f.calls = append(f.calls, request)
	f.refs = append(f.refs, ref)
	return f.response, f.err
}

type fakeSizer struct {
	err   error
	calls int
}

func (f *fakeSizer) CaptionSize(context.Context, string, clip.Caption) (float64, float64, error) {
	f.calls++
	return 500, 100, f.err
}
func newService(t *testing.T, raw string, structured bool) (*ai.Service, *fakeModels, *fakeSizer) {
	t.Helper()
	f := &fakeModels{info: llm.ModelInfo{Vision: true, VideoInput: true, StructuredOutput: structured, Stages: []string{llm.StageNameObserve, llm.StageNameWrite}}, response: llm.Response{Text: raw, Usage: llm.Usage{CompletionTokens: 100, PromptTokens: 200, CostReported: true, CostMicrousd: 10}}}
	c := &fakeSizer{}
	cfg := config.ClipAI(&config.Config{LLMReasoning: config.LLMReasoningPolicy{Observe: llm.ReasoningLow, Write: llm.ReasoningLow}})
	s, err := ai.New(f, c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s, f, c
}
func source() clip.AnalysisSource {
	return clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "frozen-fingerprint", Info: clip.MediaInfo{DurationMS: 65000, Width: 1080, Height: 1920, HasAudio: true}}, Filename: "제주 & Seoul.mp4"}
}
func chunk() clip.ChunkInput {
	return clip.ChunkInput{Source: source(), Index: 1, OffsetMS: 60000, DurationMS: 5000, URL: "https://storage.example/proxy.mp4?signature=private"}
}
func observation() map[string]any {
	return map[string]any{"source_id": "source", "chunk_index": 1, "segments": []any{map[string]any{"start_ms": 0, "end_ms": 5000, "event": "음식을 담는다", "subjects": []string{"접시"}, "speech": "", "quality": "steady and sharp", "focal": map[string]any{"x": .5, "y": .5}, "avoid": map[string]any{"x": .2, "y": .6, "width": .6, "height": .3}}}}
}
func planningInput() clip.PlanningInput {
	return clip.PlanningInput{Template: clip.Recipe{Name: "제주 & Seoul", InformationFields: []clip.InformationField{{Label: "장소 / Place", Prompt: "어디인가요?"}}, CutGuidance: "현장 소리를 남겨줘. Keep the original sound.", CopyStyles: []string{"clean", "diary"}, Accent: "coral"}, Answers: []clip.Answer{{Label: "장소 / Place", Text: "한글 <그대로> & O'Brien\nKeep 10:30 unchanged."}}, Ratio: "vertical", TargetDurationMS: 15000, Analyses: []clip.SourceAnalysis{{Source: source(), Segments: []clip.Segment{{StartMS: 0, EndMS: 65000, Event: "음식을 담는다", Subjects: []string{"접시"}, Speech: "", Quality: "steady", Focal: clip.Point{X: .5, Y: .5}, Avoid: clip.Region{X: .2, Y: .6, Width: .6, Height: .3}}}}}}
}
func plan() map[string]any {
	return map[string]any{"ratio": "vertical", "duration_ms": 15000, "cuts": []any{map[string]any{"id": "cut-one", "source_id": "source", "start_ms": 0, "end_ms": 15000, "focal": map[string]any{"x": .5, "y": .5}, "caption": map[string]any{"text": "정확한 한글 & 여행", "start_ms": 1000, "end_ms": 14000, "position": "bottom", "style": "clean", "accent": "coral"}}}}
}
func raw(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}
func firstSegment(value map[string]any) map[string]any {
	return value["segments"].([]any)[0].(map[string]any)
}
func firstCut(value map[string]any) map[string]any { return value["cuts"].([]any)[0].(map[string]any) }

func TestObservationContractPlainFallbackOffsetAndSpeech(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, speech := range []string{"", "오늘은 제주입니다. Today in Jeju."} {
			value := observation()
			firstSegment(value)["speech"] = speech
			firstSegment(value)["start_ms"] = -200
			firstSegment(value)["end_ms"] = 6000
			s, f, _ := newService(t, "```json\n"+raw(value)+"\n```", structured)
			in := chunk()
			in.Source.Info.HasAudio = speech != ""
			ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "explicit-observer"}
			got, usage, err := s.ObserveChunk(t.Context(), ref, in)
			if err != nil || got.Segments[0].StartMS != 60000 || got.Segments[0].EndMS != 65000 || got.Segments[0].Speech != speech || usage != f.response.Usage {
				t.Fatalf("%+v %+v %v", got, usage, err)
			}
			request := f.calls[0]
			if len(f.calls) != 1 || f.refs[0] != ref || request.Stage != llm.StageNameObserve || request.MaxTokens != s.Budgets().Observe || request.Reasoning != llm.ReasoningLow || (len(request.JSONSchema) > 0) != structured || !strings.Contains(request.System, string(ai.ChunkSchema())) {
				t.Fatalf("bad request %+v", request)
			}
			parts := request.Messages[0].Parts
			if len(parts) != 2 || parts[0].VideoURL != in.URL || parts[0].MIME != "video/mp4" || !strings.Contains(parts[1].Text, "60000") || !strings.Contains(parts[1].Text, in.Source.Filename) {
				t.Fatal(parts)
			}
			if strings.Contains(raw(got), "signature") {
				t.Fatal("signed URL retained")
			}
		}
	}
}
func TestObservationRejectsInvalidModelOutput(t *testing.T) {
	for _, mode := range []string{"source", "index", "extra", "missing", "null", "no description", "outside", "backwards", "zero", "fractional", "overlap", "focal", "avoid", "empty", "too many"} {
		t.Run(mode, func(t *testing.T) {
			v := observation()
			seg := firstSegment(v)
			switch mode {
			case "source":
				v["source_id"] = "invented"
			case "index":
				v["chunk_index"] = 0
			case "extra":
				seg["publish"] = true
			case "missing":
				delete(seg, "speech")
			case "null":
				seg["speech"] = nil
			case "no description":
				seg["event"] = " "
				seg["subjects"] = []string{}
			case "outside":
				seg["start_ms"] = 6000
				seg["end_ms"] = 7000
			case "backwards":
				seg["start_ms"] = 4000
				seg["end_ms"] = 3000
			case "zero":
				seg["end_ms"] = 0
			case "fractional":
				seg["end_ms"] = 1.5
			case "overlap":
				v["segments"] = []any{seg, seg}
			case "focal":
				seg["focal"].(map[string]any)["x"] = -.1
			case "avoid":
				seg["avoid"].(map[string]any)["width"] = 1
			case "empty":
				v["segments"] = []any{}
			case "too many":
				segments := make([]any, 61)
				for i := range segments {
					segments[i] = seg
				}
				v["segments"] = segments
			}
			s, f, _ := newService(t, raw(v), true)
			if _, _, err := s.ObserveChunk(t.Context(), llm.ModelRef{}, chunk()); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatal(err)
			}
			if len(f.calls) != 1 {
				t.Fatal("unplanned repair call")
			}
		})
	}
}
func TestPlanIsGroundedMeasuredAndPreservesExactAnswers(t *testing.T) {
	for _, structured := range []bool{false, true} {
		s, f, c := newService(t, "Here is the JSON:\n"+raw(plan()), structured)
		in := planningInput()
		got, usage, err := s.Plan(t.Context(), llm.ModelRef{}, in)
		if err != nil || usage != f.response.Usage || len(got.Cuts) != 1 {
			t.Fatalf("%+v %v", got, err)
		}
		cut := got.Cuts[0]
		if cut.Copy.Position != "top" || cut.Fingerprint != in.Analyses[0].Source.Fingerprint || cut.Volume == nil || *cut.Volume != 1 || cut.Copy.StartMS != 1000 || cut.Copy.EndMS != 14000 || c.calls != 1 {
			t.Fatalf("%+v", cut)
		}
		request := f.calls[0]
		if len(f.calls) != 1 || request.HasVideos() || request.HasImages() || request.MaxTokens != s.Budgets().Plan || request.Stage != llm.StageNameWrite || (request.JSONSchema != nil) != structured || !strings.Contains(request.System, string(ai.PlanSchema())) {
			t.Fatal(request)
		}
		var data struct {
			Answers []struct {
				Text string `json:"text"`
			} `json:"answers"`
		}
		if err := json.Unmarshal([]byte(request.Messages[0].Parts[0].Text), &data); err != nil || data.Answers[0].Text != in.Answers[0].Text {
			t.Fatalf("exact answers lost: %+v %v", data, err)
		}
		in.Analyses[0].Segments[0].Avoid = clip.Region{}
		got, _, err = s.Plan(t.Context(), llm.ModelRef{}, in)
		if err != nil || got.Cuts[0].Copy.Position != "bottom" {
			t.Fatalf("safe model position moved: %+v %v", got, err)
		}
	}
}
func TestPlanRejectsEveryInvalidBoundaryWithoutRepair(t *testing.T) {
	for _, mode := range []string{"unknown source", "empty cuts", "duplicate id", "empty id", "long id", "negative start", "outside source", "backwards", "short cut", "duration mismatch", "under minimum", "over maximum", "target drift", "wrong ratio", "caption negative", "caption past cut", "caption backwards", "caption missing", "free position", "disallowed style", "disallowed accent", "free coordinates", "music", "gain over", "gain under", "gain null", "fractional"} {
		t.Run(mode, func(t *testing.T) {
			v := plan()
			cut := firstCut(v)
			caption := cut["caption"].(map[string]any)
			switch mode {
			case "unknown source":
				cut["source_id"] = "invented"
			case "empty cuts":
				v["cuts"] = []any{}
			case "duplicate id":
				v["cuts"] = []any{cut, cut}
				v["duration_ms"] = 29800
			case "empty id":
				cut["id"] = " "
			case "long id":
				cut["id"] = strings.Repeat("a", 101)
			case "negative start":
				cut["start_ms"] = -1
			case "outside source":
				cut["end_ms"] = 65001
			case "backwards":
				cut["start_ms"] = 16000
			case "short cut":
				cut["end_ms"] = 400
			case "duration mismatch":
				cut["end_ms"] = 15001
			case "under minimum":
				v["duration_ms"] = 14999
			case "over maximum":
				v["duration_ms"] = 90001
			case "target drift":
				v["duration_ms"] = 17000
				cut["end_ms"] = 17000
			case "wrong ratio":
				v["ratio"] = "square"
			case "caption negative":
				caption["start_ms"] = -1
			case "caption past cut":
				caption["end_ms"] = 15001
			case "caption backwards":
				caption["start_ms"] = 14000
			case "caption missing":
				delete(caption, "start_ms")
			case "free position":
				caption["position"] = "anywhere"
			case "disallowed style":
				caption["style"] = "emphasis"
			case "disallowed accent":
				caption["accent"] = "blue"
			case "free coordinates":
				caption["x"] = 123
			case "music":
				v["background_music"] = "song.mp3"
			case "gain over":
				cut["volume"] = 1.1
			case "gain under":
				cut["volume"] = -.1
			case "gain null":
				cut["volume"] = nil
			case "fractional":
				cut["start_ms"] = 1.5
			}
			s, f, c := newService(t, raw(v), false)
			if _, _, err := s.Plan(t.Context(), llm.ModelRef{}, planningInput()); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("accepted bad plan: %v", err)
			}
			if len(f.calls) != 1 || c.calls != 0 {
				t.Fatal("invalid plan reached repair or renderer")
			}
		})
	}
}
func TestFailuresKeepStageUsageAndTruncationWithoutFallback(t *testing.T) {
	for _, stage := range []string{"analyze", "plan"} {
		for _, cause := range []error{llm.ErrRateLimited, llm.ErrProviderDisabled, llm.ErrUnsupported, nil} {
			s, f, _ := newService(t, "{\"partial\":", true)
			f.err = cause
			f.response.FinishReason = "length"
			f.response.Usage.ReasoningTokens = 90
			var usage llm.Usage
			var err error
			if stage == "analyze" {
				_, usage, err = s.ObserveChunk(t.Context(), llm.ModelRef{}, chunk())
			} else {
				_, usage, err = s.Plan(t.Context(), llm.ModelRef{}, planningInput())
			}
			want := cause
			if want == nil {
				want = llm.ErrOutputTruncated
			}
			var failure *ai.StageError
			if !errors.Is(err, want) || !errors.As(err, &failure) || failure.Stage != stage || usage != f.response.Usage || len(f.calls) != 1 {
				t.Fatalf("%+v %v", usage, err)
			}
			if cause == nil && (failure.Failure().Reason != llm.FailureReasonOutputTruncated || !strings.Contains(failure.Failure().TechnicalDetail, "90 of 100")) {
				t.Fatal(failure.Failure())
			}
		}
	}
}
func TestModelGatesAndInputValidationPrecedeNetwork(t *testing.T) {
	for _, mode := range []string{"no video", "no vision", "wrong purpose", "disabled", "bad chunk", "bad URL", "missing answer", "duplicate source"} {
		t.Run(mode, func(t *testing.T) {
			s, f, _ := newService(t, raw(observation()), true)
			in := chunk()
			planIn := planningInput()
			switch mode {
			case "no video":
				f.info.VideoInput = false
			case "no vision":
				f.info.Vision = false
			case "wrong purpose":
				f.info.Stages = []string{"video-generation"}
			case "disabled":
				f.info.Disabled = true
			case "bad chunk":
				in.OffsetMS++
			case "bad URL":
				in.URL = "file:///tmp/private.mp4"
			case "missing answer":
				planIn.Answers = nil
			case "duplicate source":
				planIn.Analyses = append(planIn.Analyses, planIn.Analyses[0])
			}
			var err error
			if mode == "missing answer" || mode == "duplicate source" {
				_, _, err = s.Plan(t.Context(), llm.ModelRef{}, planIn)
			} else {
				_, _, err = s.ObserveChunk(t.Context(), llm.ModelRef{}, in)
			}
			if err == nil || len(f.calls) != 0 {
				t.Fatal("invalid admission called provider")
			}
		})
	}
}
func TestSchemasAreClosedAndReturnedAsCopies(t *testing.T) {
	var inspect func(any)
	inspect = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if v["type"] == "object" && v["additionalProperties"] != false {
				t.Fatal("open schema object")
			}
			for _, child := range v {
				inspect(child)
			}
		case []any:
			for _, child := range v {
				inspect(child)
			}
		}
	}
	for _, schema := range []func() []byte{ai.ChunkSchema, ai.PlanSchema} {
		var value any
		if err := json.Unmarshal(schema(), &value); err != nil {
			t.Fatal(err)
		}
		inspect(value)
		a, b := schema(), schema()
		a[0] = '!'
		if reflect.DeepEqual(a, b) {
			t.Fatal("mutable embedded schema")
		}
	}
}

func TestPlanTwentySourcesNinetySecondsAndDistinctRangeReuse(t *testing.T) {
	in := planningInput()
	in.TargetDurationMS = 90000
	in.Analyses = nil
	for i := 0; i < 20; i++ {
		a := planningInput().Analyses[0]
		a.Source.ID = fmt.Sprintf("source-%d", i)
		a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
		a.Source.Info.DurationMS = 90000
		a.Segments[0].EndMS = 90000
		in.Analyses = append(in.Analyses, a)
	}
	v := plan()
	v["duration_ms"] = 90000
	cuts := make([]any, 100)
	for i := range cuts {
		c := firstCut(plan())
		c["id"] = fmt.Sprintf("cut-%d", i)
		c["source_id"] = fmt.Sprintf("source-%d", i%20)
		c["end_ms"] = 1098
		p := c["caption"].(map[string]any)
		p["start_ms"] = 0
		p["end_ms"] = 1098
		if i == 0 {
			c["volume"] = 0
		}
		cuts[i] = c
	}
	v["cuts"] = cuts
	s, f, _ := newService(t, raw(v), false)
	got, _, err := s.Plan(t.Context(), llm.ModelRef{}, in)
	if err != nil || got.DurationMS != 90000 || len(got.Cuts) != 100 || got.Cuts[0].OriginalVolume() != 0 || got.Cuts[1].OriginalVolume() != 1 || len(f.calls) != 1 || f.calls[0].MaxTokens != 32768 {
		t.Fatalf("%+v %v", got, err)
	}
}
func TestStrictFieldsSilentSpeechCancellationAndCaptionFailure(t *testing.T) {
	for _, mode := range []string{"case field", "null subject", "silent speech", "null focal"} {
		v := observation()
		seg := firstSegment(v)
		in := chunk()
		switch mode {
		case "case field":
			seg["Event"] = seg["event"]
			delete(seg, "event")
		case "null subject":
			seg["subjects"] = []any{nil}
		case "silent speech":
			in.Source.Info.HasAudio = false
			seg["speech"] = "invented audio"
		case "null focal":
			seg["focal"].(map[string]any)["x"] = nil
		}
		s, _, _ := newService(t, raw(v), false)
		if _, _, err := s.ObserveChunk(t.Context(), llm.ModelRef{}, in); !errors.Is(err, llm.ErrBadOutput) {
			t.Fatalf("%s: %v", mode, err)
		}
	}
	s, f, c := newService(t, raw(plan()), true)
	c.err = clip.ErrCopyTooLong
	if _, usage, err := s.Plan(t.Context(), llm.ModelRef{}, planningInput()); !errors.Is(err, clip.ErrCopyTooLong) || usage != f.response.Usage || len(f.calls) != 1 {
		t.Fatalf("%+v %v", usage, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	f.calls = nil
	if _, _, err := s.Plan(ctx, llm.ModelRef{}, planningInput()); !errors.Is(err, context.Canceled) || len(f.calls) != 0 {
		t.Fatal(err)
	}
	if _, _, err := s.ObserveChunk(ctx, llm.ModelRef{}, chunk()); !errors.Is(err, context.Canceled) || len(f.calls) != 0 {
		t.Fatal(err)
	}
}

func TestCutBoundsRejectIntegerWraparoundBeforeRendering(t *testing.T) {
	v := plan()
	c := firstCut(v)
	c["start_ms"] = math.MaxInt - 10000
	c["end_ms"] = math.MinInt + 4999
	s, f, measure := newService(t, raw(v), true)
	if _, _, err := s.Plan(t.Context(), llm.ModelRef{}, planningInput()); !errors.Is(err, llm.ErrBadOutput) || len(f.calls) != 1 || measure.calls != 0 {
		t.Fatalf("overflowed source range accepted: %v", err)
	}
}

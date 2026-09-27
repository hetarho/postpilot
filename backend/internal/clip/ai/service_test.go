package ai_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
)

type fakeModels struct {
	info     llm.ModelInfo
	response llm.Response
	err      error
	calls    []llm.Request
	refs     []llm.ModelRef
	// One body per call, for the paths that make more than one (a revision's
	// flow and then its narration). Empty falls back to `response`.
	responses []string
}

func (f *fakeModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) { return f.info, true }
func (f *fakeModels) Complete(_ context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	f.calls = append(f.calls, request)
	f.refs = append(f.refs, ref)
	if at := len(f.calls) - 1; at < len(f.responses) {
		out := f.response
		out.Text = f.responses[at]
		return out, f.err
	}
	return f.response, f.err
}

// structuredFixture is what testPolicy freezes as the request's schema
// presence; newService sets it from the model it fakes, the way FreezeCall reads
// the catalog, so fixtures built after newService describe the same request.
var structuredFixture = true

func newService(t *testing.T, raw string, structured bool) (*ai.Service, *fakeModels) {
	t.Helper()
	structuredFixture = structured
	t.Cleanup(func() { structuredFixture = true })
	f := &fakeModels{info: llm.ModelInfo{Vision: true, VideoInput: true, VideoDelivery: llm.VideoDelivery{InlineStaticVideo: true}, StructuredOutput: structured, Stages: []string{llm.StageNameObserve, llm.StageNameWrite}}, response: llm.Response{Text: raw, Usage: llm.Usage{CompletionTokens: 100, PromptTokens: 200, CostReported: true, CostMicrousd: 10}}}
	s, err := ai.New(f, ai.DefaultConfig(clip.Environment{}))
	if err != nil {
		t.Fatal(err)
	}
	return s, f
}
func source() clip.AnalysisSource {
	return clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "frozen-fingerprint", Info: clip.MediaInfo{DurationMS: 65000, Width: 1080, Height: 1920, HasAudio: true}}, Filename: "제주 & Seoul.mp4"}
}
func chunk() clip.ChunkInput {
	return clip.ChunkInput{Source: source(), Index: 1, OffsetMS: 60000, DurationMS: 5000, Policy: testPolicy("observe"), Video: llm.InlineVideo{MIME: "video/mp4", Size: 3, DurationMS: 5000, Sampling: llm.VideoSamplingFixed, Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("mp4")), nil }}}
}
func testRef() llm.ModelRef {
	return llm.ModelRef{ProviderID: "openrouter", ModelID: "explicit-observer"}
}
func testPolicy(stage string) llm.CallPolicy {
	// The frozen writer allowance is the one FreezeExecution actually grants a
	// text-only write stage; leaving it at zero would measure the writer against
	// the observer's much smaller inline budget.
	delivery, budget, input := llm.ExecutionInlineStatic, 8192, 0
	if stage == "write" {
		delivery, budget, input = llm.ExecutionTextOnly, 32768, llm.ClipPlanInputUnits
	}
	return llm.CallPolicy{Ref: testRef(), Stage: stage, CompletionTokens: budget, Reasoning: llm.ReasoningLow, InputTokens: input, StructuredOutput: structuredFixture, InputUSDPerMillion: "1", OutputUSDPerMillion: "2", Pricing: llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: delivery, Endpoint: "leaf", RequiredParameters: "max_tokens,reasoning,response_format,structured_outputs", PromptUSDPerMillion: "1", CompletionUSDPerMillion: "2", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0"}}
}
func observation() map[string]any {
	return map[string]any{"source_id": "source", "chunk_index": 1, "segments": []any{map[string]any{"start_ms": 0, "end_ms": 5000, "event": "음식을 담는다", "action": "담는다", "motion": "static", "subjects": []string{"접시"}, "speech": "", "quality": "steady and sharp", "focal": map[string]any{"x": .5, "y": .5}, "scene": "food", "readable_text": false, "subject": map[string]any{"x": .2, "y": .6, "width": .6, "height": .3}, "certainty": "certain", "usability": "usable"}}}
}

// planningInput is a project with no template: the server's own empty document
// frozen with nothing declared, over one observed source (CLIP-5).
func planningInput() clip.PlanningInput {
	composed := clip.NoTemplateComposition()
	return clip.PlanningInput{Policy: testPolicy("write"), Composition: &composed, Ratio: "vertical", TargetDurationMS: 15000, Analyses: []clip.SourceAnalysis{{Source: source(), Segments: []clip.Segment{{StartMS: 0, EndMS: 65000, Event: "음식을 담는다", Subjects: []string{"접시"}, Speech: "", Quality: "steady", Focal: clip.Point{X: .5, Y: .5}, Subject: clip.Region{X: .2, Y: .6, Width: .6, Height: .3}, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}}}}}
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

// flow is the flow writer's answer over planningInput's one source: three cuts
// of five seconds, each citing the one observation it lies in.
func flow() map[string]any {
	cut := func(id string, start, end int) map[string]any {
		return map[string]any{"id": id, "source_id": "source", "start_ms": start, "end_ms": end, "rate_permille": 1000,
			"focal": map[string]any{"x": .5, "y": .5}, "volume": 1, "observation_refs": []string{clip.ObservationID("source", 0)}}
	}
	return map[string]any{"ratio": "vertical", "duration_ms": 15000, "cuts": []any{
		cut("cut-one", 0, 5000), cut("cut-two", 5000, 10000), cut("cut-three", 10000, 15000),
	}}
}

func TestObservationContractPlainFallbackOffsetAndSpeech(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, speech := range []string{"", "오늘은 제주입니다. Today in Jeju."} {
			value := observation()
			firstSegment(value)["speech"] = speech
			s, f := newService(t, "```json\n"+raw(value)+"\n```", structured)
			in := chunk()
			in.Source.Info.HasAudio = speech != ""
			ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "explicit-observer"}
			got, usage, err := s.ObserveChunk(t.Context(), ref, in)
			if err != nil || got.Segments[0].StartMS != 60000 || got.Segments[0].EndMS != 65000 || got.Segments[0].Speech != speech || usage != f.response.Usage {
				t.Fatalf("%+v %+v %v", got, usage, err)
			}
			request := f.calls[0]
			if len(f.calls) != 1 || f.refs[0] != ref || request.Stage != llm.StageNameObserve || request.MaxTokens != s.Budgets().Observe || request.Reasoning != llm.ReasoningLow || (len(request.JSONSchema) > 0) != structured || !strings.Contains(request.System, `"maxLength":2000`) {
				t.Fatalf("bad request %+v", request)
			}
			parts := request.Messages[0].Parts
			if len(parts) != 2 || parts[0].VideoURL != "" || parts[0].InlineVideo == nil || parts[0].InlineVideo.MIME != "video/mp4" || !request.Execution.Matches(ref, request) || request.Execution.Call != in.Policy || !strings.Contains(parts[1].Text, "60000") || !strings.Contains(parts[1].Text, in.Source.Filename) {
				t.Fatal(parts)
			}
			if strings.Contains(raw(got), "signature") {
				t.Fatal("signed URL retained")
			}
		}
	}
}
func TestObservationRejectsInvalidModelOutput(t *testing.T) {
	for _, mode := range []string{"source", "index", "extra", "missing", "null", "no description", "outside", "backwards", "zero", "fractional", "overlap", "focal", "subject", "scene", "readable", "empty", "too many", "before start", "past chunk end", "missing start", "missing end", "internal gap", "unknown certainty", "unknown usability", "missing status"} {
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
				seg["event"], seg["action"], seg["motion"] = " ", "", ""
				seg["subjects"] = []string{}
			case "before start":
				seg["start_ms"] = -200
			case "past chunk end":
				seg["end_ms"] = 6000
			case "missing start":
				seg["start_ms"] = 200
			case "missing end":
				seg["end_ms"] = 4000
			case "internal gap":
				seg["end_ms"] = 2000
				second := observation()["segments"].([]any)[0].(map[string]any)
				second["start_ms"], second["end_ms"] = 2001, 5000
				v["segments"] = []any{seg, second}
			case "unknown certainty":
				seg["certainty"] = "probably"
			case "unknown usability":
				seg["usability"] = "meh"
			case "missing status":
				delete(seg, "certainty")
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
			case "subject":
				seg["subject"].(map[string]any)["width"] = 1
			case "scene":
				seg["scene"] = "bathroom"
			case "readable":
				seg["readable_text"] = "yes"
			case "empty":
				v["segments"] = []any{}
			case "too many":
				segments := make([]any, 61)
				for i := range segments {
					segments[i] = seg
				}
				v["segments"] = segments
			}
			s, f := newService(t, raw(v), true)
			if _, _, err := s.ObserveChunk(t.Context(), testRef(), chunk()); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatal(err)
			}
			if len(f.calls) != 1 {
				t.Fatal("unplanned repair call")
			}
		})
	}
}

func TestRecordedLiveObservationResponse(t *testing.T) {
	observationJSON, err := os.ReadFile("testdata/live-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	s, models := newService(t, string(observationJSON), true)
	in := chunk()
	in.Source = clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "synthetic", Fingerprint: "synthetic", Info: clip.MediaInfo{DurationMS: 15000, Width: 320, Height: 180, HasAudio: true}}, Filename: "synthetic.mp4"}
	in.Index, in.OffsetMS, in.DurationMS = 0, 0, 15000
	in.Video.DurationMS = 15000
	observed, _, err := s.ObserveChunk(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	limits := ai.DefaultConfig(clip.Environment{}).Analysis
	if _, err := clip.MergeAnalyses(limits, []clip.AnalysisSource{in.Source}, []clip.ChunkAnalysis{observed}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || models.calls[0].MaxTokens != 8192 || string(models.calls[0].JSONSchema) != string(ai.ChunkSchema()) {
		t.Fatal("production budget, call count or structural schema changed")
	}
}

func TestStructuralOutputStillEnforcesDomainBounds(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, field := range []string{"event", "speech", "quality", "subjects"} {
			v := observation()
			segment := firstSegment(v)
			segment[field] = strings.Repeat("한", 2001)
			if field == "subjects" {
				subjects := make([]string, 21)
				for i := range subjects {
					subjects[i] = "valid subject"
				}
				segment[field] = subjects
			}
			s, f := newService(t, raw(v), structured)
			if _, _, err := s.ObserveChunk(t.Context(), testRef(), chunk()); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("accepted out-of-bounds %s with structured=%v: %v", field, structured, err)
			}
			if len(f.calls) != 1 {
				t.Fatal("paid repair attempted")
			}
		}
	}
}

func TestFlowRepairsReadableBoundariesAndRefusesDecodeBreaks(t *testing.T) {
	for _, mode := range []string{"unknown source", "empty cuts", "duplicate id", "empty id", "long id", "negative start", "outside source", "backwards", "short cut", "wrong ratio", "free field", "music", "gain over", "gain under", "gain null", "fractional"} {
		t.Run(mode, func(t *testing.T) {
			v := flow()
			cut := firstCut(v)
			switch mode {
			case "unknown source":
				cut["source_id"] = "invented"
			case "empty cuts":
				v["cuts"] = []any{}
			case "duplicate id":
				v["cuts"] = []any{cut, cut}
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
			case "wrong ratio":
				v["ratio"] = "square"
			case "free field":
				cut["x"] = 123
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
			s, f := newService(t, raw(v), false)
			delivered, _, err := s.Flow(t.Context(), testRef(), planningInput())
			readable := slices.Contains([]string{"unknown source", "duplicate id", "empty id", "long id", "negative start", "outside source", "backwards", "short cut", "wrong ratio", "gain over", "gain under"}, mode)
			if readable {
				if err != nil || len(f.calls) != 1 {
					t.Fatalf("readable flow failed: %v", err)
				}
				assertExecutableTimeline(t, planningInput(), delivered)
				if mode != "short cut" && len(delivered.Notices) == 0 {
					t.Fatal("repair/removal lost its notice")
				}
				return
			}
			if !errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("unreadable flow accepted: %v", err)
			}
			if len(f.calls) != 1 {
				t.Fatal("an unreadable flow was retried")
			}
		})
	}
}
func TestFailuresKeepStageUsageAndTruncationWithoutFallback(t *testing.T) {
	for _, stage := range []string{"analyze", "flow"} {
		for _, cause := range []error{llm.ErrRateLimited, llm.ErrProviderDisabled, llm.ErrUnsupported, nil} {
			s, f := newService(t, "{\"partial\":", true)
			f.err = cause
			f.response.FinishReason = "length"
			f.response.Usage.ReasoningTokens = 90
			var usage llm.Usage
			var err error
			if stage == "analyze" {
				_, usage, err = s.ObserveChunk(t.Context(), testRef(), chunk())
			} else {
				_, usage, err = s.Flow(t.Context(), testRef(), planningInput())
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
	for _, mode := range []string{"no video", "no vision", "schema capability lost", "wrong purpose", "disabled", "bad chunk", "bad inline", "missing policy", "missing required value", "no composition", "duplicate source"} {
		t.Run(mode, func(t *testing.T) {
			s, f := newService(t, raw(observation()), true)
			in := chunk()
			planIn := planningInput()
			switch mode {
			case "no video":
				f.info.VideoInput = false
			case "no vision":
				f.info.Vision = false
			case "schema capability lost":
				// The frozen policy says a schema goes; the model no longer takes one.
				f.info.StructuredOutput = false
			case "missing policy":
				in.Policy = llm.CallPolicy{}
			case "wrong purpose":
				f.info.Stages = []string{"video-generation"}
			case "disabled":
				f.info.Disabled = true
			case "bad chunk":
				in.OffsetMS++
			case "bad inline":
				in.Video.Open = nil
			case "missing required value":
				planIn = flowInput()
				delete(planIn.Composition.Inputs.Values, "place")
			case "no composition":
				// Every generation freezes one (CLIP-5); a payload without one is
				// refused rather than written some other way.
				planIn.Composition = nil
			case "duplicate source":
				planIn.Analyses = append(planIn.Analyses, planIn.Analyses[0])
			}
			var err error
			if mode == "missing required value" || mode == "no composition" || mode == "duplicate source" {
				_, _, err = s.Flow(t.Context(), testRef(), planIn)
			} else {
				_, _, err = s.ObserveChunk(t.Context(), testRef(), in)
			}
			if err == nil || len(f.calls) != 0 {
				t.Fatal("invalid admission called provider")
			}
			// The raw modality is the clip context's one admission answer here; a
			// delivery-profile flag is nobody's gate any more (CLIP-30).
			if mode == "no video" && !errors.Is(err, llm.ErrVideoInputAbsent) {
				t.Fatalf("no video = %v", err)
			}
		})
	}
	s, f := newService(t, raw(observation()), true)
	f.info.VideoDelivery.InlineStaticVideo = false
	if _, _, err := s.ObserveChunk(t.Context(), testRef(), chunk()); err != nil || len(f.calls) != 1 {
		t.Fatalf("a model outside the static-processing profile was refused by flag: %v", err)
	}
}

func TestPreparationChecksKnownPromptSizeBeforeAnyPaidWork(t *testing.T) {
	s, f := newService(t, "", true)
	in := planningInput()
	if err := s.ValidatePreparation(testRef(), in, []clip.AnalysisSource{source()}); err != nil {
		t.Fatal(err)
	}
	// The largest authored input the template limits allow still fits the
	// writer's own frozen allowance, so the oversized case is measured against
	// the smaller allowance a policy may be frozen with. The check under test is
	// the same one either way: it runs before any provider call.
	in.Policy.InputTokens = llm.ClipInputUnits
	var body strings.Builder
	body.WriteString(`<clip version="1">`)
	values := map[string]string{}
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("field%d", i)
		fmt.Fprintf(&body, `<field id="%s" label="%d">%s</field>`, id, i, strings.Repeat("나", 200))
		values[id] = strings.Repeat("다", 500)
	}
	body.WriteString(`<text id="big" kind="ai" role="caption">` + strings.Repeat("가", 4000) + "</text></clip>")
	in.Template = clip.Recipe{Name: "가장 큰 템플릿", CompositionBody: body.String()}
	in.Composition = &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: clip.CompositionVersion, Body: body.String(), TemplateID: "largest"},
		Inputs: clip.CompositionInputs{Values: values, Items: map[string][]composition.Item{}}}
	if err := s.ValidatePreparation(testRef(), in, []clip.AnalysisSource{source()}); !errors.Is(err, clip.ErrInputTooLarge) {
		t.Fatal("oversized known context accepted", err)
	}
	if len(f.calls) != 0 {
		t.Fatal("preparation called provider")
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
	for _, schema := range []func() []byte{ai.ChunkSchema, ai.FlowSchema, ai.NarrationSchema} {
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

func TestStrictFieldsSilentSpeechAndCancellation(t *testing.T) {
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
		s, _ := newService(t, raw(v), false)
		in.Policy = testPolicy("observe")
		if _, _, err := s.ObserveChunk(t.Context(), testRef(), in); !errors.Is(err, llm.ErrBadOutput) {
			t.Fatalf("%s: %v", mode, err)
		}
	}
	s, f := newService(t, raw(flow()), true)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := s.Flow(ctx, testRef(), planningInput()); !errors.Is(err, context.Canceled) || len(f.calls) != 0 {
		t.Fatal(err)
	}
	if _, _, err := s.ObserveChunk(ctx, testRef(), chunk()); !errors.Is(err, context.Canceled) || len(f.calls) != 0 {
		t.Fatal(err)
	}
}

func TestCutBoundsRejectIntegerWraparoundBeforeRendering(t *testing.T) {
	v := flow()
	c := firstCut(v)
	c["start_ms"] = math.MaxInt - 10000
	c["end_ms"] = math.MinInt + 4999
	s, f := newService(t, raw(v), true)
	delivered, _, err := s.Flow(t.Context(), testRef(), planningInput())
	if err != nil || len(f.calls) != 1 || !hasNotice(delivered, "plan_cut_range") {
		t.Fatalf("overflowed cut was not removed: %v", err)
	}
	for _, cut := range delivered.Cuts {
		if cut.ID == c["id"] {
			t.Fatal("overflowed source range survived")
		}
	}
	assertExecutableTimeline(t, planningInput(), delivered)
}

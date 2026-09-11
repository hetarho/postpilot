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
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/clip/design"
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
	err       error
	fixedErr  error
	layoutErr error
	cardErr   error
	calls     int
	fixed     int
	cards     int
	layouts   int
	// The cards the composer is told to keep copy off, in the shape the real
	// renderer returns them: the hook card over the first cut and the ending
	// card over the last.
	cardPlan clip.EditPlan
}

// The composer verifies its own result through this port (CDS-52).
func (f *fakeSizer) Layout(context.Context, clip.EditPlan, []clip.RenderSource) (clip.Manifest, error) {
	f.layouts++
	return nil, f.layoutErr
}

func (f *fakeSizer) CaptionSize(context.Context, string, clip.Caption) (float64, float64, error) {
	f.calls++
	return 500, 100, f.err
}

// The badge and the chips the composer must keep copy off. The fixture puts the
// badge where 9:16 puts it (CDS-31) and no chips, so the anchor walk is driven
// by the subject box alone.
func (f *fakeSizer) FixedElements(_ context.Context, _, disclosure string, labels []string, _ []clip.Answer) (clip.Manifest, error) {
	f.fixed++
	if f.fixedErr != nil {
		return nil, f.fixedErr
	}
	out := clip.Manifest{{Kind: "badge", Region: design.Region{X: 768, Y: 270, Width: 120, Height: 60}}}
	for i := range labels {
		out = append(out, design.Element{Kind: "chip", Region: design.Region{X: 96, Y: 290 + float64(i)*76, Width: 300, Height: 60}})
	}
	return out, nil
}

// The two cards, measured at their ratio's own geometry (CDS-28, CDS-29): a
// 400 px stack centred on the ratio's card line. They cover the middle of the
// frame, which is the band a mid-frame anchor wants, so a copy on the first or
// the last cut has to move.
func (f *fakeSizer) CardElements(_ context.Context, plan clip.EditPlan) (clip.Manifest, error) {
	f.cards++
	f.cardPlan = plan
	if f.cardErr != nil {
		return nil, f.cardErr
	}
	layout, ok := design.Layout(plan.Ratio)
	if !ok || len(plan.Cuts) == 0 || plan.Hook == "" {
		return nil, nil
	}
	const height = 400
	box := func(width, centre float64) design.Region {
		return design.Region{X: (float64(layout.Canvas.Width) - width) / 2, Y: centre - height/2, Width: width, Height: height}
	}
	return clip.Manifest{
		{Cut: 0, Kind: "card", Region: box(layout.HookCard.Width, layout.HookCard.CenterY)},
		{Cut: len(plan.Cuts) - 1, Kind: "card", Region: box(layout.EndCard.Width, layout.EndCard.CenterY)},
	}, nil
}

// structuredFixture is what testPolicy freezes as the request's schema
// presence; newService sets it from the model it fakes, the way FreezeCall reads
// the catalog, so fixtures built after newService describe the same request.
var structuredFixture = true

func newService(t *testing.T, raw string, structured bool) (*ai.Service, *fakeModels, *fakeSizer) {
	t.Helper()
	structuredFixture = structured
	t.Cleanup(func() { structuredFixture = true })
	f := &fakeModels{info: llm.ModelInfo{Vision: true, VideoInput: true, VideoDelivery: llm.VideoDelivery{InlineStaticVideo: true}, StructuredOutput: structured, Stages: []string{llm.StageNameObserve, llm.StageNameWrite}}, response: llm.Response{Text: raw, Usage: llm.Usage{CompletionTokens: 100, PromptTokens: 200, CostReported: true, CostMicrousd: 10}}}
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
	return clip.ChunkInput{Source: source(), Index: 1, OffsetMS: 60000, DurationMS: 5000, Policy: testPolicy("observe"), Video: llm.InlineVideo{MIME: "video/mp4", Size: 3, DurationMS: 5000, Sampling: llm.VideoSamplingFixed, Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("mp4")), nil }}}
}
func testRef() llm.ModelRef {
	return llm.ModelRef{ProviderID: "openrouter", ModelID: "explicit-observer"}
}
func testPolicy(stage string) llm.CallPolicy {
	delivery, budget := llm.ExecutionInlineStatic, 8192
	if stage == "write" {
		delivery, budget = llm.ExecutionTextOnly, 32768
	}
	return llm.CallPolicy{Ref: testRef(), Stage: stage, CompletionTokens: budget, Reasoning: llm.ReasoningLow, StructuredOutput: structuredFixture, InputUSDPerMillion: "1", OutputUSDPerMillion: "2", Pricing: llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: delivery, Endpoint: "leaf", RequiredParameters: "max_tokens,reasoning,response_format,structured_outputs", PromptUSDPerMillion: "1", CompletionUSDPerMillion: "2", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0"}}
}
func observation() map[string]any {
	return map[string]any{"source_id": "source", "chunk_index": 1, "segments": []any{map[string]any{"start_ms": 0, "end_ms": 5000, "event": "음식을 담는다", "subjects": []string{"접시"}, "speech": "", "quality": "steady and sharp", "focal": map[string]any{"x": .5, "y": .5}, "scene": "food", "readable_text": false, "subject": map[string]any{"x": .2, "y": .6, "width": .6, "height": .3}}}}
}
func planningInput() clip.PlanningInput {
	return clip.PlanningInput{Policy: testPolicy("write"), Template: clip.Recipe{Name: "제주 & Seoul", InformationFields: []clip.InformationField{{Label: "장소 / Place", Prompt: "어디인가요?"}}, CutGuidance: "현장 소리를 남겨줘. Keep the original sound.", CopyStyles: []string{"clean", "memo"}, Accent: "coral"}, Answers: []clip.Answer{{Label: "장소 / Place", Text: "한글 <그대로> & O'Brien\nKeep 10:30 unchanged."}}, Ratio: "vertical", TargetDurationMS: 15000, Analyses: []clip.SourceAnalysis{{Source: source(), Segments: []clip.Segment{{StartMS: 0, EndMS: 65000, Event: "음식을 담는다", Subjects: []string{"접시"}, Speech: "", Quality: "steady", Focal: clip.Point{X: .5, Y: .5}, Subject: clip.Region{X: .2, Y: .6, Width: .6, Height: .3}}}}}}
}
func plan() map[string]any {
	// Words only: the model no longer names a position, a style or an accent.
	// Three cuts, because CDS-37 holds every cut to 6.0 s while a clip is at
	// least 15 s (CLIP-19): one long take is not a clip any more.
	cut := func(id string, start, end int, text string) map[string]any {
		return map[string]any{"id": id, "source_id": "source", "start_ms": start, "end_ms": end, "focal": map[string]any{"x": .5, "y": .5}, "chips": []string{},
			"caption": map[string]any{"text": text, "start_ms": 1000, "end_ms": end - start - 1000, "short_text": "한글 여행", "keyword": ""}}
	}
	return map[string]any{"ratio": "vertical", "duration_ms": 15000, "hook": "정확한 여행", "cuts": []any{
		cut("cut-one", 0, 5000, "정확한 한글 & 여행"),
		cut("cut-two", 5000, 10000, "조용한 한글 & 여행"),
		cut("cut-three", 10000, 15000, "천천히 걷는 골목"),
	}}
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
			if len(f.calls) != 1 || f.refs[0] != ref || request.Stage != llm.StageNameObserve || request.MaxTokens != s.Budgets().Observe || request.Reasoning != llm.ReasoningLow || (len(request.JSONSchema) > 0) != structured || !strings.Contains(request.System, `"maxLength": 2000`) {
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
	for _, mode := range []string{"source", "index", "extra", "missing", "null", "no description", "outside", "backwards", "zero", "fractional", "overlap", "focal", "subject", "scene", "readable", "empty", "too many"} {
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
			s, f, _ := newService(t, raw(v), true)
			if _, _, err := s.ObserveChunk(t.Context(), testRef(), chunk()); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatal(err)
			}
			if len(f.calls) != 1 {
				t.Fatal("unplanned repair call")
			}
		})
	}
}

func TestRecordedLiveClipResponses(t *testing.T) {
	observationJSON, err := os.ReadFile("testdata/live-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	planJSON, err := os.ReadFile("testdata/live-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	s, models, _ := newService(t, string(observationJSON), true)
	in := chunk()
	in.Source = clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "synthetic", Fingerprint: "synthetic", Info: clip.MediaInfo{DurationMS: 15000, Width: 320, Height: 180, HasAudio: true}}, Filename: "synthetic.mp4"}
	in.Index, in.OffsetMS, in.DurationMS = 0, 0, 15000
	in.Video.DurationMS = 15000
	observed, _, err := s.ObserveChunk(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	limits := config.ClipAI(&config.Config{}).Analysis
	analyses, err := clip.MergeAnalyses(limits, []clip.AnalysisSource{in.Source}, []clip.ChunkAnalysis{observed})
	if err != nil {
		t.Fatal(err)
	}
	models.response.Text = string(planJSON)
	input := clip.PlanningInput{Policy: testPolicy("write"), Template: clip.Recipe{Name: "합성 영상 검증", CutGuidance: "15초 한 컷으로 구성하고 자막은 '영상 생성 확인'으로 해주세요.", CopyStyles: []string{"clean"}, Accent: "coral"}, Ratio: "horizontal", TargetDurationMS: 15000, Analyses: analyses}
	// The recorded response is ONE fifteen-second take, which is exactly what
	// CDS-37 now refuses: no cut may run past 6.0 s, and a single cut cannot then
	// reach the fifteen-second floor. The evidence is kept as it was recorded and
	// the refusal is what it proves.
	var diagnostic interface{ OutputValidationCode() string }
	if _, _, err = s.Plan(t.Context(), testRef(), input); !errors.As(err, &diagnostic) || diagnostic.OutputValidationCode() != "plan_timeline" {
		t.Fatalf("a single-take plan is no longer executable under CDS-37: %v", err)
	}
	// The same words over three cuts — what the design system does admit — still
	// compile, and nothing but the cut count changed.
	var recorded map[string]any
	if err = json.Unmarshal(planJSON, &recorded); err != nil {
		t.Fatal(err)
	}
	recorded["cuts"] = splitRecordedCut(recorded["cuts"].([]any)[0].(map[string]any), 3, 5000)
	models.response.Text = raw(recorded)
	result, _, err := s.Plan(t.Context(), testRef(), input)
	if err != nil {
		t.Fatal(err)
	}
	cut := result.Cuts[0]
	start, end := cut.CaptionWindow(0)
	if result.DurationMS != 15000 || len(result.Cuts) != 3 || cut.FirstCopy().Text != "영상 생성 확인" || start != 120 || end != 4880 {
		t.Fatalf("unexpected plan: %+v", result)
	}
	// One scene throughout, so CDS-36 joins every boundary with a hard cut.
	if result.TransitionTotal() != 0 {
		t.Fatalf("invented a transition inside one scene: %+v", result.Cuts)
	}
	// The recorded response named no style or position; the design system chose
	// both from the scene and the sentence (CDS-39, CDS-40).
	if cut.FirstCopy().Style != "clean" || cut.FirstCopy().Anchor != "bottom" || cut.FirstCopy().Align != "center" || result.Decisions[0].Class != "FACT" {
		t.Fatalf("placement was not the design system's: %+v %+v", cut.FirstCopy(), result.Decisions)
	}
	if len(models.calls) != 3 || models.calls[0].MaxTokens != 8192 || models.calls[1].MaxTokens != 32768 {
		t.Fatal("production budgets or call count changed")
	}
	if string(models.calls[0].JSONSchema) != string(ai.ChunkSchema()) || string(models.calls[1].JSONSchema) != string(ai.PlanSchema()) {
		t.Fatal("did not send structural output schemas")
	}
	if models.calls[1].HasVideos() || models.calls[1].HasImages() {
		t.Fatal("composition received pixels")
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
			s, f, _ := newService(t, raw(v), structured)
			if _, _, err := s.ObserveChunk(t.Context(), testRef(), chunk()); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("accepted out-of-bounds %s with structured=%v: %v", field, structured, err)
			}
			if len(f.calls) != 1 {
				t.Fatal("paid repair attempted")
			}
		}
		v := plan()
		firstCut(v)["caption"].(map[string]any)["text"] = strings.Repeat("한", 501)
		s, f, c := newService(t, raw(v), structured)
		if _, _, err := s.Plan(t.Context(), testRef(), planningInput()); !errors.Is(err, clip.ErrCopyTooLong) {
			t.Fatalf("accepted oversized caption: %v", err)
		}
		if len(f.calls) != 1 || c.calls != 0 {
			t.Fatal("invalid result reached repair or rendering")
		}
	}
}

func TestPlanIsGroundedMeasuredAndPreservesExactAnswers(t *testing.T) {
	for _, structured := range []bool{false, true} {
		s, f, c := newService(t, "Here is the JSON:\n"+raw(plan()), structured)
		in := planningInput()
		got, usage, err := s.Plan(t.Context(), testRef(), in)
		if err != nil || usage != f.response.Usage || len(got.Cuts) != 3 {
			t.Fatalf("%+v %v", got, err)
		}
		cut := got.Cuts[0]
		// Nothing here was chosen by the model: a noun-led sentence on a food
		// close-up is 메모 by CDS-40, and 메모's own first candidate is TOP/LEFT
		// (CDS-24) — which clears the subject box at the bottom of the frame.
		if cut.FirstCopy().Style != "memo" || cut.FirstCopy().Align != "left" || cut.FirstCopy().Anchor != "top" || cut.Fingerprint != in.Analyses[0].Source.Fingerprint || cut.Volume == nil || *cut.Volume != 1 {
			t.Fatalf("%+v", cut)
		}
		// 메모 has two candidate anchors (CDS-24), so the selector measures the
		// plate at both and at neither more, and the composition is verified
		// exactly once before the plan is returned (CDS-52).
		if c.calls != 2*len(got.Cuts) || c.fixed != len(got.Cuts) || c.layouts != 1 {
			t.Fatalf("%d measurements, %d fixed-element reads, %d verifications", c.calls, c.fixed, c.layouts)
		}
		// The window is CDS-27's, not the model's: cut start + 120 ms to cut end
		// − 120 ms, which a zero start and end resolve to.
		if start, end := cut.CaptionWindow(0); start != 120 || end != 4880 {
			t.Fatalf("caption window %d..%d", start, end)
		}
		request := f.calls[0]
		if len(f.calls) != 1 || request.HasVideos() || request.HasImages() || request.MaxTokens != s.Budgets().Plan || request.Stage != llm.StageNameWrite || (request.JSONSchema != nil) != structured || !strings.Contains(request.System, `"maxLength": 500`) {
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
		// With no subject box 메모 still takes its own default anchor: the table
		// decides, and the box only ever moves it off a subject (CDS-38).
		in.Analyses[0].Segments[0].Subject = clip.Region{}
		got, _, err = s.Plan(t.Context(), testRef(), in)
		if err != nil || got.Cuts[0].FirstCopy().Anchor != "top" || got.Cuts[0].FirstCopy().Align != "left" {
			t.Fatalf("the default anchor moved: %+v %v", got, err)
		}
	}
}
func TestPlanRejectsEveryInvalidBoundaryWithoutRepair(t *testing.T) {
	for _, mode := range []string{"unknown source", "empty cuts", "duplicate id", "empty id", "long id", "negative start", "outside source", "backwards", "short cut", "wrong ratio", "caption negative", "caption outside cut", "caption backwards", "caption missing", "free position", "disallowed style", "disallowed accent", "free coordinates", "music", "gain over", "gain under", "gain null", "fractional"} {
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
			case "wrong ratio":
				v["ratio"] = "square"
			case "caption negative":
				caption["start_ms"] = -1
			case "caption outside cut":
				caption["start_ms"] = 15000
				caption["end_ms"] = 16000
			case "caption backwards":
				caption["start_ms"] = 14000
			case "caption missing":
				delete(caption, "start_ms")
			case "free position":
				caption["position"] = "anywhere"
			case "disallowed style":
				caption["style"] = "bold"
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
			if _, _, err := s.Plan(t.Context(), testRef(), planningInput()); !errors.Is(err, llm.ErrBadOutput) {
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
				_, usage, err = s.ObserveChunk(t.Context(), testRef(), chunk())
			} else {
				_, usage, err = s.Plan(t.Context(), testRef(), planningInput())
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
	for _, mode := range []string{"no video", "no vision", "schema capability lost", "wrong purpose", "disabled", "bad chunk", "bad inline", "missing policy", "missing answer", "duplicate source"} {
		t.Run(mode, func(t *testing.T) {
			s, f, _ := newService(t, raw(observation()), true)
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
			case "missing answer":
				planIn.Answers = nil
			case "duplicate source":
				planIn.Analyses = append(planIn.Analyses, planIn.Analyses[0])
			}
			var err error
			if mode == "missing answer" || mode == "duplicate source" {
				_, _, err = s.Plan(t.Context(), testRef(), planIn)
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
	s, f, _ := newService(t, raw(observation()), true)
	f.info.VideoDelivery.InlineStaticVideo = false
	if _, _, err := s.ObserveChunk(t.Context(), testRef(), chunk()); err != nil || len(f.calls) != 1 {
		t.Fatalf("a model outside the static-processing profile was refused by flag: %v", err)
	}
}

func TestPreparationChecksKnownPromptSizeBeforeAnyPaidWork(t *testing.T) {
	s, f, _ := newService(t, "", true)
	in := planningInput()
	if err := s.ValidatePreparation(testRef(), in, []clip.AnalysisSource{source()}); err != nil {
		t.Fatal(err)
	}
	in.Template.CutGuidance = strings.Repeat("가", 4000)
	in.Template.InformationFields = nil
	in.Answers = nil
	for i := 0; i < 10; i++ {
		label := fmt.Sprint(i)
		in.Template.InformationFields = append(in.Template.InformationFields, clip.InformationField{Label: label, Prompt: strings.Repeat("나", 200)})
		in.Answers = append(in.Answers, clip.Answer{Label: label, Text: strings.Repeat("다", 500)})
	}
	if err := s.ValidatePreparation(testRef(), in, []clip.AnalysisSource{source()}); !errors.Is(err, clip.ErrInvalid) {
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
	// Ninety seconds of 1.8 s cuts: CDS-37's 1.2 s floor puts a ceiling on how
	// many cuts ninety seconds can hold, so this is fifty, not a hundred.
	cuts := make([]any, 50)
	for i := range cuts {
		c := firstCut(plan())
		c["id"] = fmt.Sprintf("cut-%d", i)
		c["source_id"] = fmt.Sprintf("source-%d", i%20)
		c["end_ms"] = 1800
		p := c["caption"].(map[string]any)
		// A 1.8 s cut pays for two characters of exposure and little more
		// (CDS-41); this fixture is about the cut count, not about copy.
		p["text"] = "여행"
		p["start_ms"] = 0
		p["end_ms"] = 1800
		if i == 0 {
			c["volume"] = 0
		}
		cuts[i] = c
	}
	v["cuts"] = cuts
	s, f, _ := newService(t, raw(v), false)
	in.Policy = testPolicy("write")
	got, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil || got.DurationMS != 90000 || len(got.Cuts) != 50 || got.Cuts[0].OriginalVolume() != 0 || got.Cuts[1].OriginalVolume() != 1 || len(f.calls) != 1 || f.calls[0].MaxTokens != 32768 {
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
		in.Policy = testPolicy("observe")
		if _, _, err := s.ObserveChunk(t.Context(), testRef(), in); !errors.Is(err, llm.ErrBadOutput) {
			t.Fatalf("%s: %v", mode, err)
		}
	}
	s, f, c := newService(t, raw(plan()), true)
	c.err = clip.ErrCopyTooLong
	if _, usage, err := s.Plan(t.Context(), testRef(), planningInput()); !errors.Is(err, clip.ErrCopyTooLong) || usage != f.response.Usage || len(f.calls) != 1 {
		t.Fatalf("%+v %v", usage, err)
	}
	// A verifier failure on the composer's own result is a composition failure
	// with the check named, and no paid call is retried.
	s, f, c = newService(t, raw(plan()), true)
	c.layoutErr = clip.ErrInvalid
	if _, _, err := s.Plan(t.Context(), testRef(), planningInput()); !errors.Is(err, clip.ErrInvalid) || len(f.calls) != 1 {
		t.Fatalf("composition verification: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	f.calls = nil
	if _, _, err := s.Plan(ctx, testRef(), planningInput()); !errors.Is(err, context.Canceled) || len(f.calls) != 0 {
		t.Fatal(err)
	}
	if _, _, err := s.ObserveChunk(ctx, testRef(), chunk()); !errors.Is(err, context.Canceled) || len(f.calls) != 0 {
		t.Fatal(err)
	}
}

func TestCutBoundsRejectIntegerWraparoundBeforeRendering(t *testing.T) {
	v := plan()
	c := firstCut(v)
	c["start_ms"] = math.MaxInt - 10000
	c["end_ms"] = math.MinInt + 4999
	s, f, measure := newService(t, raw(v), true)
	if _, _, err := s.Plan(t.Context(), testRef(), planningInput()); !errors.Is(err, llm.ErrBadOutput) || len(f.calls) != 1 || measure.calls != 0 {
		t.Fatalf("overflowed source range accepted: %v", err)
	}
}

// splitRecordedCut cuts one recorded take into n consecutive cuts of the same
// source, keeping its caption, focal point and gain: the words are the model's,
// only the cut count is the design system's (CDS-37).
func splitRecordedCut(cut map[string]any, n, length int) []any {
	out := make([]any, 0, n)
	for i := 0; i < n; i++ {
		c := map[string]any{}
		for k, v := range cut {
			c[k] = v
		}
		c["id"] = fmt.Sprintf("%v-%d", cut["id"], i)
		c["start_ms"], c["end_ms"] = i*length, (i+1)*length
		caption := map[string]any{}
		for k, v := range cut["caption"].(map[string]any) {
			caption[k] = v
		}
		caption["start_ms"], caption["end_ms"] = 200, length-200
		c["caption"] = caption
		out = append(out, c)
	}
	return out
}

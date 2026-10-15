package template

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

type headingVoiceStub struct {
	text    string
	present bool
	err     error
	calls   int
	user    string
}

func (v *headingVoiceStub) DefaultVoiceStyle(_ context.Context, user string) (string, bool, error) {
	v.calls++
	v.user = user
	return v.text, v.present, v.err
}

type headingJobsStub struct {
	starts []HeadingSuggestionJob
	saved  []byte
}

func (j *headingJobsStub) EnqueueHeadingSuggestions(_ context.Context, start HeadingSuggestionJob) (string, error) {
	j.starts = append(j.starts, start)
	return "headings-1", nil
}
func (j *headingJobsStub) SaveHeadingSuggestions(_ context.Context, _ string, payload []byte) error {
	j.saved = append([]byte(nil), payload...)
	return nil
}
func (j *headingJobsStub) HeadingSuggestionsPayload(context.Context, string, string) ([]byte, error) {
	return j.saved, nil
}
func headingStart() HeadingSuggestionStart {
	return HeadingSuggestionStart{WriteModel: "p/w", TargetKind: "post", TargetPath: []string{"맛집", "식사"}, Sections: []HeadingSection{{Role: "방문 계기", Description: "찾아간 이유"}, {Role: "아쉬운 점", Description: "솔직한 아쉬움"}}}
}

const headingAnswerJSON = `{"sections":[{"headings":[" 찾아간 이유 ","발걸음이 향한 곳","오늘은 여기로"]},{"headings":["살짝 아쉬웠던 건","다음엔 달라졌으면","이 점은 알아 두세요"]}]}`

func headingHarness() (*HeadingSuggestions, *fakeModels, *headingVoiceStub, *headingJobsStub) {
	models := &fakeModels{known: true, info: writerInfo(), responses: []llm.Response{{Text: headingAnswerJSON, FinishReason: "stop"}}}
	voice := &headingVoiceStub{text: "VOICE_ACCEPTED_STYLE: 짧고 차분하게 씁니다", present: true}
	jobs := &headingJobsStub{}
	return NewHeadingSuggestions(models, voice, jobs, fakeBudget{}, &fakeEstimator{}, testLimits(), 40), models, voice, jobs
}
func runHeadings(t *testing.T, s *HeadingSuggestions, j *headingJobsStub) error {
	t.Helper()
	start := j.starts[0]
	return s.Run(t.Context(), RequestRun{ID: "headings-1", UserID: start.UserID, WriteModel: start.WriteModel, Payload: start.Payload}, nil)
}

func TestSuggestionsAreThreePerSectionInTheDefaultVoice(t *testing.T) {
	s, models, voice, jobs := headingHarness()
	start := headingStart()
	if _, err := s.Start(t.Context(), "alice", start); err != nil {
		t.Fatal(err)
	}
	voice.text = "CHANGED_LATER_STYLE"
	start.Sections[0].Description = "CHANGED_LATER_ROLE"
	if err := runHeadings(t, s, jobs); err != nil {
		t.Fatal(err)
	}
	result, err := s.Result(t.Context(), "alice", "headings-1")
	if err != nil || len(result) != 2 || len(result[0]) != 3 || result[0][0] != "찾아간 이유" {
		t.Fatalf("result=%v err=%v", result, err)
	}
	if len(models.requests) != 1 || voice.calls != 1 || voice.user != "alice" || jobs.starts[0].Calls != 1 || jobs.starts[0].CompletionTokens != 8192 {
		t.Fatalf("call admission not frozen: %+v", jobs.starts)
	}
	request := models.requests[0]
	var text strings.Builder
	for _, message := range request.Messages {
		for _, part := range message.Parts {
			text.WriteString(part.Text)
		}
	}
	if !strings.Contains(text.String(), "VOICE_ACCEPTED_STYLE") || strings.Contains(text.String(), "CHANGED_LATER") || !strings.Contains(text.String(), "찾아간 이유") {
		t.Fatalf("wrong frozen material: %s", text.String())
	}
	if request.Stage != llm.StageNameWrite || request.MaxTokens != 8192 || len(request.JSONSchema) == 0 {
		t.Fatal("wrong single write contract")
	}
	if _, err := llm.PreparedRequestInspection(request); err != nil {
		t.Fatal(err)
	}
}

func TestAMissingVoiceSendsNoVoiceBytes(t *testing.T) {
	s, models, voice, jobs := headingHarness()
	voice.present = false
	voice.text = "MISSING_VOICE_PRIVATE_BYTES"
	if _, err := s.Start(t.Context(), "alice", headingStart()); err != nil {
		t.Fatal(err)
	}
	if err := runHeadings(t, s, jobs); err != nil {
		t.Fatal(err)
	}
	request := models.requests[0]
	if strings.Contains(string(jobs.starts[0].Payload), "MISSING_VOICE_PRIVATE_BYTES") || len(request.Messages[0].Parts) != 1 || request.Composition.Mode != "plain" {
		t.Fatal("absent voice material entered request")
	}
}

func TestAShortAnswerFailsWhole(t *testing.T) {
	for _, answer := range []string{`{"sections":[{"headings":["하나","둘","셋"]}]}`, `{"sections":[{"headings":["하나","둘","셋"]},{"headings":["하나","둘"]}]}`, `{"sections":[{"headings":["하나","둘","셋"]},{"headings":[" ","둘","셋"]}]}`, `{"sections":[{"headings":["하나","둘","셋"]},{"headings":["같은 말","같은 말","셋"]}]}`, `not JSON`} {
		t.Run(answer, func(t *testing.T) {
			s, models, _, jobs := headingHarness()
			models.responses[0].Text = answer
			if _, err := s.Start(t.Context(), "alice", headingStart()); err != nil {
				t.Fatal(err)
			}
			err := runHeadings(t, s, jobs)
			if !errors.Is(err, llm.ErrBadOutput) || len(jobs.saved) != 0 || len(models.requests) != 1 || llm.NormalizeFailure(err).Reason != llm.FailureReasonOutputInvalid {
				t.Fatalf("partial or retried answer: %v", err)
			}
		})
	}
}

func TestHeadingInputRefusesUnboundedOrMissingMaterialBeforeAdmission(t *testing.T) {
	for _, change := range []func(*HeadingSuggestionStart){func(s *HeadingSuggestionStart) { s.TargetKind = "unknown" }, func(s *HeadingSuggestionStart) { s.TargetPath = nil }, func(s *HeadingSuggestionStart) { s.Sections = make([]HeadingSection, HeadingSectionsMax+1) }, func(s *HeadingSuggestionStart) { s.Sections[0].Role = " " }, func(s *HeadingSuggestionStart) {
		s.Sections[0].Description = strings.Repeat("가", testLimits().DescriptionMaxChars+1)
	}} {
		s, models, voice, jobs := headingHarness()
		start := headingStart()
		change(&start)
		if _, err := s.Start(t.Context(), "alice", start); !errors.Is(err, ErrHeadingInputInvalid) || len(jobs.starts) != 0 || voice.calls != 0 || len(models.requests) != 0 {
			t.Fatalf("invalid input admitted: %v", err)
		}
	}
}

func TestHeadingVoiceReadFailureMakesNoJobOrProviderCall(t *testing.T) {
	s, models, voice, jobs := headingHarness()
	voice.err = errors.New("style read unavailable")
	if _, err := s.Start(t.Context(), "alice", headingStart()); err == nil || len(jobs.starts) != 0 || len(models.requests) != 0 {
		t.Fatal("voice read failure became an unstyled paid call")
	}
}

func TestHeadingEstimateIsAReadAndKeepsUnknownPricesUnknown(t *testing.T) {
	s, models, voice, jobs := headingHarness()
	models.info.Levels = map[string]string{llm.StageNameWrite: "free"}
	estimate, err := s.Estimate(t.Context(), "p/w")
	if err != nil || !estimate.Free || !estimate.Available || voice.calls != 0 || len(jobs.starts) != 0 || len(models.requests) != 0 {
		t.Fatalf("estimate=%+v %v", estimate, err)
	}
	models.info.Levels[llm.StageNameWrite] = "value"
	if estimate, err = s.Estimate(t.Context(), "p/w"); err != nil || estimate.Available {
		t.Fatalf("invented estimate: %+v %v", estimate, err)
	}
}

func TestHeadingOutputBoundsAndTruncationNeverPublish(t *testing.T) {
	for _, test := range []struct {
		text, finish string
		want         error
	}{{strings.Replace(headingAnswerJSON, "찾아간 이유", strings.Repeat("가", testLimits().AskLabelMaxChars+1), 1), "stop", llm.ErrBadOutput}, {"{", "length", llm.ErrOutputTruncated}, {headingAnswerJSON, "content_filter", llm.ErrBadOutput}} {
		s, models, _, jobs := headingHarness()
		models.responses[0] = llm.Response{Text: test.text, FinishReason: test.finish}
		if _, err := s.Start(t.Context(), "alice", headingStart()); err != nil {
			t.Fatal(err)
		}
		if err := runHeadings(t, s, jobs); !errors.Is(err, test.want) || len(jobs.saved) != 0 || len(models.requests) != 1 {
			t.Fatalf("published invalid output: %v", err)
		}
	}
}

func TestClipHeadingsFreezeTheCompositionLabelBound(t *testing.T) {
	for _, length := range []int{40, 41} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			s, models, _, jobs := headingHarness()
			s.limits.AskLabelMaxChars = 80 // a configurable post bound must not widen clip headings
			start := headingStart()
			start.TargetKind = "clip"
			models.responses[0].Text = strings.Replace(headingAnswerJSON, "찾아간 이유", strings.Repeat("가", length), 1)
			if _, err := s.Start(t.Context(), "alice", start); err != nil {
				t.Fatal(err)
			}
			s.clipHeadingChars = 5 // admission freezes the owner boundary before execution
			err := runHeadings(t, s, jobs)
			if length == 40 && err != nil {
				t.Fatal(err)
			}
			if length == 41 && (!errors.Is(err, llm.ErrBadOutput) || len(jobs.saved) != 0) {
				t.Fatalf("over-bound group published: %v", err)
			}
			if len(models.requests) != 1 || !strings.Contains(models.requests[0].System, "40자") || !strings.Contains(string(models.requests[0].JSONSchema), `"maxLength":40`) {
				t.Fatal("wrong frozen clip schema or an extra call")
			}
		})
	}
}

func TestHeadingV1JobsKeepTheirAdmittedOutputContract(t *testing.T) {
	s, models, _, jobs := headingHarness()
	s.limits.AskLabelMaxChars = 80
	start := headingStart()
	start.TargetKind = "clip"
	if _, err := s.Start(t.Context(), "alice", start); err != nil {
		t.Fatal(err)
	}
	var in headingInput
	if err := json.Unmarshal(jobs.starts[0].Payload, &in); err != nil {
		t.Fatal(err)
	}
	in.Version, in.MaxHeadingChars = 1, 0
	jobs.starts[0].Payload, _ = json.Marshal(in)
	models.responses[0].Text = strings.Replace(headingAnswerJSON, "찾아간 이유", strings.Repeat("가", 41), 1)
	if err := runHeadings(t, s, jobs); err != nil {
		t.Fatal(err)
	}
	if len(models.requests) != 1 || !strings.Contains(string(models.requests[0].JSONSchema), fmt.Sprintf(`"maxLength":%d`, s.limits.AskLabelMaxChars)) {
		t.Fatal("historical admitted output was reinterpreted")
	}
}

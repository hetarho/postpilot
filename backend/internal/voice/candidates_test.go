package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func candidateFixture(counts ...int) string {
	count := CandidateCount
	if len(counts) > 0 {
		count = counts[0]
	}
	var values []WritingCandidate
	for i := 1; i <= count; i++ {
		values = append(values, WritingCandidate{Name: fmt.Sprintf("편안한 말투 %d", i), Description: fmt.Sprintf("소소한 기분을 자연스럽게 전하는 말투예요 %d", i), Sample: strings.Repeat(fmt.Sprintf("산책하다 작은 가게에 들러 따뜻한 차를 마셨어요 %d. 창가에서 쉬니 마음이 편안해졌어요. ", i), 5)})
	}
	raw, _ := json.Marshal(candidateResult{Candidates: values})
	return string(raw)
}

func TestWritingStylePreparationSupportsOnlyExactBinaryCountsAndFrozenBudgets(t *testing.T) {
	for _, count := range []int{2, 4, 8, 16} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ctx := context.Background()
			models := &candidateModelFake{text: candidateFixture(count), info: llm.ModelInfo{Stages: []string{llm.StageNameWrite}, StructuredOutput: true}}
			jobs := &candidateJobsFake{}
			estimates := &candidateEstimatorFake{}
			service := NewCandidateService(models, jobs, &candidateStoreFake{}, candidateBudgetFake{}, estimates)
			shuffles := 0
			service.shuffle = func([]string) error { shuffles++; return nil }
			ref := llm.ModelRef{ProviderID: "test", ModelID: "write"}
			if _, err := service.Estimate(ctx, ref, count); err != nil {
				t.Fatal(err)
			}
			if estimates.tokens != [2]int64{int64(750 * count), int64(8192 * count / 8)} {
				t.Fatalf("estimate %v", estimates.tokens)
			}
			if _, err := service.Start(ctx, "alice", ref, count); err != nil {
				t.Fatal(err)
			}
			var input candidateInput
			if err := json.Unmarshal(jobs.request.Payload, &input); err != nil {
				t.Fatal(err)
			}
			if input.Count != count || len(input.Directions) != count || input.CompletionTokens != 8192*count/8 || shuffles != 1 || models.calls != 0 {
				t.Fatalf("input=%+v shuffles=%d calls=%d", input, shuffles, models.calls)
			}
			if err := service.Run(ctx, CandidateRun{ID: "batch", UserID: "alice", WriteModel: "test/write", Payload: jobs.request.Payload}, func(string, int, int) {}); err != nil {
				t.Fatal(err)
			}
			if models.calls != 1 || models.req.MaxTokens != input.CompletionTokens || !strings.Contains(models.req.System, fmt.Sprintf("정확히 %d개", count)) {
				t.Fatalf("request=%+v calls=%d", models.req, models.calls)
			}
			var schema map[string]any
			if err := json.Unmarshal(models.req.JSONSchema, &schema); err != nil {
				t.Fatal(err)
			}
			array := schema["properties"].(map[string]any)["candidates"].(map[string]any)
			if array["minItems"] != float64(count) || array["maxItems"] != float64(count) {
				t.Fatalf("schema=%s", models.req.JSONSchema)
			}
			jobs.found = CandidateJob{ID: "batch", Status: "done", WriteModel: "test/write", Payload: jobs.save}
			batch, err := service.Get(ctx, "alice", "batch")
			if err != nil || batch.Count != count || len(batch.Candidates) != count {
				t.Fatalf("batch=%+v err=%v", batch, err)
			}
			if _, err := service.Adopt(ctx, "alice", "batch", fmt.Sprintf("style-%d", count), false); err != nil {
				t.Fatal(err)
			}
			if models.calls != 1 {
				t.Fatal("adoption regenerated work")
			}
		})
	}
	for _, count := range []int{-1, 1, 3, 5, 7, 9, 15, 17, 32} {
		service := NewCandidateService(&candidateModelFake{}, &candidateJobsFake{}, &candidateStoreFake{}, candidateBudgetFake{}, &candidateEstimatorFake{})
		if _, err := service.Estimate(context.Background(), llm.ModelRef{}, count); !errors.Is(err, ErrCandidateCount) {
			t.Fatalf("count %d estimate=%v", count, err)
		}
		if _, err := service.Start(context.Background(), "alice", llm.ModelRef{}, count); !errors.Is(err, ErrCandidateCount) {
			t.Fatalf("count %d start=%v", count, err)
		}
		if WritingCandidateSchema(count) != nil {
			t.Fatalf("invalid schema count %d", count)
		}
	}
}

func TestBinaryCandidateBatchesRejectIncompleteDuplicateMalformedAndUnreadyResults(t *testing.T) {
	for _, count := range []int{2, 4, 8, 16} {
		if _, err := parseCandidates(candidateFixture(count-1), count); err == nil {
			t.Fatalf("shrunk %d accepted", count)
		}
		if _, err := parseCandidates(strings.Repeat(" ", candidateResponseBytesPerStyle*count+1), count); err == nil {
			t.Fatalf("unbounded %d accepted", count)
		}
		values, err := parseCandidates(candidateFixture(count), count)
		if err != nil {
			t.Fatal(err)
		}
		values[1].Sample = values[0].Sample
		payload, _ := json.Marshal(candidateResult{Count: count, Candidates: values})
		jobs := &candidateJobsFake{found: CandidateJob{ID: "batch", Status: "done", Payload: payload}}
		service := NewCandidateService(&candidateModelFake{}, jobs, &candidateStoreFake{}, candidateBudgetFake{}, &candidateEstimatorFake{})
		if _, err := service.Get(context.Background(), "alice", "batch"); !errors.Is(err, ErrCandidatesNotReady) {
			t.Fatalf("duplicate %d err=%v", count, err)
		}
		values, _ = parseCandidates(candidateFixture(count), count)
		payload, _ = json.Marshal(candidateResult{Count: count, Candidates: values})
		for _, status := range []string{"queued", "running", "failed", "cancelled"} {
			jobs.found = CandidateJob{ID: "batch", Status: status, Payload: payload}
			if _, err := service.Adopt(context.Background(), "alice", "batch", "style-1", true); !errors.Is(err, ErrCandidatesNotReady) {
				t.Fatalf("%s adopted: %v", status, err)
			}
		}
	}
}

func TestCandidateOutputIsExactlyEightBoundedDistinctKoreanStyles(t *testing.T) {
	if _, err := parseCandidates(strings.Repeat(" ", 64*1024+1)); err == nil {
		t.Fatal("unbounded output accepted")
	}
	valid, err := parseCandidates(candidateFixture())
	if err != nil || len(valid) != 8 {
		t.Fatalf("parse=%v err=%v", valid, err)
	}
	for i, value := range valid {
		if value.ID != fmt.Sprintf("style-%d", i+1) {
			t.Fatalf("id=%q", value.ID)
		}
	}
	for _, test := range []struct {
		name   string
		change func([]WritingCandidate) []WritingCandidate
	}{
		{"seven", func(v []WritingCandidate) []WritingCandidate { return v[:7] }},
		{"nine", func(v []WritingCandidate) []WritingCandidate { return append(v, v[0]) }},
		{"duplicate names", func(v []WritingCandidate) []WritingCandidate { v[1].Name = v[0].Name; return v }},
		{"duplicate samples", func(v []WritingCandidate) []WritingCandidate { v[1].Sample = v[0].Sample; return v }},
		{"short sample", func(v []WritingCandidate) []WritingCandidate { v[0].Sample = "편안했어요."; return v }},
		{"long sample", func(v []WritingCandidate) []WritingCandidate { v[0].Sample = strings.Repeat("가", 701); return v }},
		{"long name", func(v []WritingCandidate) []WritingCandidate { v[0].Name = strings.Repeat("가", 51); return v }},
		{"long description", func(v []WritingCandidate) []WritingCandidate { v[0].Description = strings.Repeat("가", 201); return v }},
		{"english", func(v []WritingCandidate) []WritingCandidate { v[0].Sample = strings.Repeat("English. ", 30); return v }},
		{"no prose", func(v []WritingCandidate) []WritingCandidate {
			v[0].Sample = "#" + strings.Repeat("태그", 110)
			return v
		}},
		{"missing description", func(v []WritingCandidate) []WritingCandidate { v[0].Description = ""; return v }},
	} {
		t.Run(test.name, func(t *testing.T) {
			values, _ := parseCandidates(candidateFixture())
			raw, _ := json.Marshal(candidateResult{Candidates: test.change(values)})
			if _, err := parseCandidates(string(raw)); err == nil {
				t.Fatal("invalid candidate output accepted")
			}
		})
	}
}

type candidateModelFake struct {
	calls int
	req   llm.Request
	text  string
	err   error
	info  llm.ModelInfo
}

func (f *candidateModelFake) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return f.info, ref.ProviderID == "test" && ref.ModelID == "write"
}
func (f *candidateModelFake) Complete(_ context.Context, _ llm.ModelRef, req llm.Request) (llm.Response, error) {
	f.calls++
	f.req = req
	return llm.Response{Text: f.text}, f.err
}

type candidateBudgetFake struct{}

func (candidateBudgetFake) Short(native bool) int {
	if native {
		return 16384
	}
	return 8192
}

type candidateEstimatorFake struct{ tokens [2]int64 }

func (f *candidateEstimatorFake) CallCredits(_ context.Context, _ llm.ModelInfo, p, c int64) (int, bool) {
	f.tokens = [2]int64{p, c}
	return 5, true
}

type candidateStoreFake struct {
	calls    int
	adoption CandidateAdoption
}

func (f *candidateStoreFake) AdoptCandidate(_ context.Context, in CandidateAdoption) (Voice, error) {
	f.calls++
	f.adoption = in
	return in.Voice, nil
}

type candidateJobsFake struct {
	resultRows  map[string]CandidateJob
	latestQuery func(string) *CandidateJob
	request     CandidateJobRequest
	save        []byte
	found       CandidateJob
	latest      *CandidateJob
	done        *CandidateJob
	cancelled   bool
}

func (f *candidateJobsFake) EnqueueCandidates(_ context.Context, in CandidateJobRequest) (string, error) {
	f.request = in
	return "batch", nil
}
func (f *candidateJobsFake) CandidateResult(_ context.Context, user, id string) (CandidateJob, error) {
	if user != "alice" {
		return CandidateJob{}, ErrCandidateNotFound
	}
	if f.resultRows != nil {
		if found, ok := f.resultRows[id]; ok {
			return found, nil
		}
		return CandidateJob{}, ErrCandidateNotFound
	}
	if id != "batch" {
		return CandidateJob{}, ErrCandidateNotFound
	}
	return f.found, nil
}
func (f *candidateJobsFake) LatestCandidates(_ context.Context, user, status string) (*CandidateJob, error) {
	if user != "alice" {
		return nil, nil
	}
	if f.latestQuery != nil {
		return f.latestQuery(status), nil
	}
	if status == "done" {
		return f.done, nil
	}
	return f.latest, nil
}
func (f *candidateJobsFake) SaveCandidateResult(_ context.Context, _ string, payload []byte) error {
	f.save = payload
	return nil
}
func (f *candidateJobsFake) CancelCandidates(_ context.Context, user, id string) error {
	if user != "alice" || id != "batch" {
		return ErrCandidateNotFound
	}
	f.cancelled = true
	return nil
}

func TestCandidateRequestFreezesRandomDirectionsBudgetAndMakesOneCall(t *testing.T) {
	ctx := context.Background()
	models := &candidateModelFake{text: candidateFixture(), info: llm.ModelInfo{Stages: []string{llm.StageNameWrite}, StructuredOutput: true, ReasoningNativeEffort: true}}
	jobs := &candidateJobsFake{}
	store := &candidateStoreFake{}
	estimator := &candidateEstimatorFake{}
	service := NewCandidateService(models, jobs, store, candidateBudgetFake{}, estimator)
	service.shuffle = func(v []string) error {
		for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
			v[i], v[j] = v[j], v[i]
		}
		return nil
	}
	ref := llm.ModelRef{ProviderID: "test", ModelID: "write"}
	estimate, err := service.Estimate(ctx, ref)
	if err != nil || !estimate.Available || estimate.Credits != 5 || estimator.tokens != [2]int64{6000, 16384} {
		t.Fatalf("estimate=%+v err=%v tokens=%v", estimate, err, estimator.tokens)
	}
	if _, err := service.Start(ctx, "alice", ref); err != nil {
		t.Fatal(err)
	}
	if models.calls != 0 || jobs.request.CompletionTokens != 16384 || jobs.request.WriteModel != "test/write" || jobs.request.PromptTokens <= 0 {
		t.Fatalf("request=%+v calls=%d", jobs.request, models.calls)
	}
	var input candidateInput
	if err := json.Unmarshal(jobs.request.Payload, &input); err != nil {
		t.Fatal(err)
	}
	if input.Directions[0] != candidateDirections[len(candidateDirections)-1] || len(input.Directions) != 8 {
		t.Fatalf("directions=%v", input.Directions)
	}
	progress := 0
	if err := service.Run(ctx, CandidateRun{ID: "batch", UserID: "alice", WriteModel: "test/write", Payload: jobs.request.Payload}, func(string, int, int) { progress++ }); err != nil {
		t.Fatal(err)
	}
	if models.calls != 1 || models.req.MaxTokens != 16384 || models.req.Stage != llm.StageNameWrite || len(models.req.JSONSchema) == 0 || progress != 2 {
		t.Fatalf("calls=%d request=%+v progress=%d", models.calls, models.req, progress)
	}
	jobs.found = CandidateJob{ID: "batch", Status: "done", WriteModel: "test/write", Payload: jobs.save}
	jobs.done = &jobs.found
	jobs.latest = &CandidateJob{ID: "failed-replacement", Status: "failed"}
	latest, err := service.Latest(ctx, "alice")
	if err != nil || latest.JobID != "failed-replacement" || latest.ResultJobID != "batch" || len(latest.Candidates) != 8 {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
	if _, err := service.Adopt(ctx, "alice", "batch", "style-1", true); err != nil {
		t.Fatal(err)
	}
	if models.calls != 1 || store.calls != 1 || len(store.adoption.Analysis.MaterialIDs) != 0 || store.adoption.Analysis.Origin != OriginSynthetic || store.adoption.Analysis.SyntheticSample == "" {
		t.Fatalf("adoption=%+v calls=%d", store.adoption, models.calls)
	}
	if _, err := service.Get(ctx, "bob", "batch"); !errors.Is(err, ErrCandidateNotFound) {
		t.Fatalf("foreign=%v", err)
	}
	if _, err := service.Adopt(ctx, "alice", "batch", "unknown", false); !errors.Is(err, ErrCandidateNotFound) {
		t.Fatalf("unknown candidate=%v", err)
	}
	if err := service.Cancel(ctx, "alice", "batch"); err != nil || !jobs.cancelled {
		t.Fatalf("cancel=%v", err)
	}
	jobs.found.Status = "failed"
	if _, err := service.Adopt(ctx, "alice", "batch", "style-1", false); !errors.Is(err, ErrCandidatesNotReady) {
		t.Fatalf("failed adoption=%v", err)
	}
}

func TestInvalidCandidateOutputFailsWithoutPaidCorrection(t *testing.T) {
	models := &candidateModelFake{text: `{"candidates":[]}`, info: llm.ModelInfo{Stages: []string{llm.StageNameWrite}}}
	jobs := &candidateJobsFake{}
	service := NewCandidateService(models, jobs, &candidateStoreFake{}, candidateBudgetFake{}, &candidateEstimatorFake{})
	if _, err := service.Start(context.Background(), "alice", llm.ModelRef{ProviderID: "test", ModelID: "write"}); err != nil {
		t.Fatal(err)
	}
	err := service.Run(context.Background(), CandidateRun{ID: "batch", WriteModel: "test/write", Payload: jobs.request.Payload}, func(string, int, int) {})
	var invalid *CandidateOutputError
	if !errors.As(err, &invalid) || models.calls != 1 || len(jobs.save) != 0 || invalid.Failure().Reason != "WRITING_VOICE_CANDIDATE_OUTPUT_INVALID" {
		t.Fatalf("error=%v calls=%d save=%s", err, models.calls, jobs.save)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.Run(ctx, CandidateRun{ID: "batch", WriteModel: "test/write", Payload: jobs.request.Payload}, func(string, int, int) {}); !errors.Is(err, context.Canceled) || models.calls != 1 {
		t.Fatalf("cancel err=%v calls=%d", err, models.calls)
	}
}

// A newly finished request may appear between the two indexed latest reads. The
// response must identify an attempt at least as recent as its preserved result.
func TestLatestWritingCandidatesKeepCoherentRecoveryAcrossInterleavedCompletion(t *testing.T) {
	values, err := parseCandidates(candidateFixture())
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(candidateResult{Candidates: values})
	if err != nil {
		t.Fatal(err)
	}
	first := CandidateJob{ID: "first", Status: "done", WriteModel: "test/write", Payload: payload}
	second := CandidateJob{ID: "second", Status: "done", WriteModel: "test/write", Payload: payload}
	jobs := &candidateJobsFake{resultRows: map[string]CandidateJob{"first": first, "second": second}}
	reads := 0
	var order []string
	jobs.latestQuery = func(status string) *CandidateJob {
		order = append(order, status)
		reads++
		if reads == 1 {
			return &first
		}
		// The second request finished after the first read and before this one.
		return &second
	}
	models := &candidateModelFake{}
	service := NewCandidateService(models, jobs, &candidateStoreFake{}, candidateBudgetFake{}, &candidateEstimatorFake{})
	latest, err := service.Latest(context.Background(), "alice")
	if err != nil || latest.JobID != "second" || latest.ResultJobID != "first" || len(latest.Candidates) != CandidateCount || models.calls != 0 {
		t.Fatalf("incoherent recovery=%+v err=%v calls=%d", latest, err, models.calls)
	}
	if len(order) != 2 || order[0] != "done" || order[1] != "" {
		t.Fatalf("latest reads=%v", order)
	}
}

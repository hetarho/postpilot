package memory

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

type fakeModels struct {
	ref        llm.ModelRef
	found      bool
	structured bool
	answer     string
	err        error
	requests   []llm.Request
}

func (f *fakeModels) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return f.ref, f.found, nil
}

func (f *fakeModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{StructuredOutput: f.structured}, true
}

func (f *fakeModels) Complete(_ context.Context, _ llm.ModelRef, request llm.Request) (llm.Response, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return llm.Response{}, f.err
	}
	return llm.Response{Text: f.answer}, nil
}

type fakePosts struct {
	source ExtractionSource
	err    error
	asked  []string
}

func (f *fakePosts) ExtractionSource(_ context.Context, _, slug string) (ExtractionSource, error) {
	f.asked = append(f.asked, slug)
	return f.source, f.err
}

type fakeExtractions struct {
	requests []ExtractionRequest
	jobID    string
	saved    map[string][]byte
	stored   []byte
	loadErr  error
}

func (f *fakeExtractions) Enqueue(_ context.Context, request ExtractionRequest) (string, error) {
	f.requests = append(f.requests, request)
	return f.jobID, nil
}

func (f *fakeExtractions) SaveCandidates(_ context.Context, jobID string, payload []byte) error {
	if f.saved == nil {
		f.saved = map[string][]byte{}
	}
	f.saved[jobID] = payload
	return nil
}

func (f *fakeExtractions) Candidates(context.Context, string, string) ([]byte, error) {
	return f.stored, f.loadErr
}

func extractionService(t *testing.T, models *fakeModels, posts *fakePosts, jobs *fakeExtractions) (*Service, *fakeStore) {
	t.Helper()
	store := &fakeStore{}
	service := newTestService(store)
	service.ConfigureExtraction(models, posts, jobs)
	return service, store
}

const goodAnswer = `{"candidates":[
  {"text":"매운 음식을 못 먹는다","kind":"preference","tags":["음식"]},
  {"text":"연남동에 자주 간다","kind":"place","tags":["연남동","산책"]}
]}`

// MEM-14: one call, and nothing written. The candidates land on the job row and the memory
// tables are not touched at all — that is the whole difference between proposing and storing.
func TestExtractProposesAndStoresNothing(t *testing.T) {
	models := &fakeModels{answer: goodAnswer, structured: true}
	jobs := &fakeExtractions{}
	service, store := extractionService(t, models, &fakePosts{}, jobs)

	stages := []string{}
	err := service.Extract(context.Background(), ExtractionJob{
		ID: "job-1", UserID: "alice", PostSlug: "post-1", Model: "p/m",
		Source: ExtractionSource{PostSlug: "post-1", Title: "제목", Memo: "메모", Body: "본문"},
	}, func(stage string, _, _ int) { stages = append(stages, stage) })
	if err != nil {
		t.Fatal(err)
	}
	if len(models.requests) != 1 {
		t.Fatalf("provider calls = %d, want exactly one", len(models.requests))
	}
	if len(store.inserted) != 0 || len(store.patches) != 0 || len(store.dropped) != 0 {
		t.Fatalf("the extraction wrote to the memory store: %+v", store)
	}
	if len(stages) != 2 {
		t.Fatalf("progress = %v", stages)
	}

	saved := jobs.saved["job-1"]
	var payload extractionPayload
	if err := json.Unmarshal(saved, &payload); err != nil {
		t.Fatalf("saved payload is not JSON: %s", saved)
	}
	if payload.PostSlug != "post-1" || len(payload.Candidates) != 2 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Candidates[1].Kind != "place" || len(payload.Candidates[1].Tags) != 2 {
		t.Fatalf("candidate = %+v", payload.Candidates[1])
	}

	// The prompt names the five kinds and refuses a summary; the input is the post's own
	// title, memo and prose and nothing else.
	request := models.requests[0]
	if request.System != ExtractionPrompt || request.Stage != llm.StageNameAnalyze {
		t.Fatalf("request = %+v", request)
	}
	if string(request.JSONSchema) != string(MemoryCandidatesSchema()) {
		t.Fatal("a structured-output model was not given the schema")
	}
}

// VOICE-27's attach-or-fall-back: a model that does not declare structured output still
// runs, on the prompt alone, because the prompt already states the shape.
func TestExtractFallsBackToThePromptWithoutStructuredOutput(t *testing.T) {
	models := &fakeModels{answer: goodAnswer, structured: false}
	service, _ := extractionService(t, models, &fakePosts{}, &fakeExtractions{})

	if err := service.Extract(context.Background(), ExtractionJob{ID: "job-1", UserID: "alice", Model: "p/m"}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if models.requests[0].JSONSchema != nil {
		t.Fatal("a model without structured output was given a schema")
	}
}

// The schema's kinds are STRINGS. A numeric enum makes Gemini answer `{}` for the whole
// object, so this is pinned rather than left to a reader to notice.
func TestTheCandidateSchemaCarriesNoIntegerEnum(t *testing.T) {
	raw := string(MemoryCandidatesSchema())
	if !strings.Contains(raw, `"enum": ["preference", "persona", "place", "person", "history"]`) {
		t.Fatalf("the kind enum is not the five strings:\n%s", raw)
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		t.Fatalf("the schema is not valid JSON: %v", err)
	}
	var walk func(any)
	walk = func(node any) {
		switch value := node.(type) {
		case map[string]any:
			if values, ok := value["enum"].([]any); ok {
				for _, entry := range values {
					if _, isString := entry.(string); !isString {
						t.Fatalf("a non-string enum value reached the schema: %#v", entry)
					}
				}
			}
			for _, child := range value {
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(schema)
}

// A candidate is only useful if the user can save it, so the bounds a create enforces are
// applied here: what could not pass them is dropped rather than offered as a checkbox that
// answers with a refusal.
func TestExtractDropsWhatACreateWouldRefuse(t *testing.T) {
	answer := `{"candidates":[
	  {"text":"` + strings.Repeat("가", 21) + `","kind":"preference"},
	  {"text":"종류가 없다","kind":"mood"},
	  {"text":"","kind":"persona"},
	  {"text":"태그가 너무 많다","kind":"history","tags":["1","2","3","4","5"]},
	  {"text":"좋은 사실","kind":"persona","tags":["가"]},
	  {"text":" 좋은 사실 ","kind":"place","tags":["나"]}
	]}`
	models := &fakeModels{answer: answer}
	jobs := &fakeExtractions{}
	service, _ := extractionService(t, models, &fakePosts{}, jobs)

	if err := service.Extract(context.Background(), ExtractionJob{ID: "job-1", UserID: "alice", Model: "p/m"}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	var payload extractionPayload
	if err := json.Unmarshal(jobs.saved["job-1"], &payload); err != nil {
		t.Fatal(err)
	}
	texts := make([]string, 0, len(payload.Candidates))
	for _, c := range payload.Candidates {
		texts = append(texts, c.Text)
	}
	// The over-long text, the unknown kind and the empty text are gone; the over-tagged fact
	// keeps its text and loses its tags; and the repeat of one fact is one checkbox.
	if strings.Join(texts, ",") != "태그가 너무 많다,좋은 사실" {
		t.Fatalf("candidates = %v", texts)
	}
	if len(payload.Candidates[0].Tags) != 0 {
		t.Fatalf("an over-tagged candidate kept its tags: %+v", payload.Candidates[0])
	}
}

// An answer that is not the contract is a failure, not an empty proposal: "this post
// yielded nothing" is a real answer the user acts on and must stay distinguishable.
func TestExtractRefusesAnAnswerThatIsNotTheContract(t *testing.T) {
	for name, answer := range map[string]string{"empty": "   ", "prose": "여기 사실이 있습니다"} {
		jobs := &fakeExtractions{}
		service, _ := extractionService(t, &fakeModels{answer: answer}, &fakePosts{}, jobs)
		if err := service.Extract(context.Background(), ExtractionJob{ID: "job-1", Model: "p/m"}, func(string, int, int) {}); err == nil {
			t.Fatalf("%s answer was accepted", name)
		}
		if len(jobs.saved) != 0 {
			t.Fatalf("%s answer still saved a payload", name)
		}
	}
}

// StartExtraction freezes the account's analyze selection and the post it read, and refuses
// rather than substituting a model when the account has none enabled.
func TestStartExtractionFreezesTheModelAndTheSource(t *testing.T) {
	posts := &fakePosts{source: ExtractionSource{PostSlug: "post-1", Title: "제목", Memo: "메모", Body: "본문"}}
	jobs := &fakeExtractions{jobID: "job-9"}
	models := &fakeModels{ref: llm.ModelRef{ProviderID: "p", ModelID: "m"}, found: true}
	service, _ := extractionService(t, models, posts, jobs)

	id, err := service.StartExtraction(context.Background(), "alice", " post-1 ")
	if err != nil || id != "job-9" {
		t.Fatalf("start = %q, %v", id, err)
	}
	request := jobs.requests[0]
	if request.Model != "p/m" || request.PostSlug != "post-1" || request.Source.Body != "본문" {
		t.Fatalf("request = %+v", request)
	}
	if len(posts.asked) != 1 || posts.asked[0] != "post-1" {
		t.Fatalf("posts asked = %v", posts.asked)
	}

	models.found = false
	if _, err := service.StartExtraction(context.Background(), "alice", "post-1"); !errors.Is(err, ErrAnalyzeModelRequired) {
		t.Fatalf("with no analyze model = %v, want ErrAnalyzeModelRequired", err)
	}
	if _, err := service.StartExtraction(context.Background(), "alice", "  "); !errors.Is(err, ErrNotFound) {
		t.Fatalf("with no slug = %v", err)
	}
}

// The source round-trips through the job row, which is what makes "an edit during the call
// cannot change what was extracted" true.
func TestTheExtractionSourceRoundTripsThroughTheJobRow(t *testing.T) {
	source := ExtractionSource{PostSlug: "post-1", Title: "제목", Memo: "메모", Body: "본문"}
	raw, err := EncodeExtractionSource(source)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeExtractionSource(raw)
	if err != nil || back != source {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
}

// Reading a result: a job whose candidates are not there yet, and one belonging to someone
// else, are two refusals rather than an empty list.
func TestExtractionRefusesWhatIsNotAFinishedOwnedJob(t *testing.T) {
	service, _ := extractionService(t, &fakeModels{}, &fakePosts{}, &fakeExtractions{loadErr: ErrNotFound})
	if _, _, err := service.Extraction(context.Background(), "alice", "job-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a foreign job = %v", err)
	}

	service, _ = extractionService(t, &fakeModels{}, &fakePosts{}, &fakeExtractions{stored: nil})
	if _, _, err := service.Extraction(context.Background(), "alice", "job-1"); !errors.Is(err, ErrExtractionNotReady) {
		t.Fatalf("an unfinished job = %v", err)
	}

	service, _ = extractionService(t, &fakeModels{}, &fakePosts{}, &fakeExtractions{stored: []byte("not json")})
	if _, _, err := service.Extraction(context.Background(), "alice", "job-1"); !errors.Is(err, ErrExtractionNotReady) {
		t.Fatalf("an unreadable payload = %v", err)
	}

	stored, err := encodeExtraction("post-1", []Candidate{{Text: "사실", Kind: KindPersona, Tags: []string{"가"}}})
	if err != nil {
		t.Fatal(err)
	}
	service, _ = extractionService(t, &fakeModels{}, &fakePosts{}, &fakeExtractions{stored: stored})
	slug, candidates, err := service.Extraction(context.Background(), "alice", "job-1")
	if err != nil || slug != "post-1" || len(candidates) != 1 || candidates[0].Kind != KindPersona {
		t.Fatalf("extraction = %q %+v %v", slug, candidates, err)
	}
}

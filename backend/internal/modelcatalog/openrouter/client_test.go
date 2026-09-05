package openrouter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/modelcatalog/openrouter"
)

// The document below is the upstream shape, trimmed to the fields the product reads plus
// two it does not — the extra keys are the point: the provider's schema grows without
// notice, and an unknown field must not cost us a model.
const catalogDocument = `{"data":[
  {"id":"openai/gpt-x","name":"OpenAI: GPT X","description":"a model","created":1788362056,
   "context_length":1048576,"hugging_face_id":null,
   "architecture":{"modality":"text+image->text","input_modalities":["text","image"],"output_modalities":["text"]},
   "pricing":{"prompt":"0.00000125","completion":"0.00000425","web_search":"0.0025"},
   "supported_parameters":["structured_outputs","tools","reasoning"]},
  {"id":"deepseek/text-only","name":"DeepSeek: Text","created":1788000000,
   "context_length":131072,
   "architecture":{"input_modalities":["text"],"output_modalities":["text","image","video"]},
   "pricing":{"prompt":"0","completion":"0"},
   "supported_parameters":["tools"]},
  {"id":"muse/image-only","name":"Muse: Image","created":1788100000,
   "architecture":{"input_modalities":["text"],"output_modalities":["image"]},
   "pricing":{"prompt":"0","completion":"0","image_token":"0.0000024","image_output":"0.00003"},
   "supported_parameters":["seed"]},
  {"id":"google/veo-x","name":"Google: Veo X","created":1788200000,
   "architecture":{"input_modalities":["text","image"],"output_modalities":["video"]},
   "pricing":{"prompt":"0","completion":"0"},
   "supported_parameters":["seed"]},
  {"id":"","name":"Nameless id","created":1},
  {"id":"broken/no-name","created":1}
]}`

func serve(t *testing.T, body string, status int) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want the base url plus /models", r.URL.Path)
		}
		// Not decoration: the endpoint answers with TEXT-output models only unless asked
		// otherwise, so without this query the video tab can never have a candidate.
		if got := r.URL.Query().Get("output_modalities"); got != "text,image,video" {
			t.Errorf("output_modalities = %q, want the three curated modalities", got)
		}
		// The list is public: sending a key would make browsing fail on a box that has none.
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("Authorization header sent: %q", auth)
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &hits
}

func TestFetch_MapsUpstreamFieldsAndSkipsUnusableEntries(t *testing.T) {
	server, _ := serve(t, catalogDocument, http.StatusOK)
	client := openrouter.New(server.URL+"/v1", time.Second, time.Minute)

	snapshot, err := client.Fetch(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Candidates) != 4 {
		t.Fatalf("candidates = %d, want the four usable entries", len(snapshot.Candidates))
	}
	first := snapshot.Candidates[0]
	if first.ModelID != "openai/gpt-x" || first.ProviderSlug != "openai" || first.Label != "OpenAI: GPT X" {
		t.Errorf("identity = %+v", first)
	}
	if !first.Vision {
		t.Error("an image input modality did not become vision")
	}
	if !first.StructuredOutput {
		t.Error("structured_outputs did not become the structured-output flag")
	}
	if first.ImageOutput || first.VideoOutput {
		t.Errorf("text-output model carries output flags: %+v", first)
	}
	if first.ContextTokens != 1048576 || first.SourceCreatedAt != 1788362056 {
		t.Errorf("context/created = %d / %d", first.ContextTokens, first.SourceCreatedAt)
	}
	// USD per token becomes USD per million, exactly: a float round-trip would show
	// 0.00000125 back as 1.2500000000000001.
	if first.InputUSDPerMillion != "1.25" || first.OutputUSDPerMillion != "4.25" {
		t.Errorf("pricing = %q / %q, want 1.25 / 4.25", first.InputUSDPerMillion, first.OutputUSDPerMillion)
	}

	second := snapshot.Candidates[1]
	if second.Vision || second.StructuredOutput {
		t.Errorf("a text-only model got capabilities: %+v", second)
	}
	if !second.ImageOutput || !second.VideoOutput {
		t.Errorf("output modalities did not become the generation flags: %+v", second)
	}
	// A genuine zero price is a price, not a missing one — for a model billed on text
	// tokens at all, which is what makes the two cases below different.
	if second.InputUSDPerMillion != "0" {
		t.Errorf("free model input price = %q, want 0", second.InputUSDPerMillion)
	}

	byID := map[string]modelcatalog.Candidate{}
	for _, candidate := range snapshot.Candidates {
		byID[candidate.ModelID] = candidate
	}

	// A model that answers only in images is not billed on text tokens: its zero `prompt`
	// and `completion` are the absence of a price, and the image-token pair is what it
	// charges. `image_output` is per output image TOKEN despite its documented wording.
	image := byID["muse/image-only"]
	if !image.ImageOutput || image.VideoOutput || image.Vision {
		t.Errorf("image-only flags = %+v", image)
	}
	if image.InputUSDPerMillion != "2.4" || image.OutputUSDPerMillion != "30" {
		t.Errorf("image-only pricing = %q / %q, want the image-token pair", image.InputUSDPerMillion, image.OutputUSDPerMillion)
	}

	// A video model publishes no token price at all; reporting its zeros as "free" would
	// be a lie, so both come back unknown for the screen to say so.
	video := byID["google/veo-x"]
	if !video.VideoOutput || video.ImageOutput {
		t.Errorf("video flags = %+v", video)
	}
	if video.InputUSDPerMillion != "" || video.OutputUSDPerMillion != "" {
		t.Errorf("video pricing = %q / %q, want unknown rather than zero", video.InputUSDPerMillion, video.OutputUSDPerMillion)
	}
}

func TestFetch_CachesUntilTheTTLAndRefreshBypasses(t *testing.T) {
	server, hits := serve(t, catalogDocument, http.StatusOK)
	client := openrouter.New(server.URL+"/v1", time.Second, time.Minute)
	ctx := context.Background()

	if _, err := client.Fetch(ctx, false); err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.Fetch(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.FromCache || *hits != 1 {
		t.Fatalf("second read: fromCache=%v hits=%d, want a cache hit", snapshot.FromCache, *hits)
	}
	snapshot, err = client.Fetch(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.FromCache || *hits != 2 {
		t.Fatalf("refresh: fromCache=%v hits=%d, want a fresh read", snapshot.FromCache, *hits)
	}
}

// A7: a failed read is an error, not an empty catalog — the caller degrades to stored rows
// and the last good snapshot is kept.
func TestFetch_FailureKeepsTheCachedSnapshot(t *testing.T) {
	body := catalogDocument
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	client := openrouter.New(server.URL+"/v1", time.Second, time.Nanosecond)
	ctx := context.Background()

	if _, err := client.Fetch(ctx, false); err != nil {
		t.Fatal(err)
	}
	status = http.StatusBadGateway
	if _, err := client.Fetch(ctx, true); err == nil {
		t.Fatal("a 502 was reported as success")
	}

	status, body = http.StatusOK, `{"data":[]}`
	snapshot, err := client.Fetch(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Candidates) != 0 {
		t.Errorf("candidates = %d, want the empty answer honoured", len(snapshot.Candidates))
	}
}

func TestFetch_RejectsAnUnparseableBody(t *testing.T) {
	server, _ := serve(t, "not json", http.StatusOK)
	client := openrouter.New(server.URL+"/v1", time.Second, time.Minute)

	if _, err := client.Fetch(context.Background(), false); err == nil {
		t.Fatal("a non-JSON body was accepted")
	}
}

var _ modelcatalog.Upstream = (*openrouter.Client)(nil)

// The reasoning fixture uses the real shapes measured against the live endpoint on
// 2026-09-05 (change 27): a model that publishes a list, one mandatory model, one that
// reasons but publishes no list, and one carrying no `reasoning` object at all.
const reasoningDocument = `{"data":[
  {"id":"deepseek/deepseek-v4-pro-0813","name":"DeepSeek: V4 Pro","created":1788000001,
   "architecture":{"input_modalities":["text"],"output_modalities":["text"]},
   "pricing":{"prompt":"0.0000004","completion":"0.0000016"},
   "supported_parameters":["reasoning","reasoning_effort","structured_outputs"],
   "reasoning":{"mandatory":false,"supported_efforts":["max","high","low"],"default_effort":"high","default_enabled":true}},
  {"id":"google/gemini-3.8-flash","name":"Google: Gemini 3.8 Flash","created":1788000002,
   "architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},
   "pricing":{"prompt":"0.0000003","completion":"0.0000025"},
   "supported_parameters":["reasoning","include_reasoning"],
   "reasoning":{"mandatory":true,"supported_efforts":["high","medium","low"],"default_effort":"medium","supports_max_tokens":true}},
  {"id":"vendor/no-list","name":"Vendor: No List","created":1788000003,
   "architecture":{"input_modalities":["text"],"output_modalities":["text"]},
   "pricing":{"prompt":"0","completion":"0"},
   "supported_parameters":["reasoning","reasoning_effort"],
   "reasoning":{"mandatory":false,"unknown_future_key":42}},
  {"id":"vendor/no-reasoning","name":"Vendor: No Reasoning","created":1788000004,
   "architecture":{"input_modalities":["text"],"output_modalities":["text"]},
   "pricing":{"prompt":"0","completion":"0"},
   "supported_parameters":["structured_outputs"]}
]}`

// A1/A2: all six fields, from the source's own shapes. The entry with no `reasoning` object
// is recorded as not reasoning rather than skipped or failed.
func TestFetch_MapsTheReasoningObject(t *testing.T) {
	server, _ := serve(t, reasoningDocument, http.StatusOK)
	defer server.Close()
	client := openrouter.New(server.URL+"/v1", time.Second, time.Minute)
	snapshot, err := client.Fetch(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Candidates) != 4 {
		t.Fatalf("mapped %d candidates, want all 4", len(snapshot.Candidates))
	}
	byID := map[string]modelcatalog.Candidate{}
	for _, candidate := range snapshot.Candidates {
		byID[candidate.ModelID] = candidate
	}

	// A2, against a real model: max·high·low, default high, not mandatory, native effort.
	deepseek := byID["deepseek/deepseek-v4-pro-0813"].ReasoningCapability
	if !deepseek.Reasons || deepseek.Mandatory || !deepseek.NativeEffort || deepseek.MaxTokens {
		t.Fatalf("deepseek flags = %+v", deepseek)
	}
	if deepseek.DefaultEffort != "high" {
		t.Fatalf("deepseek default effort = %q", deepseek.DefaultEffort)
	}
	// The source's DESCENDING order is preserved, not sorted or normalized.
	want := []string{"max", "high", "low"}
	if len(deepseek.Efforts) != len(want) {
		t.Fatalf("deepseek efforts = %v", deepseek.Efforts)
	}
	for i := range want {
		if deepseek.Efforts[i] != want[i] {
			t.Fatalf("deepseek efforts = %v, want %v in that order", deepseek.Efforts, want)
		}
	}

	// A mandatory model, whose `supports_max_tokens` is published true. `include_reasoning`
	// is not `reasoning_effort`, so this one is not native.
	gemini := byID["google/gemini-3.8-flash"].ReasoningCapability
	if !gemini.Reasons || !gemini.Mandatory || !gemini.MaxTokens || gemini.NativeEffort {
		t.Fatalf("gemini flags = %+v", gemini)
	}

	// Present-but-listless: it reasons, and an unknown future key inside the object is
	// ignored rather than costing us the model.
	listless := byID["vendor/no-list"].ReasoningCapability
	if !listless.Reasons || len(listless.Efforts) != 0 || listless.DefaultEffort != "" {
		t.Fatalf("listless = %+v", listless)
	}
	if !listless.NativeEffort {
		t.Fatal("a listless model still declares reasoning_effort natively")
	}

	// A1: absent object is "does not reason", and the model is still mapped.
	none := byID["vendor/no-reasoning"].ReasoningCapability
	if none.Reasons || none.Mandatory || none.NativeEffort || len(none.Efforts) != 0 {
		t.Fatalf("no-reasoning = %+v", none)
	}
	if byID["vendor/no-reasoning"].Label != "Vendor: No Reasoning" {
		t.Fatal("the entry without a reasoning object was not mapped at all")
	}
}

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
	if len(snapshot.Candidates) != 2 {
		t.Fatalf("candidates = %d, want the two usable entries", len(snapshot.Candidates))
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
	// A genuine zero price is a price, not a missing one.
	if second.InputUSDPerMillion != "0" {
		t.Errorf("free model input price = %q, want 0", second.InputUSDPerMillion)
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

package openaicompat_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/llm/openaicompat"
)

func newClient(t *testing.T, handler http.HandlerFunc) (*openaicompat.Client, *httptest.Server) {
	return newClientWithReasoningFormat(t, "", handler)
}

func newClientWithReasoningFormat(t *testing.T, format string, handler http.HandlerFunc) (*openaicompat.Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := openaicompat.New(llm.AdapterConfig{
		ProviderID: "test", BaseURL: server.URL + "/v1/", APIKey: "secret", ReasoningFormat: format,
	}, server.Client())
	return client, server
}

func sse(w http.ResponseWriter, chunks ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, c := range chunks {
		io.WriteString(w, "data: "+c+"\n\n")
	}
	io.WriteString(w, "data: [DONE]\n\n")
}

func TestFactory_RequiresBaseURL(t *testing.T) {
	if _, err := openaicompat.Factory(llm.AdapterConfig{ProviderID: "x"}); err == nil || !strings.Contains(err.Error(), "base_url is required") {
		t.Fatalf("err = %v", err)
	}
	if _, err := openaicompat.Factory(llm.AdapterConfig{ProviderID: "x", BaseURL: "not a url"}); err == nil {
		t.Fatal("accepted a non-URL base_url")
	}
	if _, err := openaicompat.Factory(llm.AdapterConfig{ProviderID: "x", BaseURL: "https://openrouter.ai/api/v1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := openaicompat.Factory(llm.AdapterConfig{ProviderID: "x", BaseURL: "https://example.test/v1", ReasoningFormat: "other"}); err == nil || !strings.Contains(err.Error(), "reasoning_format") {
		t.Fatalf("unsupported reasoning format err = %v", err)
	}
}

// The request shape the compatible servers expect: system message, image parts as data
// URLs, response_format json_schema, streaming with usage.
func TestComplete_ShapesTheRequestAndJoinsTheStream(t *testing.T) {
	var got map[string]any
	var gotPath, gotAuth string
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		sse(w,
			`{"choices":[{"delta":{"role":"assistant","content":""}}]}`,
			`{"choices":[{"delta":{"content":"{\"a\":"}}]}`,
			`{"choices":[{"delta":{"content":"1}"}}]}`,
			`{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}`,
		)
	})

	resp, err := client.Complete(context.Background(), llm.Request{
		Model:      "vendor/model",
		System:     "be brief",
		MaxTokens:  99,
		JSONSchema: []byte(`{"type":"object"}`),
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.Part{llm.TextPart("what is this"), llm.ImagePart([]byte("jpegbytes"), "image/jpeg")},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q (trailing slash on base_url must not double)", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if got["model"] != "vendor/model" || got["stream"] != true || got["max_tokens"] != float64(99) {
		t.Errorf("request = %v", got)
	}
	if _, sent := got["reasoning"]; sent {
		t.Errorf("reasoning = %v, want the unresolved request shape unchanged", got["reasoning"])
	}
	messages := got["messages"].([]any)
	if len(messages) != 2 || messages[0].(map[string]any)["role"] != "system" || messages[0].(map[string]any)["content"] != "be brief" {
		t.Errorf("messages = %v", messages)
	}
	parts := messages[1].(map[string]any)["content"].([]any)
	image := parts[1].(map[string]any)
	url := image["image_url"].(map[string]any)["url"].(string)
	if image["type"] != "image_url" || !strings.HasPrefix(url, "data:image/jpeg;base64,") {
		t.Errorf("image part = %v", image)
	}
	format := got["response_format"].(map[string]any)
	if format["type"] != "json_schema" || format["json_schema"].(map[string]any)["schema"].(map[string]any)["type"] != "object" {
		t.Errorf("response_format = %v", format)
	}

	if resp.Text != `{"a":1}` {
		t.Errorf("Text = %q", resp.Text)
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 {
		t.Errorf("Usage = %+v", resp.Usage)
	}
}

func TestComplete_SendsOnlyResolvedReasoning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		effort llm.ReasoningEffort
		want   string
	}{
		{name: "unspecified"},
		{name: "unset override", effort: llm.ReasoningUnset},
		{name: "low", effort: llm.ReasoningLow, want: "low"},
		{name: "none", effort: llm.ReasoningNone, want: "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			client, _ := newClientWithReasoningFormat(t, "openrouter", func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Error(err)
				}
				sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`)
			})
			if _, err := client.Complete(context.Background(), llm.Request{Model: "m", Reasoning: tc.effort}); err != nil {
				t.Fatal(err)
			}
			reasoning, sent := got["reasoning"].(map[string]any)
			if tc.want == "" {
				if sent {
					t.Fatalf("reasoning = %v, want no key", reasoning)
				}
				return
			}
			if !sent || reasoning["effort"] != tc.want {
				t.Fatalf("reasoning = %v, want effort %q", reasoning, tc.want)
			}
		})
	}
}

func TestComplete_OmitsOpenRouterReasoningExtensionWithoutOptIn(t *testing.T) {
	var got map[string]any
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`)
	})
	if _, err := client.Complete(context.Background(), llm.Request{Model: "m", Reasoning: llm.ReasoningLow}); err != nil {
		t.Fatal(err)
	}
	if _, sent := got["reasoning"]; sent {
		t.Fatalf("reasoning = %v, want compatible endpoint request unchanged", got["reasoning"])
	}
}

func TestComplete_TextOnlyMessageIsAPlainString(t *testing.T) {
	var got map[string]any
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		sse(w, `{"choices":[{"delta":{"content":"hi"}}]}`)
	})
	if _, err := client.Complete(context.Background(), llm.Request{Model: "m", Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("hello")}}}}); err != nil {
		t.Fatal(err)
	}
	if _, isFormat := got["response_format"]; isFormat {
		t.Error("response_format sent without a schema")
	}
	if got["messages"].([]any)[0].(map[string]any)["content"] != "hello" {
		t.Errorf("content = %v, want a plain string", got["messages"])
	}
}

// A provider that ignores `stream: true` answers with one JSON document.
func TestComplete_AcceptsANonStreamedAnswer(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"plain"}}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`)
	})
	resp, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "plain" || resp.Usage.CompletionTokens != 2 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestComplete_FoldsReportedCostFromStreamAndJSON(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
	}{
		{"stream number", "text/event-stream", "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"cost\":0.0000015}}\n\ndata: [DONE]\n\n"},
		{"json string", "application/json", `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":10,"completion_tokens":2,"cost":"0.0000015"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				io.WriteString(w, tc.body)
			})
			response, err := client.Complete(context.Background(), llm.Request{Model: "m"})
			if err != nil {
				t.Fatal(err)
			}
			if !response.Usage.CostReported || response.Usage.CostMicrousd != 2 {
				t.Fatalf("usage = %+v, want rounded 2 microusd reported", response.Usage)
			}
		})
	}
}

func TestComplete_AbsentCostStaysUnreportedAndInvalidCostFails(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1}}`)
	})
	response, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if err != nil || response.Usage.CostReported || response.Usage.CostMicrousd != 0 {
		t.Fatalf("absent cost = %+v, %v", response.Usage, err)
	}

	for _, raw := range []string{`-1`, `"not-a-number"`, `999999999999999999999999999999`} {
		t.Run(raw, func(t *testing.T) {
			client, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"cost":`+raw+`}}`)
			})
			if _, err := client.Complete(context.Background(), llm.Request{Model: "m"}); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("invalid cost err = %v", err)
			}
		})
	}
}

// AC8: a 429 is ErrRateLimited with the provider's message intact.
func TestComplete_RateLimitKeepsTheProviderMessage(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"Rate limit exceeded: free-models-per-day","code":429}}`)
	})
	_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if !errors.Is(err, llm.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	var perr *llm.ProviderError
	if !errors.As(err, &perr) || perr.Status != 429 || perr.Message != "Rate limit exceeded: free-models-per-day" {
		t.Fatalf("ProviderError = %+v", perr)
	}
	if !strings.Contains(err.Error(), "free-models-per-day") {
		t.Errorf("Error() = %q, must carry the provider's message", err.Error())
	}
}

func TestComplete_MapsModelNotFound(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"404":               {http.StatusNotFound, `{"error":{"message":"No endpoints found"}}`},
		"400 model missing": {http.StatusBadRequest, `{"error":{"message":"The model 'x/y' does not exist"}}`},
		"400 invalid model": {http.StatusBadRequest, `{"error":{"message":"invalid model id"}}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
			if !errors.Is(err, llm.ErrModelUnavailable) {
				t.Fatalf("err = %v, want ErrModelUnavailable", err)
			}
		})
	}
}

func TestComplete_OtherFailuresAreGenericProviderErrors(t *testing.T) {
	// 400s that mention "model" without meaning "no such model" must stay generic, or
	// the user is told to re-pick a model that is fine.
	for _, message := range []string{
		"max_tokens too large",
		"Invalid model parameters: max_tokens must be <= 5000",
		"this model is not available with response_format json_schema",
	} {
		t.Run(message, func(t *testing.T) {
			client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": message}})
			})
			_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
			var perr *llm.ProviderError
			if !errors.As(err, &perr) || perr.Kind != nil || perr.Message != message {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestComplete_KeylessEndpointGetsNoAuthorizationHeader(t *testing.T) {
	var gotAuth string
	var hasAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, hasAuth = r.Header.Get("Authorization"), len(r.Header.Values("Authorization")) > 0
		sse(w, `{"choices":[{"delta":{"content":"hi"}}]}`)
	}))
	t.Cleanup(server.Close)
	client := openaicompat.New(llm.AdapterConfig{ProviderID: "ollama", BaseURL: server.URL}, server.Client())

	if _, err := client.Complete(context.Background(), llm.Request{Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if hasAuth {
		t.Errorf("Authorization = %q sent for a keyless provider", gotAuth)
	}
}

func TestComplete_EmptyMessageIsAnEmptyString(t *testing.T) {
	var got map[string]any
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		sse(w, `{"choices":[{"delta":{"content":"hi"}}]}`)
	})
	if _, err := client.Complete(context.Background(), llm.Request{Model: "m", Messages: []llm.Message{{Role: llm.RoleAssistant}}}); err != nil {
		t.Fatal(err)
	}
	if got["messages"].([]any)[0].(map[string]any)["content"] != "" {
		t.Errorf("content = %v, want \"\" (servers reject [])", got["messages"])
	}
}

// A quota reached after the headers went out arrives inside the stream.
func TestComplete_ErrorInsideTheStream(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"par"}}]}`, `{"error":{"message":"Rate limit exceeded","code":429}}`)
	})
	_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if !errors.Is(err, llm.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
}

func TestComplete_ErrorKeepsUsageReportedBeforeIt(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`{"choices":[],"usage":{"prompt_tokens":17,"completion_tokens":23,"cost":"0.000042"}}`,
			`{"error":{"message":"upstream stopped after billing","code":500}}`,
		)
	})
	response, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	var providerErr *llm.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Message != "upstream stopped after billing" {
		t.Fatalf("err = %v", err)
	}
	if response.Usage.PromptTokens != 17 || response.Usage.CompletionTokens != 23 ||
		!response.Usage.CostReported || response.Usage.CostMicrousd != 42 {
		t.Fatalf("usage lost with error: %+v", response.Usage)
	}
}

// A connection cut mid-stream is a clean EOF to the reader; without [DONE] or a
// finish_reason the text is a truncated draft, not an answer.
func TestComplete_StreamCutBeforeTheEndIsBadOutput(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"half a dra\"}}]}\n\n")
	})
	_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if !errors.Is(err, llm.ErrBadOutput) {
		t.Fatalf("err = %v, want ErrBadOutput", err)
	}
}

// Some servers end with a finish_reason and no [DONE]; that is a finished stream too.
func TestComplete_FinishReasonEndsTheStream(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"done\"},\"finish_reason\":\"stop\"}]}\n\n")
	})
	resp, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if err != nil || resp.Text != "done" || resp.FinishReason != "stop" {
		t.Fatalf("resp = %+v, err = %v", resp, err)
	}
}

func TestComplete_A404WithoutAModelMessageStaysGeneric(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `<html>404 page not found</html>`)
	})
	_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if errors.Is(err, llm.ErrModelUnavailable) {
		t.Fatal("a bare 404 (wrong base_url) was reported as a vanished model")
	}
	var perr *llm.ProviderError
	if !errors.As(err, &perr) || perr.Status != http.StatusNotFound {
		t.Fatalf("err = %v", err)
	}
}

func TestComplete_EmptyCompletionIsBadOutput(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"  "}}]}`)
	})
	_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if !errors.Is(err, llm.ErrBadOutput) {
		t.Fatalf("err = %v, want ErrBadOutput", err)
	}
}

func TestComplete_LengthWithoutContentIsOutputTruncatedAndKeepsUsage(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`{"choices":[{"delta":{"content":""},"finish_reason":"length"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":31,"completion_tokens":1234,"cost":0.125}}`,
		)
	})
	response, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	if !errors.Is(err, llm.ErrOutputTruncated) || errors.Is(err, llm.ErrBadOutput) {
		t.Fatalf("err = %v, want only ErrOutputTruncated", err)
	}
	if response.FinishReason != "length" || response.Usage.PromptTokens != 31 ||
		response.Usage.CompletionTokens != 1234 || !response.Usage.CostReported || response.Usage.CostMicrousd != 125_000 {
		t.Fatalf("response = %+v", response)
	}
	message := llm.UserMessage(err)
	for _, required := range []string{"출력 예산", "목표 길이", "다른 모델"} {
		if !strings.Contains(message, required) {
			t.Errorf("message = %q, want %q", message, required)
		}
	}
}

func TestComplete_HonoursTheContextDeadline(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Complete(ctx, llm.Request{Model: "m"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestFreeCallPinsZeroPriceLeafAndRefusesDriftBeforePost(t *testing.T) {
	const model = "vendor/free:free"
	var positive atomic.Bool
	var posts atomic.Int32
	client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			leaf := pricedEndpoint{Tag: "free-host", ModelID: model, SupportedParameters: []string{"max_tokens"}, Pricing: map[string]json.RawMessage{
				"prompt": json.RawMessage(`"0"`), "completion": json.RawMessage(`"0"`), "request": json.RawMessage(`"0"`),
			}}
			if positive.Load() {
				leaf.Pricing["request"] = json.RawMessage(`"0.01"`)
			}
			endpointResponse(w, model, leaf)
			return
		}
		posts.Add(1)
		var body struct {
			Provider strictRouting `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Provider.AllowFallbacks || !body.Provider.RequireParameters || len(body.Provider.Only) != 1 || body.Provider.Only[0] != "free-host" || body.Provider.MaxPrice.Prompt != "0" || body.Provider.MaxPrice.Completion != "0" {
			t.Errorf("unsafe free route: %+v", body.Provider)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"cost":0}}`))
	})
	req := llm.Request{Model: model, Stage: "write", MaxTokens: 100, FreeCall: true, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("hello")}}}}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	positive.Store(true)
	if _, err := client.Complete(context.Background(), req); !errors.Is(err, llm.ErrFreePathUnavailable) {
		t.Fatalf("price drift = %v", err)
	}
	if posts.Load() != 1 {
		t.Fatalf("price drift made %d posts", posts.Load())
	}
}

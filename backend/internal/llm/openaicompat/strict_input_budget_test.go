package openaicompat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestStrictWriterAllowanceGatesRouteAndWire(t *testing.T) {
	req := strictRequest()
	req.Stage, req.Execution.Call.Stage = "write", "write"
	req.MaxTokens, req.Execution.Call.CompletionTokens = 32768, 32768
	req.Execution.Delivery = llm.ExecutionTextOnly
	req.Execution.Call.InputTokens = llm.ClipPlanInputUnits
	req.Messages[0].Parts = []llm.Part{llm.TextPart(strings.Repeat("synthetic", 4100))}
	e := strictEndpointFixture(req.Model)
	var ok bool
	req.Execution.Call, ok = e.freeze(req.Execution.Call, req.Execution.Delivery)
	if !ok {
		t.Fatal("invalid price fixture")
	}
	for _, constraint := range []string{"prompt", "context"} {
		bounded := e
		limit := req.Execution.Call.InputTokenLimit() - 1
		if constraint == "prompt" {
			bounded.MaxPromptTokens = &limit
		} else {
			limit += req.MaxTokens
			bounded.ContextLength = &limit
		}
		if err := bounded.route(req.Execution.Call); !errors.Is(err, llm.ErrInlineEndpointUnavailable) {
			t.Fatal("under-capacity route accepted", err)
		}
		limit++
		if err := bounded.route(req.Execution.Call); err != nil {
			t.Fatal("exact capacity refused", err)
		}
	}
	posts, gets := 0, 0
	client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			gets++
			endpointResponse(w, req.Model, e)
			return
		}
		posts++
		data, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(data), strings.Repeat("synthetic", 4100)) {
			t.Error("input was truncated")
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":40000,"completion_tokens":2,"cost":0}}`)
	})
	legacy := req
	policy := *req.Execution
	legacy.Execution = &policy
	legacy.Execution.Call.InputTokens = 0
	if _, err := client.Complete(t.Context(), legacy); !errors.Is(err, llm.ErrUnsupported) || posts != 0 || gets != 0 {
		t.Fatal("legacy input admitted or dispatched", err)
	}
	if _, err := client.Complete(context.Background(), req); err != nil || posts != 1 {
		t.Fatal("new frozen writer allowance not honored", err)
	}
}

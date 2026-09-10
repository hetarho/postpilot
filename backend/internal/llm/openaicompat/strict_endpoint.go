package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

const maxEndpointTag = 128

var endpointTag = regexp.MustCompile(`^[a-z0-9][a-z0-9_/-]*$`)

type pricedEndpoint struct {
	Tag                 string                     `json:"tag"`
	ModelID             string                     `json:"model_id"`
	Pricing             map[string]json.RawMessage `json:"pricing"`
	SupportedParameters []string                   `json:"supported_parameters"`
	MaxCompletionTokens *int                       `json:"max_completion_tokens"`
}

// Discovery is bounded, read-only and never retried. Metadata is not a model call.
func (c *Client) priceEndpoints(ctx context.Context, client *http.Client, model string) (endpoints []pricedEndpoint, err error) {
	diagnostic := llm.CallDiagnostic{Operation: "metadata"}
	defer func() {
		if err != nil {
			err = strictDiagnostic(err, diagnostic)
		}
	}()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models/"+model+"/endpoints", nil)
	if err != nil {
		return nil, llm.ErrUnsupported
	}
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	request.Header.Set("Accept", "application/json")
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	diagnostic.HTTPStatus = resp.StatusCode
	diagnostic.RequestID = c.responseRequestID(resp.Header)
	if resp.StatusCode != http.StatusOK {
		return nil, llm.ErrUnsupported
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, clipLimits.EndpointBytes+1))
	if err != nil || int64(len(raw)) > clipLimits.EndpointBytes {
		if err != nil {
			diagnostic.Class = strictFailureClass(err, diagnostic)
		}
		if int64(len(raw)) > clipLimits.EndpointBytes {
			diagnostic.Class = "response_limit"
		}
		return nil, llm.ErrUnsupported
	}
	var response struct {
		Data struct {
			ID        string           `json:"id"`
			Endpoints []pricedEndpoint `json:"endpoints"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Data.ID != model || len(response.Data.Endpoints) == 0 || len(response.Data.Endpoints) > 128 {
		diagnostic.Class = "invalid_response"
		return nil, llm.ErrUnsupported
	}
	return response.Data.Endpoints, nil
}

// Parent slugs include variants. Select a unique leaf, not a broad parent that
// could route to an unquoted priority/region variant.
func uniqueLeaf(e pricedEndpoint, endpoints []pricedEndpoint) bool {
	matches := 0
	for _, other := range endpoints {
		if other.Tag == e.Tag {
			matches++
		} else if strings.HasPrefix(other.Tag, e.Tag+"/") {
			return false
		}
	}
	return matches == 1
}

func (c *Client) FreezePricing(ctx context.Context, call llm.CallPolicy, delivery llm.ExecutionDelivery) (llm.CallPolicy, error) {
	if c.reasoningFormat != "openrouter" || call.Ref.ProviderID != c.name || !strictModelID.MatchString(call.Ref.ModelID) || call.Ref.ModelID == "openrouter/auto" || (delivery != llm.ExecutionTextOnly && delivery != llm.ExecutionInlineStatic) || (delivery == llm.ExecutionInlineStatic && !c.VideoDelivery(call.Ref.ModelID).InlineStaticVideo) {
		return llm.CallPolicy{}, llm.ErrUnsupported
	}
	endpoints, err := c.priceEndpoints(ctx, c.strictHTTP(), call.Ref.ModelID)
	if err != nil {
		return llm.CallPolicy{}, err
	}
	slices.SortFunc(endpoints, func(a, b pricedEndpoint) int { return strings.Compare(a.Tag, b.Tag) })
	var selected llm.CallPolicy
	var cheapest int64
	for _, e := range endpoints {
		if !uniqueLeaf(e, endpoints) {
			continue
		}
		priced, ok := e.freeze(call, delivery)
		if !ok {
			continue
		}
		cost, ok := priced.QuoteMicrousd()
		if ok && (selected.Pricing.Version == 0 || cost < cheapest) {
			selected, cheapest = priced, cost
		}
	}
	if !selected.Pricing.Valid() {
		return llm.CallPolicy{}, llm.ErrUnsupported
	}
	return selected, nil
}

func (c *Client) strictEndpoint(ctx context.Context, client *http.Client, req llm.Request) (string, error) {
	endpoints, err := c.priceEndpoints(ctx, client, req.Model)
	if err != nil {
		return "", err
	}
	for _, e := range endpoints {
		if uniqueLeaf(e, endpoints) && e.eligible(req) {
			return e.Tag, nil
		}
	}
	return "", llm.ErrUnsupported
}

func (e pricedEndpoint) eligible(req llm.Request) bool {
	if req.Execution == nil || !req.Execution.Call.Pricing.Valid() {
		return false
	}
	p, ok := e.freeze(req.Execution.Call, req.Execution.Delivery)
	return ok && p == req.Execution.Call
}

func (e pricedEndpoint) supports(call llm.CallPolicy, delivery llm.ExecutionDelivery) bool {
	if len(e.Tag) == 0 || len(e.Tag) > maxEndpointTag || !endpointTag.MatchString(e.Tag) || e.ModelID != call.Ref.ModelID || call.CompletionTokens <= 0 || !call.Reasoning.Valid() {
		return false
	}
	if delivery == llm.ExecutionInlineStatic && (!strings.HasPrefix(e.ModelID, "google/gemini-") || (e.Tag != "google-ai-studio" && !strings.HasPrefix(e.Tag, "google-ai-studio/") && e.Tag != "google-vertex" && !strings.HasPrefix(e.Tag, "google-vertex/"))) {
		return false
	}
	for _, parameter := range []string{"max_tokens", "response_format", "structured_outputs"} {
		if !slices.Contains(e.SupportedParameters, parameter) {
			return false
		}
	}
	if (call.DisableReasoning || (call.Reasoning != llm.ReasoningUnspecified && call.Reasoning != llm.ReasoningUnset)) && !slices.Contains(e.SupportedParameters, "reasoning") {
		return false
	}
	return e.MaxCompletionTokens == nil || *e.MaxCompletionTokens >= call.CompletionTokens
}

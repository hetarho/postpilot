package openaicompat

import (
	"context"
	"slices"

	"github.com/postpilot/backend/internal/llm"
)

func (c *Client) HasFreePath(ctx context.Context, model string, path llm.FreePath) (bool, error) {
	if c.reasoningFormat != "openrouter" || !strictModelID.MatchString(model) {
		return false, nil
	}
	endpoints, err := c.endpoints(ctx, c.strictHTTP(), model)
	if err != nil {
		return false, err
	}
	for _, endpoint := range endpoints {
		if freeLeaf(endpoint, endpoints, model, path) {
			return true, nil
		}
	}
	return false, nil
}

func freeLeaf(endpoint pricedEndpoint, all []pricedEndpoint, model string, path llm.FreePath) bool {
	return endpoint.ModelID == model && endpoint.Tag != "" && len(endpoint.Tag) <= maxEndpointTag &&
		endpointTag.MatchString(endpoint.Tag) && uniqueLeaf(endpoint, all) &&
		(endpoint.Status == nil || *endpoint.Status >= 0) && llm.FreePriceProfile(endpoint.Pricing, path)
}

func (c *Client) freeEndpoint(ctx context.Context, req llm.Request) (string, error) {
	if c.reasoningFormat != "openrouter" || !strictModelID.MatchString(req.Model) || req.Model == "openrouter/auto" {
		return "", llm.ErrUnsupported
	}
	path := llm.FreeText
	if req.HasVideos() {
		path = llm.FreeVideoInput
	} else if req.HasImages() {
		path = llm.FreeImageInput
	}
	endpoints, err := c.endpoints(ctx, c.strictHTTP(), req.Model)
	if err != nil {
		return "", err
	}
	needed := []string{"max_tokens"}
	if len(req.JSONSchema) > 0 {
		needed = append(needed, "response_format", "structured_outputs")
	}
	if req.DisableReasoning || req.Reasoning != llm.ReasoningUnspecified && req.Reasoning != llm.ReasoningUnset {
		needed = append(needed, "reasoning")
	}
	for _, endpoint := range endpoints {
		if !freeLeaf(endpoint, endpoints, req.Model, path) || endpoint.MaxCompletionTokens != nil && *endpoint.MaxCompletionTokens < req.MaxTokens {
			continue
		}
		eligible := true
		for _, parameter := range needed {
			if !slices.Contains(endpoint.SupportedParameters, parameter) {
				eligible = false
				break
			}
		}
		if eligible {
			return endpoint.Tag, nil
		}
	}
	return "", llm.ErrFreePathUnavailable
}

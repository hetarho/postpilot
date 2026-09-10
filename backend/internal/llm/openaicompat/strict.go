package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// Code-owned safety ceilings, independent of the public RPC body limit. The
// metadata bound also applies to a planner's structured, text-only input.
type strictLimits struct {
	InlineBytes, RequestBytes, MetadataBytes, ResponseBytes, EndpointBytes int64
}

var clipLimits = strictLimits{8 << 20, 12 << 20, 1 << 20, 4 << 20, 1 << 20}

// Documented profiles only. The raw modality remains untouched. In particular,
// Gemini accepts inline MP4, but its URL support is YouTube-only (AI Studio), or
// absent (Vertex): neither is an ordinary signed object URL. See testdata/README.md.
func (c *Client) VideoDelivery(model string) llm.VideoDelivery {
	return llm.VideoDelivery{InlineStaticVideo: c.reasoningFormat == "openrouter" && strings.HasPrefix(model, "google/gemini-") && strictModelID.MatchString(model)}
}

var strictModelID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*/[a-zA-Z0-9][a-zA-Z0-9._-]*(?::free)?$`)

type strictRouting struct {
	AllowFallbacks    bool         `json:"allow_fallbacks"`
	RequireParameters bool         `json:"require_parameters"`
	Only              []string     `json:"only"`
	Order             []string     `json:"order"`
	MaxPrice          strictPrices `json:"max_price"`
}

type strictPrices struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Request    string `json:"request"`
	Image      string `json:"image"`
	Audio      string `json:"audio"`
}

func (c *Client) strictEnvelope(req llm.Request, endpoint string, limits strictLimits) ([]byte, *llm.InlineVideo, error) {
	policy := req.Execution
	if policy == nil || c.reasoningFormat != "openrouter" || !strictModelID.MatchString(req.Model) || req.Model == "openrouter/auto" || !policy.Matches(llm.ModelRef{ProviderID: c.name, ModelID: req.Model}, req) {
		return nil, nil, llm.ErrUnsupported
	}
	if !policy.Call.Pricing.Valid() || policy.Call.Pricing.Delivery != policy.Delivery {
		return nil, nil, llm.ErrUnsupported
	}
	var video *llm.InlineVideo
	parts := 0
	metadata := int64(len(req.System)) + int64(len(req.Model)) + int64(len(req.JSONSchema))
	for _, m := range req.Messages {
		metadata += int64(len(m.Role))
		for _, part := range m.Parts {
			parts++
			metadata += int64(len(part.Text))
			if part.InlineVideo != nil {
				video = part.InlineVideo
			}
		}
	}
	if metadata > limits.MetadataBytes {
		return nil, nil, llm.ErrUnsupported
	}
	if video != nil && (video.Size > limits.InlineBytes || !c.VideoDelivery(req.Model).InlineStaticVideo) {
		return nil, nil, llm.ErrUnsupported
	}
	// Bound unencoded text/schema bytes conservatively within the input allowance.
	// At most 60 static 1-FPS frames plus audio fit inside the 20k media allowance;
	// leave an additional 1k units for message/schema wrapper tokens. No truncation.
	inputLimit := int64(llm.ClipInputUnits - 1024)
	if video != nil {
		inputLimit -= 20_000
	}
	if metadata > inputLimit || len(req.Messages) > 8 || parts > 16 {
		return nil, nil, llm.ErrUnsupported
	}
	wire := c.buildRequest(req)
	prices := policy.Call.Pricing
	wire.Provider = &strictRouting{RequireParameters: true, Only: []string{endpoint}, Order: []string{endpoint}, MaxPrice: strictPrices{
		Prompt: wirePrice(prices.PromptUSDPerMillion), Completion: wirePrice(prices.CompletionUSDPerMillion),
		Request: wirePrice(prices.RequestUSD), Image: wirePrice(prices.ImageUSD), Audio: wirePrice(prices.AudioUSDPerToken),
	}}
	body, err := json.Marshal(wire)
	if err != nil || int64(len(body)) > limits.MetadataBytes {
		return nil, nil, llm.ErrUnsupported
	}
	size := int64(len(body))
	if video != nil {
		size += (video.Size + 2) / 3 * 4
	}
	if size > limits.RequestBytes {
		return nil, nil, llm.ErrUnsupported
	}
	return body, video, nil
}

func (c *Client) strictHTTP() *http.Client {
	client := *c.http
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}

func (c *Client) completeStrict(ctx context.Context, req llm.Request) (out llm.Response, err error) {
	// Even a metadata GET must not precede rejection of an oversized payload. A
	// maximum-length endpoint tag makes this bound conservative before discovery.
	if _, _, err = c.strictEnvelope(req, strings.Repeat("x", maxEndpointTag), clipLimits); err != nil {
		return out, err
	}
	defer func() {
		if err != nil {
			out.Text = ""
			err = sanitizedStrictError(err)
		}
	}()
	client := c.strictHTTP()
	endpoint, err := c.strictEndpoint(ctx, client, req)
	if err != nil {
		return out, err
	}
	envelope, video, err := c.strictEnvelope(req, endpoint, clipLimits)
	if err != nil {
		return out, err
	}
	body, size, err := streamStrictBody(ctx, envelope, video)
	if err != nil {
		return out, err
	}
	defer body.Close()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", body)
	if err != nil {
		return out, err
	}
	httpReq.ContentLength = size
	// A non-replayable body and no idempotency key prevent Transport from retrying
	// POST even on a reused connection; 307/308 are not followed either.
	httpReq.GetBody = nil
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream, application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// A failed HTTP response may still carry purchased usage. Read it once,
		// bounded, preserve evidence and sanitize the separately mapped failure.
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, clipLimits.ResponseBytes+1))
		if readErr != nil || int64(len(raw)) > clipLimits.ResponseBytes {
			return out, llm.ErrBadOutput
		}
		out, _ = c.readJSON(bytes.NewReader(raw))
		copy := *resp
		copy.Body = io.NopCloser(bytes.NewReader(raw))
		return out, c.httpError(&copy)
	}
	limited := &io.LimitedReader{R: resp.Body, N: clipLimits.ResponseBytes + 1}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		out, err = c.readStream(limited)
	} else {
		var raw []byte
		raw, err = io.ReadAll(limited)
		if err == nil && int64(len(raw)) <= clipLimits.ResponseBytes {
			out, err = c.readJSON(bytes.NewReader(raw))
		}
	}
	if limited.N == 0 {
		return out, llm.ErrBadOutput
	}
	if err != nil {
		return out, err
	}
	if out.FinishReason == "length" {
		return out, &llm.TruncatedError{ReasoningTokens: out.Usage.ReasoningTokens, CompletionTokens: out.Usage.CompletionTokens}
	}
	if strings.TrimSpace(out.Text) == "" {
		return out, llm.ErrBadOutput
	}
	return out, nil
}

func sanitizedStrictError(err error) error {
	var truncated *llm.TruncatedError
	if errors.As(err, &truncated) {
		return truncated
	}
	var provider *llm.ProviderError
	if errors.As(err, &provider) {
		return &llm.ProviderError{Status: provider.Status, Message: "provider rejected the bounded request", Kind: provider.Kind}
	}
	for _, kind := range []error{context.Canceled, context.DeadlineExceeded, llm.ErrUnsupported, llm.ErrBadOutput, llm.ErrRateLimited, llm.ErrModelUnavailable} {
		if errors.Is(err, kind) {
			return kind
		}
	}
	return fmt.Errorf("bounded provider request failed")
}

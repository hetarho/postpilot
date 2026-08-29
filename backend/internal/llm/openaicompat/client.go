// Package openaicompat is the adapter for every provider that speaks the OpenAI
// chat-completions protocol — OpenRouter, Together, Groq, Fireworks, DeepInfra, OpenAI,
// Ollama, vLLM, LM Studio, Google's compatible endpoint (PRD §6.4). Only the base URL
// differs between them.
//
// It is imported by the composition root alone (cmd/api); nothing above internal/llm
// sees it (ARCHITECTURE §2.1, enforced by llm's boundary test).
package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// maxErrorBody bounds how much of an error response is read for its message.
const maxErrorBody = 64 << 10

// maxSSELine bounds one server-sent event line. A chunk is one token's worth of JSON
// normally, but a provider that does not stream sends the whole completion as one
// line, and a long draft has to fit.
const maxSSELine = 4 << 20

// Client implements llm.Provider over POST {base_url}/chat/completions.
type Client struct {
	name    string
	baseURL string
	apiKey  string
	http    *http.Client
}

// Factory is the llm.AdapterFactory for `adapter: openai_compatible`.
func Factory(cfg llm.AdapterConfig) (llm.Provider, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("openai_compatible: base_url is required")
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("openai_compatible: base_url must be an http(s) URL, got %q", cfg.BaseURL)
	}
	return New(cfg, nil), nil
}

// New builds a client. A nil httpClient means http.DefaultClient; tests pass an
// httptest server's client. Per-call deadlines come from the context the registry
// sets, so the client itself carries no timeout.
func New(cfg llm.AdapterConfig, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		name:    cfg.ProviderID,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		http:    httpClient,
	}
}

func (c *Client) Name() string { return c.name }

// Complete runs one chat completion. The response is requested as a stream and joined
// here: a long draft can take minutes, and an idle connection with no bytes flowing is
// what intermediaries cut (PRD §6.6). The stream never leaves the process.
func (c *Client) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	body, err := json.Marshal(buildRequest(req))
	if err != nil {
		return llm.Response{}, fmt.Errorf("%s: encode request: %w", c.name, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return llm.Response{}, fmt.Errorf("%s: build request: %w", c.name, err)
	}
	// A keyless endpoint (a local Ollama) gets no header at all rather than "Bearer ".
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream, application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return llm.Response{}, fmt.Errorf("%s: %w", c.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return llm.Response{}, c.httpError(resp)
	}

	var out llm.Response
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		out, err = c.readStream(resp.Body)
	} else {
		// A provider that ignores `stream: true` answers with the whole completion.
		out, err = c.readJSON(resp.Body)
	}
	if err != nil {
		return llm.Response{}, err
	}
	if strings.TrimSpace(out.Text) == "" {
		return llm.Response{}, fmt.Errorf("%w: %s returned an empty completion", llm.ErrBadOutput, c.name)
	}
	return out, nil
}

// --- request shape ---

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Stream         bool            `json:"stream"`
	StreamOptions  *streamOptions  `json:"stream_options,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role string `json:"role"`
	// Content is a string for a text-only message and a part list when an image is
	// present — the string form is what every compatible server accepts.
	Content any `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

func buildRequest(req llm.Request) chatRequest {
	messages := make([]chatMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		messages = append(messages, chatMessage{Role: string(m.Role), Content: content(m.Parts)})
	}
	out := chatRequest{
		Model:         req.Model,
		Messages:      messages,
		MaxTokens:     req.MaxTokens,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}
	if req.JSONSchema != nil {
		// `strict` is deliberately not set: strict mode demands every property be required
		// and additionalProperties=false, and the schemas belong to the callers.
		out.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: jsonSchema{Name: "response", Schema: req.JSONSchema},
		}
	}
	return out
}

func content(parts []llm.Part) any {
	// A text-only message goes as a string: it is the one form every compatible server
	// accepts, and an empty part list must be "" rather than [] for the same reason.
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 && !parts[0].IsImage() {
		return parts[0].Text
	}
	out := make([]contentPart, 0, len(parts))
	for _, p := range parts {
		if p.IsImage() {
			mime := p.MIME
			if mime == "" {
				mime = "image/jpeg"
			}
			out = append(out, contentPart{
				Type:     "image_url",
				ImageURL: &imageURL{URL: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(p.Image)},
			})
			continue
		}
		out = append(out, contentPart{Type: "text", Text: p.Text})
	}
	return out
}

// --- response shape ---

type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int             `json:"prompt_tokens"`
		CompletionTokens int             `json:"completion_tokens"`
		Cost             json.RawMessage `json:"cost"`
	} `json:"usage"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
	Code    any    `json:"code"`
}

func (c *Client) readStream(body io.Reader) (llm.Response, error) {
	var out llm.Response
	var text strings.Builder
	finished := false
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), maxSSELine)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("data:")) {
			// Comments (": OPENROUTER PROCESSING"), event names, blank separators.
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 {
			continue
		}
		if bytes.Equal(data, []byte("[DONE]")) {
			finished = true
			break
		}
		var chunk chatChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return llm.Response{}, fmt.Errorf("%w: %s sent an unreadable stream chunk: %v", llm.ErrBadOutput, c.name, err)
		}
		if err := c.apply(chunk, &text, &out); err != nil {
			return llm.Response{}, err
		}
		for _, choice := range chunk.Choices {
			finished = finished || choice.FinishReason != ""
		}
	}
	if err := scanner.Err(); err != nil {
		return llm.Response{}, fmt.Errorf("%s: read stream: %w", c.name, err)
	}
	// A connection cut mid-stream looks like a clean EOF. Without a terminal marker —
	// `[DONE]` or a `finish_reason` — the text collected so far is a truncated draft, and
	// handing it on as the answer would be worse than failing.
	if !finished {
		return llm.Response{}, fmt.Errorf("%w: %s stream ended before the completion finished", llm.ErrBadOutput, c.name)
	}
	out.Text = text.String()
	return out, nil
}

func (c *Client) readJSON(body io.Reader) (llm.Response, error) {
	var chunk chatChunk
	if err := json.NewDecoder(body).Decode(&chunk); err != nil {
		return llm.Response{}, fmt.Errorf("%w: %s sent an unreadable response: %v", llm.ErrBadOutput, c.name, err)
	}
	var out llm.Response
	var text strings.Builder
	if err := c.apply(chunk, &text, &out); err != nil {
		return llm.Response{}, err
	}
	out.Text = text.String()
	return out, nil
}

// apply folds one chunk — a stream delta or a whole non-streamed answer — into the
// text and usage being collected. The two shapes differ only in where the content sits.
func (c *Client) apply(chunk chatChunk, text *strings.Builder, out *llm.Response) error {
	// A provider can fail mid-stream (a quota reached after the headers went out) and
	// says so inside the body rather than with a status code.
	if chunk.Error != nil {
		return &llm.ProviderError{Provider: c.name, Status: http.StatusOK, Message: chunk.Error.Message, Kind: kindOfMessage(chunk.Error)}
	}
	for _, choice := range chunk.Choices {
		text.WriteString(choice.Message.Content)
		text.WriteString(choice.Delta.Content)
	}
	if chunk.Usage != nil {
		out.Usage.PromptTokens = chunk.Usage.PromptTokens
		out.Usage.CompletionTokens = chunk.Usage.CompletionTokens
		if len(chunk.Usage.Cost) > 0 && !bytes.Equal(bytes.TrimSpace(chunk.Usage.Cost), []byte("null")) {
			cost, err := decimalUSDTomicrousd(chunk.Usage.Cost)
			if err != nil {
				return fmt.Errorf("%w: %s sent invalid usage.cost: %v", llm.ErrBadOutput, c.name, err)
			}
			out.Usage.CostMicrousd = cost
			out.Usage.CostReported = true
		}
	}
	return nil
}

func decimalUSDTomicrousd(raw []byte) (int64, error) {
	value := strings.TrimSpace(string(raw))
	if strings.HasPrefix(value, "\"") {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return 0, err
		}
		value = decoded
	}
	amount, ok := new(big.Rat).SetString(value)
	if !ok || amount.Sign() < 0 {
		return 0, fmt.Errorf("must be a non-negative decimal")
	}
	amount.Mul(amount, big.NewRat(1_000_000, 1))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(amount.Num(), amount.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(amount.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("overflows int64 microusd")
	}
	return quotient.Int64(), nil
}

// --- error mapping ---

// modelNotFound matches the phrasings compatible servers use for an unknown model in a
// 400/404 body. A vanished free model lands here (PRD §6.5). Kept narrow on purpose: a
// 400 about `max_tokens` or `response_format` that happens to mention the word "model"
// must stay a generic error, or the user is told to re-pick a model that is fine.
var modelNotFound = regexp.MustCompile(`(?i)\bmodel\b[^.]{0,40}\b(not found|does not exist|not exist)\b|\b(no such|unknown) model\b|\binvalid model (id|name)\b|\bmodel not found\b|\bno endpoints found\b`)

func (c *Client) httpError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	message := strings.TrimSpace(string(raw))
	var envelope struct {
		Error *apiError `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) == nil && envelope.Error != nil && envelope.Error.Message != "" {
		message = envelope.Error.Message
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	var kind error
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		kind = llm.ErrRateLimited
	// A 404 alone is not "the model is gone": a wrong base_url or a gateway page is a 404
	// too. Only a body that says so means the model.
	case (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest) && modelNotFound.MatchString(message):
		kind = llm.ErrModelUnavailable
	}
	return &llm.ProviderError{Provider: c.name, Status: resp.StatusCode, Message: message, Kind: kind}
}

// kindOfMessage classifies an error delivered inside a 200 body, where there is no
// status to go by. OpenRouter puts the upstream status in `code`.
func kindOfMessage(e *apiError) error {
	if code, ok := e.Code.(float64); ok {
		switch int(code) {
		case http.StatusTooManyRequests:
			return llm.ErrRateLimited
		case http.StatusNotFound:
			return llm.ErrModelUnavailable
		}
	}
	if modelNotFound.MatchString(e.Message) {
		return llm.ErrModelUnavailable
	}
	if strings.Contains(strings.ToLower(e.Message), "rate limit") {
		return llm.ErrRateLimited
	}
	return nil
}

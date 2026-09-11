package openaicompat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Documented on 2026-09-12 against the official OpenRouter references named in
// testdata/README.md: the endpoint listing (`GET /models/{id}/endpoints`, one
// `PublicEndpoint` per provider leaf with `tag`, `model_id`, `status`,
// `supported_parameters`, `pricing`, `context_length`, `max_prompt_tokens`,
// `max_completion_tokens`), provider routing (`provider.only` / `order`,
// `allow_fallbacks`, `require_parameters`, string-valued `max_price`) and the
// video-input guide (`video_url` with an optional `processing` mode that only a
// documented Gemini profile honours).
const maxEndpointTag = llm.MaxEndpointTag

var endpointTag = regexp.MustCompile(`^[a-z0-9][a-z0-9_/-]*$`)

// Code-owned bounds for endpoint discovery: how many documents may be read at
// once, and the defaults for the catalog TTL and fetch timeout an adapter is
// built without.
const (
	endpointFetchConcurrency   = 4
	defaultEndpointCacheTTL    = 5 * time.Minute
	defaultEndpointFetchTimout = 15 * time.Second
)

type pricedEndpoint struct {
	Tag                 string                     `json:"tag"`
	ModelID             string                     `json:"model_id"`
	Pricing             map[string]json.RawMessage `json:"pricing"`
	SupportedParameters []string                   `json:"supported_parameters"`
	MaxCompletionTokens *int                       `json:"max_completion_tokens"`
	MaxPromptTokens     *int                       `json:"max_prompt_tokens"`
	ContextLength       *int                       `json:"context_length"`
	// Status is OpenRouter's health integer: 0 healthy, negative degraded. It is
	// read for admission and left out of the fingerprint, which pins the profile,
	// not the weather.
	Status *int `json:"status"`
}

// fingerprint hashes the route and its billing profile: tag, model, prices,
// the parameter set in a normalized order and the token limits. Latency,
// uptime and status are deliberately absent.
func (e pricedEndpoint) fingerprint() (string, bool) {
	canonical := struct {
		Tag                 string                     `json:"tag"`
		ModelID             string                     `json:"model_id"`
		Pricing             map[string]json.RawMessage `json:"pricing"`
		SupportedParameters []string                   `json:"supported_parameters"`
		MaxCompletionTokens *int                       `json:"max_completion_tokens"`
		MaxPromptTokens     *int                       `json:"max_prompt_tokens"`
		ContextLength       *int                       `json:"context_length"`
	}{e.Tag, e.ModelID, e.Pricing, slices.Sorted(slices.Values(e.SupportedParameters)), e.MaxCompletionTokens, e.MaxPromptTokens, e.ContextLength}
	raw, err := json.Marshal(canonical)
	if err != nil {
		return "", false
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), true
}

// endpointDocument is one successfully read listing and when it was read.
type endpointDocument struct {
	endpoints []pricedEndpoint
	fetched   time.Time
}

type endpointFetch struct {
	done      chan struct{}
	endpoints []pricedEndpoint
	err       error
}

// endpointCache serves qualification and the pre-completion recheck from one
// bounded reader: a successful document is reused for the catalog TTL, an
// expired or failed one never is, concurrent readers of one model share a
// single fetch, and at most endpointFetchConcurrency fetches are in flight.
type endpointCache struct {
	mu       sync.Mutex
	docs     map[string]endpointDocument
	inflight map[string]*endpointFetch
	sem      chan struct{}
	ttl      time.Duration
	timeout  time.Duration
	now      func() time.Time
}

func newEndpointCache(ttl, timeout time.Duration) *endpointCache {
	if ttl <= 0 {
		ttl = defaultEndpointCacheTTL
	}
	if timeout <= 0 {
		timeout = defaultEndpointFetchTimout
	}
	return &endpointCache{docs: map[string]endpointDocument{}, inflight: map[string]*endpointFetch{}, sem: make(chan struct{}, endpointFetchConcurrency), ttl: ttl, timeout: timeout, now: time.Now}
}

// endpoints returns the current document for a model, reading it at most once
// per TTL. The caller's context cancels only the caller's wait; a fetch that
// other callers share runs to its own timeout.
func (c *Client) endpoints(ctx context.Context, client *http.Client, model string) ([]pricedEndpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cache := c.endpointDocs
	cache.mu.Lock()
	if doc, ok := cache.docs[model]; ok && cache.now().Sub(doc.fetched) < cache.ttl {
		cache.mu.Unlock()
		return slices.Clone(doc.endpoints), nil
	}
	fetch, ok := cache.inflight[model]
	if !ok {
		fetch = &endpointFetch{done: make(chan struct{})}
		cache.inflight[model] = fetch
		go c.fetchEndpoints(client, model, fetch)
	}
	cache.mu.Unlock()
	select {
	case <-fetch.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if fetch.err != nil {
		return nil, fetch.err
	}
	return slices.Clone(fetch.endpoints), nil
}

func (c *Client) fetchEndpoints(client *http.Client, model string, fetch *endpointFetch) {
	cache := c.endpointDocs
	defer func() {
		cache.mu.Lock()
		delete(cache.inflight, model)
		if fetch.err == nil {
			cache.docs[model] = endpointDocument{endpoints: fetch.endpoints, fetched: cache.now()}
		}
		cache.mu.Unlock()
		close(fetch.done)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), cache.timeout)
	defer cancel()
	select {
	case cache.sem <- struct{}{}:
		defer func() { <-cache.sem }()
	case <-ctx.Done():
		fetch.err = ctx.Err()
		return
	}
	fetch.endpoints, fetch.err = c.priceEndpoints(ctx, client, model)
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

// unavailable restates a document that could not be read as the admission
// failure it is — no current proof, no eligible route — keeping only the safe
// diagnostic (operation, class, status), never the body or the URL.
func unavailable(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	if diagnostic, ok := llm.DiagnosticOf(err); ok {
		return llm.WithCallDiagnostic(llm.ErrInlineEndpointUnavailable, diagnostic)
	}
	return llm.ErrInlineEndpointUnavailable
}

func (c *Client) strictCall(call llm.CallPolicy, delivery llm.ExecutionDelivery) bool {
	return c.reasoningFormat == "openrouter" && call.Ref.ProviderID == c.name && strictModelID.MatchString(call.Ref.ModelID) && call.Ref.ModelID != "openrouter/auto" && (delivery == llm.ExecutionTextOnly || delivery == llm.ExecutionInlineStatic)
}

// FreezePricing qualifies every current provider leaf of the model for the
// exact request the policy describes and freezes the one it will route to. No
// provider slug or model family is consulted: the document, the request shape
// and the price policy decide (CLIP-30). The failure names the furthest check
// any leaf reached, so a model with a route but no schema support is
// "parameters", not "endpoint".
func (c *Client) FreezePricing(ctx context.Context, call llm.CallPolicy, delivery llm.ExecutionDelivery) (llm.CallPolicy, error) {
	if !c.strictCall(call, delivery) {
		return llm.CallPolicy{}, llm.ErrUnsupported
	}
	endpoints, err := c.endpoints(ctx, c.strictHTTP(), call.Ref.ModelID)
	if err != nil {
		return llm.CallPolicy{}, unavailable(err)
	}
	return selectEndpoint(endpoints, call, delivery)
}

// selectEndpoint is deterministic: leaves are read in tag order, the cheapest
// fully qualified one wins and a tie keeps the first.
func selectEndpoint(endpoints []pricedEndpoint, call llm.CallPolicy, delivery llm.ExecutionDelivery) (llm.CallPolicy, error) {
	endpoints = slices.Clone(endpoints)
	slices.SortStableFunc(endpoints, func(a, b pricedEndpoint) int { return strings.Compare(a.Tag, b.Tag) })
	var selected llm.CallPolicy
	var cheapest int64
	furthest := error(llm.ErrInlineEndpointUnavailable)
	for _, e := range endpoints {
		if !uniqueLeaf(e, endpoints) {
			continue
		}
		priced, err := e.qualify(call, delivery)
		if err != nil {
			furthest = further(furthest, err)
			continue
		}
		cost, ok := priced.QuoteMicrousd()
		if ok && (selected.Pricing.Version == 0 || cost < cheapest) {
			selected, cheapest = priced, cost
		}
	}
	if !selected.Pricing.Valid() {
		return llm.CallPolicy{}, furthest
	}
	return selected, nil
}

// further keeps whichever of two admission failures got past more checks:
// endpoint < parameters < price.
func further(current, candidate error) error {
	rank := func(err error) int {
		switch {
		case errors.Is(err, llm.ErrPriceCeilingUnavailable):
			return 3
		case errors.Is(err, llm.ErrRequiredParametersUnsupported):
			return 2
		default:
			return 1
		}
	}
	if rank(candidate) > rank(current) {
		return candidate
	}
	return current
}

// strictEndpoint is the pre-completion recheck: the frozen leaf must still be
// listed, still unique, and still qualify to exactly the frozen policy. Any
// drift refuses the call — no retry, no other leaf, no relaxed limit.
func (c *Client) strictEndpoint(ctx context.Context, client *http.Client, req llm.Request) (string, error) {
	if req.Execution == nil || !req.Execution.Call.Pricing.Valid() {
		return "", llm.ErrUnsupported
	}
	frozen := req.Execution.Call
	endpoints, err := c.endpoints(ctx, client, req.Model)
	if err != nil {
		return "", unavailable(err)
	}
	for _, e := range endpoints {
		if e.Tag != frozen.Pricing.Endpoint {
			continue
		}
		if !uniqueLeaf(e, endpoints) {
			return "", llm.ErrInlineEndpointUnavailable
		}
		current, err := e.qualify(frozen, req.Execution.Delivery)
		if err != nil {
			return "", err
		}
		if current != frozen {
			return "", llm.ErrPriceCeilingUnavailable
		}
		return e.Tag, nil
	}
	return "", llm.ErrInlineEndpointUnavailable
}

// eligible reports whether this leaf still qualifies to exactly the request's
// frozen policy.
func (e pricedEndpoint) eligible(req llm.Request) bool {
	if req.Execution == nil || !req.Execution.Call.Pricing.Valid() {
		return false
	}
	p, err := e.qualify(req.Execution.Call, req.Execution.Delivery)
	return err == nil && p == req.Execution.Call
}

// route is the first check: a well-formed, healthy leaf for this exact model
// whose limits hold the bounded request. The listing says nothing about video
// per leaf, so the catalog's modality (checked before any read) plus a current
// leaf that can take the request's size is what "a bounded inline route" means.
func (e pricedEndpoint) route(call llm.CallPolicy) error {
	if call.CompletionTokens <= 0 || !call.Reasoning.Valid() {
		return llm.ErrUnsupported
	}
	if len(e.Tag) == 0 || len(e.Tag) > maxEndpointTag || !endpointTag.MatchString(e.Tag) || e.ModelID != call.Ref.ModelID {
		return llm.ErrInlineEndpointUnavailable
	}
	if e.Status != nil && *e.Status < 0 {
		return llm.ErrInlineEndpointUnavailable
	}
	if e.MaxCompletionTokens != nil && *e.MaxCompletionTokens < call.CompletionTokens {
		return llm.ErrInlineEndpointUnavailable
	}
	if e.MaxPromptTokens != nil && *e.MaxPromptTokens < llm.ClipInputUnits {
		return llm.ErrInlineEndpointUnavailable
	}
	if e.ContextLength != nil && *e.ContextLength > 0 && *e.ContextLength < llm.ClipInputUnits+call.CompletionTokens {
		return llm.ErrInlineEndpointUnavailable
	}
	return nil
}

// parameters is the second check: every parameter the actual request sends.
func (e pricedEndpoint) parameters(call llm.CallPolicy) error {
	for _, parameter := range call.RequiredParameters() {
		if !slices.Contains(e.SupportedParameters, parameter) {
			return llm.ErrRequiredParametersUnsupported
		}
	}
	return nil
}

// qualify runs the three checks in order and freezes the leaf into the policy.
func (e pricedEndpoint) qualify(call llm.CallPolicy, delivery llm.ExecutionDelivery) (llm.CallPolicy, error) {
	if err := e.route(call); err != nil {
		return llm.CallPolicy{}, err
	}
	if err := e.parameters(call); err != nil {
		return llm.CallPolicy{}, err
	}
	priced, ok := e.freeze(call, delivery)
	if !ok {
		return llm.CallPolicy{}, llm.ErrPriceCeilingUnavailable
	}
	return priced, nil
}

// Package openrouter reads the upstream model catalog. It is the modelcatalog context's
// outbound adapter: plain HTTP against a public JSON document, mapped into the context's
// own Candidate type at this edge so no upstream field name travels inward.
//
// It is deliberately NOT an llm adapter. Nothing here calls a model — it reads the list of
// models that exist — so the provider boundary (ARCHITECTURE §2.1) is untouched and no SDK
// is involved.
package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/modelcatalog"
)

// maxBodyBytes bounds the response read. The real document is ~1.5 MB for ~420 models;
// the cap exists so a wrong base_url pointing at something enormous cannot exhaust memory.
const maxBodyBytes int64 = 32 << 20

// requestedOutputModalities is the one query the catalog is read with, and it is not
// optional: the endpoint DEFAULTS TO TEXT-OUTPUT MODELS ONLY, so a request without it
// cannot see a single video model and sees only the image models that also answer in text.
// The three values are exactly what the five curated purposes need — text for photo
// analysis, style analysis and writing; image and video for the two generation purposes.
const requestedOutputModalities = "text,image,video"

// Client fetches and caches the upstream catalog.
//
// The cache is what keeps the admin screen off the network for every keystroke of a
// filter, and it is bypassed only by an explicit refresh. No user-facing path reaches this
// type at all: users read curated rows from the database, so an upstream outage cannot
// change what they see.
type Client struct {
	baseURL string
	http    *http.Client
	ttl     time.Duration
	now     func() time.Time

	mu        sync.Mutex
	cached    []modelcatalog.Candidate
	fetchedAt time.Time
}

// New returns a catalog client for the registered endpoint's base URL.
func New(baseURL string, timeout, ttl time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
		ttl:     ttl,
		now:     time.Now,
	}
}

// Fetch returns the upstream catalog, from cache when it is still fresh and `refresh` is
// false. A failed fetch returns the error and leaves any cached snapshot untouched — the
// caller degrades to stored rows rather than showing an empty catalog.
func (c *Client) Fetch(ctx context.Context, refresh bool) (modelcatalog.Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !refresh && c.cached != nil && c.now().Sub(c.fetchedAt) < c.ttl {
		return modelcatalog.Snapshot{
			Candidates: append([]modelcatalog.Candidate(nil), c.cached...),
			FetchedAt:  c.fetchedAt,
			FromCache:  true,
		}, nil
	}

	candidates, err := c.get(ctx)
	if err != nil {
		return modelcatalog.Snapshot{}, err
	}
	c.cached = candidates
	c.fetchedAt = c.now()
	return modelcatalog.Snapshot{
		Candidates: append([]modelcatalog.Candidate(nil), candidates...),
		FetchedAt:  c.fetchedAt,
	}, nil
}

// The upstream shape, narrowed to the fields the product consumes. Unknown fields are
// ignored on purpose — this document is the provider's schema, not ours, and it grows
// without notice; the strictness that protects providers.yaml would only turn their
// release into our outage.
type modelsDocument struct {
	Data []modelDocument `json:"data"`
}

type modelDocument struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Created      int64  `json:"created"`
	ContextLen   int64  `json:"context_length"`
	Architecture struct {
		InputModalities  []string `json:"input_modalities"`
		OutputModalities []string `json:"output_modalities"`
	} `json:"architecture"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
		// The image-token prices, which are what a model answering in images charges
		// instead of the text pair above.
		ImageToken  string `json:"image_token"`
		ImageOutput string `json:"image_output"`
	} `json:"pricing"`
	SupportedParameters []string `json:"supported_parameters"`
	// A POINTER so "the object is absent" and "the object is present with everything false"
	// stay distinguishable: absent means the model does not reason at all, while present
	// with `mandatory: false` means it reasons and can be turned off. Only ~300 of 427
	// entries carry it, and only ~154 of those publish supported_efforts.
	Reasoning *struct {
		Mandatory bool     `json:"mandatory"`
		Supported []string `json:"supported_efforts"`
		Default   string   `json:"default_effort"`
		// DefaultEnabled is read but deliberately not persisted: it is display context
		// change 27 does not scope.
		DefaultEnabled  bool `json:"default_enabled"`
		SupportsMaxToks bool `json:"supports_max_tokens"`
	} `json:"reasoning"`
}

func (c *Client) get(ctx context.Context) ([]modelcatalog.Candidate, error) {
	endpoint := c.baseURL + "/models?output_modalities=" + requestedOutputModalities
	// No Authorization header: the list is public, so browsing the catalog keeps working on
	// a box whose API key is not configured yet — the models simply come up disabled.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build catalog request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch model catalog: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch model catalog: unexpected status %d", res.StatusCode)
	}

	var doc modelsDocument
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBodyBytes)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode model catalog: %w", err)
	}

	out := make([]modelcatalog.Candidate, 0, len(doc.Data))
	skipped := 0
	for _, item := range doc.Data {
		candidate, ok := toCandidate(item)
		if !ok {
			skipped++
			continue
		}
		out = append(out, candidate)
	}
	if skipped > 0 {
		// One line for the batch, not one per entry: an upstream shape change would
		// otherwise write hundreds of identical warnings on every refresh.
		slog.Warn("skipped unusable catalog entries", "count", skipped, "total", len(doc.Data))
	}
	return out, nil
}

// toCandidate maps one upstream entry. An entry without an id or a name cannot be shown or
// stored, so it is dropped rather than turned into a blank row.
func toCandidate(item modelDocument) (modelcatalog.Candidate, bool) {
	id := strings.TrimSpace(item.ID)
	name := strings.TrimSpace(item.Name)
	if id == "" || name == "" {
		return modelcatalog.Candidate{}, false
	}
	imageOutput := contains(item.Architecture.OutputModalities, "image")
	input, output := tokenPrices(item, contains(item.Architecture.OutputModalities, "text"), imageOutput)
	return modelcatalog.Candidate{
		ModelID:      id,
		ProviderSlug: modelcatalog.ProviderSlugOf(id),
		Label:        name,
		Description:  strings.TrimSpace(item.Description),
		// An image PART on the input side is what the observe stage needs the model to
		// accept; the output side is what the image/video GENERATION purposes gate on.
		Vision:              contains(item.Architecture.InputModalities, "image"),
		StructuredOutput:    contains(item.SupportedParameters, "structured_outputs"),
		ImageOutput:         imageOutput,
		VideoOutput:         contains(item.Architecture.OutputModalities, "video"),
		ContextTokens:       max(item.ContextLen, 0),
		InputUSDPerMillion:  input,
		OutputUSDPerMillion: output,
		ReasoningCapability: reasoningCapability(item),
		SourceCreatedAt:     item.Created,
	}, true
}

// reasoningCapability reads the source's top-level `reasoning` object. An entry without one
// is a model that does not reason — recorded as such rather than skipped or failed — and an
// unknown extra key inside it stays ignored, which is the endpoint's own contract.
//
// The native-effort signal comes from a DIFFERENT field: `supported_parameters` says whether
// the provider takes `reasoning_effort` itself, as opposed to OpenRouter converting the
// effort into a token percentage on the way through.
func reasoningCapability(item modelDocument) modelcatalog.ReasoningCapability {
	capability := modelcatalog.ReasoningCapability{
		NativeEffort: contains(item.SupportedParameters, "reasoning_effort"),
	}
	if item.Reasoning == nil {
		return capability
	}
	capability.Reasons = true
	capability.Mandatory = item.Reasoning.Mandatory
	capability.MaxTokens = item.Reasoning.SupportsMaxToks
	capability.DefaultEffort = strings.TrimSpace(item.Reasoning.Default)
	// The source's DESCENDING order is preserved: it is the order a selector should offer
	// the values in, so sorting or normalizing here would throw away meaning. Blanks are
	// dropped because an empty option is not a value anyone can pick.
	for _, effort := range item.Reasoning.Supported {
		if trimmed := strings.TrimSpace(effort); trimmed != "" {
			capability.Efforts = append(capability.Efforts, trimmed)
		}
	}
	return capability
}

// tokenPrices picks the per-token prices that actually apply to a model, in the
// USD-per-million convention the product displays.
//
// A model that answers in text keeps the text pair, where a zero is a real price (a free
// model). One that answers only in images is not billed on text tokens at all, so a zero
// there means "not published" and the image pair is what it charges. A model that answers
// in video publishes neither: every one of them reports `prompt` and `completion` as "0",
// which is the ABSENCE of a price — rendering it as free would be a lie, so it comes back
// unknown and the screen says so.
//
// `image_output` is documented as "per output image" but is per output image TOKEN in
// practice: ×10⁶ reproduces the vendors' own published $/1M figures exactly (Gemini 2.5
// Flash Image $30, Gemini 3 Pro Image $120, GPT-5 Image $40). It therefore shares the
// column with the text prices rather than needing a unit of its own.
func tokenPrices(item modelDocument, textOutput, imageOutput bool) (input, output string) {
	switch {
	case textOutput:
		return perMillion(item.Pricing.Prompt), perMillion(item.Pricing.Completion)
	case imageOutput:
		return firstPriced(item.Pricing.Prompt, item.Pricing.ImageToken),
			firstPriced(item.Pricing.Completion, item.Pricing.ImageOutput)
	default:
		return "", ""
	}
}

// firstPriced returns the first value naming an actual price. A zero is skipped rather
// than reported, the caller having already established that this model is not billed on
// that axis.
func firstPriced(values ...string) string {
	for _, value := range values {
		if converted := perMillion(value); converted != "" && converted != "0" {
			return converted
		}
	}
	return ""
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// perMillion converts the upstream USD-per-TOKEN decimal string to the USD-per-million
// convention the product displays. It goes through big.Rat rather than float64 because a
// price like "0.00000125" loses its last digits to binary rounding and would be shown back
// as a number the provider never published.
func perMillion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	rat, ok := new(big.Rat).SetString(trimmed)
	if !ok || rat.Sign() < 0 {
		return ""
	}
	rat.Mul(rat, new(big.Rat).SetInt64(1_000_000))
	out := strings.TrimRight(rat.FloatString(6), "0")
	return strings.TrimSuffix(out, ".")
}

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
		InputModalities []string `json:"input_modalities"`
	} `json:"architecture"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
	SupportedParameters []string `json:"supported_parameters"`
}

func (c *Client) get(ctx context.Context) ([]modelcatalog.Candidate, error) {
	url := c.baseURL + "/models"
	// No Authorization header: the list is public, so browsing the catalog keeps working on
	// a box whose API key is not configured yet — the models simply come up disabled.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	return modelcatalog.Candidate{
		ModelID:      id,
		ProviderSlug: modelcatalog.ProviderSlugOf(id),
		Label:        name,
		Description:  strings.TrimSpace(item.Description),
		// Only the input side matters: the product consumes text, and an image PART is what
		// the observe stage needs the model to accept.
		Vision:              contains(item.Architecture.InputModalities, "image"),
		StructuredOutput:    contains(item.SupportedParameters, "structured_outputs"),
		ContextTokens:       max(item.ContextLen, 0),
		InputUSDPerMillion:  perMillion(item.Pricing.Prompt),
		OutputUSDPerMillion: perMillion(item.Pricing.Completion),
		SourceCreatedAt:     item.Created,
	}, true
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

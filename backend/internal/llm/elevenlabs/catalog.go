package elevenlabs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"slices"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// These are the design endpoint's documented vocabulary, never an account default.
var designModels = []string{"eleven_multilingual_ttv_v2", "eleven_ttv_v3"}

func (p *Provider) ReadSpeechCatalog(ctx context.Context, refresh bool) (llm.SpeechCatalog, error) {
	if err := p.ready(p.id); err != nil {
		return llm.SpeechCatalog{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, llm.SpeechCatalogTimeout)
	defer cancel()
	p.catalogMu.Lock()
	defer p.catalogMu.Unlock()
	if err := ctx.Err(); err != nil {
		return llm.SpeechCatalog{}, err
	}
	if !refresh && !p.catalog.CheckedAt.IsZero() && time.Since(p.catalog.CheckedAt) < llm.SpeechCatalogTTL {
		return cloneCatalog(p.catalog), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/models", nil)
	if err != nil {
		return llm.SpeechCatalog{}, err
	}
	req.Header.Set("xi-api-key", p.key)
	res, err := p.client.Do(req)
	if err != nil {
		return llm.SpeechCatalog{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return llm.SpeechCatalog{}, fmt.Errorf("%w: speech catalog status %d", llm.ErrModelUnavailable, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, llm.SpeechCatalogMaxBytes+1))
	if err != nil {
		return llm.SpeechCatalog{}, err
	}
	if len(body) > llm.SpeechCatalogMaxBytes {
		return llm.SpeechCatalog{}, llm.ErrBadOutput
	}
	var rows []struct {
		ID           string      `json:"model_id"`
		Name         string      `json:"name"`
		Speech       bool        `json:"can_do_text_to_speech"`
		Style        bool        `json:"can_use_style"`
		SpeakerBoost bool        `json:"can_use_speaker_boost"`
		Alpha        bool        `json:"requires_alpha_access"`
		Max          int         `json:"maximum_text_length_per_request"`
		Factor       json.Number `json:"token_cost_factor"`
		Languages    []struct {
			ID string `json:"language_id"`
		} `json:"languages"`
		Rates struct {
			Character json.Number `json:"character_cost_multiplier"`
			Discount  json.Number `json:"cost_discount_multiplier"`
		} `json:"model_rates"`
	}
	if err := json.Unmarshal(body, &rows); err != nil || len(rows) > llm.SpeechCatalogMaxModels {
		return llm.SpeechCatalog{}, llm.ErrBadOutput
	}
	out := llm.SpeechCatalog{CheckedAt: time.Now().UTC(), ConnectionScope: p.connectionScope()}
	seen := map[string]bool{}
	for _, row := range rows {
		if !validHandle(row.ID) || row.Name == "" || seen[row.ID] {
			return llm.SpeechCatalog{}, llm.ErrBadOutput
		}
		seen[row.ID] = true
		ko := slices.ContainsFunc(row.Languages, func(l struct {
			ID string `json:"language_id"`
		}) bool {
			return l.ID == "ko"
		})
		out.Models = append(out.Models, llm.SpeechModel{Ref: llm.ModelRef{ProviderID: p.id, ModelID: row.ID}, Label: row.Name,
			Synthesis: row.Speech, Korean: ko, Style: row.Style, SpeakerBoost: row.SpeakerBoost, RequiresAlpha: row.Alpha, MaxText: row.Max,
			TokenCostFactor: row.Factor.String(), CharacterCostMultiplier: row.Rates.Character.String(), CostDiscountMultiplier: row.Rates.Discount.String()})
		// Only the unambiguous unit-rate path has a qualified bound here.
		// Other factors stay catalog candidates, without guessed billing rules.
		if unitBillingFactor(row.Factor) && unitBillingFactor(row.Rates.Character) && unitBillingFactor(row.Rates.Discount) {
			out.BillingRules = append(out.BillingRules, llm.SpeechBillingRule{Ref: llm.ModelRef{ProviderID: p.id, ModelID: row.ID}, Operation: "speech", Unit: llm.SpeechUnitCharacterCost, UnitsPerInputCharacter: "1", Source: speechBillingSource})
		}
	}
	for _, id := range designModels {
		out.Models = append(out.Models, llm.SpeechModel{Ref: llm.ModelRef{ProviderID: p.id, ModelID: id}, Label: id, Design: true, MaxText: llm.SpeechPreviewMax})
		out.BillingRules = append(out.BillingRules, llm.SpeechBillingRule{Ref: llm.ModelRef{ProviderID: p.id, ModelID: id}, Operation: "voice_design", Unit: llm.SpeechUnitCharacterCost, UnitsPerInputCharacter: "1", Source: designBillingSource})
	}
	p.catalog = cloneCatalog(out)
	return out, nil
}

const speechBillingSource = "https://elevenlabs.io/docs/api-reference/models/list"
const designBillingSource = "https://help.elevenlabs.io/hc/en-us/articles/29315418701073-How-much-does-Voice-Design-cost"

func cloneCatalog(c llm.SpeechCatalog) llm.SpeechCatalog {
	c.Models = slices.Clone(c.Models)
	c.BillingRules = slices.Clone(c.BillingRules)
	return c
}

var _ llm.SpeechCatalogReader = (*Provider)(nil)

func unitBillingFactor(v json.Number) bool {
	n, ok := new(big.Rat).SetString(v.String())
	return ok && n.Cmp(big.NewRat(1, 1)) == 0
}

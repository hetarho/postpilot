// Package elevenlabs is the speech protocol edge. Product code imports llm only.
package elevenlabs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/postpilot/backend/internal/llm"
)

const DefaultBaseURL = "https://api.elevenlabs.io"

var reportedQuantity = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,9})?$`)

type Provider struct {
	id, baseURL, key string
	client           *http.Client
	catalogMu        sync.Mutex
	catalog          llm.SpeechCatalog
}

func Factory(cfg llm.SpeechAdapterConfig) (llm.SpeechProvider, error) {
	return New(cfg, http.DefaultClient)
}

func (p *Provider) connectionScope() string {
	// Key material never leaves this edge. Conservatively invalidate the binding
	// even for a key rotation within one account; never infer account equivalence.
	h := sha256.Sum256([]byte("elevenlabs-connection-v1\x00" + p.id + "\x00" + p.baseURL + "\x00" + p.key))
	return hex.EncodeToString(h[:])
}

func New(cfg llm.SpeechAdapterConfig, client *http.Client) (*Provider, error) {
	if strings.TrimSpace(cfg.ProviderID) == "" {
		return nil, fmt.Errorf("elevenlabs: provider id required")
	}
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return nil, fmt.Errorf("elevenlabs: base_url must be an HTTP(S) origin")
	}
	if client == nil {
		return nil, fmt.Errorf("elevenlabs: HTTP client required")
	}
	bounded := *client
	if bounded.Timeout <= 0 || bounded.Timeout > llm.SpeechProviderTimeout {
		bounded.Timeout = llm.SpeechProviderTimeout
	}
	// Do not redirect paid POSTs or forward a private key to a different origin.
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Provider{id: cfg.ProviderID, baseURL: strings.TrimRight(base, "/"), key: cfg.APIKey, client: &bounded}, nil
}

func (p *Provider) ready(provider string) error {
	if p.key == "" {
		return llm.ErrProviderDisabled
	}
	if provider != p.id {
		return llm.ErrModelUnavailable
	}
	return nil
}

func (p *Provider) DesignVoice(ctx context.Context, req llm.VoiceDesignRequest) (out llm.VoiceDesignResponse, err error) {
	if err = p.ready(req.Model.ProviderID); err != nil {
		return out, err
	}
	if err = req.Validate(); err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, llm.SpeechProviderTimeout)
	defer cancel()
	wire := struct {
		Description  string `json:"voice_description"`
		Text         string `json:"text"`
		Model        string `json:"model_id"`
		AutoGenerate bool   `json:"auto_generate_text"`
		Enhance      bool   `json:"should_enhance"`
		Stream       bool   `json:"stream_previews"`
	}{Description: req.Description, Text: req.PreviewText, Model: req.Model.ModelID}
	var response struct {
		Previews []struct {
			Audio     string   `json:"audio_base_64"`
			ID        string   `json:"generated_voice_id"`
			MediaType string   `json:"media_type"`
			Duration  *float64 `json:"duration_secs"`
			Language  string   `json:"language"`
		} `json:"previews"`
		Text string `json:"text"`
	}
	out.Evidence, err = p.post(ctx, "/v1/text-to-voice/design?output_format="+llm.SpeechOutputFormat, wire, &response)
	if err != nil {
		return out, err
	}
	if len(response.Previews) < 1 || len(response.Previews) > llm.SpeechMaxCandidates || response.Text != req.PreviewText {
		return out, bad("invalid voice candidates")
	}
	seen := make(map[string]bool)
	for _, preview := range response.Previews {
		if !validHandle(preview.ID) || seen[preview.ID] || preview.MediaType != "audio/mpeg" ||
			preview.Duration != nil && (math.IsNaN(*preview.Duration) || math.IsInf(*preview.Duration, 0) || *preview.Duration <= 0 || *preview.Duration > llm.SpeechMaxDuration.Seconds()) {
			return out, bad("invalid voice candidate metadata")
		}
		seen[preview.ID] = true
		audio, err := decodeAudio(ctx, preview.Audio)
		if err != nil {
			return out, err
		}
		out.Candidates = append(out.Candidates, llm.VoiceCandidate{Handle: llm.CandidateHandle(preview.ID), Audio: audio, Language: preview.Language, ReportedDurationSeconds: preview.Duration})
	}
	out.PreviewText = response.Text
	return out, nil
}

func (p *Provider) ConfirmVoice(ctx context.Context, req llm.VoiceConfirmationRequest) (out llm.VoiceConfirmationResponse, err error) {
	if err = p.ready(req.DesignModel.ProviderID); err != nil {
		return out, err
	}
	if err = req.Validate(); err != nil {
		return out, err
	}
	if !validHandle(string(req.Candidate)) {
		return out, bad("invalid candidate handle")
	}
	ctx, cancel := context.WithTimeout(ctx, llm.SpeechProviderTimeout)
	defer cancel()
	wire := struct {
		Name        string `json:"voice_name"`
		Description string `json:"voice_description"`
		Candidate   string `json:"generated_voice_id"`
	}{req.Name, req.Description, string(req.Candidate)}
	var response struct {
		Voice string `json:"voice_id"`
	}
	out.Evidence, err = p.post(ctx, "/v1/text-to-voice", wire, &response)
	if err != nil {
		return out, err
	}
	if !validHandle(response.Voice) {
		return out, bad("missing confirmed voice")
	}
	out.Voice = llm.VoiceHandle(response.Voice)
	return out, nil
}

type wireAlignment struct {
	Characters []string  `json:"characters"`
	Starts     []float64 `json:"character_start_times_seconds"`
	Ends       []float64 `json:"character_end_times_seconds"`
}

func (a *wireAlignment) timings(audio llm.EncodedAudio) ([]llm.CharacterTiming, error) {
	if a == nil {
		return nil, nil
	}
	if len(a.Characters) == 0 || len(a.Characters) != len(a.Starts) || len(a.Characters) != len(a.Ends) {
		return nil, bad("incomplete character timing")
	}
	timing := make([]llm.CharacterTiming, len(a.Characters))
	for i, ch := range a.Characters {
		timing[i] = llm.CharacterTiming{Character: ch, StartSeconds: a.Starts[i], EndSeconds: a.Ends[i]}
	}
	if err := llm.ValidateSpeechAlignment(timing, audio); err != nil {
		return nil, err
	}
	return timing, nil
}

func (p *Provider) SynthesizeSpeech(ctx context.Context, req llm.SpeechRequest) (out llm.SpeechResponse, err error) {
	if err = p.ready(req.Model.ProviderID); err != nil {
		return out, err
	}
	if err = req.Validate(); err != nil {
		return out, err
	}
	if !validHandle(string(req.Voice)) {
		return out, bad("invalid confirmed voice handle")
	}
	ctx, cancel := context.WithTimeout(ctx, llm.SpeechProviderTimeout)
	defer cancel()
	settings := struct {
		Stability    float64 `json:"stability"`
		Similarity   float64 `json:"similarity_boost"`
		Style        float64 `json:"style"`
		SpeakerBoost bool    `json:"use_speaker_boost"`
		Speed        float64 `json:"speed"`
	}{req.Settings.Stability, req.Settings.SimilarityBoost, req.Settings.Style, req.Settings.SpeakerBoost, req.Settings.Speed}
	wire := struct {
		Text     string `json:"text"`
		Model    string `json:"model_id"`
		Settings any    `json:"voice_settings"`
	}{req.Text, req.Model.ModelID, settings}
	var response struct {
		Audio      string         `json:"audio_base64"`
		Alignment  *wireAlignment `json:"alignment"`
		Normalized *wireAlignment `json:"normalized_alignment"`
	}
	out.Evidence, err = p.post(ctx, "/v1/text-to-speech/"+string(req.Voice)+"/with-timestamps?output_format="+llm.SpeechOutputFormat, wire, &response)
	if err != nil {
		return out, err
	}
	out.Audio, err = decodeAudio(ctx, response.Audio)
	if err != nil {
		return out, err
	}
	out.Alignment, err = response.Alignment.timings(out.Audio)
	if err != nil {
		return out, err
	}
	out.NormalizedAlignment, err = response.Normalized.timings(out.Audio)
	return out, err
}

func decodeAudio(ctx context.Context, encoded string) (llm.EncodedAudio, error) {
	if len(encoded) > base64.StdEncoding.EncodedLen(llm.SpeechMaxAudioBytes) {
		return llm.EncodedAudio{}, bad("encoded audio too large")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return llm.EncodedAudio{}, bad("invalid base64 audio")
	}
	return llm.InspectSpeechAudio(ctx, data)
}

func bad(message string) error { return fmt.Errorf("%w: %s", llm.ErrBadOutput, message) }

func validHandle(handle string) bool {
	if len(handle) == 0 || len(handle) > 256 {
		return false
	}
	for _, ch := range handle {
		if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}

// post issues exactly one request. Evidence is read before the body so failures,
// cancellation during reads and invalid output retain actual reported consumption.
func (p *Provider) post(ctx context.Context, path string, payload, out any) (evidence llm.SpeechEvidence, err error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return evidence, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return evidence, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", p.key)
	response, err := p.client.Do(req)
	if err != nil {
		return evidence, err
	}
	defer response.Body.Close()
	evidence.RequestID = response.Header.Get("request-id")
	if value := response.Header.Get("character-cost"); reportedQuantity.MatchString(value) {
		evidence.Units = append(evidence.Units, llm.SpeechUnitEvidence{Unit: llm.SpeechUnitCharacterCost, Quantity: value})
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, llm.SpeechMaxResponseBytes+1))
	if err != nil {
		return evidence, err
	}
	if len(data) > llm.SpeechMaxResponseBytes {
		return evidence, bad("response body too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		kind := llm.ErrBadOutput
		switch response.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			kind = llm.ErrProviderDisabled
		case http.StatusNotFound:
			kind = llm.ErrModelUnavailable
		case http.StatusTooManyRequests:
			kind = llm.ErrRateLimited
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			kind = llm.ErrUnsupported
		}
		var detail struct {
			Detail struct{ Status, Message string }
		}
		_ = json.Unmarshal(data, &detail)
		return evidence, &llm.ProviderError{Provider: p.id, Status: response.StatusCode, Code: response.StatusCode, Message: detail.Detail.Message, Kind: kind}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return evidence, bad("invalid speech response JSON")
	}
	return evidence, nil
}

var _ llm.SpeechProvider = (*Provider)(nil)

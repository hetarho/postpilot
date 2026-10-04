package llm

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// Speech limits belong to the provider boundary, independently of completion caps.
const (
	SpeechProviderTimeout  = 5 * time.Minute
	SpeechMaxDuration      = 300 * time.Second
	SpeechMaxAudioBytes    = 8 << 20
	SpeechMaxResponseBytes = 40 << 20
	SpeechMaxCandidates    = 3
	SpeechOutputFormat     = "mp3_44100_128"
	SpeechDescriptionMin   = 20
	SpeechDescriptionMax   = 1000
	SpeechPreviewMin       = 100
	SpeechPreviewMax       = 1000
	SpeechMaxText          = 1000
)

// Handles are private supplier-account identities. They must never cross customer DTOs.
type CandidateHandle string
type VoiceHandle string

type SpeechUnit string

const (
	SpeechUnitCharacters SpeechUnit = "characters"
	SpeechUnitCredits    SpeechUnit = "supplier_credits"
	SpeechUnitRequests   SpeechUnit = "requests"
	SpeechUnitSeconds    SpeechUnit = "seconds"
	// A supplier's reported character-cost is a billing quantity, not evidence
	// of input length. Its price/unit mapping must be qualified separately.
	SpeechUnitCharacterCost SpeechUnit = "character_cost"
)

// SpeechEvidence records reported units only. An empty Units is unknown usage;
// an explicitly reported zero remains a zero entry. No token or product-credit fields.
type SpeechEvidence struct {
	RequestID string
	Units     []SpeechUnitEvidence
}

type SpeechUnitEvidence struct {
	Unit SpeechUnit
	// Decimal text retains fractional billing units without float rounding.
	Quantity string
}

// EncodedAudio owns the original encoded bytes. Samples is the decoded stereo
// frame count, not a supplier duration hint. Consumers retain these bytes unchanged.
type EncodedAudio struct {
	Bytes      []byte
	Format     string
	SampleRate int
	Channels   int
	Samples    int64
	SHA256     string
}

func (a EncodedAudio) Duration() time.Duration {
	if a.SampleRate <= 0 {
		return 0
	}
	return time.Duration(a.Samples) * time.Second / time.Duration(a.SampleRate)
}

type VoiceDesignRequest struct {
	Model       ModelRef
	Description string
	PreviewText string
}

type VoiceCandidate struct {
	Handle                  CandidateHandle
	Audio                   EncodedAudio
	Language                string
	ReportedDurationSeconds *float64
}

type VoiceDesignResponse struct {
	Candidates  []VoiceCandidate
	PreviewText string
	Evidence    SpeechEvidence
}

type VoiceConfirmationRequest struct {
	DesignModel ModelRef
	Candidate   CandidateHandle
	Name        string
	Description string
}

type VoiceConfirmationResponse struct {
	Voice    VoiceHandle
	Evidence SpeechEvidence
}

// Settings are explicit so the immutable profile can reproduce the auditioned
// voice. Natural speed is the only supported product playback/synthesis rate.
type SpeechSettings struct {
	Stability       float64
	SimilarityBoost float64
	Style           float64
	SpeakerBoost    bool
	Speed           float64
}

func (s SpeechSettings) Validate() error {
	for _, value := range []float64{s.Stability, s.SimilarityBoost, s.Style} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return fmt.Errorf("%w: invalid speech settings", ErrUnsupported)
		}
	}
	if s.Speed != 1 {
		return fmt.Errorf("%w: speech speed must be 1", ErrUnsupported)
	}
	return nil
}

type SpeechRequest struct {
	Model    ModelRef
	Voice    VoiceHandle
	Text     string
	Settings SpeechSettings
}

type CharacterTiming struct {
	Character    string
	StartSeconds float64
	EndSeconds   float64
}

type SpeechResponse struct {
	Audio               EncodedAudio
	Alignment           []CharacterTiming
	NormalizedAlignment []CharacterTiming
	Evidence            SpeechEvidence
}

type VoiceDesigner interface {
	DesignVoice(context.Context, VoiceDesignRequest) (VoiceDesignResponse, error)
}
type VoiceConfirmer interface {
	ConfirmVoice(context.Context, VoiceConfirmationRequest) (VoiceConfirmationResponse, error)
}
type SpeechSynthesizer interface {
	SynthesizeSpeech(context.Context, SpeechRequest) (SpeechResponse, error)
}
type SpeechProvider interface {
	VoiceDesigner
	VoiceConfirmer
	SpeechSynthesizer
}

type SpeechAdapterConfig struct{ ProviderID, BaseURL, APIKey string }
type SpeechAdapterFactory func(SpeechAdapterConfig) (SpeechProvider, error)

func validSpeechText(text string, min, max int) bool {
	return utf8.ValidString(text) && strings.TrimSpace(text) != "" && utf8.RuneCountInString(text) >= min && utf8.RuneCountInString(text) <= max
}

func (r VoiceDesignRequest) Validate() error {
	if strings.TrimSpace(r.Model.ProviderID) == "" || strings.TrimSpace(r.Model.ModelID) == "" ||
		!validSpeechText(r.Description, SpeechDescriptionMin, SpeechDescriptionMax) || !validSpeechText(r.PreviewText, SpeechPreviewMin, SpeechPreviewMax) {
		return fmt.Errorf("%w: explicit design model, description and preview text required", ErrUnsupported)
	}
	return nil
}

func (r VoiceConfirmationRequest) Validate() error {
	if strings.TrimSpace(r.DesignModel.ProviderID) == "" || strings.TrimSpace(r.DesignModel.ModelID) == "" || r.Candidate == "" || !validSpeechText(r.Name, 1, 100) || !validSpeechText(r.Description, SpeechDescriptionMin, SpeechDescriptionMax) {
		return fmt.Errorf("%w: candidate, name and description required", ErrUnsupported)
	}
	return nil
}

func (r SpeechRequest) Validate() error {
	if strings.TrimSpace(r.Model.ProviderID) == "" || strings.TrimSpace(r.Model.ModelID) == "" || r.Voice == "" || !validSpeechText(r.Text, 1, SpeechMaxText) {
		return fmt.Errorf("%w: explicit model, confirmed voice and bounded text required", ErrUnsupported)
	}
	return r.Settings.Validate()
}

// SpeechConnection describes availability, not catalog eligibility. Profile
// capability/pricing admission is owned by modelcatalog and usage respectively.
type SpeechConnection struct {
	ProviderID, DisabledReason string
	Disabled                   bool
}

func (r *Registry) SpeechConnection() SpeechConnection {
	if r.speech == nil {
		return SpeechConnection{Disabled: true, DisabledReason: DisabledReasonNoKey}
	}
	return SpeechConnection{ProviderID: r.speechID, Disabled: r.speechDisabled, DisabledReason: r.speechDisabledReason}
}

func (r *Registry) resolveSpeech(providerID string) error {
	if r.speech == nil || r.speechDisabled {
		return ErrProviderDisabled
	}
	if providerID != r.speechID {
		return ErrModelUnavailable
	}
	return nil
}

func (r *Registry) DesignVoice(ctx context.Context, req VoiceDesignRequest) (VoiceDesignResponse, error) {
	if err := r.resolveSpeech(req.Model.ProviderID); err != nil {
		return VoiceDesignResponse{}, err
	}
	if err := req.Validate(); err != nil {
		return VoiceDesignResponse{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, SpeechProviderTimeout)
	defer cancel()
	return r.speech.DesignVoice(ctx, req)
}

func (r *Registry) ConfirmVoice(ctx context.Context, req VoiceConfirmationRequest) (VoiceConfirmationResponse, error) {
	if err := r.resolveSpeech(req.DesignModel.ProviderID); err != nil {
		return VoiceConfirmationResponse{}, err
	}
	if err := req.Validate(); err != nil {
		return VoiceConfirmationResponse{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, SpeechProviderTimeout)
	defer cancel()
	return r.speech.ConfirmVoice(ctx, req)
}

func (r *Registry) SynthesizeSpeech(ctx context.Context, req SpeechRequest) (SpeechResponse, error) {
	if err := r.resolveSpeech(req.Model.ProviderID); err != nil {
		return SpeechResponse{}, err
	}
	if err := req.Validate(); err != nil {
		return SpeechResponse{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, SpeechProviderTimeout)
	defer cancel()
	return r.speech.SynthesizeSpeech(ctx, req)
}

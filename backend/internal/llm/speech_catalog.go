package llm

import (
	"context"
	"time"
)

const (
	SpeechCatalogTimeout   = 15 * time.Second
	SpeechCatalogTTL       = 5 * time.Minute
	SpeechCatalogMaxBytes  = 1 << 20
	SpeechCatalogMaxModels = 256
)

// SpeechModel is metadata, not evidence of live voice quality or USD pricing.
// Decimal factors retain the supplier's spelling and never enter customer DTOs.
type SpeechModel struct {
	Ref                     ModelRef
	Label                   string
	Design                  bool
	Synthesis               bool
	Korean                  bool
	Style                   bool
	SpeakerBoost            bool
	RequiresAlpha           bool
	MaxText                 int
	TokenCostFactor         string
	CharacterCostMultiplier string
	CostDiscountMultiplier  string
}

type SpeechCatalog struct {
	// Private connection identity. Rotation requires recuration/qualification;
	// a catalog/model ID alone cannot authorize another supplier account's voice.
	ConnectionScope string
	Models          []SpeechModel
	CheckedAt       time.Time
}

type SpeechCatalogReader interface {
	ReadSpeechCatalog(context.Context, bool) (SpeechCatalog, error)
}

// This optional read capability keeps completion and speech execution ports
// independent. A factory without metadata cannot qualify a profile.
func (r *Registry) ReadSpeechCatalog(ctx context.Context, refresh bool) (SpeechCatalog, error) {
	if err := r.resolveSpeech(r.speechID); err != nil {
		return SpeechCatalog{}, err
	}
	reader, ok := r.speech.(SpeechCatalogReader)
	if !ok {
		return SpeechCatalog{}, ErrUnsupported
	}
	return reader.ReadSpeechCatalog(ctx, refresh)
}

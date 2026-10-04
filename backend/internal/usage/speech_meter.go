package usage

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// SpeechMeter is the application behaviour at the llm seam. The composition
// root only supplies the real provider and ledger, without owning call policy.
type SpeechMeter struct {
	Provider llm.SpeechProvider
	Ledger   *Service
}

func (m SpeechMeter) begin(ctx context.Context, input llm.SpeechInput, err error) (UnitClaim, error) {
	if err != nil {
		return UnitClaim{}, err
	}
	if m.Provider == nil || m.Ledger == nil {
		return UnitClaim{}, ErrUnitCall
	}
	return m.Ledger.AdmitUnitCall(ctx, input)
}

func (m SpeechMeter) record(ctx context.Context, c UnitClaim, evidence llm.SpeechEvidence, providerErr error) error {
	// A failed or cancelled request may still report a bill. Its audit write
	// survives caller cancellation; settlement never reopens for late evidence.
	audit, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := m.Ledger.RecordUnitCall(audit, c, evidence); err != nil {
		return errors.Join(providerErr, err)
	}
	return providerErr
}

func (m SpeechMeter) DesignVoice(ctx context.Context, r llm.VoiceDesignRequest) (llm.VoiceDesignResponse, error) {
	input, err := r.Input()
	c, err := m.begin(ctx, input, err)
	if err != nil {
		return llm.VoiceDesignResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return llm.VoiceDesignResponse{}, m.record(ctx, c, llm.SpeechEvidence{}, err)
	}
	out, err := m.Provider.DesignVoice(ctx, r)
	return out, m.record(ctx, c, out.Evidence, err)
}
func (m SpeechMeter) ConfirmVoice(ctx context.Context, r llm.VoiceConfirmationRequest) (llm.VoiceConfirmationResponse, error) {
	input, err := r.Input()
	c, err := m.begin(ctx, input, err)
	if err != nil {
		return llm.VoiceConfirmationResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return llm.VoiceConfirmationResponse{}, m.record(ctx, c, llm.SpeechEvidence{}, err)
	}
	out, err := m.Provider.ConfirmVoice(ctx, r)
	return out, m.record(ctx, c, out.Evidence, err)
}
func (m SpeechMeter) SynthesizeSpeech(ctx context.Context, r llm.SpeechRequest) (llm.SpeechResponse, error) {
	input, err := r.Input()
	c, err := m.begin(ctx, input, err)
	if err != nil {
		return llm.SpeechResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return llm.SpeechResponse{}, m.record(ctx, c, llm.SpeechEvidence{}, err)
	}
	out, err := m.Provider.SynthesizeSpeech(ctx, r)
	return out, m.record(ctx, c, out.Evidence, err)
}

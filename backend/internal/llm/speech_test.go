package llm_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

type fakeSpeech struct {
	calls   int
	design  llm.VoiceDesignRequest
	confirm llm.VoiceConfirmationRequest
	speech  llm.SpeechRequest
}

func (f *fakeSpeech) DesignVoice(ctx context.Context, req llm.VoiceDesignRequest) (llm.VoiceDesignResponse, error) {
	f.calls++
	f.design = req
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > llm.SpeechProviderTimeout {
		panic("missing speech deadline")
	}
	return llm.VoiceDesignResponse{}, nil
}
func (f *fakeSpeech) ConfirmVoice(_ context.Context, req llm.VoiceConfirmationRequest) (llm.VoiceConfirmationResponse, error) {
	f.calls++
	f.confirm = req
	return llm.VoiceConfirmationResponse{Voice: "confirmed"}, nil
}
func (f *fakeSpeech) SynthesizeSpeech(_ context.Context, req llm.SpeechRequest) (llm.SpeechResponse, error) {
	f.calls++
	f.speech = req
	return llm.SpeechResponse{}, nil
}

func speechOptions(s *fakeSpeech) llm.Options {
	out := opts
	out.SpeechAdapters = map[string]llm.SpeechAdapterFactory{"elevenlabs": func(cfg llm.SpeechAdapterConfig) (llm.SpeechProvider, error) {
		if cfg.BaseURL == "invalid" {
			return nil, errors.New("invalid speech URL")
		}
		return s, nil
	}}
	return out
}

const speechYAML = `
speech_provider:
  id: elevenlabs
  adapter: elevenlabs
  base_url: https://api.elevenlabs.io
  api_key_env: SPEECH_KEY
`

func TestSpeechRegistryKeepsCompletionIndependent(t *testing.T) {
	for _, speech := range []string{"", speechYAML} {
		t.Run(speech, func(t *testing.T) {
			provider, audio := &fakeProvider{}, &fakeSpeech{}
			reg, err := llm.Parse([]byte(goodYAML+speech), env(map[string]string{"TEST_KEY": "text"}), adaptersWith(provider), twoModels(), speechOptions(audio))
			if err != nil {
				t.Fatal(err)
			}
			_, err = reg.Complete(context.Background(), llm.ModelRef{ProviderID: "openrouter", ModelID: "text-only"}, llm.Request{})
			if err != nil || provider.calls != 1 {
				t.Fatalf("completion: %v, calls %d", err, provider.calls)
			}
			_, err = reg.DesignVoice(context.Background(), llm.VoiceDesignRequest{})
			if !errors.Is(err, llm.ErrProviderDisabled) || audio.calls != 0 || llm.NormalizeFailure(err).Reason != llm.FailureReasonProviderDisabled {
				t.Fatalf("disabled speech: %v", err)
			}
		})
	}
}

func TestSpeechRegistryValidationAndExplicitRouting(t *testing.T) {
	speech := &fakeSpeech{}
	reg, err := llm.Parse([]byte(goodYAML+speechYAML), env(map[string]string{"TEST_KEY": "text", "SPEECH_KEY": "speech"}), adaptersWith(&fakeProvider{}), twoModels(), speechOptions(speech))
	if err != nil {
		t.Fatal(err)
	}
	design := llm.VoiceDesignRequest{Model: llm.ModelRef{ProviderID: "elevenlabs", ModelID: "explicit-design"}, Description: strings.Repeat("한", 20), PreviewText: strings.Repeat("안", 100)}
	if _, err := reg.DesignVoice(context.Background(), design); err != nil {
		t.Fatal(err)
	}
	confirm := llm.VoiceConfirmationRequest{DesignModel: design.Model, Candidate: "auditioned", Name: "내 목소리", Description: design.Description}
	if _, err := reg.ConfirmVoice(context.Background(), confirm); err != nil {
		t.Fatal(err)
	}
	req := llm.SpeechRequest{Model: llm.ModelRef{ProviderID: "elevenlabs", ModelID: "explicit-speech"}, Voice: "confirmed", Text: "안녕하세요", Settings: llm.SpeechSettings{Speed: 1, Stability: .5, SimilarityBoost: .7}}
	if _, err := reg.SynthesizeSpeech(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if speech.design != design || speech.confirm != confirm || speech.speech != req {
		t.Fatal("changed explicit identity/model/settings")
	}
	req.Model.ProviderID = "openrouter"
	if _, err := reg.SynthesizeSpeech(context.Background(), req); !errors.Is(err, llm.ErrModelUnavailable) {
		t.Fatal(err)
	}
	design.Model.ModelID = ""
	if _, err := reg.DesignVoice(context.Background(), design); !errors.Is(err, llm.ErrUnsupported) {
		t.Fatal(err)
	}
	if speech.calls != 3 {
		t.Fatal("invalid request reached adapter")
	}
	for _, value := range []float64{-1, 2, math.NaN(), math.Inf(1)} {
		req.Model.ProviderID = "elevenlabs"
		req.Settings.Stability = value
		if _, err := reg.SynthesizeSpeech(context.Background(), req); !errors.Is(err, llm.ErrUnsupported) {
			t.Fatal(err)
		}
	}
	design.Description = strings.Repeat("한", 19)
	if err := design.Validate(); !errors.Is(err, llm.ErrUnsupported) {
		t.Fatal("description must count codepoints")
	}
	design.Description = strings.Repeat("한", 1000)
	design.PreviewText = strings.Repeat("안", 1000)
	design.Model.ModelID = "explicit-design"
	if err := design.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSpeechConfigFailsClosedEvenWithoutCredentials(t *testing.T) {
	for _, yaml := range []string{
		strings.Replace(speechYAML, "id: elevenlabs", "id: openrouter", 1),
		strings.Replace(speechYAML, "id: elevenlabs", "id: ''", 1),
		strings.Replace(speechYAML, "adapter: elevenlabs", "adapter: unknown", 1),
		strings.Replace(speechYAML, "api_key_env: SPEECH_KEY", "api_key_env: ''", 1),
		strings.Replace(speechYAML, "https://api.elevenlabs.io", "invalid", 1),
		speechYAML + "  models: []\n",
	} {
		if _, err := llm.Parse([]byte(goodYAML+yaml), env(nil), adaptersWith(&fakeProvider{}), twoModels(), speechOptions(&fakeSpeech{})); err == nil {
			t.Fatalf("accepted invalid YAML: %s", yaml)
		}
	}
}

func TestSpeechAudioIsBoundedValidatedAndOwned(t *testing.T) {
	data, err := os.ReadFile("testdata/speech-tone.mp3")
	if err != nil {
		t.Fatal(err)
	}
	audio, err := llm.InspectSpeechAudio(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if audio.Format != llm.SpeechOutputFormat || audio.SampleRate != 44100 || audio.Samples <= 0 || audio.Duration() < 250*time.Millisecond || audio.Duration() > 350*time.Millisecond || len(audio.SHA256) != 64 {
		t.Fatalf("invalid measured audio: %+v", audio)
	}
	data[0] ^= 1
	if bytes.Equal(data, audio.Bytes) {
		t.Fatal("audio aliases provider bytes")
	}
	for _, bad := range [][]byte{nil, []byte("bad audio"), audio.Bytes[:len(audio.Bytes)-1], append(bytes.Clone(audio.Bytes), 0), make([]byte, llm.SpeechMaxAudioBytes+1), bytes.Repeat(audio.Bytes, 1100)} {
		if _, err := llm.InspectSpeechAudio(context.Background(), bad); !errors.Is(err, llm.ErrBadOutput) {
			t.Fatalf("accepted invalid audio: %d: %v", len(bad), err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := llm.InspectSpeechAudio(ctx, audio.Bytes); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, stop := context.WithTimeout(context.Background(), time.Millisecond)
	defer stop()
	if _, err := llm.InspectSpeechAudio(ctx, bytes.Repeat(audio.Bytes, 100)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("decode did not preserve deadline: %v", err)
	}
	if err := llm.ValidateSpeechAlignment([]llm.CharacterTiming{{Character: "한", EndSeconds: .1}}, llm.EncodedAudio{}); !errors.Is(err, llm.ErrBadOutput) {
		t.Fatal("accepted timing without a clock")
	}
	if err := llm.ValidateSpeechAlignment(nil, audio); err != nil {
		t.Fatal(err)
	}
	for _, timing := range [][]llm.CharacterTiming{
		{{Character: "한", StartSeconds: math.NaN(), EndSeconds: .1}},
		{{Character: "한", EndSeconds: math.Inf(1)}},
		{{Character: "한", StartSeconds: -.1, EndSeconds: .1}},
		{{Character: "한", StartSeconds: .2, EndSeconds: .1}},
		{{Character: "한", EndSeconds: 1}},
		{{Character: "한", StartSeconds: .1, EndSeconds: .2}, {Character: "글", StartSeconds: 0, EndSeconds: .1}},
	} {
		if err := llm.ValidateSpeechAlignment(timing, audio); !errors.Is(err, llm.ErrBadOutput) {
			t.Fatalf("accepted timing: %+v", timing)
		}
	}
}

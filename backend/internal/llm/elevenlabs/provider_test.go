package elevenlabs_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/llm/elevenlabs"
)

func audioFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../testdata/speech-tone.mp3")
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func speechRequest() llm.SpeechRequest {
	return llm.SpeechRequest{Model: llm.ModelRef{ProviderID: "elevenlabs", ModelID: "explicit-speech"}, Voice: "chosen-voice", Text: "안녕하세요", Settings: llm.SpeechSettings{Stability: .5, SimilarityBoost: .75, Style: .2, SpeakerBoost: true, Speed: 1}}
}

func provider(t *testing.T, server *httptest.Server) *elevenlabs.Provider {
	t.Helper()
	p, err := elevenlabs.New(llm.SpeechAdapterConfig{ProviderID: "elevenlabs", BaseURL: server.URL, APIKey: "private-fixture-key"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDesignConfirmAndSynthesizeExactChosenVoice(t *testing.T) {
	audio := audioFixture(t)
	design := llm.VoiceDesignRequest{Model: llm.ModelRef{ProviderID: "elevenlabs", ModelID: "explicit-design"}, Description: strings.Repeat("차분한", 10), PreviewText: strings.Repeat("안녕하세요. ", 20)}
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method != http.MethodPost || r.Header.Get("xi-api-key") != "private-fixture-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("wrong protocol/authentication")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("request-id", "fixture-request")
		w.Header().Set("character-cost", "104")
		switch r.URL.Path {
		case "/v1/text-to-voice/design":
			if body["model_id"] != design.Model.ModelID || body["text"] != design.PreviewText || body["voice_description"] != design.Description || body["auto_generate_text"] != false || body["should_enhance"] != false || body["stream_previews"] != false || len(body) != 6 {
				t.Errorf("unexpected design: %+v", body)
			}
			if r.URL.Query().Get("output_format") != llm.SpeechOutputFormat {
				t.Error("implicit audio format")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"text": design.PreviewText, "previews": []any{map[string]any{"audio_base_64": audio, "generated_voice_id": "auditioned-candidate", "media_type": "audio/mpeg", "duration_secs": .28, "language": "ko"}}})
		case "/v1/text-to-voice":
			if body["generated_voice_id"] != "auditioned-candidate" || body["voice_name"] != "내 목소리" || body["voice_description"] != design.Description || len(body) != 3 {
				t.Errorf("changed selected identity: %+v", body)
			}
			_, _ = io.WriteString(w, `{"voice_id":"chosen-voice"}`)
		case "/v1/text-to-speech/chosen-voice/with-timestamps":
			expected := map[string]any{"stability": .5, "similarity_boost": .75, "style": .2, "use_speaker_boost": true, "speed": float64(1)}
			if body["model_id"] != "explicit-speech" || body["text"] != "안녕하세요" || !reflect.DeepEqual(body["voice_settings"], expected) || len(body) != 3 || r.URL.Query().Get("output_format") != llm.SpeechOutputFormat {
				t.Errorf("implicit speech model/settings: %+v", body)
			}
			alignment := map[string]any{"characters": []string{"안", "녕"}, "character_start_times_seconds": []float64{0, .1}, "character_end_times_seconds": []float64{.1, .2}}
			_ = json.NewEncoder(w).Encode(map[string]any{"audio_base64": audio, "alignment": alignment, "normalized_alignment": alignment})
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	p := provider(t, server)
	candidates, err := p.DesignVoice(context.Background(), design)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates.Candidates) != 1 || candidates.Candidates[0].Audio.Duration() < 250*time.Millisecond || candidates.Evidence.RequestID != "fixture-request" {
		t.Fatal("missing audition/evidence")
	}
	confirmed, err := p.ConfirmVoice(context.Background(), llm.VoiceConfirmationRequest{DesignModel: design.Model, Candidate: candidates.Candidates[0].Handle, Name: "내 목소리", Description: design.Description})
	if err != nil {
		t.Fatal(err)
	}
	req := speechRequest()
	req.Voice = confirmed.Voice
	spoken, err := p.SynthesizeSpeech(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if spoken.Audio.SHA256 != candidates.Candidates[0].Audio.SHA256 || len(spoken.Alignment) != 2 || len(spoken.NormalizedAlignment) != 2 || spoken.Evidence.Units[0].Quantity != "104" || spoken.Evidence.Units[0].Unit != llm.SpeechUnitCharacterCost {
		t.Fatal("missing immutable audio/timing/usage")
	}
	if len(paths) != 3 {
		t.Fatal("paid call retried")
	}
}

func TestTimingAndUsageEvidence(t *testing.T) {
	audio := audioFixture(t)
	for _, tc := range []struct {
		name, tail, usage string
		bad               bool
	}{
		{"absent", "", "", false},
		{"null", `,"alignment":null,"normalized_alignment":null`, "0", false},
		{"fractional-billing", "", "7.5", false},
		{"malformed", `,"alignment":{"characters":["안"],"character_start_times_seconds":[],"character_end_times_seconds":[0.1]}`, "7", true},
		{"empty", `,"alignment":{}`, "", true},
		{"out-of-range", `,"alignment":{"characters":["안"],"character_start_times_seconds":[0],"character_end_times_seconds":[9]}`, "7", true},
		{"nonmonotone", `,"alignment":{"characters":["안","녕"],"character_start_times_seconds":[0.1,0],"character_end_times_seconds":[0.2,0.1]}`, "7", true},
		{"invalid-normalized", `,"normalized_alignment":{"characters":["안"],"character_start_times_seconds":[0.2],"character_end_times_seconds":[0.1]}`, "", true},
		{"invalid-reported", "", "invalid", false},
		{"negative-reported", "", "-1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("character-cost", tc.usage)
				_, _ = io.WriteString(w, `{"audio_base64":"`+audio+`"`+tc.tail+`}`)
			}))
			defer server.Close()
			out, err := provider(t, server).SynthesizeSpeech(context.Background(), speechRequest())
			if tc.bad != errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("output %v", err)
			}
			known := tc.usage == "0" || tc.usage == "7" || tc.usage == "7.5"
			if known != (len(out.Evidence.Units) == 1) {
				t.Fatalf("invented/lost reported usage: %+v", out.Evidence)
			}
			if known && (out.Evidence.Units[0].Quantity != tc.usage || out.Evidence.Units[0].Unit != llm.SpeechUnitCharacterCost) {
				t.Fatal("reinterpreted supplier billing units")
			}
			if !tc.bad && out.Audio.Samples <= 0 {
				t.Fatal("missing audio clock")
			}
		})
	}
}

func TestFailureStatusesPreserveUsageAndNeverRetry(t *testing.T) {
	for _, tc := range []struct {
		status int
		kind   error
	}{{401, llm.ErrProviderDisabled}, {429, llm.ErrRateLimited}, {404, llm.ErrModelUnavailable}, {422, llm.ErrUnsupported}, {500, llm.ErrBadOutput}} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("character-cost", "8")
				w.Header().Set("request-id", "failed-call")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, `{"detail":{"status":"fixture_status","message":"private supplier detail"}}`)
			}))
			defer server.Close()
			out, err := provider(t, server).SynthesizeSpeech(context.Background(), speechRequest())
			if !errors.Is(err, tc.kind) || calls.Load() != 1 || out.Evidence.RequestID != "failed-call" || len(out.Evidence.Units) != 1 || out.Evidence.Units[0].Quantity != "8" {
				t.Fatalf("failure lost usage or retried: %+v %v", out.Evidence, err)
			}
			failure := llm.NormalizeFailure(err)
			if len(failure.Params) != 0 || failure.TechnicalDetail != "private supplier detail" {
				t.Fatal("supplier detail escaped normalization")
			}
		})
	}
}

func TestBoundedResponsesAndAudio(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(http.ResponseWriter)
	}{
		{"response-cap", func(w http.ResponseWriter) {
			_, _ = io.CopyN(w, strings.NewReader(strings.Repeat("x", llm.SpeechMaxResponseBytes+2)), llm.SpeechMaxResponseBytes+2)
		}},
		{"audio-cap", func(w http.ResponseWriter) {
			_ = json.NewEncoder(w).Encode(map[string]any{"audio_base64": strings.Repeat("A", base64.StdEncoding.EncodedLen(llm.SpeechMaxAudioBytes)+4)})
		}},
		{"invalid-base64", func(w http.ResponseWriter) { _, _ = io.WriteString(w, `{"audio_base64":"!invalid"}`) }},
		{"invalid-audio", func(w http.ResponseWriter) { _, _ = io.WriteString(w, `{"audio_base64":"YWJj"}`) }},
		{"invalid-json", func(w http.ResponseWriter) { _, _ = io.WriteString(w, `{"audio_base64":`) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("character-cost", "9"); tc.write(w) }))
			defer server.Close()
			out, err := provider(t, server).SynthesizeSpeech(context.Background(), speechRequest())
			if !errors.Is(err, llm.ErrBadOutput) || len(out.Evidence.Units) != 1 {
				t.Fatalf("unbounded or lost usage: %v", err)
			}
		})
	}
}

func TestCancellationTimeoutAndDisabledBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("character-cost", "4")
		w.Header().Set("request-id", "interrupted")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()
	p := provider(t, server)
	transport := pTestTransport(server.Client().Transport, started)
	client := server.Client()
	client.Transport = transport
	p, makeErr := elevenlabs.New(llm.SpeechAdapterConfig{ProviderID: "elevenlabs", BaseURL: server.URL, APIKey: "key"}, client)
	if makeErr != nil {
		t.Fatal(makeErr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.SynthesizeSpeech(ctx, speechRequest()); !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatalf("pre-cancel reached network: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	type result struct {
		out llm.SpeechResponse
		err error
	}
	completed := make(chan result, 1)
	go func() { out, err := p.SynthesizeSpeech(ctx, speechRequest()); completed <- result{out, err} }()
	<-started
	cancel()
	got := <-completed
	if !errors.Is(got.err, context.Canceled) || got.out.Evidence.RequestID != "interrupted" || len(got.out.Evidence.Units) != 1 {
		t.Fatalf("cancel lost evidence: %+v %v", got.out.Evidence, got.err)
	}
	client.Timeout = 30 * time.Millisecond
	// Avoid a wall-clock race with local TCP scheduling: the transport controls
	// whether headers were received, while http.Client still owns the deadline.
	for _, headersReceived := range []bool{false, true} {
		client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if !headersReceived {
				<-r.Context().Done()
				return nil, r.Context().Err()
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Request-Id": {"interrupted"}, "Character-Cost": {"4"}}, Body: &deadlineBody{ctx: r.Context()}}, nil
		})
		p, err := elevenlabs.New(llm.SpeechAdapterConfig{ProviderID: "elevenlabs", BaseURL: server.URL, APIKey: "key"}, client)
		if err != nil {
			t.Fatal(err)
		}
		out, err := p.SynthesizeSpeech(context.Background(), speechRequest())
		if !errors.Is(err, context.DeadlineExceeded) || (len(out.Evidence.Units) == 1) != headersReceived {
			t.Fatalf("deadline evidence: headers %v, %+v, %v", headersReceived, out.Evidence, err)
		}
	}
	disabled, err := elevenlabs.New(llm.SpeechAdapterConfig{ProviderID: "elevenlabs", BaseURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = disabled.SynthesizeSpeech(context.Background(), speechRequest()); !errors.Is(err, llm.ErrProviderDisabled) || calls.Load() != 3 {
		t.Fatal("disabled provider called network")
	}
}

type deadlineBody struct{ ctx context.Context }

func (b *deadlineBody) Read([]byte) (int, error) { <-b.ctx.Done(); return 0, b.ctx.Err() }
func (b *deadlineBody) Close() error             { return nil }

type observedBody struct {
	io.ReadCloser
	once    sync.Once
	started chan<- struct{}
}

func (b *observedBody) Read(p []byte) (int, error) {
	b.once.Do(func() { b.started <- struct{}{} })
	return b.ReadCloser.Read(p)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func pTestTransport(inner http.RoundTripper, started chan<- struct{}) http.RoundTripper {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response, err := inner.RoundTrip(r)
		if err == nil {
			response.Body = &observedBody{ReadCloser: response.Body, started: started}
		}
		return response, err
	})
}

func TestDesignRejectsExcessOrDuplicateCandidates(t *testing.T) {
	audio := audioFixture(t)
	req := llm.VoiceDesignRequest{Model: llm.ModelRef{ProviderID: "elevenlabs", ModelID: "design"}, Description: strings.Repeat("한", 20), PreviewText: strings.Repeat("글", 100)}
	for _, count := range []int{0, 2, 4} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				previews := []any{}
				for range count {
					previews = append(previews, map[string]any{"audio_base_64": audio, "generated_voice_id": "duplicate", "media_type": "audio/mpeg"})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"text": req.PreviewText, "previews": previews})
			}))
			defer server.Close()
			if _, err := provider(t, server).DesignVoice(context.Background(), req); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatal(err)
			}
		})
	}
}

func TestConnectionValidationAndNoRedirect(t *testing.T) {
	for _, base := range []string{"garbage", "ftp://example.test", "https://key@example.test", "https://example.test?key=x", "https://example.test#fragment", "https://example.test/v1"} {
		if _, err := elevenlabs.New(llm.SpeechAdapterConfig{ProviderID: "elevenlabs", BaseURL: base}, http.DefaultClient); err == nil {
			t.Fatalf("invalid origin: %s", base)
		}
	}
	var forwarded atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { forwarded.Add(1) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer server.Close()
	if _, err := provider(t, server).SynthesizeSpeech(context.Background(), speechRequest()); !errors.Is(err, llm.ErrBadOutput) || forwarded.Load() != 0 {
		t.Fatal("redirected paid request/key")
	}
}

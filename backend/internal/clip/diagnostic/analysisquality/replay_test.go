package analysisquality

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func fixture(t *testing.T) (Corpus, string, time.Time) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	original := save(t, root, "original.mp4", []byte("explicitly synthetic original"))
	source := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: original.SHA256, Info: clip.MediaInfo{DurationMS: 65000, Width: 1080, Height: 1920, FrameRateNumerator: 30, FrameRateDenominator: 1, DecodedFrames: 1950, HasAudio: true, AudioRate: 48000, AudioChannels: 2}}, Filename: "source.mp4"}
	cs := Case{ID: "case", Source: source, Original: original, MeasurementProvenance: "synthetic_test_only", Rights: Rights{Basis: "operator_owned", Issuer: "synthetic test author", EvidenceSHA256: hash([]byte("synthetic authorization")), ProviderProcessing: true, PrivateRetention: true, ExpiresAt: now.Add(time.Hour)}, Annotator: "synthetic test author", AnnotatedAt: now.Add(-time.Minute), GroundedOriginalSHA256: original.SHA256, Tags: []string{"korean_text", "brief_action", "speech", "shake"}, Labels: []Label{
		{ID: "action", Kind: "event", StartMS: 60000, EndMS: 60100, ToleranceMS: 10, Required: true, Known: true, Expected: "잔을 든다", Normalization: "exact", Evidence: "synthetic written label"},
		{ID: "price", Kind: "number", StartMS: 60000, EndMS: 61000, Required: true, Critical: true, Known: true, Expected: "8,900원", Normalization: "exact", Evidence: "synthetic written label"},
		{ID: "silence", Kind: "speech", StartMS: 60000, EndMS: 61000, Required: true, Critical: true, Known: true, Expected: "", Normalization: "exact", Evidence: "synthetic silence"},
		{ID: "sign", Kind: "text", StartMS: 61000, EndMS: 65000, Required: true, Known: false, UnknownReason: "unreadable", Normalization: "exact", Evidence: "synthetic unknown sign"},
		{ID: "focus", Kind: "quality", StartMS: 61000, EndMS: 65000, Known: true, Expected: "sharp", Normalization: "exact", Evidence: "synthetic reviewed quality"},
		{ID: "use", Kind: "usability", StartMS: 61000, EndMS: 65000, Required: true, Critical: true, Known: true, Expected: "unusable", Normalization: "exact", Evidence: "synthetic usability"},
		{ID: "missing", Kind: "scene", StartMS: 61000, EndMS: 65000, Required: true, Known: true, Expected: "brief scene", Normalization: "exact", Evidence: "synthetic missing scene"},
	}}
	p := llm.CallPolicy{Ref: llm.ModelRef{ProviderID: "offline", ModelID: "synthetic"}, Stage: llm.StageNameObserve, CompletionTokens: 8192, InputTokens: 30000, Reasoning: llm.ReasoningLow, StructuredOutput: true, InputUSDPerMillion: "1", OutputUSDPerMillion: "1", Pricing: llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionInlineStatic, Endpoint: "offline_leaf", RequiredParameters: "max_tokens,reasoning,response_format,structured_outputs", PromptUSDPerMillion: "1", CompletionUSDPerMillion: "1", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0"}}
	c := Corpus{Version: Version, Format: Format, CorpusVersion: "synthetic-v1", TruthVersion: "synthetic-v1", GitCommit: strings.Repeat("a", 40), Origin: "synthetic_mock", Language: "ko", Policy: p, Replicates: 1, Cases: []Case{cs}}
	w, h := clip.BrowserAnalysisGeometry(source.Info, 720)
	for _, arm := range arms {
		f := save(t, root, arm+".mp4", []byte("synthetic proxy "+arm))
		c.Inputs = append(c.Inputs, Input{ID: arm, CaseID: cs.ID, Arm: arm, Profile: clip.BrowserAnalysisProfileVersion, PreparationVersion: "synthetic-production-contract", Encoder: Encoder{Name: arm, Version: "synthetic-v1", Settings: map[string]string{"fps": "15"}, BrowserDeviceBuild: "synthetic test; no device qualification"}, File: f, Copy: clip.AnalysisCopy{Slot: clip.MediaAnalysisSlot(source.ID, 1), SourceID: source.ID, Fingerprint: source.Fingerprint, Index: 1, OffsetMS: 60000, DurationMS: 5000, Width: w, Height: h, HasAudio: true, Bytes: f.Bytes, Digest: f.SHA256}})
	}
	return c, root, now
}

func save(t *testing.T, root, name string, data []byte) File {
	t.Helper()
	if e := os.WriteFile(filepath.Join(root, name), data, 0600); e != nil {
		t.Fatal(e)
	}
	return File{Path: name, SHA256: hash(data), Bytes: int64(len(data))}
}
func rawObservation(price, speech string) string {
	segment := func(start, end int, event, speech, scene string) map[string]any {
		return map[string]any{"start_ms": start, "end_ms": end, "event": event, "action": "action", "motion": "static", "subjects": []string{}, "speech": speech, "quality": "steady and sharp", "focal": map[string]float64{"x": .5, "y": .5}, "scene": scene, "readable_text": true, "subject": map[string]float64{"x": 0, "y": 0, "width": 0, "height": 0}, "certainty": "certain", "usability": "usable"}
	}
	raw, _ := json.Marshal(map[string]any{"source_id": "source", "chunk_index": 1, "segments": []any{segment(0, 1000, "잔을 든다 "+price, speech, "person"), segment(1000, 5000, "실내 매장", "", "interior")}})
	return string(raw)
}
func addReplay(t *testing.T, c *Corpus, root string, now time.Time, id string, rep int, raw string) {
	t.Helper()
	prompts, e := ReplayKeys(*c, now)
	if e != nil {
		t.Fatal(e)
	}
	r := Record{Version: Version, Origin: c.Origin, Key: prompts[id].ReplayKey, Response: llm.Response{Text: raw, Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 100, CostMicrousd: 10, CostReported: true}, FinishReason: "stop"}}
	data, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	f := save(t, root, id+"-"+strings.ReplaceAll(key("rep", rep), "/", "-")+".json", data)
	c.Replays = append(c.Replays, Replay{InputID: id, Replicate: rep, File: f})
}
func fakeVerification(_ context.Context, in Input, _ []byte) (clip.AnalysisCopyVerification, error) {
	return clip.AnalysisCopyVerification{Slot: in.Copy.Slot, Digest: in.File.SHA256, Bytes: in.File.Bytes, Provenance: clip.AnalysisCopyProvenance, VideoPackets: 75, AudioPackets: 235, AudioSamples: 240000, Info: clip.MediaInfo{DurationMS: 5000, ContainerDurationMS: 5000, VideoDurationMS: 5000, AudioDurationMS: 5000, Width: in.Copy.Width, Height: in.Copy.Height, PixelFormat: "yuv420p", SampleAspectRatio: "1:1", FrameRateNumerator: 15, FrameRateDenominator: 1, HasAudio: true, AudioRate: 48000, AudioChannels: 1, DecodedFrames: 75, DecodedDurationMS: 5000, CadenceVerified: true, Streams: []clip.MediaStream{{Index: 0, Kind: "video", Codec: "h264"}, {Index: 1, Kind: "audio", Codec: "aac"}}}}, nil
}
func annotate(c *Corpus, id string, rep int, label, status string, segment int, field string, start, end int, now time.Time) {
	a := Assessment{InputID: id, Replicate: rep, LabelID: label, Status: status, Segment: segment, Field: field, StartRune: start, EndRune: end, Reviewer: "synthetic reviewer", ReviewedAt: now.Add(-time.Second)}
	for _, rp := range c.Replays {
		if rp.InputID == id && rp.Replicate == rep {
			a.ResponseSHA256 = rp.File.SHA256
		}
	}
	for _, cs := range c.Cases {
		for _, l := range cs.Labels {
			if l.ID == label {
				a.TruthLabelDigest = digest(l)
			}
		}
	}
	c.Assessments = append(c.Assessments, a)
}

func TestProductionReplaySeparateMetricsUnknownsAndCriticalFailures(t *testing.T) {
	c, root, now := fixture(t)
	for _, id := range arms {
		price, speech := "8,900원", ""
		if id == "browser" {
			price, speech = "9,800원", "공짜입니다"
		}
		addReplay(t, &c, root, now, id, 0, rawObservation(price, speech))
	}
	for _, id := range arms {
		annotate(&c, id, 0, "action", "correct", 0, "event", 0, 5, now)
		annotate(&c, id, 0, "price", "correct", 0, "event", 6, 12, now)
		end := 0
		if id == "browser" {
			end = 5
		}
		annotate(&c, id, 0, "silence", "correct", 0, "speech", 0, end, now)
		annotate(&c, id, 0, "sign", "unknown", -1, "", 0, 0, now)
		annotate(&c, id, 0, "focus", "correct", 1, "quality", 0, 16, now)
		annotate(&c, id, 0, "use", "correct", 1, "usability", 0, 6, now)
		annotate(&c, id, 0, "missing", "omitted", -1, "", 0, 0, now)
	}
	r, e := Run(t.Context(), c, root, "replay", now, fakeVerification)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Attempts) != 3 || r.Attempts[0].Parsed.Segments[0].StartMS != 60000 || r.Attempts[0].Parsed.Segments[1].EndMS != 65000 {
		t.Fatal("production parser/source offset lost", r.Attempts)
	}
	m := r.Metrics[2]
	if m.Families["event"].Correct != 1 || m.Families["number"].Incorrect != 1 || m.Families["number"].RawExact != 0 || m.Families["speech"].Invented != 1 || m.Families["speech"].SilenceHallucination != 1 || m.Families["text"].TruthUnknown != 1 || m.Families["text"].Unknown != 1 || m.Families["scene"].Omitted != 1 || m.Families["usability"].Incorrect != 1 {
		t.Fatalf("independent scores lost: %+v", m)
	}
	if !m.Boundaries[0].OutsideTolerance || m.Boundaries[0].EndSignedMS != 900 || len(m.CriticalFailures) != 3 || len(r.Qualification.CriticalRegressions) == 0 {
		t.Fatal("critical/time defects averaged away", m)
	}
	if r.Qualification.Qualified || r.Qualification.ProfileEnabled || r.Qualification.SemanticEvidence || r.ProviderCallsSent != 0 || r.MeasuredLiveSpend != nil || r.Variance[0].Status != "unmeasured" || clip.BrowserAnalysisQualified(clip.BrowserAnalysisProfileVersion) {
		t.Fatal("offline evidence opened a real gate")
	}
	if e = writePrivate(filepath.Join(root, "report.json"), r); e != nil {
		t.Fatal(e)
	}
	// Optional explicit artifact destination contains only the synthetic test's
	// arithmetic/provenance. No private corpus, raw response or human identity.
	if path := os.Getenv("POSTPILOT_ANALYSIS_TEST_EVIDENCE"); path != "" {
		proof := struct {
			Version          int               `json:"version"`
			Origin           string            `json:"origin"`
			VerifierOrigin   string            `json:"verifierOrigin"`
			Summary          Summary           `json:"summary"`
			AnalysisContract string            `json:"analysisContract"`
			Profile          string            `json:"profile"`
			Prompts          map[string]Prompt `json:"prompts"`
			Metrics          []Metric          `json:"metrics"`
			Pairs            []Pair            `json:"pairs"`
			Variance         []Variance        `json:"variance"`
			Qualification    Qualification     `json:"qualification"`
			Limits           []string          `json:"limits"`
		}{Version, c.Origin, "synthetic_stub", r.Summary(), r.AnalysisContract, clip.BrowserAnalysisProfileVersion, r.Prompts, r.Metrics, r.Pairs, r.Variance, r.Qualification, r.Limits}
		if e = writePrivate(path, proof); e != nil {
			t.Fatal(e)
		}
	}
}

func TestReplayRefusesIncompatibleFilesAndKeysBeforeVerifier(t *testing.T) {
	for _, failure := range []string{"missing_rights", "expired_rights", "duplicate_id", "changed_copy", "changed_source", "symlink", "traversal", "wrong_profile", "wrong_response_key", "oversized", "unknown_json", "trailing_json", "metadata"} {
		t.Run(failure, func(t *testing.T) {
			c, root, now := fixture(t)
			addReplay(t, &c, root, now, "native", 0, rawObservation("8,900원", ""))
			switch failure {
			case "missing_rights":
				c.Cases[0].Rights.ProviderProcessing = false
			case "expired_rights":
				c.Cases[0].Rights.ExpiresAt = now
			case "duplicate_id":
				c.Inputs[1].ID = c.Inputs[0].ID
			case "changed_copy":
				os.WriteFile(filepath.Join(root, c.Inputs[0].File.Path), []byte("changed"), 0600)
			case "changed_source":
				os.WriteFile(filepath.Join(root, c.Cases[0].Original.Path), []byte("changed"), 0600)
			case "symlink":
				os.Remove(filepath.Join(root, c.Inputs[0].File.Path))
				os.Symlink(filepath.Join(root, c.Inputs[1].File.Path), filepath.Join(root, c.Inputs[0].File.Path))
			case "traversal":
				c.Inputs[0].File.Path = "../copy.mp4"
			case "wrong_profile":
				c.Inputs[0].Profile = "unqualified_profile"
			case "wrong_response_key":
				c.Policy.Pricing.Endpoint = "changed_leaf"
			case "oversized":
				c.Inputs[0].File.Bytes = 9 << 20
				c.Inputs[0].Copy.Bytes = 9 << 20
			case "metadata":
				c.Cases[0].Source.Info.FrameRateDenominator = 0
			case "unknown_json", "trailing_json":
				rp := &c.Replays[0]
				data, _ := os.ReadFile(filepath.Join(root, rp.File.Path))
				if failure == "unknown_json" {
					data = append([]byte(`{"untrusted":true,`), data[1:]...)
				} else {
					data = append(data, []byte(` {}`)...)
				}
				rp.File = save(t, root, rp.File.Path, data)
			}
			calls := 0
			_, e := Run(t.Context(), c, root, "replay", now, func(ctx context.Context, in Input, data []byte) (clip.AnalysisCopyVerification, error) {
				calls++
				return fakeVerification(ctx, in, data)
			})
			if !errors.Is(e, ErrInput) || calls != 0 {
				t.Fatalf("bad input entered verifier/models: %v calls=%d", e, calls)
			}
		})
	}
}

func TestParserFailuresVarianceAndAnnotationProvenance(t *testing.T) {
	c, root, now := fixture(t)
	c.Replicates = 2
	addReplay(t, &c, root, now, "browser", 0, rawObservation("8,900원", ""))
	addReplay(t, &c, root, now, "browser", 1, rawObservation("9,800원", ""))
	annotate(&c, "browser", 0, "price", "correct", 0, "event", 6, 12, now)
	annotate(&c, "browser", 1, "price", "incorrect", 0, "event", 6, 12, now)
	r, e := Run(t.Context(), c, root, "replay", now, fakeVerification)
	if e != nil {
		t.Fatal(e)
	}
	if r.Variance[2].Completed != 2 || r.Variance[2].Status != "measured_offline" || len(r.Variance[2].Disagreements) != 1 || r.Variance[2].LabelDistributions["price"]["correct"] != 1 || r.Variance[2].LabelDistributions["price"]["incorrect"] != 1 || r.Variance[0].Status != "incomplete" {
		t.Fatal("variance lost outputs/missing arms", r.Variance)
	}
	c.Assessments[0].ResponseSHA256 = strings.Repeat("a", 64)
	if _, e = Run(t.Context(), c, root, "replay", now, fakeVerification); !errors.Is(e, ErrInput) {
		t.Fatal("stale output annotation reused", e)
	}
	for _, bad := range []string{"source", "index", "gap", "overlap", "speech"} {
		t.Run(bad, func(t *testing.T) {
			c, root, now := fixture(t)
			raw := rawObservation("8,900원", "")
			switch bad {
			case "source":
				raw = strings.ReplaceAll(raw, `"source"`, `"invented"`)
			case "index":
				raw = strings.ReplaceAll(raw, `"chunk_index":1`, `"chunk_index":0`)
			case "gap":
				raw = strings.ReplaceAll(raw, `"start_ms":1000`, `"start_ms":1001`)
			case "overlap":
				raw = strings.ReplaceAll(raw, `"start_ms":1000`, `"start_ms":999`)
			case "speech":
				c.Cases[0].Source.Info.HasAudio = false
				c.Cases[0].Source.Info.AudioRate = 0
				c.Cases[0].Source.Info.AudioChannels = 0
				for i := range c.Inputs {
					c.Inputs[i].Copy.HasAudio = false
				}
				raw = rawObservation("8,900원", "invented")
			}
			addReplay(t, &c, root, now, "browser", 0, raw)
			verify := fakeVerification
			if bad == "speech" {
				verify = func(ctx context.Context, in Input, data []byte) (clip.AnalysisCopyVerification, error) {
					v, e := fakeVerification(ctx, in, data)
					v.AudioPackets = 0
					v.AudioSamples = 0
					v.Info.HasAudio = false
					v.Info.AudioRate = 0
					v.Info.AudioChannels = 0
					v.Info.AudioDurationMS = 0
					v.Info.Streams = v.Info.Streams[:1]
					return v, e
				}
			}
			r, e := Run(t.Context(), c, root, "replay", now, verify)
			if e != nil || r.Attempts[2].Status != "parse_failed" || r.Attempts[2].Parsed != nil || r.Attempts[2].Diagnostic == nil {
				t.Fatal("production parse failure hidden", e, r.Attempts)
			}
		})
	}
}

func TestCompatibleIndependentCaseKeyAndTruthChanges(t *testing.T) {
	c, _, now := fixture(t)
	before, e := ReplayKeys(c, now)
	if e != nil {
		t.Fatal(e)
	}
	c.Cases[0].Labels[0].Expected = "new grounded description"
	after, e := ReplayKeys(c, now)
	if e != nil || before["native"].ReplayKey != after["native"].ReplayKey {
		t.Fatal("truth change discarded compatible response", e)
	}
	c.Inputs[2].Encoder.Version = "changed_encoder"
	after, e = ReplayKeys(c, now)
	if e != nil || before["native"].ReplayKey != after["native"].ReplayKey || before["browser"].ReplayKey == after["browser"].ReplayKey {
		t.Fatal("profile change did not invalidate only its own row", e)
	}
}

func TestPrivateCLIOutputAndFailureEvidence(t *testing.T) {
	c, root, now := fixture(t)
	addReplay(t, &c, root, now, "native", 0, rawObservation("8,900원", ""))
	data, _ := json.Marshal(c)
	input := save(t, root, "corpus.json", data)
	if _, _, e := Load(filepath.Join(root, input.Path), now); e != nil {
		t.Fatal(e)
	}
	factories := 0
	factory := func(string, string, string) (Verifier, error) { factories++; return fakeVerification, nil }
	var out bytes.Buffer
	output := filepath.Join(root, "run")
	if e := execute(t.Context(), []string{"--mode", "replay", "--input", filepath.Join(root, input.Path), "--output", output}, &out, now, factory); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"report.json", "summary.json"} {
		s, e := os.Stat(filepath.Join(output, name))
		if e != nil || s.Mode().Perm() != 0600 {
			t.Fatal("nonprivate evidence", e)
		}
	}
	if strings.Contains(out.String(), "synthetic reviewer") || strings.Contains(out.String(), "source.mp4") || strings.Contains(out.String(), "8,900") || strings.Contains(out.String(), root) || strings.Contains(out.String(), "offline_leaf") || strings.Contains(out.String(), "event") {
		t.Fatal("private content escaped into public summary", out.String())
	}
	if e := execute(t.Context(), []string{"--input", filepath.Join(root, input.Path), "--output", output}, &out, now, factory); !errors.Is(e, ErrOutput) {
		t.Fatal("existing output overwritten", e)
	}
	before := factories
	for _, args := range [][]string{{"--live", "--input", "missing", "--output", "missing"}, {"--unknown"}, {"--mode", "run", "--input", "missing", "--output", "missing"}, {"--input", "missing", "--output", "missing"}} {
		if e := execute(t.Context(), args, &out, now, factory); e == nil {
			t.Fatal("invalid/live command accepted")
		}
	}
	if factories != before {
		t.Fatal("bad input/live constructed dependencies")
	}
	// Durable failure output is retained, without raw error/response text.
	os.WriteFile(filepath.Join(root, c.Inputs[0].File.Path), []byte("changed"), 0600)
	failure := filepath.Join(root, "failure")
	if e := execute(t.Context(), []string{"--input", filepath.Join(root, input.Path), "--output", failure}, &out, now, factory); !errors.Is(e, ErrInput) {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(failure, "report.json")); e != nil || factories != before {
		t.Fatal("failure evidence absent or dependency constructed", e)
	}
}

func TestMissingArmsUnrunResponsesAndSpeechDenominatorsStayExplicit(t *testing.T) {
	c, root, now := fixture(t)
	c.Cases[0].Labels = append(c.Cases[0].Labels, Label{ID: "utterance", Kind: "speech", StartMS: 61000, EndMS: 65000, Required: true, Known: true, Expected: "오늘은 만 원", Normalization: "exact", Evidence: "synthetic speech label"})
	for _, id := range []string{"reference", "native"} {
		addReplay(t, &c, root, now, id, 0, rawObservation("8,900원", ""))
	}
	annotate(&c, "native", 0, "utterance", "omitted", -1, "", 0, 0, now)
	r, e := Run(t.Context(), c, root, "replay", now, fakeVerification)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range r.Pairs {
		if p.Index == 1 && p.To == "browser" && !p.Missing {
			t.Fatal("unrun browser implied completed paired comparison", p)
		}
	}
	if r.Variance[2].Unrun != 1 || r.Variance[2].Failures != 0 {
		t.Fatal("unrun response counted as actual failure")
	}
	if m := r.Metrics[1].Families["speech"]; m.Omitted != 1 || m.SpeechEdits != len([]rune("오늘은 만 원")) || m.SpeechReferenceRunes != len([]rune("오늘은 만 원")) {
		t.Fatal("omitted speech disappeared from CER", m)
	}
	if m := r.Metrics[2].Families["speech"]; m.Unreviewed != 2 || m.SpeechUnscoredReferenceRunes != len([]rune("오늘은 만 원")) {
		t.Fatal("unreviewed speech masqueraded as zero error", m)
	}
}

func TestStrictPrivateManifestAndStaleTruthAssessment(t *testing.T) {
	c, root, now := fixture(t)
	addReplay(t, &c, root, now, "native", 0, rawObservation("8,900원", ""))
	data, _ := json.Marshal(c)
	for _, invalid := range [][]byte{append([]byte(`{"version":1,`), data[1:]...), append([]byte(`{"Version":1,`), data[1:]...), append([]byte(`{"unknown":true,`), data[1:]...), append(append([]byte(nil), data...), []byte(` {}`)...)} {
		f := save(t, root, "invalid.json", invalid)
		if _, _, e := Load(filepath.Join(root, f.Path), now); !errors.Is(e, ErrInput) {
			t.Fatal("ambiguous/unknown/trailing JSON accepted", e)
		}
	}
	annotate(&c, "native", 0, "price", "correct", 0, "event", 6, 12, now)
	c.Cases[0].Labels[1].Expected = "9,800원"
	if _, e := Run(t.Context(), c, root, "replay", now, fakeVerification); !errors.Is(e, ErrInput) {
		t.Fatal("stale truth mapping reused", e)
	}
	if e := os.Chmod(root, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := Run(t.Context(), c, root, "inspect", now, fakeVerification); !errors.Is(e, ErrInput) {
		t.Fatal("public corpus root accepted", e)
	}
}

package clip

import (
	"encoding/json"
	"strings"
	"testing"
)

func analysisOriginalFixture() (SourceBatch, []BrowserOriginalMeasurement, MediaConfig) {
	cfg := DefaultMediaConfig(Environment{})
	source := SourceLease{ID: "source", SourceMetadata: SourceMetadata{Filename: "source.mp4", ContentType: "video/mp4", Bytes: 10, DurationMS: 61000, Width: 1280, Height: 720, Fingerprint: strings.Repeat("a", 64)}}
	return SourceBatch{Sources: []SourceLease{source}}, []BrowserOriginalMeasurement{{SourceID: "source", Fingerprint: source.Fingerprint, Provenance: BrowserOriginalProvenance, Info: MediaInfo{DurationMS: 61000, Width: 1280, Height: 720, FrameRateNumerator: 30, FrameRateDenominator: 1, DecodedFrames: 1830, CadenceVerified: true}}}, cfg
}
func TestBrowserAnalysisCoverageAndClientProvenance(t *testing.T) {
	batch, originals, cfg := analysisOriginalFixture()
	if e := ValidateBrowserOriginals(batch, originals, cfg); e != nil {
		t.Fatal(e)
	}
	copies, e := ExpectedAnalysisCopies(originals, nil, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if len(copies) != 2 || copies[0].OffsetMS != 0 || copies[0].DurationMS != 60000 || copies[1].OffsetMS != 60000 || copies[1].DurationMS != 1000 || copies[0].Width != 720 || copies[0].Height != 404 {
		t.Fatalf("coverage/geometry: %+v", copies)
	}
	reuse := []AnalysisChunk{{SourceID: "source", Fingerprint: originals[0].Fingerprint, Index: 0, OffsetMS: 0, DurationMS: 60000}}
	missing, e := ExpectedAnalysisCopies(originals, reuse, cfg)
	if e != nil || len(missing) != 1 || missing[0].Index != 1 {
		t.Fatalf("missing=%+v error=%v", missing, e)
	}
	for _, mutate := range []func(*BrowserOriginalMeasurement){
		func(v *BrowserOriginalMeasurement) { v.Provenance = "native_original" },
		func(v *BrowserOriginalMeasurement) { v.Fingerprint = strings.Repeat("b", 64) },
		func(v *BrowserOriginalMeasurement) { v.Info.DurationMS = 120000 },
		func(v *BrowserOriginalMeasurement) { v.Info.Width = 640 },
		func(v *BrowserOriginalMeasurement) { v.Info.FrameRateDenominator = 0 },
		func(v *BrowserOriginalMeasurement) { v.Info.HasAudio = true },
	} {
		changed := originals[0]
		mutate(&changed)
		if ValidateBrowserOriginals(batch, []BrowserOriginalMeasurement{changed}, cfg) == nil {
			t.Fatal("forged original claim accepted")
		}
	}
	if BrowserAnalysisQualified(BrowserAnalysisProfileVersion) {
		t.Fatal("semantic qualification gate opened without T603")
	}
}

func TestAnalysisSourceProvenancePreservesNativeJSONAndBrowserRecoveryIdentity(t *testing.T) {
	native := AnalysisSource{RenderSource: RenderSource{ID: "source", Fingerprint: "fingerprint", Info: MediaInfo{DurationMS: 1000, Width: 320, Height: 180}}, Filename: "source.mp4"}
	raw, e := json.Marshal(native)
	if e != nil || strings.Contains(string(raw), "OriginalMeasurementProvenance") {
		t.Fatal("native serialized cache changed", e)
	}
	browser := native
	browser.OriginalMeasurementProvenance = BrowserOriginalProvenance
	state := RecoveryState{Version: 1, Sources: []AnalysisSource{browser}}
	raw, e = json.Marshal(state)
	var roundTrip RecoveryState
	if e != nil || json.Unmarshal(raw, &roundTrip) != nil || roundTrip.Sources[0].OriginalMeasurementProvenance != BrowserOriginalProvenance {
		t.Fatal("browser recovery erased client provenance", e)
	}
}
func TestAnalysisCopyVerdictRejectsForgedBoundsAndCoverage(t *testing.T) {
	cfg := DefaultMediaConfig(Environment{})
	c := AnalysisCopy{Slot: MediaAnalysisSlot("source", 0), SourceID: "source", Fingerprint: strings.Repeat("a", 64), Index: 0, DurationMS: 1000, Width: 320, Height: 180, Bytes: 20, Digest: strings.Repeat("b", 64)}
	v := AnalysisCopyVerification{Slot: c.Slot, Digest: c.Digest, Bytes: c.Bytes, Provenance: AnalysisCopyProvenance, VideoPackets: 15, Info: MediaInfo{Width: 320, Height: 180, DurationMS: 1000, ContainerDurationMS: 1000, VideoDurationMS: 1000, DecodedDurationMS: 1000, FrameRateNumerator: 15, FrameRateDenominator: 1, CadenceVerified: true, DecodedFrames: 15, PixelFormat: "yuv420p", SampleAspectRatio: "1:1", Streams: []MediaStream{{Kind: "video", Codec: "h264"}}}}
	if e := ValidateAnalysisCopyVerification(c, v, cfg); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*AnalysisCopyVerification){
		func(v *AnalysisCopyVerification) { v.Info.ContainerDurationMS = 65000 },
		func(v *AnalysisCopyVerification) { v.Info.DecodedFrames = 975 },
		func(v *AnalysisCopyVerification) { v.Info.Width = 1920 },
		func(v *AnalysisCopyVerification) { v.Info.CadenceVerified = false },
		func(v *AnalysisCopyVerification) { v.Bytes++ },
		func(v *AnalysisCopyVerification) { v.Digest = strings.Repeat("c", 64) },
		func(v *AnalysisCopyVerification) { v.Info.HasAudio = true },
		func(v *AnalysisCopyVerification) { v.Provenance = BrowserOriginalProvenance },
	} {
		copy := v
		mutate(&copy)
		if ValidateAnalysisCopyVerification(c, copy, cfg) == nil {
			t.Fatal("forged copy accepted")
		}
	}
	task := AnalysisVerificationTask{Version: 1, ProfileVersion: BrowserAnalysisProfileVersion, ManifestDigest: strings.Repeat("f", 64), Copies: []AnalysisCopy{c}}
	if e := task.Validate(); e != nil {
		t.Fatal(e)
	}
	if ValidateAnalysisVerification(task, AnalysisVerificationResult{Version: 1, ProfileVersion: task.ProfileVersion, ManifestDigest: task.ManifestDigest}, cfg) == nil {
		t.Fatal("missing coverage accepted")
	}
	if AnalysisBoundQuoteDigest(strings.Repeat("a", 64), task.ManifestDigest) == strings.Repeat("a", 64) {
		t.Fatal("browser binding remained a legacy native quote")
	}
	native := MediaWorkerProfile{Operation: MediaPrepare, ContractVersion: 3, RendererVersion: MediaRendererVersion, AssetVersion: MediaAssetVersion, Profile: "cpu"}
	if !native.Compatible() {
		t.Fatal("native v3 changed")
	}
	native.Operation = MediaVerifyAnalysis
	if native.Compatible() {
		t.Fatal("native role profile verified copies")
	}
}

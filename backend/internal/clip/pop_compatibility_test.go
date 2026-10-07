package clip

import "testing"

func TestPopExposureRejectsOldExecutionWithoutReinterpretingLegacyResults(t *testing.T) {
	profile := MediaWorkerProfile{ContractVersion: MediaContractVersion, RendererVersion: "cpu-v2", AssetVersion: MediaAssetVersion, Profile: MediaCPUProfile}
	if profile.Compatible() {
		t.Fatal("old hidden-word renderer admitted")
	}
	profile.RendererVersion = MediaRendererVersion
	if !profile.Compatible() {
		t.Fatal("current renderer rejected")
	}
	if (BrowserCompositionContract{Version: BrowserCompositionVersion, SnapshotFingerprint: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Components: "native-cds-r33-v1", Fonts: BrowserFontVersion, Assets: BrowserAssetVersion}).Validate(true) == nil {
		t.Fatal("old browser component registry admitted")
	}
	if (Result{Kind: ""}).RenderKind() != RenderServer {
		t.Fatal("legacy stored server result reinterpreted")
	}
}

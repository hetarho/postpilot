package clip_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestBrowserAuthoritativeIdentityBindsOwnerPlanSourcesSoundAndSpeech(t *testing.T) {
	p := clip.Project{UserID: "alice", ID: "project", EditPlanRevision: 3, EditPlan: "exact stored data", Ratio: "vertical"}
	plan := clip.EditPlan{Cuts: []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: strings.Repeat("a", 64)}}}
	sources := []clip.RenderSource{{ID: "source", Fingerprint: plan.Cuts[0].Fingerprint, Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}
	identity := clip.BrowserCompositionIdentity(p, plan, sources)
	if err := (clip.BrowserCompositionContract{Version: clip.BrowserCompositionVersion, SnapshotFingerprint: identity, Components: clip.BrowserComponentVersion, Fonts: clip.BrowserFontVersion, Assets: clip.BrowserAssetVersion}).Validate(true); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"owner", "revision", "plan", "geometry", "fingerprint", "sound", "speech"} {
		next, edited, refs := p, plan, append([]clip.RenderSource{}, sources...)
		switch change {
		case "owner":
			next.UserID = "bob"
		case "revision":
			next.EditPlanRevision++
		case "plan":
			next.EditPlan += "changed text"
		case "geometry":
			refs[0].Info.Width++
		case "fingerprint":
			refs[0].Fingerprint = strings.Repeat("b", 64)
		case "sound":
			edited.SourceAudio = &clip.SourceAudioSettings{Values: []clip.SourceAudioSetting{{SourceID: "source", Fingerprint: plan.Cuts[0].Fingerprint, RetainOriginal: false}}}
		case "speech":
			edited.Narration = &clip.NarrationPlan{Enabled: true, Segments: []clip.SpokenSegment{{ID: "speech", Speech: &clip.SpeechRef{AudioHash: "audio"}}}}
		}
		if got := clip.BrowserCompositionIdentity(next, edited, refs); got == identity {
			t.Fatal("unbound", change)
		}
	}
}

func TestBrowserKnownResourceVersionsMatchTheFrontendContract(t *testing.T) {
	data, err := os.ReadFile("../../../frontend/src/entities/clip-design/config/browser-composition.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{clip.BrowserCompositionVersion, clip.BrowserComponentVersion, clip.BrowserFontVersion, clip.BrowserAssetVersion} {
		if !strings.Contains(string(data), "'"+id+"'") {
			t.Fatal("frontend/backend version mismatch", id)
		}
	}
	static := regexp.MustCompile(`CLIP_BROWSER_STATIC_CAPTIONS = \[([^]]*)\]`).FindSubmatch(data)
	if len(static) != 2 {
		t.Fatal("missing static caption contract")
	}
	for _, style := range design.CaptionStyles() {
		if strings.Contains(string(static[1]), "'"+style.ID+"'") != style.Static() {
			t.Fatal("static/sequence interval mismatch", style.ID)
		}
	}
	fontAuthority, err := os.ReadFile("media/copy.go")
	if err != nil {
		t.Fatal(err)
	}
	fonts := regexp.MustCompile(`SHA256: "([a-f0-9]{64})"`).FindAllSubmatch(fontAuthority, -1)
	if len(fonts) != 5 {
		t.Fatal("unrecognized font authority inventory")
	}
	for _, font := range fonts {
		if !strings.Contains(string(data), string(font[1])) {
			t.Fatal("font provenance mismatch", string(font[1]))
		}
	}
}

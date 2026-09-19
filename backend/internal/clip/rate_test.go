package clip_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// cadence30 is footage whose decoded frames agree with its declared 30 fps, so
// its cadence counts as verified evidence.
func cadence30(durationMS int) clip.MediaInfo {
	return clip.MediaInfo{DurationMS: durationMS, Width: 1920, Height: 1080,
		FrameRateNumerator: 30, FrameRateDenominator: 1,
		DecodedDurationMS: durationMS, CadenceVerified: true, DecodedFrames: durationMS * 30 / 1000}
}

func ratePlan(t *testing.T, rate int) (clip.Project, clip.EditPlan) {
	t.Helper()
	sources := []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "a", Fingerprint: "fa", Info: cadence30(60000)}, Filename: "a.mp4"}}}
	volume := 1.0
	plan := clip.EditPlan{Ratio: "vertical", Cuts: []clip.Cut{{ID: "one", SourceID: "a", Fingerprint: "fa", EndMS: 20000,
		PlaybackRatePermille: rate, Volume: &volume, Copies: []clip.Caption{{Text: "조용한 골목", Anchor: "bottom", Align: "center", Style: "clean"}}}}}
	plan.DurationMS = plan.Cuts[0].OutputDurationMS()
	// The snapshot is the server's: encoding derives it, so the expectation
	// carries the same derived value a reload will.
	plan.SourceAudio = clip.LegacySourceAudio(plan.Cuts)
	a, _ := json.Marshal(sources)
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	return clip.Project{Ratio: "vertical", Analysis: string(a), EditPlan: raw, EditPlanRevision: 1}, plan
}

func TestEverySupportedRateRoundTripsThroughVersionSix(t *testing.T) {
	for _, rate := range clip.PlaybackRates() {
		p, plan := ratePlan(t, rate)
		if !strings.Contains(p.EditPlan, `"Version":6`) {
			t.Fatal("new plan was not written as an assembly envelope", p.EditPlan)
		}
		again, err := clip.DecodeEditPlan(p.EditPlan)
		if err != nil || !reflect.DeepEqual(again, plan) {
			t.Fatal(rate, again, err)
		}
		if again.Cuts[0].Rate() != rate {
			t.Fatal("rate changed on the way through storage", rate, again.Cuts[0].Rate())
		}
	}
}

func TestTransformedDurationIsExactAndSeparateFromSourceTime(t *testing.T) {
	// 20 s of footage at each rate, to the nearest millisecond.
	for _, c := range []struct{ rate, want int }{{500, 40000}, {750, 26667}, {1000, 20000}, {1250, 16000}, {1500, 13333}, {2000, 10000}} {
		got, ok := clip.TransformedDuration(20000, c.rate)
		if !ok || got != c.want {
			t.Fatal(c.rate, got, ok)
		}
	}
	// Nearest, not truncating: 3 ms at 2x is 2 ms rather than 1 ms.
	if got, _ := clip.TransformedDuration(3, 2000); got != 2 {
		t.Fatal("rounding is not to the nearest millisecond", got)
	}
	for _, c := range []struct{ span, rate int }{{0, 1000}, {-1, 1000}, {1000, 0}, {1000, 1100}, {1000, -1000}} {
		if _, ok := clip.TransformedDuration(c.span, c.rate); ok {
			t.Fatal("accepted an impossible transform", c)
		}
	}
	// A span large enough to overflow the checked multiplication is refused
	// rather than wrapping into a negative timeline.
	if _, ok := composition.TransformedDurationMS(1<<62, 500); ok {
		t.Fatal("unchecked multiplication")
	}
	cut := clip.Cut{StartMS: 5000, EndMS: 25000, PlaybackRatePermille: 2000}
	if cut.SourceSpanMS() != 20000 || cut.OutputDurationMS() != 10000 {
		t.Fatal("source and output time were conflated", cut.SourceSpanMS(), cut.OutputDurationMS())
	}
	// The default caption window is measured on the OUTPUT length.
	withCopy := cut
	withCopy.Copies = []clip.Caption{{Text: "hi"}}
	if start, end := withCopy.CaptionWindow(0); start != 120 || end != 9880 {
		t.Fatal("caption window used source time", start, end)
	}
}

func TestFasterCutValidatesOnTheTransformedTimeline(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	_, plan := ratePlan(t, 2000)
	refs := []clip.RenderSource{{ID: "a", Fingerprint: "fa", Info: cadence30(60000)}}
	if plan.DurationMS != 10000 {
		t.Fatal("20 s at 2x was not a 10 s cut", plan.DurationMS)
	}
	// Below the 15 s floor once transformed, even though the source span is not.
	if err := clip.ValidateEditPlan(cfg, plan, refs); err == nil {
		t.Fatal("duration floor was measured on the source span")
	}
	long := plan
	long.Cuts[0].EndMS = 60000
	long.DurationMS = long.Cuts[0].OutputDurationMS()
	if long.DurationMS != 30000 {
		t.Fatal(long.DurationMS)
	}
	if err := clip.ValidateEditPlan(cfg, long, refs); err != nil {
		t.Fatal(err)
	}
}

func TestSlowRateNeedsVerifiedOriginalCadence(t *testing.T) {
	// 30 fps reaches the output at 1x and above only: 30 × 0.75 is 22.5 fps.
	if got := clip.AllowedPlaybackRates(cadence30(20000)); !reflect.DeepEqual(got, []int{1000, 1250, 1500, 2000}) {
		t.Fatal(got)
	}
	// 60 fps reaches it at 0.5x and everything above.
	fast := cadence30(20000)
	fast.FrameRateNumerator, fast.DecodedFrames = 60, 20000*60/1000
	if got := clip.AllowedPlaybackRates(fast); !reflect.DeepEqual(got, clip.PlaybackRates()) {
		t.Fatal(got)
	}
	// An unmeasured source, and one whose declared rate disagrees with what the
	// decode counted, are both ineligible for a slow rate rather than 1x.
	unmeasured := fast
	unmeasured.DecodedFrames = 0
	unknown := fast
	unknown.CadenceVerified = false
	variable := fast
	variable.DecodedFrames = 20000 * 24 / 1000
	for _, info := range []clip.MediaInfo{unmeasured, variable, unknown} {
		if clip.VerifiedCadence(info).Verified() {
			t.Fatal("unverified cadence passed as evidence", info)
		}
		if got := clip.AllowedPlaybackRates(info); reflect.DeepEqual(got, clip.PlaybackRates()) {
			t.Fatal("slow rate admitted without cadence", got)
		}
	}
	// The plan refuses the rate outright; it is never substituted by 1x.
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	_, plan := ratePlan(t, 750)
	err := clip.ValidateEditPlan(cfg, plan, []clip.RenderSource{{ID: "a", Fingerprint: "fa", Info: cadence30(60000)}})
	if err == nil || plan.Cuts[0].Rate() != 750 {
		t.Fatal("an inadmissible slow rate was accepted or quietly replaced", err)
	}
}

func TestUnsupportedRateAndMalformedAudioSettingsAreRejected(t *testing.T) {
	p, plan := ratePlan(t, 1000)
	for _, bad := range []int{0, 1, 900, 2500, -1000} {
		broken := plan
		broken.Cuts = []clip.Cut{plan.Cuts[0]}
		broken.Cuts[0].PlaybackRatePermille = bad
		if bad != 0 && clip.ValidPlaybackRate(bad) {
			t.Fatal("unsupported rate admitted", bad)
		}
		raw, err := clip.EncodeEditPlan(broken)
		if err != nil {
			continue
		}
		if _, err := clip.DecodeEditPlan(raw); bad != 0 && err == nil {
			t.Fatal("a stored plan kept an unsupported rate", bad)
		}
	}
	// A version-6 plan whose rate map does not name every cut is not readable.
	if _, err := clip.DecodeEditPlan(strings.Replace(p.EditPlan, `"Rates":{"one":1000}`, `"Rates":{}`, 1)); err == nil {
		t.Fatal("a version-6 plan without a stated rate was accepted")
	}
	if _, err := clip.DecodeEditPlan(strings.Replace(p.EditPlan, `"Rates":{"one":1000}`, `"Rates":{"one":0}`, 1)); err == nil {
		t.Fatal("an explicit zero rate was read as 1x")
	}
	audio := `"SourceAudio":[{"SourceID":"a","Fingerprint":"fa","RetainOriginal":true}]`
	if !strings.Contains(p.EditPlan, audio) {
		t.Fatal("the snapshot was not written", p.EditPlan)
	}
	for _, broken := range []string{
		`"SourceAudio":[]`,
		`"SourceAudio":[{"SourceID":"a","Fingerprint":"fa","RetainOriginal":true},{"SourceID":"a","Fingerprint":"fa","RetainOriginal":false}]`,
		`"SourceAudio":[{"SourceID":"a","Fingerprint":"foreign","RetainOriginal":true}]`,
		`"SourceAudio":[{"SourceID":"","Fingerprint":"fa","RetainOriginal":true}]`,
	} {
		if _, err := clip.DecodeEditPlan(strings.Replace(p.EditPlan, audio, broken, 1)); err == nil {
			t.Fatal("malformed source-audio settings accepted", broken)
		}
	}
}

func TestLegacyPlansReadAtOneTimesWithTheirOriginalAudioMeaning(t *testing.T) {
	p, _ := correctionFixture(t)
	base, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	// The version-0 cut: no Volume field at all, which rendered at full original
	// sound. Version 1–4 state the volume in permille instead.
	type v0Cut struct {
		ID, SourceID, Fingerprint string
		StartMS, EndMS            int
		TransitionMS              int
		Focal                     clip.Point
		Copy                      clip.Caption
		Chips                     []string
		Volume                    *float64
	}
	v0 := struct {
		Ratio      string
		DurationMS int
		Cuts       []v0Cut
	}{base.Ratio, base.DurationMS, nil}
	for _, c := range base.Cuts {
		v0.Cuts = append(v0.Cuts, v0Cut{c.ID, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS, c.TransitionMS, c.Focal, c.FirstCopy(), c.Chips, nil})
	}
	rawV0, _ := json.Marshal(v0)
	silent := `{"Version":4,"Ratio":"vertical","Plan":{"DurationMS":20000,"Cuts":[` +
		`{"ID":"one","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":10000,"TransitionMS":0,` +
		`"Copies":[{"Text":"조용한 골목","Anchor":"bottom","Align":"center","Style":"clean","Accent":"","Keyword":"","StartMS":0,"EndMS":0}],"Chips":null,"VolumePermille":0},` +
		`{"ID":"two","SourceID":"s","Fingerprint":"f","StartMS":10000,"EndMS":20000,"TransitionMS":0,` +
		`"Copies":[{"Text":"9900원","Anchor":"top","Align":"left","Style":"memo","Accent":"","Keyword":"","StartMS":0,"EndMS":0}],"Chips":null,"VolumePermille":0}],"Hook":""},` +
		`"Focals":{"one":{"X":0.5,"Y":0.5},"two":{"X":0.5,"Y":0.5}},"CopyStyles":["clean","memo"]}`
	for _, c := range []struct {
		name, raw string
		retain    bool
	}{{"v0 nil volume", string(rawV0), true}, {"v4 muted", silent, false}} {
		decoded, err := clip.DecodeEditPlan(c.raw)
		if err != nil {
			t.Fatal(c.name, err)
		}
		if decoded.SourceAudio == nil {
			t.Fatal(c.name, "no snapshot was derived")
		}
		for _, cut := range decoded.Cuts {
			if decoded.RetainsOriginalAudio(cut) != c.retain {
				t.Fatal(c.name, "audio meaning changed", cut.ID)
			}
		}
		// Per-cut volume is an independent gain and is left exactly as saved.
		if c.name == "v0 nil volume" && decoded.Cuts[0].OriginalVolume() != 1 {
			t.Fatal("per-cut volume changed")
		}
		for _, cut := range decoded.Cuts {
			if cut.Rate() != clip.RateUnitPermille {
				t.Fatal(c.name, "a legacy cut did not read at 1x")
			}
		}
	}
	// Encoding it again is version 6, and reading that back is identical.
	rewritten, err := clip.EncodeEditPlan(base)
	if err != nil || !strings.Contains(rewritten, `"Version":6`) {
		t.Fatal(rewritten, err)
	}
	again, err := clip.DecodeEditPlan(rewritten)
	if err != nil || !reflect.DeepEqual(again, base) {
		t.Fatal("rewriting a legacy plan changed it", err)
	}
}

func TestLegacyOverlapSurvivesButCannotGrow(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	sources := []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "a", Fingerprint: "fa", Info: cadence30(60000)}, Filename: "a.mp4"}}}
	analysis, _ := json.Marshal(sources)
	copies := []clip.Caption{{Text: "조용한 골목", Anchor: "bottom", Align: "center", Style: "clean"}}
	// Two cuts of the same footage that already share 2 s.
	overlapping := clip.EditPlan{Ratio: "vertical", DurationMS: 20000, Cuts: []clip.Cut{
		{ID: "one", SourceID: "a", Fingerprint: "fa", StartMS: 0, EndMS: 10000, Copies: copies},
		{ID: "two", SourceID: "a", Fingerprint: "fa", StartMS: 8000, EndMS: 18000, Copies: copies},
	}}
	raw, err := clip.EncodeEditPlan(overlapping)
	if err != nil {
		t.Fatal(err)
	}
	p := clip.Project{Ratio: "vertical", Analysis: string(analysis), EditPlan: raw, EditPlanRevision: 1}
	saved, err := clip.DecodeEditPlan(raw)
	if err != nil {
		t.Fatal("a saved overlap stopped being readable", err)
	}
	if len(clip.SourceOverlaps(saved.Cuts)) != 1 {
		t.Fatal("the saved overlap disappeared")
	}
	// A new plan may not create one at all.
	if err := clip.ValidateSourceRanges(saved, nil); err == nil {
		t.Fatal("a new plan was allowed to overlap its own footage")
	}
	// Adjacent, touching ranges are not an overlap: source ranges are half-open.
	adjacent := saved
	adjacent.Cuts = []clip.Cut{saved.Cuts[0], saved.Cuts[1]}
	adjacent.Cuts[1].StartMS = 10000
	if len(clip.SourceOverlaps(adjacent.Cuts)) != 0 {
		t.Fatal("touching endpoints were read as an overlap")
	}
	draft := clip.CorrectionFromPlan(saved)
	// Saving it unchanged keeps the existing overlap.
	if _, err := clip.ApplyCorrection(cfg, p, draft); err != nil {
		t.Fatal("an unchanged legacy overlap was refused", err)
	}
	// Enlarging it is refused, and so is a new one elsewhere.
	grown := clip.CorrectionFromPlan(saved)
	grown.Cuts[1].StartMS = 6000
	grown.DurationMS = 22000
	if _, err := clip.ApplyCorrection(cfg, p, grown); err == nil {
		t.Fatal("an existing overlap was allowed to grow")
	}
	shrunk := clip.CorrectionFromPlan(saved)
	shrunk.Cuts[1].StartMS = 9000
	shrunk.DurationMS = 19000
	if _, err := clip.ApplyCorrection(cfg, p, shrunk); err != nil {
		t.Fatal("shrinking an existing overlap was refused", err)
	}
}

func TestOwnerAudioSnapshotSurvivesCorrectionAndRefusesContradiction(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, draft := correctionFixture(t)
	saved, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	draft.SourceAudio = saved.SourceAudio.Values
	next, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next.SourceAudio.Values, saved.SourceAudio.Values) {
		t.Fatal("saving a plan changed the owner's source-sound choice", next.SourceAudio)
	}
	contradiction := draft
	contradiction.SourceAudio = []clip.SourceAudioSetting{{SourceID: "a", Fingerprint: "fa", RetainOriginal: !saved.RetainsOriginalAudio(saved.Cuts[0])}}
	if _, err := clip.ApplyCorrection(cfg, p, contradiction); err == nil {
		t.Fatal("a plan save was allowed to change source sound")
	}
	// Deleting a cut rebuilds the snapshot around the sources that remain.
	deleted := draft
	deleted.Cuts = []clip.CorrectionCut{draft.Cuts[0]}
	deleted.Cuts[0].EndMS = 20000
	deleted.DurationMS = 20000
	fewer, err := clip.ApplyCorrection(cfg, p, deleted)
	if err != nil {
		t.Fatal(err)
	}
	if len(fewer.SourceAudio.Values) != 1 || fewer.SourceAudio.Values[0].SourceID != "a" {
		t.Fatal("the snapshot did not follow the remaining sources", fewer.SourceAudio)
	}
}

// An owner sound change must be invisible to everything that spends credits or
// reuses model output: the quote digest, the planning recovery identity and the
// frozen job manifest all read the source, never the choice made about it.
func TestOwnerSoundIsOutsideEveryPaidIdentity(t *testing.T) {
	lease := clip.SourceLease{ID: "a", Key: "clip-inputs/alice/a", State: "ready", ActualBytes: 1,
		SourceMetadata: clip.SourceMetadata{Filename: "a.mp4", ContentType: "video/mp4", Fingerprint: "fa", Bytes: 1, DurationMS: 10000, Width: 1920, Height: 1080}}
	silent := clip.SourceBatch{ID: "batch", UserID: "alice", ProjectID: "project", State: "ready", Sources: []clip.SourceLease{lease}}
	audible := silent
	audible.Sources = []clip.SourceLease{lease}
	audible.Sources[0].RetainOriginalAudio = true
	if !clip.SameSourceManifest(silent.Sources, audible.Sources) {
		t.Fatal("a sound change invalidated a frozen job manifest")
	}
	p := clip.Project{ID: "project", Ratio: "vertical", TargetDurationMS: 30000, Disclosure: "sponsored"}
	pricing := clip.GenerationPricing{}
	if clip.QuoteInputDigest(p, clip.VideoTemplate{}, silent, pricing) != clip.QuoteInputDigest(p, clip.VideoTemplate{}, audible, pricing) {
		t.Fatal("a sound change changed the approved quote")
	}
	// And the render payload freezes the LEASES, so the executor is handed what
	// the owner has chosen right now.
	cuts := []clip.Cut{{ID: "one", SourceID: "a", Fingerprint: "fa", EndMS: 10000}}
	if clip.FreezeSourceAudio(silent, cuts).Values[0].RetainOriginal {
		t.Fatal("a silent source was frozen as audible")
	}
	if !clip.FreezeSourceAudio(audible, cuts).Values[0].RetainOriginal {
		t.Fatal("the owner's choice was lost on the way to the renderer")
	}
	// A source the batch does not carry has authorized nothing.
	foreign := clip.FreezeSourceAudio(audible, []clip.Cut{{ID: "two", SourceID: "b", Fingerprint: "fb", EndMS: 10000}})
	if foreign.Values[0].RetainOriginal {
		t.Fatal("an unleased source contributed audio")
	}
}

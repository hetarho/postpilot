package clip

import (
	"strings"
	"testing"
)

func TestInitialDraftKeepsNaturalAudioAndRegionsWhileDisplayingIndependentWords(t *testing.T) {
	d, e := NewSpokenDraft([]string{"정확한 대본 그대로."}, nil)
	if e != nil {
		t.Fatal(e)
	}
	d.Narration.VoiceID = "v"
	d.Narration.BindingDigest = strings.Repeat("a", 64)
	ref := SpeechRef{AssetID: "a", VoiceID: "v", BindingDigest: d.Narration.BindingDigest, InputHash: d.Narration.Segments[0].InputHash, SettingsHash: strings.Repeat("b", 64), AudioHash: strings.Repeat("c", 64), ProfileID: "p", ProfileRevision: 1, SampleRate: 44100, Channels: 2, Samples: 44100 * 8}
	d.Narration.Segments[0].Speech = &ref
	duration, e := MeasuredSpokenDraft(&d, 3000, 2000, 15000)
	if e != nil || duration != 15000 || d.Narration.Segments[0].StartMS != 3000 || d.Narration.Segments[0].EndMS != 11000 {
		t.Fatal(duration, e, d)
	}
	p := EditPlan{DurationMS: duration, Portable: &PortablePlan{}, Narration: &d.Narration}
	InitialSpokenCaptions(&p)
	if len(p.Portable.Elements) != 1 || p.Portable.Elements[0].Derived.SegmentID != "spoken-1" {
		t.Fatal("lost origin", p)
	}
	p.Portable.Elements[0].Resolved.Text = "별도의 자막"
	if d.Narration.Segments[0].Text != "정확한 대본 그대로." {
		t.Fatal("caption rewrote speech")
	}
	ref.Samples = 44100 * 20
	if _, e = MeasuredSpokenDraft(&d, 3000, 2000, 15000); e == nil || d.Narration.Segments[0].Speech != &ref {
		t.Fatal("overlong audio was changed or not conflicted")
	}
}
func TestSpokenCorpusEnforcesBothSegmentAndTotalUnicodeBounds(t *testing.T) {
	for _, lines := range [][]string{nil, make([]string, 33), {strings.Repeat("가", 501)}, {strings.Repeat("가", 500), strings.Repeat("가", 500), strings.Repeat("가", 500), strings.Repeat("가", 500), "한"}} {
		if _, e := NewSpokenDraft(lines, nil); e == nil {
			t.Fatal("out-of-scope corpus admitted")
		}
	}
	d, e := NewSpokenDraft([]string{strings.Repeat("가", 500), strings.Repeat("나", 500), strings.Repeat("다", 500), strings.Repeat("라", 500)}, nil)
	if e != nil || len(d.Narration.Segments) != 4 {
		t.Fatal(e)
	}
}

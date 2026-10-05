package clip_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"strings"
	"testing"
)

func spokenFixture(t *testing.T) clip.EditPlan {
	t.Helper()
	project, _ := correctionFixture(t)
	p, err := clip.DecodeEditPlan(project.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	n := &clip.NarrationPlan{Enabled: true, VoiceID: "voice", BindingDigest: strings.Repeat("a", 64), VolumePermille: 1000}
	for i, text := range []string{"서울에서 만난 새로운 맛", "가격은 12,000원입니다"} {
		id := []string{"spoken-1", "spoken-2"}[i]
		hash := clip.SpokenInputHash(text)
		a := &clip.SpeechRef{AssetID: "asset-" + id, VoiceID: "voice", BindingDigest: n.BindingDigest, InputHash: hash, SettingsHash: strings.Repeat("b", 64), AudioHash: strings.Repeat("c", 64), ProfileID: "profile", ProfileRevision: 1, Samples: 44100, SampleRate: 44100, Channels: 2, Timing: []clip.SpeechTiming{{Text: text, StartMS: 0, EndMS: 1000}}}
		n.Segments = append(n.Segments, clip.SpokenSegment{ID: id, Text: text, TextRevision: 1, InputHash: hash, StartMS: i * 3000, EndMS: (i + 1) * 3000, Speech: a})
	}
	p.Narration = n
	return p
}
func TestSpokenPlanRoundtripLegacyAndStrictVersions(t *testing.T) {
	p := spokenFixture(t)
	gain := 321
	p.SourceVolumePermille = &gain
	raw, err := clip.EncodeEditPlan(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := clip.DecodeEditPlan(raw)
	if err != nil || !reflect.DeepEqual(p, got) {
		t.Fatalf("roundtrip: %v\n%#v", err, got)
	}
	var old map[string]any
	if json.Unmarshal([]byte(raw), &old) != nil {
		t.Fatal("json")
	}
	old["Version"] = 6
	delete(old, "Narration")
	delete(old, "SourceVolumePermille")
	b, _ := json.Marshal(old)
	legacy, err := clip.DecodeEditPlan(string(b))
	if err != nil || legacy.Narration != nil || legacy.SourceGainPermille() != 1000 {
		t.Fatalf("old plan acquired speech: %v %+v", err, legacy)
	}
	for _, version := range []int{0, 8, 99} {
		old["Version"] = version
		b, _ = json.Marshal(old)
		if _, err := clip.DecodeEditPlan(string(b)); err == nil {
			t.Fatalf("version %d accepted", version)
		}
	}
	if _, err := clip.DecodeEditPlan(strings.Replace(raw, `"Narration":`, `"Unknown":true,"Narration":`, 1)); err == nil {
		t.Fatal("unknown field accepted")
	}
}
func TestSpokenCorrectionSelectiveStaleAndReuse(t *testing.T) {
	p := spokenFixture(t)
	in := clip.CorrectionFromPlan(p)
	in.Narration.Segments[0].Text = "새로운 서울 이야기"
	next := p
	if err := clip.CorrectNarration(p, in, &next); err != nil {
		t.Fatal(err)
	}
	if next.Narration.Segments[0].TextRevision != 2 || clip.CompatibleSpeech(next.Narration, next.Narration.Segments[0]) || !clip.CompatibleSpeech(next.Narration, next.Narration.Segments[1]) {
		t.Fatal("wrong invalidation")
	}
	if !reflect.DeepEqual(next.Narration.Segments[0].Speech, p.Narration.Segments[0].Speech) {
		t.Fatal("old audio erased")
	}
	in = clip.CorrectionFromPlan(next)
	in.Narration.Segments[0].Text = p.Narration.Segments[0].Text
	reverted := next
	if err := clip.CorrectNarration(next, in, &reverted); err != nil {
		t.Fatal(err)
	}
	if !clip.CompatibleSpeech(reverted.Narration, reverted.Narration.Segments[0]) {
		t.Fatal("exact input not reused")
	}
	in = clip.CorrectionFromPlan(p)
	in.Narration.VoiceID = "another"
	next = p
	if err := clip.CorrectNarration(p, in, &next); err != nil {
		t.Fatal(err)
	}
	for _, s := range next.Narration.Segments {
		if clip.CompatibleSpeech(next.Narration, s) {
			t.Fatal("old voice ready")
		}
	}
}
func TestSpokenIdentityReadinessAndLatePublication(t *testing.T) {
	p := spokenFixture(t)
	if err := clip.NarrationReadiness(p); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*clip.CorrectionPlan){
		func(in *clip.CorrectionPlan) { in.Narration.Segments[0].ID = "foreign" },
		func(in *clip.CorrectionPlan) { in.Narration.Segments[0].Speech.AssetID = "foreign" },
		func(in *clip.CorrectionPlan) { in.Narration.Segments[0].Speech.Samples = 1 },
		func(in *clip.CorrectionPlan) { in.Narration.Segments[0].Speech.AudioHash = strings.Repeat("d", 64) },
	} {
		in := clip.CorrectionFromPlan(p)
		mutate(&in)
		next := p
		if err := clip.CorrectNarration(p, in, &next); err == nil {
			t.Fatal("forged identity accepted")
		}
	}
	in := clip.CorrectionFromPlan(p)
	in.Narration.Segments[1].Text = "바뀐 대본"
	next := p
	if err := clip.CorrectNarration(p, in, &next); err != nil {
		t.Fatal(err)
	}
	var refusal *clip.SpokenError
	if !errors.As(clip.NarrationReadiness(next), &refusal) || refusal.SegmentID != "spoken-2" {
		t.Fatalf("wrong unresolved: %+v", refusal)
	}
	if _, err := clip.PublishSpeech(p, 1, 2, "spoken-1", p.Narration.Segments[0].InputHash, p.Narration.BindingDigest, *p.Narration.Segments[0].Speech); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("late publication accepted")
	}
	in = clip.CorrectionFromPlan(p)
	in.Narration.Segments[0].EndMS = 500
	next = p
	if err := clip.CorrectNarration(p, in, &next); err != nil {
		t.Fatal("editing refused a fit conflict", err)
	}
	if !errors.As(clip.NarrationReadiness(next), &refusal) || refusal.Reason != "spoken_timing_conflict" {
		t.Fatal("fit silently accepted")
	}
	in = clip.CorrectionFromPlan(p)
	in.Narration.Segments = append(in.Narration.Segments, clip.SpokenSegment{Creation: true, Text: "새 문장", StartMS: 6000, EndMS: 9000})
	next = p
	if err := clip.CorrectNarration(p, in, &next); err != nil || next.Narration.Segments[2].ID != "spoken-3" {
		t.Fatal("identity mint", err)
	}
}

func TestSpokenDerivedCaptionFlagsSeparateWordsTimingAndStyle(t *testing.T) {
	project, p := narrationFixture(t)
	link := &clip.DerivedCaption{SegmentID: "spoken-1", TextRevision: 3}
	p.Portable.Elements[2].Derived = link
	raw, err := clip.EncodeEditPlan(p)
	if err != nil {
		t.Fatal(err)
	}
	project.EditPlan = raw
	for _, kind := range []string{"words", "timing", "appearance"} {
		in := clip.CorrectionFromPlan(p)
		text := &in.Elements[2]
		switch kind {
		case "words":
			text.Text = "표시할 별도 문구"
		case "timing":
			start := 2100
			text.StartMS = &start
		case "appearance":
			text.Accent = "teal"
		}
		// Origin and edit flags are read-only. A client cannot relink a caption
		// to a foreign sentence or clear a protection through its projection.
		text.Derived = &clip.DerivedCaption{SegmentID: "foreign", TextRevision: 99}
		next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), project, in)
		if err != nil {
			t.Fatal(kind, err)
		}
		got := next.Portable.Elements[2].Derived
		if got.SegmentID != link.SegmentID || got.TextRevision != 3 || got.TextEdited != (kind == "words") || got.TimingEdited != (kind == "timing") {
			t.Fatalf("%s: %+v", kind, got)
		}
	}
}

func TestSpokenUnicodeAndWholeScriptBounds(t *testing.T) {
	p := spokenFixture(t)
	p.Narration.Segments = nil
	for i := range 4 {
		text := strings.Repeat("한", 500)
		p.Narration.Segments = append(p.Narration.Segments, clip.SpokenSegment{ID: fmt.Sprintf("spoken-%d", i+1), Text: text, InputHash: clip.SpokenInputHash(text), TextRevision: 1, StartMS: i * 1000, EndMS: (i + 1) * 1000})
	}
	if err := clip.ValidateNarration(p); err != nil {
		t.Fatal("2000 Korean code points refused", err)
	}
	extra := clip.SpokenSegment{ID: "spoken-5", Text: "가", InputHash: clip.SpokenInputHash("가"), TextRevision: 1, EndMS: 1}
	p.Narration.Segments = append(p.Narration.Segments, extra)
	if err := clip.ValidateNarration(p); err == nil {
		t.Fatal("whole script overflow accepted")
	}
	p.Narration.Segments = p.Narration.Segments[:1]
	p.Narration.Segments[0].Text += "가"
	p.Narration.Segments[0].InputHash = clip.SpokenInputHash(p.Narration.Segments[0].Text)
	if err := clip.ValidateNarration(p); err == nil {
		t.Fatal("segment overflow accepted")
	}
	p.Narration.Segments = nil
	for i := range 33 {
		s := extra
		s.ID = fmt.Sprintf("spoken-%d", i+1)
		p.Narration.Segments = append(p.Narration.Segments, s)
	}
	if err := clip.ValidateNarration(p); err == nil {
		t.Fatal("segment count overflow accepted")
	}
}

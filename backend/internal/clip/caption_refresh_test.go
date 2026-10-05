package clip_test

import (
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"testing"
)

func TestCaptionWordsKeepsKoreanUnicodeAndContent(t *testing.T) {
	if got := clip.CaptionWords("서울😀에서 만나요", 2); !reflect.DeepEqual(got, []string{"서울😀에서", "만나요"}) {
		t.Fatal(got)
	}
	if got := clip.CaptionWords("한글🙂", 2); !reflect.DeepEqual(got, []string{"한", "글🙂"}) {
		t.Fatal(got)
	}
	if got := clip.CaptionWords("a b", 3); !reflect.DeepEqual(got, []string{"a", "", "b"}) {
		t.Fatal(got)
	}
}
func TestCaptionRefreshIntentRechecksProtectedOriginsAndKeepsTimingSpeech(t *testing.T) {
	project, plan := nativeHistoryFixture(t)
	plan.Narration = spokenFixture(t).Narration
	segment := &plan.Narration.Segments[0]
	segment.Text, segment.TextRevision = "변경된 서울의 맛", 2
	segment.InputHash = clip.SpokenInputHash(segment.Text)
	caption := &plan.Portable.Elements[0]
	caption.Derived = &clip.DerivedCaption{SegmentID: segment.ID, TextRevision: 1, TimingEdited: true}
	project.EditPlan, _ = clip.EncodeEditPlan(plan)
	before := clip.CorrectionFromPlan(plan)
	draft := clip.CorrectionFromPlan(plan)
	draft.RefreshDerivedCaptions = true
	draft.Elements[0].Text = segment.Text
	next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), project, draft)
	if err != nil {
		t.Fatal(err)
	}
	actual := next.Portable.Elements[0]
	if actual.Derived.TextEdited || actual.Derived.TextRevision != 2 || !actual.Derived.TimingEdited {
		t.Fatal(actual.Derived)
	}
	after := clip.CorrectionFromPlan(next)
	if !reflect.DeepEqual(after.Narration, before.Narration) || !reflect.DeepEqual(after.Elements[0].StartMS, before.Elements[0].StartMS) || !reflect.DeepEqual(after.Elements[0].EndMS, before.Elements[0].EndMS) {
		t.Fatal("refresh retimed or synthesized speech")
	}
	// A forged flag/link is ignored; explicit refresh never waives owner authorship.
	plan.Portable.Elements[0].Derived.TextEdited = true
	project.EditPlan, _ = clip.EncodeEditPlan(plan)
	draft.Elements[0].Derived = &clip.DerivedCaption{SegmentID: "foreign", TextRevision: 99}
	protected, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), project, draft)
	if err != nil || protected.Portable.Elements[0].Derived.SegmentID != segment.ID || !protected.Portable.Elements[0].Derived.TextEdited {
		t.Fatal(err, protected)
	}
}
func TestCaptionRefreshRetainsDeletedOriginsAndOnlyOffersExactUneditedWording(t *testing.T) {
	_, plan := nativeHistoryFixture(t)
	plan.Narration = spokenFixture(t).Narration
	plan.Portable.Elements[0].Derived = &clip.DerivedCaption{SegmentID: "removed", TextRevision: 1}
	if len(clip.RefreshCaptionWords(plan)) != 0 {
		t.Fatal("overwrote deleted origin")
	}
	plan.Portable.Elements[0].Derived.SegmentID = "spoken-1"
	plan.Narration.Segments[0].TextRevision++
	plan.Portable.Elements[0].Derived.TextEdited = true
	if len(clip.RefreshCaptionWords(plan)) != 0 {
		t.Fatal("overwrote owner words")
	}
}

func TestCaptionPhraseWordingAndTimingHaveIndependentProtection(t *testing.T) {
	project, plan := nativeHistoryFixture(t)
	plan.Narration = spokenFixture(t).Narration
	plan.Narration.Segments[0].Text = "새 말"
	plan.Narration.Segments[0].InputHash = clip.SpokenInputHash("새 말")
	plan.Narration.Segments[0].TextRevision = 2
	original := &plan.Portable.Elements[0]
	original.Pace = "rapid"
	original.Phrases = []clip.EditablePhrase{{Text: "첫", StartMS: 120, EndMS: 1120}, {Text: "끝", StartMS: 1200, EndMS: 2200}}
	original.Derived = &clip.DerivedCaption{SegmentID: "spoken-1", TextRevision: 1}
	project.EditPlan, _ = clip.EncodeEditPlan(plan)
	in := clip.CorrectionFromPlan(plan)
	in.RefreshDerivedCaptions = true
	in.Elements[0].Text = "새 말"
	in.Elements[0].Phrases[0].Text, in.Elements[0].Phrases[1].Text = "새", "말"
	next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), project, in)
	if err != nil {
		t.Fatal(err)
	}
	caption := next.Portable.Elements[0]
	if caption.Derived.TextEdited || caption.Derived.TimingEdited || caption.Derived.TextRevision != 2 || caption.Phrases[0].StartMS != 120 || caption.Phrases[1].EndMS != 2200 {
		t.Fatal(caption)
	}
	project.EditPlan, _ = clip.EncodeEditPlan(next)
	in = clip.CorrectionFromPlan(next)
	in.Elements[0].Text, in.Elements[0].Phrases[0].Text = "내 말", "내"
	owner, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), project, in)
	if err != nil || !owner.Portable.Elements[0].Derived.TextEdited || owner.Portable.Elements[0].Derived.TimingEdited {
		t.Fatal(err, owner)
	}
}

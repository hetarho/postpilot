package clip_test

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func answers() []clip.Answer {
	return []clip.Answer{
		{Label: "상호", Text: "연남 김밥"},
		{Label: "위치", Text: "서울 연남동"},
		{Label: "가격", Text: "9,900원"},
		{Label: "메뉴", Text: "김치말이국수 / Kimchi Noodles"},
	}
}

// CDS-42 and V11: every number and proper noun a caption states must already be
// in the owner's answers. A wrong price loses the advertiser at once.
func TestGroundedRefusesInventedNumbersAndNames(t *testing.T) {
	for _, text := range []string{
		"", "조용하고 아늑했어요", "9,900원이면 괜찮아요", "9900원", "서울 연남동에서",
		"김치말이국수 강추", "Kimchi Noodles 추천",
	} {
		if !clip.Grounded(text, answers()) {
			t.Fatalf("%q was refused though the answers carry it", text)
		}
	}
	for _, text := range []string{
		// An invented price, an invented time and an invented Latin name.
		"12,000원이었어요", "오후 7시에 갔어요", "Starbucks 앞",
		// And the voice CDS-42 refuses.
		"진짜 최고였어요", "ㅋㅋ 웃겼어요", "맛있어요 🙂",
	} {
		if clip.Grounded(text, answers()) {
			t.Fatalf("%q was accepted with nothing behind it", text)
		}
	}
	// Thousands separators do not decide the match.
	if !clip.Grounded("9900원", answers()) || !clip.Grounded("9,900원", []clip.Answer{{Label: "가격", Text: "9900원"}}) {
		t.Fatal("separator handling")
	}
}

// CDS-41's order — shorten, extend, drop — with the choice recorded.
func TestComposeExposureFallbacksInOrder(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	fits := func(clip.Caption) (clip.Region, bool, error) {
		return clip.Region{X: 100, Y: 1270, Width: 400, Height: 110}, true, nil
	}
	long := "서울 연남동에서 제일 조용한 자리"                                             // 14 characters → 2160 ms
	cut := clip.Cut{ID: "one", EndMS: 2600, Focal: clip.Point{X: .5, Y: .5}} // window 2360 ms
	written := clip.Written{Text: long, ShortText: "조용한 자리", Answers: answers()}

	// ① it fits as written.
	got, decision, err := clip.Compose(canvas, cut, written, "scenery", false, clip.Region{}, nil, []string{"clean", "memo"}, "coral", nil, "", cut.EndMS, fits)
	if err != nil || got.Copy.Text != long || decision.Fallback != "" {
		t.Fatalf("%+v %+v %v", got.Copy, decision, err)
	}
	// ② too short for the sentence, long enough for the alternative.
	short := cut
	short.EndMS = 1600 // window 1360 ms; 조용한 자리 needs 1350
	got, decision, err = clip.Compose(canvas, short, written, "scenery", false, clip.Region{}, nil, []string{"clean", "memo"}, "coral", nil, "", short.EndMS, fits)
	if err != nil || got.Copy.Text != "조용한 자리" || decision.Fallback != "short_text" {
		t.Fatalf("%+v %+v %v", got.Copy, decision, err)
	}
	// ③ too short for both, but the source has room: the cut is extended.
	tiny := cut
	tiny.EndMS = 1200
	got, decision, err = clip.Compose(canvas, tiny, written, "scenery", false, clip.Region{}, nil, []string{"clean", "memo"}, "coral", nil, "", 6000, fits)
	if err != nil || decision.Fallback != "extended_cut" || got.EndMS <= tiny.EndMS {
		t.Fatalf("%+v %+v %v", got.Copy, decision, err)
	}
	if start, end := got.CaptionWindow(); end-start < clip.MinExposureMS(got.Copy.Text) {
		t.Fatalf("the extended cut still does not fit its copy: %d..%d", start, end)
	}
	// ④ no room anywhere: the copy is dropped, and the cut keeps its footage.
	got, decision, err = clip.Compose(canvas, tiny, written, "scenery", false, clip.Region{}, nil, []string{"clean", "memo"}, "coral", nil, "", tiny.EndMS, fits)
	if err != nil || got.Copy.Text != "" || decision.Fallback != "dropped" || got.EndMS != tiny.EndMS {
		t.Fatalf("%+v %+v %v", got.Copy, decision, err)
	}
	// A dropped copy carries no placement at all, which is a valid plan.
	if got.Copy.Style != "" || got.Copy.Anchor != "" || got.Copy.Align != "" {
		t.Fatalf("a dropped copy kept a placement: %+v", got.Copy)
	}
}

// An ungrounded sentence falls to its alternative, and then away.
func TestComposeGroundingFallback(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	fits := func(clip.Caption) (clip.Region, bool, error) {
		return clip.Region{X: 100, Y: 1270, Width: 400, Height: 110}, true, nil
	}
	cut := clip.Cut{ID: "one", EndMS: 6000, Focal: clip.Point{X: .5, Y: .5}}
	invented := clip.Written{Text: "12,000원에 두 그릇", ShortText: "9,900원", Answers: answers()}
	got, decision, err := clip.Compose(canvas, cut, invented, "food", false, clip.Region{}, nil, []string{"clean", "memo", "mark"}, "coral", nil, "", cut.EndMS, fits)
	if err != nil || got.Copy.Text != "9,900원" || decision.Fallback != "short_text" {
		t.Fatalf("%+v %+v %v", got.Copy, decision, err)
	}
	hopeless := clip.Written{Text: "12,000원", ShortText: "15,000원", Answers: answers()}
	got, decision, err = clip.Compose(canvas, cut, hopeless, "food", false, clip.Region{}, nil, []string{"clean"}, "coral", nil, "", cut.EndMS, fits)
	if err != nil || got.Copy.Text != "" || decision.Fallback != "dropped" {
		t.Fatalf("%+v %+v %v", got.Copy, decision, err)
	}
}

// A sentence past its style's own character limit is shortened, not regenerated
// (CDS-20, CDS-23..26) — there is no second paid call to make.
func TestComposeShortensPastAStyleLimit(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	fits := func(clip.Caption) (clip.Region, bool, error) {
		return clip.Region{X: 96, Y: 290, Width: 400, Height: 90}, true, nil
	}
	cut := clip.Cut{ID: "one", EndMS: 6000, Focal: clip.Point{X: .5, Y: .5}}
	// 메모 takes one line of eighteen; nineteen is past it.
	long := strings.Repeat("가", 19)
	written := clip.Written{Text: long, ShortText: strings.Repeat("가", 12), Answers: answers()}
	got, decision, err := clip.Compose(canvas, cut, written, "food", false, clip.Region{}, nil, []string{"clean", "memo"}, "", nil, "", cut.EndMS, fits)
	if err != nil || decision.Fallback != "short_text" || design.Chars(got.Copy.Text) > design.Styles[got.Copy.Style].Chars {
		t.Fatalf("%+v %+v %v", got.Copy, decision, err)
	}
	// A measurement that FAILED is surfaced, never read as "does not fit".
	broken := func(clip.Caption) (clip.Region, bool, error) { return clip.Region{}, false, clip.ErrCopyTooLong }
	if _, _, err := clip.Compose(canvas, cut, clip.Written{Text: "조용한 자리", Answers: answers()}, "food", false, clip.Region{}, nil, []string{"clean", "memo"}, "", nil, "", cut.EndMS, broken); err == nil {
		t.Fatal("a measurement failure was swallowed")
	}
}

// The scene and the readable-text flag come from the segments a cut spans, and
// the subject box arrives in canvas pixels for CDS-38's 15 % rule.
func TestCutSceneAndSubjectProjection(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	analysis := clip.SourceAnalysis{
		Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "s", Fingerprint: "f", Info: clip.MediaInfo{DurationMS: 20000, Width: 1080, Height: 1920}}},
		Segments: []clip.Segment{
			{StartMS: 0, EndMS: 5000, Scene: "menu", ReadableText: true, Subject: clip.Region{X: .25, Y: .5, Width: .5, Height: .25}},
			{StartMS: 5000, EndMS: 20000, Scene: "food"},
		},
	}
	cut := clip.Cut{StartMS: 0, EndMS: 4000, Focal: clip.Point{X: .5, Y: .5}}
	scene, readable := clip.CutScene(cut, analysis)
	if scene != "menu" || !readable {
		t.Fatalf("scene=%s readable=%v", scene, readable)
	}
	subject := clip.CutSubject(canvas, cut, analysis)
	if subject.X != 270 || subject.Width != 540 || subject.Y != 960 || subject.Height != 480 {
		t.Fatalf("subject in canvas pixels: %+v", subject)
	}
	// A stored analysis written before scenes existed reads as the default row
	// with no subject box, never as an unknown scene.
	legacy := clip.SourceAnalysis{Source: analysis.Source, Segments: []clip.Segment{{StartMS: 0, EndMS: 20000}}}
	scene, readable = clip.CutScene(cut, legacy)
	if scene != design.Guards.DefaultScene || readable {
		t.Fatalf("legacy scene=%s readable=%v", scene, readable)
	}
	if got := clip.CutSubject(canvas, cut, legacy); got != (clip.Region{}) {
		t.Fatalf("legacy subject %+v", got)
	}
}

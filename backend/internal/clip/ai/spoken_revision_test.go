package ai_test

import (
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"testing"
)

func TestNarratedAIRevisionChangesSpokenScriptWithoutCaptionOrSynthesisWork(t *testing.T) {
	in := revisionInput(t, clip.RevisionNarration, "더 짧게 설명해요")
	draft, e := clip.NewSpokenDraft([]string{"고기를 올렸어요", "식사가 준비됐어요"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	in.Current.Narration = &draft.Narration
	in.Current.Narration.Enabled = true
	old := in.Current.Portable
	s, models := newService(t, `{"spoken_lines":["고기를 올렸어요","준비됐어요"]}`, true)
	got, _, e := s.Revise(t.Context(), testRef(), in)
	if e != nil {
		t.Fatal(e)
	}
	if len(models.calls) != 1 || !reflect.DeepEqual(got.Cuts, in.Current.Cuts) || !reflect.DeepEqual(got.Portable, old) {
		t.Fatal("spoken revision changed footage or displayed captions")
	}
	if got.Narration.Segments[0].ID != "spoken-1" || got.Narration.Segments[0].TextRevision != 1 || got.Narration.Segments[1].ID != "spoken-2" || got.Narration.Segments[1].TextRevision != 2 || got.Narration.Segments[1].Text != "준비됐어요" {
		t.Fatal(got.Narration)
	}
	if got.Narration.Segments[1].Speech != nil {
		t.Fatal("hidden synthesis")
	}
}
func TestNarratedAIRevisionKeepsTextBoundsBeforeSeparateSpeechApproval(t *testing.T) {
	in := revisionInput(t, clip.RevisionNarration, "고쳐요")
	d, _ := clip.NewSpokenDraft([]string{"음성 원문"}, nil)
	in.Current.Narration = &d.Narration
	s, _ := newService(t, `{"spoken_lines":[]}`, false)
	if _, _, e := s.Revise(t.Context(), testRef(), in); e == nil {
		t.Fatal("unbounded/empty spoken revision accepted")
	}
}

func TestNarratedFlowRevisionHasTwoWritingCallsAndKeepsDisplayedCaptions(t *testing.T) {
	in := revisionInput(t, clip.RevisionBoth, "다른 장면부터 보여주고 대본을 줄여요")
	draft, _ := clip.NewSpokenDraft([]string{"음성 원문"}, nil)
	in.Current.Narration = &draft.Narration
	modelsResponse := revisionFlow(flowResponse(flowCut("cut-one", 15000, 22500, 1000), flowCut("cut-two", 0, 7500, 1000)))
	s, models := newService(t, modelsResponse, true)
	models.responses = []string{modelsResponse, `{"spoken_lines":["짧은 음성"]}`}
	got, _, e := s.Revise(t.Context(), testRef(), in)
	if e != nil {
		t.Fatal(e)
	}
	if len(models.calls) != 2 || got.Narration.Segments[0].Text != "짧은 음성" {
		t.Fatal("flow/script calls", len(models.calls), got.Narration)
	}
	if len(got.Portable.Elements) != len(in.Current.Portable.Elements) {
		t.Fatal("caption count changed")
	}
	for i, text := range got.Portable.Elements {
		prior := in.Current.Portable.Elements[i]
		if text.Resolved.Text != prior.Resolved.Text || text.Resolved.StartMS != prior.Resolved.StartMS || text.Resolved.EndMS != prior.Resolved.EndMS {
			t.Fatal("displayed caption changed", text, prior)
		}
	}
}

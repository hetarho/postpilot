package ai_test

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"strings"
	"testing"
)

func TestSpokenWriterPrecedesFlowWithoutCaptionWritingAndKeepsExactWords(t *testing.T) {
	s, f := newService(t, "", false)
	in := planningInput()
	text := "관찰한 음식을 접시에 담아요."
	body := map[string]any{"storyline": []any{map[string]any{"text": "접시에 음식을 담는 모습", "observation_ids": []string{clip.ObservationID(in.Analyses[0].Source.ID, 0)}}}, "region_slots": []any{}, "spoken_lines": []string{text}}
	raw, _ := json.Marshal(body)
	f.response.Text = string(raw)
	draft, _, e := s.SpokenScript(t.Context(), testRef(), in)
	if e != nil {
		t.Fatal(e)
	}
	if len(f.calls) != 1 || draft.Narration.Segments[0].Text != text || draft.Narration.Segments[0].Speech != nil {
		t.Fatal("script requested hidden work", draft, f.calls)
	}
	if !strings.Contains(f.calls[0].System, "2000") || !strings.Contains(f.calls[0].System, "natural") {
		t.Fatal("missing enforced spoken contract")
	}
	// The bounded schema rejects before any downstream speech invocation.
	body["spoken_lines"] = []string{strings.Repeat("가", 501)}
	raw, _ = json.Marshal(body)
	f.response.Text = string(raw)
	kept, _, e := s.SpokenScript(t.Context(), testRef(), in)
	if e == nil || len(kept.Narration.Segments) != 1 || len([]rune(kept.Narration.Segments[0].Text)) != 501 {
		t.Fatal("oversized script not refused and preserved", kept, e)
	}
}

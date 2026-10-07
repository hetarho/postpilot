package app

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice/spoken"
	"strings"
	"testing"
)

func TestSpokenNativeCompositionsContainActualFieldsWithoutChatRolesOrHandles(t *testing.T) {
	o := spoken.Operation{Name: "name", Description: "description", PreviewText: "preview", CandidateHandle: llm.CandidateHandle("private-candidate"), VoiceHandle: llm.VoiceHandle("private-voice"), Texts: [2]string{"first text", "second text"}}
	o.Profile.Settings = llm.SpeechSettings{Stability: .4, SimilarityBoost: .5, Style: .2, Speed: 1}
	for _, c := range []*llm.RequestComposition{designRequest(o).Composition, confirmRequest(o).Composition, speechRequest(o, 1).Composition} {
		got, err := llm.PreparedCompositionInspection(c)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Fragments) != 0 || len(got.NativeFields) != 2 || got.Status != llm.InspectionPrepared || got.IssuedAt != nil {
			t.Fatalf("invented chat roles or dispatch: %+v", got)
		}
		raw, _ := json.Marshal(got)
		if strings.Contains(string(raw), "private-candidate") || strings.Contains(string(raw), "private-voice") {
			t.Fatal("provider handle leaked")
		}
	}
	c := speechRequest(o, 1).Composition
	if c.NativeFields[0].Text != o.Texts[1] || c.NativeFields[1].Text != llm.SafeSpeechSettingsText(o.Profile.Settings) || len(RequestCompositions()) != 3 {
		t.Fatal("native request differs from actual frozen fields")
	}
}

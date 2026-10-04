package llm

import (
	"strings"
	"testing"
)

func TestSpeechInputBindsExactModelsPrivateIdentityTextAndSettings(t *testing.T) {
	d := VoiceDesignRequest{Model: ModelRef{ProviderID: "p", ModelID: "d"}, Description: strings.Repeat("a", 20), PreviewText: strings.Repeat("가", 100)}
	original, err := d.Input()
	if err != nil || original.InputCharacters != 100 || original.Operation != "voice_design" {
		t.Fatal(original, err)
	}
	changed := d
	changed.Description += "!"
	next, _ := changed.Input()
	if next.Digest == original.Digest {
		t.Fatal("description not bound")
	}
	changed = d
	changed.Model.ModelID = "other"
	next, _ = changed.Input()
	if next.Digest == original.Digest {
		t.Fatal("model not bound")
	}
	r := SpeechRequest{Model: d.Model, Voice: "private-one", Text: "가 나!", Settings: SpeechSettings{Stability: .5, SimilarityBoost: .75, Speed: 1}}
	first, err := r.Input()
	if err != nil || first.InputCharacters != 4 {
		t.Fatal(first, err)
	}
	for _, mutate := range []func(*SpeechRequest){func(v *SpeechRequest) { v.Text = "가나!" }, func(v *SpeechRequest) { v.Voice = "private-two" }, func(v *SpeechRequest) { v.Settings.Style = .2 }} {
		copy := r
		mutate(&copy)
		other, err := copy.Input()
		if err != nil || other.Digest == first.Digest {
			t.Fatal("request change accepted", other, err)
		}
	}
	c := VoiceConfirmationRequest{DesignModel: d.Model, Candidate: "candidate", Name: "one", Description: d.Description}
	ci, err := c.Input()
	if err != nil || ci.InputCharacters != 0 {
		t.Fatal(ci, err)
	}
	c.Candidate = "other"
	ci2, _ := c.Input()
	if ci.Digest == ci2.Digest {
		t.Fatal("candidate not bound")
	}
}

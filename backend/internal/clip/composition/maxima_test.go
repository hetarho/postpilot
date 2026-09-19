package composition_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// A field's effective maximum folds in the positions its ANSWER reaches (CLIP-117). An `ai`
// position is not one of them: the answer is material the writer reads there, and the position's
// own bound belongs to what the model writes (CLIP-118). Folding it in capped a 500-character
// note at the 18 characters of the one line the model writes from it, so an answer the control
// had accepted was refused at generation with the wrong number on screen.
func TestAnAIPositionDoesNotCapTheFieldItReads(t *testing.T) {
	body := `<clip version="1" intro="b" caption="bold" outro="e">` +
		`<field id="note" label="기타 정보" required="false"/>` +
		`<field id="name" label="상호명" required="true"/>` +
		`<repeat for="scenes"><scene id="shot" scope="scene">` +
		`<text id="line" kind="ai" role="info" basis="cut"><value field="note"/>에서 하나만 골라 짧게 쓰세요.</text>` +
		`</scene></repeat>` +
		`<text id="intro" kind="fixed" role="hook" basis="output-start" chars="9"><value field="name"/></text>` +
		`<text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
	d, problem := composition.ReadStored(body, clip.DefaultCompositionLimits())
	if problem != nil {
		t.Fatal(problem)
	}
	if got := d.Maxima["note"]; got != clip.DefaultCompositionLimits().AnswerChars {
		t.Fatalf("an ai position capped the field it only reads: note = %d", got)
	}
	// A fixed position still caps the field it prints: that text lands on screen as typed.
	if got := d.Maxima["name"]; got != 9 {
		t.Fatalf("a fixed position stopped capping the field it prints: name = %d", got)
	}
}

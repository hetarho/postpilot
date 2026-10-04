package media

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// A layout measures each role's values in one resvg call: six captions in one
// style are one measurement however many anchors each tries, and six rows of
// one information role are one more.
func TestALayoutMeasuresEachRoleOnce(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		parts      int
	}{
		{"captions", `<clip version="1">` +
			`<text id="c1" kind="fixed" role="caption" basis="output-start" start="0" end="2">첫 장면</text>` +
			`<text id="c2" kind="fixed" role="caption" basis="output-start" start="2.2" end="4.2">둘째 장면은 여기</text>` +
			`<text id="c3" kind="fixed" role="caption" basis="output-start" start="4.4" end="6.4">셋째 장면</text>` +
			`<text id="c4" kind="fixed" role="caption" basis="output-start" start="6.6" end="8.6">넷째 장면은 저기</text>` +
			`<text id="c5" kind="fixed" role="caption" basis="output-start" start="8.8" end="10.8">다섯째 장면</text>` +
			`<text id="c6" kind="fixed" role="caption" basis="output-start" start="11" end="13">마지막 장면</text></clip>`, 6},
		{"information rows", `<clip version="1"><text id="info" kind="fixed" role="info" position="bottom" basis="output-start" start="0" end="4">` +
			`<row role="caption">체험권</row><row role="caption">주차 가능</row><row role="caption">예약 필수</row>` +
			`<row role="caption">반려견 동반</row><row role="caption">포장 가능</row><row role="caption">단체석</row></text></clip>`, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, r := measured(t)
			runner := &fakeRunner{run: a.runner.Run}
			a.runner = runner
			plan := declaredPlan(t, tc.body, "vertical")
			var layout declaredLayout
			if err := a.WithWorkspace(t.Context(), "batched", func(ws clip.MediaWorkspace) error {
				var err error
				layout, err = r.layoutComposition(t.Context(), ws, plan)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			copies := 0
			for _, element := range layout.elements() {
				for _, part := range element.Parts {
					if part.Kind == "copy" {
						copies++
					}
				}
			}
			if copies != tc.parts {
				t.Fatalf("the layout drew %d copies, want %d", copies, tc.parts)
			}
			if n := measurementCalls(runner); n != 1 {
				t.Fatalf("the layout measured %d times, want once", n)
			}
		})
	}
}

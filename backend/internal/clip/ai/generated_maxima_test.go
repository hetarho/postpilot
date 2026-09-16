package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestWriterIsToldEachDeclaredTextMaximum(t *testing.T) {
	in := nativeInput()
	setNativeBody(&in, `<clip version="1" intro="a" caption="bold" outro="e"><text id="line" kind="ai" role="caption" basis="whole" chars="7">관찰</text><text id="pair" kind="ai" role="info" position="bottom" basis="whole"><row role="label">위치</row><row role="caption" chars="5">관찰</row></text><text id="free" kind="ai" role="caption" basis="whole">관찰</text><text id="opening" kind="fixed" role="hook" basis="output-start"><row>주제</row><row>부제</row></text><text id="closing" kind="fixed" role="ending" basis="output-end"><row>라벨</row><row>점수</row><row>마무리</row></text></clip>`)
	_, user := ai.BuildPlanPrompt(in, 200, config.ClipCompositionLimits())
	var payload struct {
		Limits []struct {
			Element string `json:"element_id"`
			Chars   int    `json:"chars"`
			Row     *int   `json:"row_index"`
		} `json:"generated_text_limits"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	// Only declared maxima, and never a region row — those carry theirs in
	// generated_region_slots with the single-line rule beside them.
	if len(payload.Limits) != 2 {
		t.Fatal(payload.Limits)
	}
	if payload.Limits[0].Element != "line" || payload.Limits[0].Chars != 7 || payload.Limits[0].Row != nil {
		t.Fatal(payload.Limits[0])
	}
	if payload.Limits[1].Element != "pair" || payload.Limits[1].Chars != 5 || payload.Limits[1].Row == nil || *payload.Limits[1].Row != 1 {
		t.Fatal(payload.Limits[1])
	}
	if !strings.Contains(user, "generated_text_limits") {
		t.Fatal("the request never states the bound")
	}
}

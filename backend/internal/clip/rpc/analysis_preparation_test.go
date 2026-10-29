package rpc

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestAnalysisPreparationProfileCarriesBrowserAudioRate(t *testing.T) {
	p := analysisPreparationMessage(clip.AnalysisPreparation{ID: "prep", ProfileVersion: clip.BrowserAnalysisProfileVersion}).GetProfile()
	if p.GetVersion() != "clip-browser-analysis-v2" || p.GetAudioCodec() != "aac" || p.GetAudioChannels() != 1 || p.GetAudioRate() != 48000 || p.GetAudioBitrate() != 96000 || p.GetQualified() {
		t.Fatal("browser analysis profile response drifted", p)
	}
}

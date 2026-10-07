package media

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestAnalysisPacketEOFRejectsHiddenClocksAndIncompleteCoverage(t *testing.T) {
	cfg := clip.DefaultMediaConfig(clip.Environment{})
	c := clip.AnalysisCopy{DurationMS: 1000, Bytes: 100000}
	info := clip.MediaInfo{Streams: []clip.MediaStream{{Index: 0, Kind: "video"}}}
	base := make([]analysisPacket, 15)
	for i := range base {
		timestamp := fmt.Sprintf("%.6f", float64(i)/15)
		base[i] = analysisPacket{StreamIndex: 0, PTS: timestamp, DTS: timestamp, Duration: "0.066667", Size: "100"}
	}
	for name, mutate := range map[string]func([]analysisPacket) []analysisPacket{
		"valid":            func(p []analysisPacket) []analysisPacket { return p },
		"hidden tail":      func(p []analysisPacket) []analysisPacket { p[14].PTS = "65.000000"; return p },
		"nonfinite":        func(p []analysisPacket) []analysisPacket { p[14].PTS = "NaN"; return p },
		"hidden stream":    func(p []analysisPacket) []analysisPacket { p[14].StreamIndex = 7; return p },
		"gap":              func(p []analysisPacket) []analysisPacket { return append(p[:5], p[6:]...) },
		"duplicate sample": func(p []analysisPacket) []analysisPacket { p[14].PTS = p[13].PTS; return p },
		"short coverage":   func(p []analysisPacket) []analysisPacket { return p[:8] },
		"allocation claim": func(p []analysisPacket) []analysisPacket { p[14].Size = "999999999999999999"; return p },
	} {
		t.Run(name, func(t *testing.T) {
			packets := mutate(append([]analysisPacket(nil), base...))
			raw, e := json.Marshal(analysisPackets{Packets: packets})
			if e != nil {
				t.Fatal(e)
			}
			_, _, e = validateAnalysisPackets(raw, info, c, cfg)
			if (e == nil) != (name == "valid") {
				t.Fatal("packet EOF safety result", e)
			}
		})
	}
}

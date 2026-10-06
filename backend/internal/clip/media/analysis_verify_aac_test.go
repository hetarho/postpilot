package media

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// Real T601 WebCodecs synthetic copy, SHA256 facb48b73c141f37edf2d3c4d85a23b6630fb81cc77797a82b1395ba98ad4362.
// Pinned CPU ffprobe 9.0.1 packet EOF equals host 9.0.1; full presentation decode is
// 15 video frames and 48000 samples. No content/semantic assertion is made.
func TestAnalysisPacketEOFAllowsBoundedAACPrimingAndTail(t *testing.T) {
	packets, e := os.ReadFile("testdata/analysis-aac-browser-packets.json")
	if e != nil {
		t.Fatal(e)
	}
	frames, e := os.ReadFile("testdata/analysis-aac-browser-frames.json")
	if e != nil {
		t.Fatal(e)
	}
	cfg := clip.DefaultMediaConfig(clip.Environment{})
	c := clip.AnalysisCopy{DurationMS: 1000, Width: 160, Height: 90, HasAudio: true, Bytes: 8291}
	info := clip.MediaInfo{Streams: []clip.MediaStream{{Index: 0, Kind: "video", Codec: "h264"}, {Index: 1, Kind: "audio", Codec: "aac"}}}
	video, audio, e := validateAnalysisPackets(packets, info, c, cfg)
	if e != nil || video != 15 || audio != 49 {
		t.Fatalf("legitimate2112-sample priming +64-sample tail rejected: video=%d audio=%d error=%v", video, audio, e)
	}
	decoded, samples, e := validateAnalysisFrames(frames, c, cfg)
	if e != nil || decoded != 15 || samples != 48000 {
		t.Fatalf("presentation decode weakened: %d %d %v", decoded, samples, e)
	}
	var base analysisPackets
	if json.Unmarshal(packets, &base) != nil {
		t.Fatal("invalid fixture")
	}
	for name, mutate := range map[string]func(*analysisPackets){
		"extra AAC frame": func(p *analysisPackets) {
			p.Packets = append(p.Packets, analysisPacket{StreamIndex: 1, PTS: "1.045333", DTS: "1.045333", Duration: "0.021333", Size: "1"})
		},
		"hidden audio65s":   func(p *analysisPackets) { p.Packets[0].PTS = "65.000000" },
		"audio gap":         func(p *analysisPackets) { p.Packets = append(p.Packets[:3], p.Packets[4:]...) },
		"duplicate audio":   func(p *analysisPackets) { p.Packets[1].PTS = p.Packets[0].PTS },
		"nonfinite audio":   func(p *analysisPackets) { p.Packets[0].PTS = "NaN" },
		"oversized audio":   func(p *analysisPackets) { p.Packets[0].Size = "8292" },
		"long audio packet": func(p *analysisPackets) { p.Packets[0].Duration = "0.042667" },
	} {
		t.Run(name, func(t *testing.T) {
			p := analysisPackets{Packets: append([]analysisPacket(nil), base.Packets...)}
			mutate(&p)
			raw, e := json.Marshal(p)
			if e != nil {
				t.Fatal(e)
			}
			if _, _, e = validateAnalysisPackets(raw, info, c, cfg); e == nil {
				t.Fatal("raw packet safety guard relaxed", name)
			}
		})
	}
}

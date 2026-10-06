package design

import (
	"math"
	"regexp"
	"strconv"
	"testing"
)

func TestPopKeepsEveryOrdinaryWordTimingExactly(t *testing.T) {
	for count := 1; count <= 4; count++ {
		for index := 0; index < count; index++ {
			for frame := 0; frame <= 180; frame++ {
				progress := float64(frame) / 180
				old := clamp01((progress - (0.05 + float64(index)*0.13)) / 0.30)
				if got := popWordProgress(progress, index, count); got != old {
					t.Fatalf("%d words index%d frame%d: %.17g != %.17g", count, index, frame, got, old)
				}
			}
		}
	}
}

func TestPopExposesLongExactTextAtRepresentativeAndBeforeLastFrame(t *testing.T) {
	style, _ := LookupCaptionStyle("pop")
	alpha := regexp.MustCompile(`rotate\([^)]*\) scale\([^)]*\) translate\([^)]*\)" opacity="([0-9.]+)"`)
	for _, count := range []int{9, 10, 22} {
		frame := CaptionFrame{Style: style, Family: FontFamily(style.Face), Size: 84, Progress: 0.5, DurationMS: 300}
		line := CaptionLine{Y: 720}
		for index := 0; index < count; index++ {
			line.Words = append(line.Words, CaptionWord{Text: "가", X: float64(index * 80), Width: 70})
		}
		frame.Lines = []CaptionLine{line}
		_, body, _ := DrawCaptionFrame(frame)
		matches := alpha.FindAllStringSubmatch(body, -1)
		if len(matches) != count {
			t.Fatalf("%d words: %d drawable word nodes", count, len(matches))
		}
		for index, match := range matches {
			value, _ := strconv.ParseFloat(match[1], 64)
			if value < 0.44-1e-9 {
				t.Fatalf("word%d has unreadable opacity %f", index, value)
			}
		}
		for index := 0; index < count; index++ {
			if p := popWordProgress(0.74, index, count); math.Abs(p-1) > 1e-12 {
				t.Fatalf("word%d never settles: %f", index, p)
			}
		}
	}
}

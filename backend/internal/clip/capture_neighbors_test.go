package clip_test

import (
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"testing"
	"time"
)

func captureClock(n int) *time.Time { at := time.Date(2026, 10, 15, 0, n, 0, 0, time.UTC); return &at }
func videoAt(name string, clock *time.Time) clip.SourceLease {
	return clip.SourceLease{ID: name, SourceMetadata: clip.SourceMetadata{Filename: name, CapturedAt: clock}}
}
func wantNeighbors(t *testing.T, sections []clip.LinkedPostSection, source clip.SourceLease, want []int) {
	t.Helper()
	got := clip.CaptureNeighbors(sections, []clip.SourceLease{source})[source.ID]
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: got %v, want %v", source.Filename, got, want)
	}
}
func TestCaptureNeighborsByTime(t *testing.T) {
	sections := []clip.LinkedPostSection{
		{Photos: []clip.LinkedPhoto{{File: "IMG_100.jpg", CapturedAt: captureClock(30)}}},
		{Photos: []clip.LinkedPhoto{{File: "OTHER_999.jpg", CapturedAt: captureClock(10)}}},
		{Photos: []clip.LinkedPhoto{{File: "IMG_102.jpg"}}},
	}
	wantNeighbors(t, sections, videoAt("IMG_101.mp4", captureClock(20)), []int{0, 1}) // clock wins, no mixing in unclocked section2
	wantNeighbors(t, sections, videoAt("IMG_101.mp4", nil), []int{0, 2})
	wantNeighbors(t, sections, videoAt("different.mp4", captureClock(5)), []int{1})
	wantNeighbors(t, sections, videoAt("different.mp4", captureClock(40)), []int{0})
}
func TestCaptureNeighborsByFileSequence(t *testing.T) {
	sections := []clip.LinkedPostSection{{Photos: []clip.LinkedPhoto{{File: "CAM_000100.jpg"}}}, {Photos: []clip.LinkedPhoto{{File: "CAM_000102.jpg"}}}}
	wantNeighbors(t, sections, videoAt("CAM_000101.MOV", nil), []int{0, 1})
	wantNeighbors(t, sections, videoAt("CAM_000099.mp4", nil), []int{0})
	wantNeighbors(t, sections, videoAt("CAM_000103.mp4", nil), []int{1})
	// Equal digit widths compare without overflowing uint64.
	sections[0].Photos[0].File = "X999999999999999999990.jpg"
	sections[1].Photos[0].File = "X999999999999999999992.jpg"
	wantNeighbors(t, sections, videoAt("X999999999999999999991.mp4", nil), []int{0, 1})
}
func TestCaptureNeighborsNoSharedSequence(t *testing.T) {
	sections := []clip.LinkedPostSection{{Photos: []clip.LinkedPhoto{{File: "IMG_001.jpg"}}}}
	for _, name := range []string{"OTHER_001.mp4", "IMG_01.mp4", "IMG_.mp4", "camera.mp4"} {
		wantNeighbors(t, sections, videoAt(name, nil), []int{})
	}
	wantNeighbors(t, nil, videoAt("IMG_002.mp4", nil), []int{})
}
func TestCaptureNeighborsBothSides(t *testing.T) {
	// The canonical post reorders 31151 before31148; both temporal neighbors remain candidates.
	sections := []clip.LinkedPostSection{{Photos: []clip.LinkedPhoto{{File: "31151.jpg"}}}, {Photos: []clip.LinkedPhoto{{File: "31148.jpg"}}}}
	wantNeighbors(t, sections, videoAt("31149.mp4", nil), []int{0, 1})
	sections[1].Photos = append(sections[1].Photos, clip.LinkedPhoto{File: "31152.jpg"})
	wantNeighbors(t, sections, videoAt("31152.mp4", nil), []int{1})
}
func TestCaptureNeighborsChamSunJooFixture(t *testing.T) {
	headings := []string{"외관", "주문", "상차림", "곁들임", "냉면", "미나리국밥", "마무리"}
	numbers := [][]int{{31134, 31135}, {31137, 31138, 31139}, {31140, 31141, 31142}, {31143, 31144, 31145, 31146}, {31147, 31151, 31148, 31149, 31150, 31152}, {31154, 31155, 31156, 31157, 31158}, {31159, 31160, 31161, 31162, 31163}}
	sections := make([]clip.LinkedPostSection, len(headings))
	for i, group := range numbers {
		sections[i].Heading = headings[i]
		for _, n := range group {
			sections[i].Photos = append(sections[i].Photos, clip.LinkedPhoto{File: fmt.Sprintf("%d.jpg", n)})
		}
	}
	for _, tc := range []struct {
		n    int
		want []int
	}{{31136, []int{0, 1}}, {31153, []int{4, 5}}, {31164, []int{6}}, {31165, []int{6}}} {
		wantNeighbors(t, sections, videoAt(fmt.Sprintf("%d.mp4", tc.n), nil), tc.want)
	}
}

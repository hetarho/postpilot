package clip

import (
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

var cameraSequence = regexp.MustCompile(`^(.*?)([0-9]+)$`)

// CaptureNeighbors gives zero-based section candidates, never an asserted scene match.
// A source uses one metric: available capture instants first, otherwise a shared
// camera sequence. Equal-length digit strings compare numerically without overflow.
func CaptureNeighbors(sections []LinkedPostSection, sources []SourceLease) map[string][]int {
	out := make(map[string][]int, len(sources))
	for _, source := range sources {
		candidates := []int{}
		type photoOrder struct {
			section int
			at      time.Time
			digits  string
		}
		photos := []photoOrder{}
		byTime := source.CapturedAt != nil && !source.CapturedAt.IsZero()
		if byTime {
			for i, section := range sections {
				for _, photo := range section.Photos {
					if photo.CapturedAt != nil && !photo.CapturedAt.IsZero() {
						photos = append(photos, photoOrder{section: i, at: *photo.CapturedAt})
					}
				}
			}
		}
		byTime = byTime && len(photos) > 0
		var digits string
		if !byTime {
			photos = nil
			stem := strings.TrimSuffix(source.Filename, path.Ext(source.Filename))
			sequence := cameraSequence.FindStringSubmatch(stem)
			if sequence != nil {
				digits = sequence[2]
				for i, section := range sections {
					for _, photo := range section.Photos {
						name := strings.TrimSuffix(photo.File, path.Ext(photo.File))
						other := cameraSequence.FindStringSubmatch(name)
						if other != nil && other[1] == sequence[1] && len(other[2]) == len(digits) {
							photos = append(photos, photoOrder{section: i, digits: other[2]})
						}
					}
				}
			}
		}
		before, after := -1, -1
		compare := func(a, b photoOrder) int {
			if byTime {
				return a.at.Compare(b.at)
			}
			return strings.Compare(a.digits, b.digits)
		}
		video := photoOrder{digits: digits}
		if byTime {
			video.at = *source.CapturedAt
		}
		for i, photo := range photos {
			order := compare(photo, video)
			if order <= 0 && (before < 0 || compare(photo, photos[before]) >= 0) {
				before = i
			}
			if order >= 0 && (after < 0 || compare(photo, photos[after]) < 0) {
				after = i
			}
		}
		for _, i := range []int{before, after} {
			if i >= 0 && !slices.Contains(candidates, photos[i].section) {
				candidates = append(candidates, photos[i].section)
			}
		}
		slices.Sort(candidates)
		out[source.ID] = candidates
	}
	return out
}

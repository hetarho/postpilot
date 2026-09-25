package ai

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// CDS-86: a generated row that shrinks or wraps into its slot is kept as the
// writer wrote it; only one still too wide at the floor takes the grounded
// shorter alternative, and none fitting removes it.
func TestRepairJudgesARegionRowByItsSlotFit(t *testing.T) {
	spec, width, ok := design.RegionSlotAt("intro", "a", "vertical", 0)
	if !ok {
		t.Fatal("intro a has no first slot")
	}
	long := "연남동 골목에서 30년째 숯불 한우만 굽는 집"
	if value, action := repairGeneratedSlot(long, nil, spec, width, 0); value != long || action != "" {
		t.Fatalf("a wrapping row was repaired: %q %q", value, action)
	}
	unbreakable := strings.Repeat("하나둘셋넷", 5)
	short := []clip.CopyAlternative{{Text: "해미 한우"}}
	if value, action := repairGeneratedSlot(unbreakable, short, spec, width, 0); value != "해미 한우" || action != "repair" {
		t.Fatalf("an overflowing row kept or lost its alternative: %q %q", value, action)
	}
	if value, action := repairGeneratedSlot(unbreakable, nil, spec, width, 0); value != "" || action != "removal" {
		t.Fatalf("an overflowing row with no alternative stayed: %q %q", value, action)
	}
	// The row's own declared maximum stays the stricter bound (CLIP-116).
	if value, action := repairGeneratedSlot("연남동 숯불 한우 오마카세", short, spec, width, 5); value != "해미 한우" || action != "repair" {
		t.Fatalf("a declared maximum was ignored: %q %q", value, action)
	}
}

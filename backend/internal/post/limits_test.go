package post

import "testing"

// POST-63's range moved here from platform/config with T269. The frontend
// mirrors the three as POST_TAG_COUNT_DEFAULT / _MIN / _MAX.
func TestTagCountRange(t *testing.T) {
	if TagCountRange.Default != 4 || TagCountRange.Min != 1 || TagCountRange.Max != 10 {
		t.Fatalf("tag count range = %+v", TagCountRange)
	}
	for _, n := range []int{0, 11, -1} {
		if TagCountRange.Allows(n) {
			t.Fatalf("%d accepted", n)
		}
	}
	for _, n := range []int{1, 4, 10} {
		if !TagCountRange.Allows(n) {
			t.Fatalf("%d refused", n)
		}
	}
}

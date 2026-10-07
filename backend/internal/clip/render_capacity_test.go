package clip

import "testing"

func TestNativeCapacityDefaultsAndFiniteOverrides(t *testing.T) {
	l := DefaultRenderCapacity(Environment{})
	if l != (RenderCapacity{Active: 1, Waiting: 2, PerAccount: 1}) || l.Validate() != nil {
		t.Fatal(l)
	}
	zero := 0
	l = DefaultRenderCapacity(Environment{ServerRenderActive: 2, ServerRenderWaiting: &zero, ServerRenderPerAccount: 2})
	if l != (RenderCapacity{Active: 2, Waiting: 0, PerAccount: 2}) || l.Validate() != nil {
		t.Fatal(l)
	}
	for _, l := range []RenderCapacity{{}, {1, -1, 1}, {ServerRenderActiveMax + 1, 2, 1}, {1, ServerRenderWaitingMax + 1, 1}, {1, 0, 2}, {1, 2, 0}} {
		if l.Validate() == nil {
			t.Fatalf("unbounded/invalid limit accepted: %+v", l)
		}
	}
}

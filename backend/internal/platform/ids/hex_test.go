package ids_test

import (
	"encoding/hex"
	"regexp"
	"testing"

	"github.com/postpilot/backend/internal/platform/ids"
)

func TestHex128RetainsTheStoredEntityIDContract(t *testing.T) {
	shape := regexp.MustCompile(`^[0-9a-f]{32}$`)
	seen := make(map[string]bool)
	for range 256 {
		value, err := ids.NewHex128()
		if err != nil {
			t.Fatal(err)
		}
		bytes, err := hex.DecodeString(value)
		if !shape.MatchString(value) || err != nil || len(bytes) != 16 {
			t.Fatalf("incompatible entity ID %q: bytes=%d err=%v", value, len(bytes), err)
		}
		if seen[value] {
			t.Fatal("random ID collision in the smoke sample")
		}
		seen[value] = true
	}
}

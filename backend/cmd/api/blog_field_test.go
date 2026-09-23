package main

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/quality"
)

// The wire mapper lives in platform, which may not import quality, so the two lists of 분야 ids
// meet here: every 분야 number, in enum order, names the catalogue's id at the same position
// (ARCH-3, QUAL-23).
func TestBlogFieldWireIdsAreTheQualityCatalogue(t *testing.T) {
	catalogue := quality.Fields()
	if got, want := len(postpilotv1.BlogField_name)-1, len(catalogue); got != want {
		t.Fatalf("the wire names %d 분야 and the catalogue %d", got, want)
	}
	for position, field := range catalogue {
		number := postpilotv1.BlogField(position + 1)
		id, ok := rpcserver.BlogFieldFromProto(number)
		if !ok || id != field.ID {
			t.Errorf("%s maps to %q, %v; the catalogue's %d is %q", number, id, ok, position+1, field.ID)
		}
	}
}

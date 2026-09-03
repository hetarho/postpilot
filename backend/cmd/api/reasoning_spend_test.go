package main

import "testing"

// The two contexts key a model differently, and the adapter between them is the only place
// that knows both: the ledger records the REGISTRY ref (`openrouter/z-ai/glm-5.3-flash`),
// while a catalog row is the provider-local id (`z-ai/glm-5.3-flash`). Getting this wrong is
// silent — every spend signal simply fails to join and the curation surface shows nothing
// however many calls were recorded — so it is asserted here rather than left to the port's
// own tests, which see already-translated rows.
func TestCatalogModelIDStripsOnlyTheRegistrysProviderSegment(t *testing.T) {
	for _, test := range []struct {
		name       string
		recorded   string
		providerID string
		want       string
		wantOK     bool
	}{
		{
			// A provider-local id contains slashes of its own, so only the FIRST segment goes.
			name: "vendor-qualified id keeps its own slash", recorded: "openrouter/z-ai/glm-5.3-flash",
			providerID: "openrouter", want: "z-ai/glm-5.3-flash", wantOK: true,
		},
		{
			name: "single-segment id", recorded: "openrouter/free",
			providerID: "openrouter", want: "free", wantOK: true,
		},
		{
			// Another registry's rows have no catalog row to join to, and are dropped rather
			// than mangled into one.
			name: "another provider's ref is not ours", recorded: "elsewhere/some-model",
			providerID: "openrouter",
		},
		{
			// The prefix must be a whole segment: a provider id that is a prefix of the
			// vendor segment must not be shaved off the front of it.
			name: "a prefix that is not a segment", recorded: "openrouterX/model",
			providerID: "openrouter",
		},
		{name: "no provider id at all", recorded: "openrouter/free"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := catalogModelID(test.recorded, test.providerID)
			if ok != test.wantOK || got != test.want {
				t.Fatalf("catalogModelID(%q, %q) = %q, %v; want %q, %v",
					test.recorded, test.providerID, got, ok, test.want, test.wantOK)
			}
		})
	}
}

package clip

import "reflect"

// SourceManifest strips mutable lifecycle fields before payload identity comparison.
// IDs, immutable object keys, metadata and independently confirmed byte counts remain.
// The owner's source-sound choice is deliberately NOT one of them: changing it
// must never invalidate a frozen payload or look like different footage to a
// model, because it changes the output only (CLIP-100).
func SourceManifest(sources []SourceLease) []SourceLease {
	out := make([]SourceLease, len(sources))
	for i, v := range sources {
		out[i] = SourceLease{ID: v.ID, Key: v.Key, State: v.State, SourceMetadata: v.SourceMetadata, ActualBytes: v.ActualBytes}
	}
	return out
}
func SameSourceManifest(a, b []SourceLease) bool {
	return reflect.DeepEqual(SourceManifest(a), SourceManifest(b))
}

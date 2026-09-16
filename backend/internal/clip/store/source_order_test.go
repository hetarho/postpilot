package store_test

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func orderedIDs(b clip.SourceBatch) []string {
	out := []string{}
	for _, source := range b.Sources {
		out = append(out, source.ID)
	}
	return out
}

// The owner arranges the footage in ① and every later read takes that order,
// which is the order the writer plans in when no instruction says otherwise
// (CLIP-136).
func TestTheOwnersArrangementIsWhatEveryLaterReadReturns(t *testing.T) {
	h := generationSetup(t)
	before, err := h.sources.AvailableBatch(t.Context(), "alice", h.batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := orderedIDs(before)
	if len(ids) < 2 {
		t.Fatal("the fixture has nothing to arrange", ids)
	}
	reversed := make([]string, len(ids))
	for i, id := range ids {
		reversed[len(ids)-1-i] = id
	}
	after, err := h.sources.Reorder(t.Context(), "alice", h.project.ID, h.batch.ID, reversed)
	if err != nil {
		t.Fatal(err)
	}
	if got := orderedIDs(after); !equalIDs(got, reversed) {
		t.Fatal("the arrangement was not applied", got, reversed)
	}
	reloaded, err := h.sources.AvailableBatch(t.Context(), "alice", h.batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := orderedIDs(reloaded); !equalIDs(got, reversed) {
		t.Fatal("a fresh read lost the arrangement", got)
	}
	// Nothing else moved: the sources themselves are the same leases.
	if len(reloaded.Sources) != len(before.Sources) {
		t.Fatal("a source was lost by the reorder")
	}
}

func TestAPartialOrArbitraryArrangementIsRefused(t *testing.T) {
	h := generationSetup(t)
	batch, err := h.sources.AvailableBatch(t.Context(), "alice", h.batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := orderedIDs(batch)
	for name, list := range map[string][]string{
		"a missing source":  ids[:1],
		"a repeated source": {ids[0], ids[0]},
		"a foreign source":  {ids[0], "not-this-batch"},
		"nothing at all":    {},
	} {
		if _, err := h.sources.Reorder(t.Context(), "alice", h.project.ID, h.batch.ID, list); err == nil {
			t.Fatal("the batch was arranged with " + name)
		}
	}
	// Another owner cannot arrange this batch at all.
	if _, err := h.sources.Reorder(t.Context(), "bob", h.project.ID, h.batch.ID, ids); err == nil {
		t.Fatal("a foreign owner arranged the footage")
	}
	// And the order is unchanged after every refusal.
	reloaded, err := h.sources.AvailableBatch(t.Context(), "alice", h.batch.ID)
	if err != nil || !equalIDs(orderedIDs(reloaded), ids) {
		t.Fatal("a refused arrangement still moved the footage", err)
	}
	// A batch already being consumed is not arrangeable.
	h.start(t)
	if _, err := h.sources.Reorder(t.Context(), "alice", h.project.ID, h.batch.ID, ids); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal("a consuming batch was rearranged", err)
	}
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

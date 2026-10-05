package store_test

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"testing"
)

func TestStoppedSpokenRecoveryEditsAreOwnerBoundCASAndPreserveCompatibleAudio(t *testing.T) {
	h, f, p := narratedSetup(t)
	f.failAt = 2
	q := quote(t, h)
	id, e := accept(h, q)
	if e != nil {
		t.Fatal(e)
	}
	p.id = id
	if e = narratedRun(t, h); e == nil {
		t.Fatal("expected partial failure")
	}
	old, e := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if e != nil || old.Spoken == nil {
		t.Fatal(old, e)
	}
	n := old.Spoken.Narration
	n.Segments = append([]clip.SpokenSegment(nil), n.Segments...)
	first := n.Segments[0]
	n.Segments[1].Text = "새로운 마지막 문장"
	if _, e = h.service.CorrectSpokenRecovery(t.Context(), "bob", h.project.ID, clip.RecoveryDigest(old), &n); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal("foreign correction", e)
	}
	if _, e = h.service.CorrectSpokenRecovery(t.Context(), "alice", h.project.ID, "old-checkpoint", &n); !errors.Is(e, clip.ErrPlanConflict) {
		t.Fatal("stale correction", e)
	}
	next, e := h.service.CorrectSpokenRecovery(t.Context(), "alice", h.project.ID, clip.RecoveryDigest(old), &n)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(next.Spoken.Narration.Segments[0], first) || next.Spoken.Narration.Segments[1].TextRevision != 2 || next.PlanReady || !next.FlowReady || next.Plan != "" || next.PlanDigest != old.PlanDigest {
		t.Fatal("checkpoint reuse/protection lost", next)
	}
	if f.calls != 2 || p.scripts != 1 {
		t.Fatal("editing made a paid call")
	}
	if _, e = h.service.CorrectSpokenRecovery(t.Context(), "alice", h.project.ID, clip.RecoveryDigest(old), &n); !errors.Is(e, clip.ErrPlanConflict) {
		t.Fatal("same checkpoint overwrote newer script", e)
	}
}

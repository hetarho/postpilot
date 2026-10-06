package store_test

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"strings"
	"testing"
)

func TestLocalBackgroundVerdictAndUploadRemainRevisionAndCancellationBound(t *testing.T) {
	for _, change := range []string{"digest retry", "revision", "cancel", "promote"} {
		t.Run(change, func(t *testing.T) {
			h, p, draft := completedClip(t)
			h.cfg.BrowserCompositionQualified = true
			h.withCredits(&quotePricing{}, nil)
			id, c, e := h.service.StartBrowserCompositionRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.BrowserCompositionVersion)
			if e != nil {
				t.Fatal(e)
			}
			m := browserMeasurements()
			m.CompositionVersion = c.Version
			m.SnapshotFingerprint = c.SnapshotFingerprint
			m.BackgroundVersion = clip.BrowserBackgroundVersion
			m.BackgroundSnapshotFingerprint = c.SnapshotFingerprint
			m.BackgroundDigest = strings.Repeat("b", 64)
			m.BackgroundComplete = true
			m.BackgroundSampleCount = 3
			m.BackgroundNotices = []clip.PlanNotice{{CopyFallback: clip.CopyFallback{Reason: "composition_contrast", ElementID: "caption", CutID: "cut"}, Action: "shortfall"}}
			if _, e = h.service.PrepareBrowserUpload(t.Context(), "alice", id, 1234); e != nil {
				t.Fatal(e)
			}
			if _, e = h.service.ReportRenderVerdict(t.Context(), "bob", id, m, true); !errors.Is(e, clip.ErrNotFound) {
				t.Fatal(e)
			}
			v, e := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, true)
			if e != nil || !v.Passed || len(v.Notices) != 1 {
				t.Fatal(v, e)
			}
			r, e := h.store.GetBrowserRender(t.Context(), "alice", id)
			if e != nil {
				t.Fatal(e)
			}
			h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
			switch change {
			case "digest retry":
				m.BackgroundDigest = strings.Repeat("c", 64)
				if _, e = h.service.ReportRenderVerdict(t.Context(), "alice", id, m, true); !errors.Is(e, clip.ErrPlanConflict) {
					t.Fatal("changed evidence replaced verdict", e)
				}
				return
			case "revision":
				draft.Cuts[0].VolumePermille = 0
				_, e = h.service.SaveCorrection(t.Context(), "alice", p.ID, p.EditPlanRevision, draft)
			case "cancel":
				_, e = h.service.CancelBrowserRender(t.Context(), "alice", id)
			}
			if e != nil {
				t.Fatal(e)
			}
			got, e := h.service.CompleteBrowserUpload(t.Context(), "alice", id)
			if change == "promote" {
				if e != nil || got.Result.Key != r.ResultKey() {
					t.Fatal(got, e)
				}
				stored, e := h.store.GetBrowserRender(t.Context(), "alice", id)
				if e != nil || stored.StoredAt == nil || stored.Verdict == nil || len(stored.Verdict.Notices) != 1 || stored.Verdict.Measurements.BackgroundDigest != m.BackgroundDigest {
					t.Fatal(stored, e)
				}
				if _, e = h.service.CancelBrowserRender(t.Context(), "alice", id); e != nil {
					t.Fatal(e)
				}
			} else if e == nil {
				t.Fatal("stale/cancelled evidence promoted")
			}
			if h.media.probes != 0 || h.renderer.calls != 0 || len(h.admitter.calls) != 0 {
				t.Fatal("background verdict dispatched paid or native media work")
			}
		})
	}
}

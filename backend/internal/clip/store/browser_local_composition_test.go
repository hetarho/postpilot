package store_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestLocalBrowserVersionDoesNotOpenTheQualificationGate(t *testing.T) {
	h, p, _ := completedClip(t)
	for _, version := range []string{"", "unknown", clip.BrowserCompositionVersion} {
		_, _, err := h.service.StartBrowserCompositionRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, version)
		if version == clip.BrowserCompositionVersion && !errors.Is(err, clip.ErrRenderUnavailable) || version != clip.BrowserCompositionVersion && !errors.Is(err, clip.ErrBrowserCompositionVersion) {
			t.Fatal(version, err)
		}
	}
	if h.renderer.calls != 0 || h.media.probes != 0 {
		t.Fatal("closed gate performed media work")
	}
}

func TestQualifiedLocalBrowserStillRefusesExpiredSourceAdmission(t *testing.T) {
	h, p, _ := completedClip(t)
	h.cfg.BrowserCompositionQualified = true
	h.withCredits(&quotePricing{}, nil)
	if _, err := h.db.Writer.Exec(`UPDATE clip_source_batches SET expires_at='2000-01-01T00:00:00Z' WHERE id=?`, h.batch.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec(`UPDATE clip_source_leases SET retention_expires_at='2000-01-01T00:00:00Z' WHERE batch_id=?`, h.batch.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.service.StartBrowserCompositionRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.BrowserCompositionVersion); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal(err)
	}
}

func TestQualifiedLocalBrowserAdmissionUsesServerIdentityWithoutRasterOrSampling(t *testing.T) {
	h, p, _ := completedClip(t)
	h.cfg.BrowserCompositionQualified = true
	h.withCredits(&quotePricing{}, nil)
	h.renderer.captionErr = clip.ErrCopyTooLong // A native raster/layout call would fail.
	if _, _, err := h.service.StartBrowserCompositionRender(t.Context(), "bob", p.ID, h.batch.ID, p.EditPlanRevision, clip.BrowserCompositionVersion); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("foreign admission", err)
	}
	id, contract, err := h.service.StartBrowserCompositionRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.BrowserCompositionVersion)
	if err != nil || id == "" || contract.Validate(true) != nil {
		t.Fatal(id, contract, err)
	}
	r, err := h.store.GetBrowserRender(t.Context(), "alice", id)
	if err != nil || r.Composition == nil || *r.Composition != contract || r.SampleJobID != "" || r.SampledAt != nil {
		t.Fatal(r, err)
	}
	if h.renderer.calls != 0 || h.media.probes != 0 || len(h.admitter.calls) != 0 {
		t.Fatal("local admission used media or paid execution")
	}
	m := browserMeasurements()
	if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, true); !errors.Is(err, clip.ErrInvalidMedia) {
		t.Fatal("unbound report", err)
	}
	m.CompositionVersion, m.SnapshotFingerprint = contract.Version, contract.SnapshotFingerprint
	m.BackgroundVersion, m.BackgroundSnapshotFingerprint, m.BackgroundDigest = clip.BrowserBackgroundVersion, contract.SnapshotFingerprint, strings.Repeat("b", 64)
	m.BackgroundComplete, m.BackgroundSampleCount = true, 3
	if _, err := h.service.PrepareBrowserUpload(t.Context(), "alice", id, 1234); err != nil {
		t.Fatal(err)
	}
	h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
	if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, true); err != nil {
		t.Fatal(err)
	}
	stored, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id)
	if err != nil || stored.Result.Kind != clip.RenderBrowser || stored.Result.Key != r.ResultKey() {
		t.Fatal(stored, err)
	}
}

func TestQualifiedLocalBrowserPreservesPublicationGuards(t *testing.T) {
	for _, change := range []string{"revision", "cancel", "finalized", "retention"} {
		t.Run(change, func(t *testing.T) {
			h, p, draft := completedClip(t)
			h.cfg.BrowserCompositionQualified = true
			h.withCredits(&quotePricing{}, nil)
			id, _, err := h.service.StartBrowserCompositionRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.BrowserCompositionVersion)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "revision":
				draft.Cuts[0].VolumePermille = 0
				_, err = h.service.SaveCorrection(t.Context(), "alice", p.ID, p.EditPlanRevision, draft)
			case "cancel":
				_, err = h.service.CancelBrowserRender(t.Context(), "alice", id)
			case "finalized":
				_, err = h.db.Writer.Exec(`UPDATE clip_projects SET finalized_at='2026-10-06T00:00:00Z',finalized_plan_revision=edit_plan_revision,finalized_result_key=result_key,source_access_revoked_at='2026-10-06T00:00:00Z' WHERE id=?`, p.ID)
			case "retention":
				_, err = h.db.Writer.Exec(`UPDATE clip_projects SET source_access_revoked_at='2026-10-06T00:00:00Z' WHERE id=?`, p.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.service.PrepareBrowserUpload(t.Context(), "alice", id, 1234); err == nil {
				t.Fatal("changed state published")
			}
		})
	}
}

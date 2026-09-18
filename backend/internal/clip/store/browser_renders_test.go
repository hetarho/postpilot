package store_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestEitherRenderKindRefusesLayoutBeforeWork(t *testing.T) {
	for _, kind := range []clip.RenderKind{clip.RenderServer, clip.RenderBrowser} {
		t.Run(string(kind), func(t *testing.T) {
			h, p, _ := completedClip(t)
			h.renderer.captionErr = clip.ErrCopyTooLong
			_, err := h.service.StartRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, kind)
			if !errors.Is(err, clip.ErrCopyTooLong) {
				t.Fatal(err)
			}
			if h.renderer.calls != 0 || h.media.probes != 0 || len(h.admitter.calls) != 0 {
				t.Fatal("work before admission")
			}
			for _, table := range []string{"clip_browser_renders", "generation_jobs", "usage_admissions"} {
				var count int
				if err := h.db.Reader.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
					t.Fatal(table, count, err)
				}
			}
		})
	}
}

func TestBrowserVerdictIsOwnedRevisionBoundAndDoesNotDecodeOrPromoteAFile(t *testing.T) {
	h, p, draft := completedClip(t)
	id, err := h.service.StartRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.RenderBrowser)
	if err != nil {
		t.Fatal(err)
	}
	m := clip.RenderMeasurements{Width: 1080, Height: 1920, FrameRateNumerator: 30, FrameRateDenominator: 1, VideoFrames: 900, VideoCodec: "h264", VideoProfile: "High"}
	if _, err := h.service.ReportRenderVerdict(t.Context(), "bob", id, m, true); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	v, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, true)
	if err != nil || !v.Passed {
		t.Fatal(v, err)
	}
	if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, true); err != nil {
		t.Fatal("retry", err)
	}
	r, err := h.store.GetBrowserRender(t.Context(), "alice", id)
	if err != nil || r.Verdict == nil || !reflect.DeepEqual(*r.Verdict, v) {
		t.Fatal(r, err)
	}
	m.Width = 720
	if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, false); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("changed report accepted", err)
	}
	unchanged, err := h.projects.GetProject(t.Context(), "alice", p.ID)
	if err != nil || unchanged.Result.Key != p.Result.Key || unchanged.Result.Kind != clip.RenderServer {
		t.Fatal(unchanged, err)
	}
	if h.media.probes != 0 || h.renderer.calls != 0 || len(h.admitter.calls) != 0 {
		t.Fatal("verdict performed media or paid work")
	}
	draft.Cuts[0].VolumePermille = 0
	if _, err := h.service.SaveCorrection(t.Context(), "alice", p.ID, p.EditPlanRevision, draft); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, false); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("stale verdict accepted", err)
	}
}

func TestFailingBrowserVerdictRetainsMeasurementsAndNotices(t *testing.T) {
	h, p, _ := completedClip(t)
	id, err := h.service.StartRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.RenderBrowser)
	if err != nil {
		t.Fatal(err)
	}
	m := clip.RenderMeasurements{Width: 720, Height: 1280, FrameRateNumerator: 24, FrameRateDenominator: 1, VideoFrames: 500, VideoCodec: "hevc", VideoProfile: "Main"}
	v, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, true)
	if err != nil || v.Passed || len(v.Notices) != 4 {
		t.Fatal(v, err)
	}
	r, err := h.store.GetBrowserRender(t.Context(), "alice", id)
	if err != nil || !reflect.DeepEqual(r.Verdict.Measurements, m) || len(r.Verdict.Notices) != 4 {
		t.Fatal(r, err)
	}
}

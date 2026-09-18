package store_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func browserUpload(t *testing.T, h *generationHarness, p clip.Project) (string, clip.BrowserRender) {
	t.Helper()
	id, err := h.service.StartRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.RenderBrowser)
	if err != nil {
		t.Fatal(err)
	}
	put, err := h.service.PrepareBrowserUpload(t.Context(), "alice", id, 1234)
	if err != nil || put.Headers["If-None-Match"] != "*" {
		t.Fatal(put, err)
	}
	r, err := h.store.GetBrowserRender(t.Context(), "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	return id, r
}

func browserMeasurements() clip.RenderMeasurements {
	return clip.RenderMeasurements{Width: 1080, Height: 1920, FrameRateNumerator: 30, FrameRateDenominator: 1, VideoFrames: 900, VideoCodec: "h264", VideoProfile: "High"}
}

func TestSlowBrowserEncodeCanUploadAndCompleteWithAFreshStoredFile(t *testing.T) {
	h, p, _ := completedClip(t)
	id, err := h.service.StartRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.RenderBrowser)
	if err != nil {
		t.Fatal(err)
	}
	// It took two hours to encode. No object existed during that time.
	if _, err := h.db.Writer.Exec("UPDATE clip_browser_renders SET created_at=? WHERE id=?", time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339Nano), id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.PrepareBrowserUpload(t.Context(), "alice", id, 1234); err != nil {
		t.Fatal(err)
	}
	r, err := h.store.GetBrowserRender(t.Context(), "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
	if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, browserMeasurements(), true); err != nil {
		t.Fatal(err)
	}
	stored, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id)
	if err != nil || stored.Result.Key != r.ResultKey() {
		t.Fatal(stored, err)
	}
}

func TestBrowserUploadPromotesOnlyStoredPassingFileAndRetryDoesNotDeleteIt(t *testing.T) {
	h, p, _ := completedClip(t)
	id, r := browserUpload(t, h, p)
	if _, err := h.service.PrepareBrowserUpload(t.Context(), "bob", id, 1234); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := h.service.CompleteBrowserUpload(t.Context(), "bob", id); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := h.service.PrepareBrowserUpload(t.Context(), "alice", id, 1235); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal(err)
	}
	if _, err := h.service.PrepareBrowserUpload(t.Context(), "alice", id, 1234); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal(err)
	}
	if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, browserMeasurements(), true); err != nil {
		t.Fatal(err)
	}
	for _, info := range []clip.SourceObjectInfo{{}, {Bytes: 12, ContentType: "video/mp4"}, {Bytes: 1234, ContentType: "text/html"}} {
		if info.Bytes > 0 {
			h.objects.info[r.ResultKey()] = info
		}
		if _, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id); err == nil {
			t.Fatal("accepted absent or mismatched object")
		}
		current, _ := h.projects.GetProject(t.Context(), "alice", p.ID)
		if current.Result.Key != p.Result.Key {
			t.Fatal("previous result replaced")
		}
	}
	h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
	stored, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id)
	if err != nil || stored.Result.Kind != clip.RenderBrowser || stored.Result.Key != r.ResultKey() || stored.RenderedPlanRevision != p.EditPlanRevision || stored.Result.Bytes != 1234 {
		t.Fatal(stored, err)
	}
	r, err = h.store.GetBrowserRender(t.Context(), "alice", id)
	if err != nil || r.StoredAt == nil || r.Verdict == nil || !r.Verdict.Passed {
		t.Fatal(r, err)
	}
	retry, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id)
	if err != nil || retry.Result.ID != stored.Result.ID {
		t.Fatal(retry, err)
	}
	deletions, err := h.store.DeletionKeys(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(deletions, ","), p.Result.Key) {
		t.Fatal("old file not queued for cleanup", deletions)
	}
	for _, key := range deletions {
		if key == r.ResultKey() {
			t.Fatal("retry deleted new result")
		}
	}
	if h.media.probes != 0 || h.renderer.calls != 0 || len(h.admitter.calls) != 0 {
		t.Fatal("paid or decode work during browser promotion")
	}
}

func TestBrowserUploadFailureVerdictStalenessAndAtomicPromotion(t *testing.T) {
	for _, failure := range []string{"verdict", "revision", "transaction", "expired"} {
		t.Run(failure, func(t *testing.T) {
			h, p, draft := completedClip(t)
			id, r := browserUpload(t, h, p)
			h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
			m := browserMeasurements()
			if failure == "verdict" {
				m.Width = 720
			}
			v, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, m, true)
			if err != nil {
				t.Fatal(err)
			}
			if failure == "verdict" && (v.Passed || len(v.Notices) == 0) {
				t.Fatal(v)
			}
			if failure == "revision" {
				if _, err := h.service.SaveCorrection(t.Context(), "alice", p.ID, p.EditPlanRevision, draft); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "transaction" {
				if _, err := h.db.Writer.Exec("CREATE TRIGGER fail_browser_completion BEFORE UPDATE OF stored_at ON clip_browser_renders BEGIN SELECT RAISE(ABORT,'completion failed'); END;"); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "expired" {
				h.objects.results = append(h.objects.results, clip.StoredObject{Key: r.ResultKey(), Modified: time.Now().Add(-2 * time.Hour)})
				if err := h.service.Sweep(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id); err == nil {
				t.Fatal("failed attempt promoted")
			}
			current, _ := h.projects.GetProject(t.Context(), "alice", p.ID)
			if current.Result.ID != p.Result.ID || current.Result.Key != p.Result.Key {
				t.Fatal("failed attempt changed result")
			}
			r, _ = h.store.GetBrowserRender(t.Context(), "alice", id)
			if r.StoredAt != nil {
				t.Fatal("failed attempt marked stored")
			}
			if failure == "expired" {
				if r.CancelledAt == nil {
					t.Fatal("orphan deletion did not fence completion")
				}
				// A delayed PUT cannot revive an attempt the sweeper claimed.
				h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
				if _, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id); !errors.Is(err, clip.ErrSourceState) {
					t.Fatal(err)
				}
			}
		})
	}
}

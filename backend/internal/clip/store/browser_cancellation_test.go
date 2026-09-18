package store_test

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"testing"
)

func TestBrowserCancellationFencesLateUploadAndPromotion(t *testing.T) {
	for _, stage := range []string{"reserved", "uploaded", "reported"} {
		t.Run(stage, func(t *testing.T) {
			h, p, _ := completedClip(t)
			id, r := browserUpload(t, h, p)
			if stage != "reserved" {
				h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
			}
			if stage == "reported" {
				if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, browserMeasurements(), true); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := h.service.CancelBrowserRender(t.Context(), "bob", id); !errors.Is(err, clip.ErrNotFound) {
				t.Fatal(err)
			}
			for range 2 {
				accepted, err := h.service.CancelBrowserRender(t.Context(), "alice", id)
				if err != nil || !accepted {
					t.Fatal(accepted, err)
				}
			}
			if _, err := h.service.PrepareBrowserUpload(t.Context(), "alice", id, 1234); !errors.Is(err, clip.ErrSourceState) {
				t.Fatal(err)
			}
			if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, browserMeasurements(), true); !errors.Is(err, clip.ErrSourceState) {
				t.Fatal(err)
			}
			if _, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id); !errors.Is(err, clip.ErrSourceState) {
				t.Fatal(err)
			}
			current, err := h.projects.GetProject(t.Context(), "alice", p.ID)
			if err != nil || current.Result.ID != p.Result.ID || current.EditPlan != p.EditPlan {
				t.Fatal(current, err)
			}
			r, err = h.store.GetBrowserRender(t.Context(), "alice", id)
			if err != nil || r.CancelledAt == nil || r.StoredAt != nil {
				t.Fatal(r, err)
			}
		})
	}
}

func TestBrowserCompletionWinsBeforeLateCancellation(t *testing.T) {
	h, p, _ := completedClip(t)
	id, r := browserUpload(t, h, p)
	h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
	if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, browserMeasurements(), true); err != nil {
		t.Fatal(err)
	}
	stored, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := h.service.CancelBrowserRender(t.Context(), "alice", id)
	if err != nil || accepted {
		t.Fatal(accepted, err)
	}
	current, err := h.projects.GetProject(t.Context(), "alice", p.ID)
	if err != nil || current.Result.ID != stored.Result.ID {
		t.Fatal(current, err)
	}
}

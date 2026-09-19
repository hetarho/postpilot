package store_test

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestBrowserOrphanSweepKeepsCurrentFileAndReapsRecreatedSupersededFile(t *testing.T) {
	h, p, _ := completedClip(t)
	complete := func() (clip.Project, clip.BrowserRender) {
		id, r := browserUpload(t, h, p)
		h.objects.info[r.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
		if _, err := h.service.ReportRenderVerdict(t.Context(), "alice", id, browserMeasurements(), true); err != nil {
			t.Fatal(err)
		}
		project, err := h.service.CompleteBrowserUpload(t.Context(), "alice", id)
		if err != nil {
			t.Fatal(err)
		}
		return project, r
	}
	p, first := complete()
	h.objects.results = append(h.objects.results, clip.StoredObject{Key: first.ResultKey(), Modified: time.Now().Add(-2 * time.Hour)})
	if err := h.service.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.info[first.ResultKey()]; !ok {
		t.Fatal("sweeper deleted current browser result")
	}
	p, second := complete()
	if err := h.service.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	// A previously signed PUT finishes after the former result was deleted.
	h.objects.info[first.ResultKey()] = clip.SourceObjectInfo{Bytes: 1234, ContentType: "video/mp4"}
	if err := h.service.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.info[first.ResultKey()]; ok {
		t.Fatal("completed marker kept a superseded orphan")
	}
	if _, ok := h.objects.info[second.ResultKey()]; !ok {
		t.Fatal("sweeper deleted latest result")
	}
}

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func TestPreparationFaultsNeverReserveOrCallAI(t *testing.T) {
	for _, mode := range []string{"last oversize", "last corrupt", "last missing", "missing earlier proxy", "disk", "price drift", "delivery drift", "legacy pricing", "old payload", "ceiling"} {
		t.Run(mode, func(t *testing.T) {
			h := generationSetup(t)
			id := h.start(t)
			switch mode {
			case "last oversize", "last corrupt", "last missing":
				h.media.chunkHook = func(c *clip.AnalysisChunk) error {
					if c.SourceID != h.batch.Sources[1].ID {
						return nil
					}
					switch mode {
					case "last oversize":
						c.Bytes = 8<<20 + 1
					case "last corrupt":
						return clip.ErrInvalidMedia
					case "last missing":
						return os.Remove(c.Path)
					}
					return nil
				}
			case "missing earlier proxy":
				h.media.chunkHook = func(c *clip.AnalysisChunk) error {
					if c.SourceID == h.batch.Sources[1].ID {
						return os.Remove(h.media.prepared[0].Path)
					}
					return nil
				}
			case "disk":
				h.media.capacityErr = clip.ErrWorkspaceLimit
			case "price drift":
				h.service.WithCredits(&quotePricing{inputRate: "20"}, nil)
			case "delivery drift":
				h.planner.gate = clip.ErrModelInputUnsupported
			case "legacy pricing", "old payload", "ceiling":
				j, err := h.jobs.GetByID(context.Background(), id)
				if err != nil {
					t.Fatal(err)
				}
				var p map[string]json.RawMessage
				if err = json.Unmarshal(j.Payload, &p); err != nil {
					t.Fatal(err)
				}
				if mode == "old payload" {
					p["Version"] = json.RawMessage("1")
				} else {
					var a clip.GenerationApproval
					if err = json.Unmarshal(p["Approval"], &a); err != nil {
						t.Fatal(err)
					}
					if mode == "legacy pricing" {
						a.Pricing.Observe.Pricing = llm.CallPricing{}
					} else {
						a.MaxCredits--
					}
					p["Approval"], err = json.Marshal(a)
					if err != nil {
						t.Fatal(err)
					}
				}
				data, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = h.db.Writer.Exec("UPDATE generation_jobs SET payload=? WHERE id=?", data, id); err != nil {
					t.Fatal(err)
				}
			}
			err := h.run(t)
			if err == nil {
				t.Fatal("invalid preparation accepted")
			}
			if mode == "last oversize" && !errors.Is(err, clip.ErrAnalysisTooLarge) {
				t.Fatal(err)
			}
			if mode == "disk" && !errors.Is(err, clip.ErrWorkspaceLimit) {
				t.Fatal(err)
			}
			if len(h.admitter.calls) != 0 || h.planner.observe != 0 || h.planner.plans != 0 || h.renderer.calls != 0 {
				t.Fatal("preparation failure crossed admission")
			}
			h.assertClean(t)
		})
	}
}

func TestPreparationReleasesEachProxyAndNeverUploadsOrSignsIt(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	for _, c := range h.media.prepared {
		if _, err := os.Stat(c.Path); !os.IsNotExist(err) {
			t.Fatal("proxy remains")
		}
	}
	if len(h.objects.results) != 1 {
		t.Fatal("missing original render")
	}
	if len(h.objects.uploads) != 1 || h.objects.uploads[0] != h.objects.results[0].Key || len(h.objects.signs) != 0 {
		t.Fatal("proxy object uploaded or signed")
	}
	if len(h.admitter.calls) != 1 || h.admitter.calls[0].Calls[0].Count != 3 {
		t.Fatal("wrong exact reservation")
	}
	h.assertClean(t)
}

func TestPreparationPanicDoesNotRestartOnRecovery(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	h.media.panicChunks = true
	func() { defer func() { _ = recover() }(); _ = h.run(t) }()
	if _, err := h.queue.SweepRunning(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.service.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if j, _ := h.jobs.PickNextQueued(context.Background(), time.Now()); j.ID != "" {
		t.Fatal("interrupted work replayed")
	}
	h.assertClean(t)
}

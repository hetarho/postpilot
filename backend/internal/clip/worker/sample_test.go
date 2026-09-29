package worker_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/worker"
)

// groundSampler stands in for the renderer's sampler: it reads the original the
// loader hands it and reports what it was told to.
type groundSampler struct {
	clip.Renderer
	grounds []clip.SampledGround
	loaded  []string
}

func (s *groundSampler) SampleGrounds(ctx context.Context, _ clip.MediaWorkspace, plan clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) ([]clip.SampledGround, error) {
	if plan.Portable == nil || len(sources) != 1 {
		return nil, clip.ErrInvalid
	}
	err := load(ctx, sources[0].ID, func(m clip.MediaSource) error {
		data, err := os.ReadFile(m.Path)
		if err != nil {
			return err
		}
		s.loaded = append(s.loaded, string(data))
		return nil
	})
	return s.grounds, err
}

func sampleWork(t *testing.T, data []byte) clip.MediaWork {
	t.Helper()
	info := clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 15000}
	m := clip.SourceMetadata{Filename: "take.mp4", ContentType: "video/mp4", Bytes: int64(len(data)), DurationMS: info.DurationMS, Width: info.Width, Height: info.Height}
	header := []byte{1}
	header = binary.BigEndian.AppendUint64(header, uint64(m.Bytes))
	header = binary.BigEndian.AppendUint32(header, uint32(len(m.ContentType)))
	header = append(header, m.ContentType...)
	header = binary.BigEndian.AppendUint64(header, uint64(m.DurationMS))
	header = append(append(header, data...), data...)
	sum := sha256.Sum256(header)
	m.Fingerprint = hex.EncodeToString(sum[:])
	composed := clip.NoTemplateComposition()
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000,
		Cuts:     []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: m.Fingerprint, EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}, PlaybackRatePermille: clip.RateUnitPermille}},
		Portable: &clip.PortablePlan{Snapshot: composed.Snapshot, Inputs: composed.Inputs, Cuts: []composition.Cut{{ID: "cut", SourceID: "source", EndMS: 15000, PlaybackRatePermille: clip.RateUnitPermille}}}}
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	task := clip.MediaTask{Version: clip.MediaContractVersion, Plan: raw, Render: clip.FreezeMediaRenderInputs(plan), Sources: []clip.MediaTaskSource{{ID: "source", SourceMetadata: m, Info: info}}}
	payload, err := mediacodec.EncodeTask(task)
	if err != nil {
		t.Fatal(err)
	}
	return clip.MediaWork{Operation: clip.MediaSample, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Payload: payload, InputDigest: clip.MediaPayloadDigest(payload), Credentials: clip.MediaLeaseCredentials{AttemptID: "attempt"}}
}

// CLIP-192: a sample stage runs the render's frozen task over the verified
// originals and reports the sampler's grounds, writing no file; a renderer that
// cannot sample and a measurement outside its bounds are both refused.
func TestASampleStageReportsTheGroundsAndWritesNoFile(t *testing.T) {
	data := []byte("original")
	grounds := []clip.SampledGround{{InstanceID: "project-outro", Mean: .8, Sigma: .01, R: .9, G: .9, B: .9, Frames: []float64{.8, .8, .8}}}
	for _, mode := range []string{"ok", "no sampler", "out of bounds"} {
		t.Run(mode, func(t *testing.T) {
			media := &taskMedia{dir: t.TempDir(), info: clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 15000}}
			transfer := &taskTransfer{data: bytes.Clone(data)}
			sampler := &groundSampler{grounds: grounds}
			var renderer clip.Renderer = sampler
			if mode == "no sampler" {
				renderer = nil
			}
			if mode == "out of bounds" {
				bad := grounds[0]
				bad.Mean = 1.5
				sampler.grounds = []clip.SampledGround{bad}
			}
			raw, err := worker.NewExecutor(media, renderer, transfer, clip.DefaultMediaConfig(clip.Environment{})).Execute(t.Context(), sampleWork(t, data))
			switch mode {
			case "ok":
				if err != nil {
					t.Fatal(err)
				}
				result, err := mediacodec.DecodeResult(raw)
				if err != nil || len(result.Outputs) != 0 || result.Plan != "" || !reflect.DeepEqual(result.Grounds, grounds) || len(result.Sources) != 1 {
					t.Fatalf("the sample stage reported %+v %v", result, err)
				}
				if len(transfer.uploaded) != 0 || len(sampler.loaded) != 1 || sampler.loaded[0] != string(data) {
					t.Fatalf("uploaded %v, sampled from %v", transfer.uploaded, sampler.loaded)
				}
			case "no sampler":
				if !errors.Is(err, clip.ErrMediaIncompatible) || raw != "" {
					t.Fatal("a renderer that cannot sample produced a receipt", err)
				}
			default:
				if err == nil || raw != "" {
					t.Fatal("a ground outside 0..1 produced a receipt")
				}
			}
			if dirs, _ := os.ReadDir(media.dir); len(dirs) > 0 {
				t.Fatal("workspace leaked on", mode)
			}
		})
	}
}

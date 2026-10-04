package worker_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/worker"
)

// groundSampler stands in for the renderer's sampler: it reads the originals
// the loader hands it — the ones named in reads, or else the first — and
// reports what it was told to.
type groundSampler struct {
	clip.Renderer
	grounds []clip.SampledGround
	reads   []string
	loaded  []string
}

func (s *groundSampler) SampleGrounds(ctx context.Context, _ clip.MediaWorkspace, plan clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) ([]clip.SampledGround, error) {
	if plan.Portable == nil || len(sources) == 0 {
		return nil, clip.ErrInvalid
	}
	reads := s.reads
	if len(reads) == 0 {
		reads = []string{sources[0].ID}
	}
	for _, id := range reads {
		err := load(ctx, id, func(m clip.MediaSource) error {
			data, err := os.ReadFile(m.Path)
			if err != nil {
				return err
			}
			s.loaded = append(s.loaded, string(data))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return s.grounds, nil
}

// taskOriginal is one synthetic original: its bytes' v1 fingerprint over the
// metadata, with the facts its preparation verified.
func taskOriginal(id string, data []byte, info clip.MediaInfo) clip.MediaTaskSource {
	m := clip.SourceMetadata{Filename: id + ".mp4", ContentType: "video/mp4", Bytes: int64(len(data)), DurationMS: info.DurationMS, Width: info.Width, Height: info.Height}
	header := []byte{1}
	header = binary.BigEndian.AppendUint64(header, uint64(m.Bytes))
	header = binary.BigEndian.AppendUint32(header, uint32(len(m.ContentType)))
	header = append(header, m.ContentType...)
	header = binary.BigEndian.AppendUint64(header, uint64(m.DurationMS))
	header = append(append(header, data...), data...)
	sum := sha256.Sum256(header)
	m.Fingerprint = hex.EncodeToString(sum[:])
	return clip.MediaTaskSource{ID: id, SourceMetadata: m, Info: info}
}

// planWork is a render or sample stage over a 15-second portable plan whose
// cut i draws sources[draws[i]].
func planWork(t *testing.T, op clip.MediaOperation, sources []clip.MediaTaskSource, draws []int) clip.MediaWork {
	t.Helper()
	composed := clip.NoTemplateComposition()
	plan := clip.EditPlan{Ratio: "vertical", Portable: &clip.PortablePlan{Snapshot: composed.Snapshot, Inputs: composed.Inputs}}
	length := 15000 / len(draws)
	for i, draw := range draws {
		s := sources[draw]
		id := fmt.Sprintf("cut-%02d", i)
		plan.Cuts = append(plan.Cuts, clip.Cut{ID: id, SourceID: s.ID, Fingerprint: s.Fingerprint, EndMS: length, Focal: clip.Point{X: .5, Y: .5}, PlaybackRatePermille: clip.RateUnitPermille})
		plan.Portable.Cuts = append(plan.Portable.Cuts, composition.Cut{ID: id, SourceID: s.ID, EndMS: length, PlaybackRatePermille: clip.RateUnitPermille})
		plan.DurationMS += length
	}
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	task := clip.MediaTask{Version: clip.MediaContractVersion, Plan: raw, Render: clip.FreezeMediaRenderInputs(plan), Sources: sources}
	payload, err := mediacodec.EncodeTask(task)
	if err != nil {
		t.Fatal(err)
	}
	return clip.MediaWork{Operation: op, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Payload: payload, InputDigest: clip.MediaPayloadDigest(payload), Credentials: clip.MediaLeaseCredentials{AttemptID: "attempt"}}
}

func sampleWork(t *testing.T, data []byte) clip.MediaWork {
	t.Helper()
	return planWork(t, clip.MediaSample, []clip.MediaTaskSource{taskOriginal("source", data, clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 15000})}, []int{0})
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

package clip

import (
	"context"
	"math"
	"time"

	"github.com/postpilot/backend/internal/clip/design"
)

// A browser render has an identity but is not a queued server job. Its verdict
// is retained independently of the latest stored result; reporting measurements
// alone must never replace the file the owner can already download (CLIP-158).
type BrowserRender struct {
	ID, UserID, ProjectID string
	Revision              int
	Ratio                 string
	DurationMS            int
	Audio                 bool
	CreatedAt             time.Time
	Verdict               *RenderVerdict
}

type RenderMeasurements struct {
	Width, Height, FrameRateNumerator, FrameRateDenominator, VideoFrames int
	VideoCodec, VideoProfile                                             string
	HasAudio, Silent                                                     bool
	AudioCodec                                                           string
	AudioRate                                                            int
	LoudnessLUFS                                                         *float64
}

type RenderVerdict struct {
	Measurements   RenderMeasurements
	ReportedPassed bool
	Passed         bool
	Notices        []PlanNotice
}

type BrowserRenderStore interface {
	BeginBrowserRender(context.Context, BrowserRender) error
	GetBrowserRender(context.Context, string, string) (BrowserRender, error)
	SaveBrowserRenderVerdict(context.Context, string, string, RenderVerdict, time.Time) error
}

func (s *GenerationService) beginBrowserRender(ctx context.Context, p Project, plan EditPlan, sources []RenderSource) (string, error) {
	store, ok := s.store.(BrowserRenderStore)
	if !ok {
		return "", ErrRenderUnavailable
	}
	r := BrowserRender{ID: newID(), UserID: p.UserID, ProjectID: p.ID, Revision: p.EditPlanRevision, Ratio: plan.Ratio, DurationMS: plan.DurationMS, CreatedAt: s.now()}
	for _, cut := range plan.Cuts {
		for _, source := range sources {
			if source.ID == cut.SourceID && source.Info.HasAudio && plan.RetainsOriginalAudio(cut) {
				r.Audio = true
			}
		}
	}
	if err := store.BeginBrowserRender(ctx, r); err != nil {
		return "", err
	}
	return r.ID, nil
}

// ReportRenderVerdict consumes bounded measurements, never an object key or
// media bytes. The upload/promotion path consumes this retained verdict later.
func (s *GenerationService) ReportRenderVerdict(ctx context.Context, user, id string, measurements RenderMeasurements, passed bool) (RenderVerdict, error) {
	store, ok := s.store.(BrowserRenderStore)
	if !ok {
		return RenderVerdict{}, ErrRenderUnavailable
	}
	r, err := store.GetBrowserRender(ctx, user, id)
	if err != nil {
		return RenderVerdict{}, err
	}
	v, err := CheckRenderMeasurements(s.cfg.Render, r, measurements)
	if err != nil {
		return RenderVerdict{}, err
	}
	v.ReportedPassed = passed
	if !passed && v.Passed {
		v.Passed = false
		v.Notices = []PlanNotice{{CopyFallback: CopyFallback{Reason: "render_output_verdict"}, Action: "shortfall"}}
	}
	if err := store.SaveBrowserRenderVerdict(ctx, user, id, v, s.now()); err != nil {
		return RenderVerdict{}, err
	}
	return v, nil
}

func CheckRenderMeasurements(cfg RenderConfig, r BrowserRender, m RenderMeasurements) (RenderVerdict, error) {
	if m.Width <= 0 || m.Height <= 0 || m.FrameRateNumerator <= 0 || m.FrameRateDenominator <= 0 || m.VideoFrames <= 0 ||
		m.Width > 16384 || m.Height > 16384 || m.FrameRateNumerator > 1000000 || m.FrameRateDenominator > 1000000 || m.VideoFrames > 1000000 ||
		len(m.VideoCodec) > 64 || len(m.VideoProfile) > 64 || len(m.AudioCodec) > 64 || m.AudioRate < 0 || m.AudioRate > 1000000 ||
		m.LoudnessLUFS != nil && (math.IsNaN(*m.LoudnessLUFS) || math.IsInf(*m.LoudnessLUFS, 0)) {
		return RenderVerdict{}, ErrInvalid
	}
	canvas, err := ClipCanvas(r.Ratio)
	if err != nil {
		return RenderVerdict{}, err
	}
	v := RenderVerdict{Measurements: m, Passed: true}
	notice := func(check string) {
		v.Passed = false
		v.Notices = append(v.Notices, PlanNotice{CopyFallback: CopyFallback{Reason: check}, Action: "shortfall"})
	}
	if m.Width != canvas.Width || m.Height != canvas.Height {
		notice("render_output_canvas")
	}
	if m.FrameRateNumerator != cfg.FPS*m.FrameRateDenominator {
		notice("render_output_frame_rate")
	}
	if m.VideoCodec != "h264" || m.VideoProfile != "High" || m.HasAudio && m.AudioCodec != "aac" {
		notice("render_output_codec")
	}
	if m.HasAudio != r.Audio {
		notice("render_output_audio")
	}
	if m.HasAudio && m.AudioRate != cfg.AudioRate {
		notice("render_output_audio_rate")
	}
	if m.HasAudio && !m.Silent && (m.LoudnessLUFS == nil || math.Abs(*m.LoudnessLUFS-design.Audio.Loudnorm.I) > 1) {
		notice("render_output_loudness")
	}
	duration := float64(m.VideoFrames) * 1000 * float64(m.FrameRateDenominator) / float64(m.FrameRateNumerator)
	if math.Abs(duration-float64(r.DurationMS)) > 1000/float64(cfg.FPS) {
		notice("render_output_duration")
	}
	return v, nil
}

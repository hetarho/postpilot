package clip

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

const (
	AnalysisVideoCodec                        = "h264"
	AnalysisPixelFormat                       = "yuv420p"
	AnalysisAudioCodec                        = "aac"
	AnalysisAudioChannels                     = 1
	BrowserAnalysisProfileVersion             = "clip-browser-analysis-v1"
	BrowserOriginalProvenance                 = "browser_client"
	AnalysisCopyProvenance                    = "server_copy_decode"
	AnalysisVerificationRenderer              = "analysis-verifier-v1"
	AnalysisVerificationAssets                = "analysis-profile-v1"
	AnalysisVerificationProfile               = "cpu-analysis-verify-v1"
	AnalysisPreparationPrefix                 = "clip-analysis/"
	AnalysisPreparationTTL                    = 2 * time.Hour
	AnalysisVerificationTimeout               = 2 * time.Minute
	AnalysisVerificationWorkspaceBytes  int64 = 32 << 20
	AnalysisVerificationLogBytes              = 1 << 20
	AnalysisVerificationPacketMax             = 4096
	AnalysisVerificationAllocationBytes       = 16 << 20
	AnalysisPreparationMaxCopies              = 49
)

var (
	ErrAnalysisPreparationState      = errors.New("analysis preparation is no longer live")
	ErrAnalysisPreparationOverloaded = errors.New("analysis verification capacity is full")
	ErrAnalysisProfileUnqualified    = errors.New("browser analysis profile awaits qualification")
)

// Qualification is code-owned evidence, never an operator bypass or a client
// claim. T603 must supply its independent semantic evidence before activation.
func BrowserAnalysisQualified(profile string) bool { return false }

type BrowserOriginalMeasurement struct {
	SourceID, Fingerprint, Provenance string
	Info                              MediaInfo
}

type AnalysisPreparationInput struct {
	ProjectID, BatchID, QuoteID, ProfileVersion string
	ExpectedRevision                            int
	Originals                                   []BrowserOriginalMeasurement
}

type AnalysisCopy struct {
	Slot, SourceID, Fingerprint                string
	Index, OffsetMS, DurationMS, Width, Height int
	HasAudio                                   bool
	Bytes                                      int64
	Digest, ObjectKey, State                   string
	Info                                       MediaInfo
	PutExpiresAt                               time.Time
}

type AnalysisPreparation struct {
	Sources                                                                               []AnalysisSource
	ID, UserID, ProjectID, BatchID, QuoteID, ProfileVersion                               string
	OriginalDigest, BoundQuoteDigest, QuoteManifestDigest, ManifestDigest, RecoveryDigest string
	ExpectedRevision                                                                      int
	Originals                                                                             []BrowserOriginalMeasurement
	Reused                                                                                []AnalysisChunk
	Copies                                                                                []AnalysisCopy
	State, ParentJobID, CurrentAttemptID, Receipt, Failure                                string
	Attempts, Progress                                                                    int
	CreatedAt, ExpiresAt, QueueDeadlineAt, DeadlineAt                                     time.Time
}

type AnalysisPreparationLimits struct {
	Capacity                 RenderCapacity
	Stages                   MediaStageLimits
	TTL, PutTTL, OrphanGrace time.Duration
}

func (l AnalysisPreparationLimits) Validate() error {
	if l.Capacity.Validate() != nil || l.Stages.Validate() != nil || l.TTL <= 0 || l.TTL > MediaStageTimeoutMax || l.PutTTL <= 0 || l.PutTTL > MediaArtifactAccessTTL || l.OrphanGrace <= 0 {
		return ErrInvalid
	}
	return nil
}

func DefaultAnalysisPreparationLimits(env Environment) AnalysisPreparationLimits {
	capacity := RenderCapacity{Active: 1, Waiting: 2, PerAccount: 1}
	if env.AnalysisVerificationActive != 0 {
		capacity.Active = env.AnalysisVerificationActive
	}
	if env.AnalysisVerificationWaiting != nil {
		capacity.Waiting = *env.AnalysisVerificationWaiting
	}
	if env.AnalysisVerificationPerAccount != 0 {
		capacity.PerAccount = env.AnalysisVerificationPerAccount
	}
	put, grace := env.PutTTL, env.OrphanMinAge
	if put <= 0 {
		put = MediaArtifactAccessTTL
	}
	if grace <= 0 {
		grace = time.Hour
	}
	return AnalysisPreparationLimits{capacity, DefaultMediaStageLimits(env), AnalysisPreparationTTL, min(put, MediaArtifactAccessTTL), grace}
}

func ValidSHA256(value string) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == sha256.Size && value == strings.ToLower(value)
}

func AnalysisBoundQuoteDigest(original, manifest string) string {
	sum := sha256.Sum256([]byte("clip-browser-analysis-quote-v1\x00" + original + "\x00" + manifest))
	return hex.EncodeToString(sum[:])
}

func AnalysisStageID(id string) string { return "analysis:" + id }
func AnalysisPreparationStage(stage string) (string, bool) {
	return strings.CutPrefix(stage, "analysis:")
}
func AnalysisPreparationLive(state string) bool {
	return state == "preparing" || state == "verifying" || state == "accepted" || state == "consumed"
}

// The measurements match an authorized immutable source selection. They remain
// browser claims; this does not decode or certify the original.
func ValidateBrowserOriginals(batch SourceBatch, originals []BrowserOriginalMeasurement, cfg MediaConfig) error {
	if len(originals) != len(batch.Sources) || len(originals) == 0 || len(originals) > cfg.Sources.MaxCount {
		return ErrInvalidMedia
	}
	total := 0
	for i, m := range originals {
		s, p := batch.Sources[i], m.Info
		if m.SourceID != s.ID || m.Fingerprint != s.Fingerprint || !ValidSHA256(m.Fingerprint) || m.Provenance != BrowserOriginalProvenance || p.DurationMS <= 0 || p.DurationMS > cfg.Sources.MaxDurationMS-total || math.Abs(float64(p.DurationMS-s.DurationMS)) > float64(cfg.DurationToleranceMS) || p.Width != s.Width || p.Height != s.Height || min(p.Width, p.Height) < 2 || max(p.Width, p.Height) > cfg.MaxDimension || p.FrameRateNumerator <= 0 || p.FrameRateNumerator > 1000000 || p.FrameRateDenominator <= 0 || p.FrameRateDenominator > 1000000 || p.DecodedFrames <= 0 || p.DecodedFrames > 100000000 || p.HasAudio && (p.AudioRate < 8000 || p.AudioRate > 192000 || p.AudioChannels < 1 || p.AudioChannels > 16) || !p.HasAudio && (p.AudioRate != 0 || p.AudioChannels != 0) {
			return ErrInvalidMedia
		}
		total += p.DurationMS
	}
	return nil
}

func BrowserAnalysisGeometry(info MediaInfo, edge int) (int, int) {
	scale := math.Min(1, float64(edge)/float64(max(info.Width, info.Height)))
	return max(2, int(float64(info.Width)*scale)/2*2), max(2, int(float64(info.Height)*scale)/2*2)
}

func ExpectedAnalysisCopies(originals []BrowserOriginalMeasurement, reused []AnalysisChunk, cfg MediaConfig) ([]AnalysisCopy, error) {
	var out []AnalysisCopy
	for _, m := range originals {
		w, h := BrowserAnalysisGeometry(m.Info, cfg.LongEdge)
		for index, offset := 0, 0; offset < m.Info.DurationMS; index, offset = index+1, offset+cfg.ChunkDurationMS {
			duration := min(cfg.ChunkDurationMS, m.Info.DurationMS-offset)
			if slices.ContainsFunc(reused, func(c AnalysisChunk) bool {
				return c.SourceID == m.SourceID && c.Fingerprint == m.Fingerprint && c.Index == index && c.OffsetMS == offset && c.DurationMS == duration
			}) {
				continue
			}
			out = append(out, AnalysisCopy{Slot: MediaAnalysisSlot(m.SourceID, index), SourceID: m.SourceID, Fingerprint: m.Fingerprint, Index: index, OffsetMS: offset, DurationMS: duration, Width: w, Height: h, HasAudio: m.Info.HasAudio, State: "expected"})
			if len(out) > AnalysisPreparationMaxCopies {
				return nil, ErrInvalidMedia
			}
		}
	}
	return out, nil
}

type AnalysisVerificationTask struct {
	Version                        int
	ProfileVersion, ManifestDigest string
	Copies                         []AnalysisCopy
}
type AnalysisCopyVerification struct {
	Slot, Digest, Provenance   string
	Bytes                      int64
	Info                       MediaInfo
	VideoPackets, AudioPackets int
	AudioSamples               int64
}
type AnalysisVerificationResult struct {
	Version                        int
	ProfileVersion, ManifestDigest string
	Copies                         []AnalysisCopyVerification
}

func (t AnalysisVerificationTask) Validate() error {
	if t.Version != 1 || t.ProfileVersion != BrowserAnalysisProfileVersion || !ValidSHA256(t.ManifestDigest) || len(t.Copies) == 0 || len(t.Copies) > AnalysisPreparationMaxCopies {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, c := range t.Copies {
		if seen[c.Slot] || c.Slot != MediaAnalysisSlot(c.SourceID, c.Index) || !ValidMediaLabel(c.SourceID) || !ValidSHA256(c.Fingerprint) || c.Index < 0 || c.Index >= SourceDurationMS/60000 || c.OffsetMS != c.Index*60000 || c.DurationMS <= 0 || c.DurationMS > 60000 || c.Bytes <= 0 || c.Bytes > 8<<20 || !ValidSHA256(c.Digest) || min(c.Width, c.Height) < 2 || max(c.Width, c.Height) > 720 || c.Width%2 != 0 || c.Height%2 != 0 || c.ObjectKey != "" || c.State != "" || !c.PutExpiresAt.IsZero() {
			return ErrInvalid
		}
		seen[c.Slot] = true
	}
	return nil
}

func ValidateAnalysisCopyVerification(c AnalysisCopy, v AnalysisCopyVerification, cfg MediaConfig) error {
	p := v.Info
	frame := (1000 + cfg.FPS - 1) / cfg.FPS
	if v.Slot != c.Slot || v.Bytes != c.Bytes || v.Digest != c.Digest || v.Provenance != AnalysisCopyProvenance || p.Width != c.Width || p.Height != c.Height || p.Rotation != 0 || p.PixelFormat != "yuv420p" || p.SampleAspectRatio != "1:1" || p.FrameRateDenominator <= 0 || p.FrameRateNumerator != cfg.FPS*p.FrameRateDenominator || p.HasAudio != c.HasAudio || p.ContainerDurationMS <= 0 || p.ContainerDurationMS > cfg.ChunkDurationMS || p.VideoDurationMS <= 0 || p.VideoDurationMS > cfg.ChunkDurationMS || absAnalysis(p.ContainerDurationMS-c.DurationMS) > frame || absAnalysis(p.VideoDurationMS-c.DurationMS) > frame || p.DecodedFrames <= 0 || p.DecodedFrames > cfg.FPS*60 || absAnalysis(p.DecodedFrames*1000/cfg.FPS-c.DurationMS) > frame || p.DecodedFrames > 1 && !p.CadenceVerified || v.VideoPackets < 1 || v.VideoPackets > cfg.FPS*60 || v.AudioPackets < 0 || v.AudioPackets > AnalysisVerificationPacketMax || v.AudioPackets+v.VideoPackets > AnalysisVerificationPacketMax || p.DecodedDurationMS <= 0 || absAnalysis(p.DecodedDurationMS-c.DurationMS) > frame {
		return ErrInvalidMedia
	}
	video, audio := 0, 0
	for _, s := range p.Streams {
		switch s.Kind {
		case "video":
			video++
			if s.Codec != "h264" {
				return ErrInvalidMedia
			}
		case "audio":
			audio++
			if s.Codec != "aac" {
				return ErrInvalidMedia
			}
		default:
			return ErrInvalidMedia
		}
	}
	if video != 1 || audio != boolAnalysis(c.HasAudio) {
		return ErrInvalidMedia
	}
	if c.HasAudio && (p.AudioRate != cfg.AudioRate || p.AudioChannels != 1 || absAnalysis(p.AudioDurationMS-c.DurationMS) > 22 || v.AudioPackets == 0 || v.AudioSamples <= 0 || math.Abs(float64(v.AudioSamples-int64(c.DurationMS)*int64(cfg.AudioRate)/1000)) > 1024) {
		return ErrInvalidMedia
	}
	if !c.HasAudio && (v.AudioPackets != 0 || v.AudioSamples != 0) {
		return ErrInvalidMedia
	}
	return nil
}
func ValidateAnalysisVerification(t AnalysisVerificationTask, r AnalysisVerificationResult, cfg MediaConfig) error {
	if r.Version != 1 || r.ProfileVersion != t.ProfileVersion || r.ManifestDigest != t.ManifestDigest || len(r.Copies) != len(t.Copies) {
		return ErrInvalidMedia
	}
	for i, c := range t.Copies {
		if err := ValidateAnalysisCopyVerification(c, r.Copies[i], cfg); err != nil {
			return fmt.Errorf("%w: copy %d", err, i)
		}
	}
	return nil
}
func absAnalysis(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func boolAnalysis(v bool) int {
	if v {
		return 1
	}
	return 0
}

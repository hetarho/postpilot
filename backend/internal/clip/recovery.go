package clip

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/postpilot/backend/internal/llm"
	"slices"
)

const AnalysisContractVersion = "clip-observation-v1"

type RecoveryState struct {
	Version                           int
	JobID, Contract, PlanDigest, Plan string
	Observe                           llm.ModelRef
	Pricing                           GenerationPricing
	PlanReady                         bool
	Sources                           []AnalysisSource
	Chunks                            []ChunkAnalysis
	Legacy                            *AttemptCheckpoint
}
type RecoveryStore interface {
	GetRecovery(context.Context, string, string) (*RecoveryState, error)
	SaveRecovery(context.Context, string, string, RecoveryState) error
}

func RecoveryDigest(r *RecoveryState) string {
	if r == nil {
		return ""
	}
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func planRecoveryDigest(p generationPayload) string {
	sources := make([][2]string, 0, len(p.Batch.Sources))
	for _, source := range p.Batch.Sources {
		sources = append(sources, [2]string{source.ID, source.Fingerprint})
	}
	raw, _ := json.Marshal(struct {
		Composition                   *ProjectComposition
		Template                      Recipe
		Answers                       []Answer
		Ratio, Write, Disclosure, CTA string
		Target                        int
		Hide                          bool
		Version                       int
		Sources                       [][2]string
	}{p.Composition, p.Template, p.Answers, p.Ratio, p.Write, p.Disclosure, p.CTA, p.TargetDurationMS, p.HideDisclosure, CompositionPlanVersion, sources})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func (s *GenerationService) saveRecovery(ctx context.Context, user, project string, r RecoveryState) error {
	if store, ok := s.store.(RecoveryStore); ok {
		return store.SaveRecovery(ctx, user, project, r)
	}
	return nil
}
func (s *GenerationService) loadRecovery(ctx context.Context, user, project string) (*RecoveryState, error) {
	store, ok := s.store.(RecoveryStore)
	if !ok {
		return nil, nil
	}
	return store.GetRecovery(ctx, user, project)
}

// Convert only completed legacy source evidence whose originating payload proves
// the same model and manifest. Incomplete legacy sources are never guessed.
func (s *GenerationService) upgradeRecovery(ctx context.Context, user, project string, r *RecoveryState) (*RecoveryState, error) {
	if r == nil || r.Legacy == nil {
		return r, nil
	}
	reader, ok := s.jobs.(interface {
		Snapshot(context.Context, string, string, string) (*ClipJob, error)
	})
	if !ok {
		return nil, nil
	}
	job, err := reader.Snapshot(ctx, user, project, r.JobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	var p generationPayload
	if json.Unmarshal(job.Payload, &p) != nil || p.ProjectID != project || p.Batch.UserID != user || r.Legacy.EvidenceLimited {
		return nil, nil
	}
	out := &RecoveryState{Version: 1, JobID: r.JobID, Contract: AnalysisContractVersion, Observe: modelRef(p.Observe)}
	if p.Approval != nil {
		out.Pricing = p.Approval.Pricing
	}
	remaining := r.Legacy.CompletedChunks
	for _, a := range r.Legacy.Observations {
		count := (a.Source.Info.DurationMS + 59999) / 60000
		if count < 1 || remaining < count {
			break
		}
		remaining -= count
		lease, ok := sourceLease(p.Batch, a.Source.ID)
		if !ok || !recoverySourceMatches(a.Source, lease) {
			return nil, nil
		}
		out.Sources = append(out.Sources, a.Source)
		for index := 0; index < count; index++ {
			c := ChunkAnalysis{SourceID: a.Source.ID, Fingerprint: a.Source.Fingerprint, Index: index, OffsetMS: index * 60000, DurationMS: min(60000, a.Source.Info.DurationMS-index*60000)}
			for _, seg := range a.Segments {
				if seg.StartMS >= c.OffsetMS && seg.EndMS <= c.OffsetMS+c.DurationMS {
					c.Segments = append(c.Segments, seg)
				}
			}
			if ValidateSegments(s.cfg.Analysis, c.Segments, c.OffsetMS, c.OffsetMS+c.DurationMS) != nil {
				return nil, nil
			}
			out.Chunks = append(out.Chunks, c)
		}
	}
	return out, nil
}
func sourceLease(b SourceBatch, id string) (SourceLease, bool) {
	for _, v := range b.Sources {
		if v.ID == id {
			return v, true
		}
	}
	return SourceLease{}, false
}
func recoverySourceMatches(a AnalysisSource, v SourceLease) bool {
	return a.ID == v.ID && a.Fingerprint == v.Fingerprint && a.Fingerprint != "" && a.Info.Width == v.Width && a.Info.Height == v.Height && a.Info.DurationMS > 0 && absRecovery(a.Info.DurationMS-v.DurationMS) <= 1000
}
func absRecovery(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func (s *GenerationService) selectRecovery(r *RecoveryState, b SourceBatch, observe llm.ModelRef) RecoveryState {
	out := RecoveryState{Version: 1, Contract: AnalysisContractVersion, Observe: observe}
	if r == nil || r.Version != 1 || r.Contract != AnalysisContractVersion || r.Observe != observe {
		return out
	}
	out.JobID, out.Pricing = r.JobID, r.Pricing
	for _, v := range b.Sources {
		for _, a := range r.Sources {
			if !recoverySourceMatches(a, v) || ValidateAnalysisSources(s.cfg.Analysis, []AnalysisSource{a}) != nil {
				continue
			}
			if _, exists := recoverySource(out, a.ID); exists {
				continue
			}
			out.Sources = append(out.Sources, a)
			seen := map[int]bool{}
			for _, c := range r.Chunks {
				if c.SourceID != a.ID || c.Fingerprint != a.Fingerprint || seen[c.Index] || ValidateChunkInput(s.cfg.Analysis, ChunkInput{Source: a, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS}) != nil || ValidateSegments(s.cfg.Analysis, c.Segments, c.OffsetMS, c.OffsetMS+c.DurationMS) != nil {
					continue
				}
				seen[c.Index] = true
				out.Chunks = append(out.Chunks, c)
			}
		}
	}
	slices.SortFunc(out.Chunks, func(a, b ChunkAnalysis) int {
		if a.SourceID == b.SourceID {
			return a.Index - b.Index
		}
		return cmp.Compare(a.SourceID, b.SourceID)
	})
	if len(out.Chunks) == len(r.Chunks) && len(out.Sources) == len(r.Sources) {
		out.PlanDigest, out.Plan, out.PlanReady = r.PlanDigest, r.Plan, r.PlanReady
	}
	return out
}
func recoveryChunk(r RecoveryState, id string, index int) *ChunkAnalysis {
	for _, c := range r.Chunks {
		if c.SourceID == id && c.Index == index {
			return &c
		}
	}
	return nil
}
func recoverySource(r RecoveryState, id string) (AnalysisSource, bool) {
	for _, a := range r.Sources {
		if a.ID == id {
			return a, true
		}
	}
	return AnalysisSource{}, false
}

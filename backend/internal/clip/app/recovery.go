package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func planRecoveryDigest(p clip.GenerationPayload) string {
	sources := make([][2]string, 0, len(p.Batch.Sources))
	for _, source := range p.Batch.Sources {
		sources = append(sources, [2]string{source.ID, source.Fingerprint})
	}
	// The digest is SEMANTIC: it says which candidate plan is still the plan
	// for this input. The observation contract belongs in it because a plan
	// written against v1 scenes was never checked for complete coverage, a
	// contained scene or a scene status; the observations themselves stay
	// reusable under CLIP-93. The owner's instruction is writer input a
	// generation freezes (CLIP-69), so a changed one leaves no plan to reuse.
	// The owner's source-sound setting is deliberately absent — it is
	// render-only and rides the render revision instead.
	raw, _ := json.Marshal(struct {
		Composition                             *clip.ProjectComposition
		Template                                clip.Recipe
		Answers                                 []clip.Answer
		Ratio, Write, Disclosure, CTA, Language string
		Instruction                             string
		Target                                  int
		Hide                                    bool
		Version                                 int
		Analysis                                string
		Sources                                 [][2]string
	}{p.Composition, p.Template, p.Answers, p.Ratio, p.Write, p.Disclosure, p.CTA, p.Language, p.Instruction, p.TargetDurationMS, p.HideDisclosure, clip.CompositionPlanVersion, clip.AnalysisContractVersion, sources})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *GenerationService) saveRecovery(ctx context.Context, user, project string, r clip.RecoveryState) error {
	if store, ok := s.store.(clip.RecoveryStore); ok {
		return store.SaveRecovery(ctx, user, project, r)
	}
	return nil
}

func (s *GenerationService) loadRecovery(ctx context.Context, user, project string) (*clip.RecoveryState, error) {
	store, ok := s.store.(clip.RecoveryStore)
	if !ok {
		return nil, nil
	}
	return store.GetRecovery(ctx, user, project)
}

// Legacy attempt evidence was recorded under clip-observation-v1, which never
// proved complete source coverage and never carried a scene status. It is kept
// READABLE for inspection, but it is never converted, relabelled or reused for
// new generation: a successor re-observes instead (CLIP-87, CLIP-93).
func (s *GenerationService) upgradeRecovery(_ context.Context, _, _ string, r *clip.RecoveryState) (*clip.RecoveryState, error) {
	if r == nil || r.Legacy == nil {
		return r, nil
	}
	return nil, nil
}

func recoverySourceMatches(a clip.AnalysisSource, v clip.SourceLease) bool {
	return a.ID == v.ID && a.Fingerprint == v.Fingerprint && a.Fingerprint != "" && a.Info.Width == v.Width && a.Info.Height == v.Height && a.Info.DurationMS > 0 && absRecovery(a.Info.DurationMS-v.DurationMS) <= 1000
}

func absRecovery(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (s *GenerationService) selectRecovery(r *clip.RecoveryState, b clip.SourceBatch, observe llm.ModelRef, language string) clip.RecoveryState {
	out := clip.RecoveryState{Version: 1, Contract: clip.AnalysisContractVersion, Observe: observe, Language: language}
	if r == nil || r.Version != 1 || r.Contract != clip.AnalysisContractVersion || r.Observe != observe || r.Language != language || !clip.ValidLanguage(language) {
		return out
	}
	out.JobID, out.Pricing = r.JobID, r.Pricing
	for _, v := range b.Sources {
		for _, a := range r.Sources {
			if !recoverySourceMatches(a, v) || clip.ValidateAnalysisSources(s.cfg.Analysis, []clip.AnalysisSource{a}) != nil {
				continue
			}
			if _, exists := recoverySource(out, a.ID); exists {
				continue
			}
			out.Sources = append(out.Sources, a)
			seen := map[int]bool{}
			for _, c := range r.Chunks {
				if c.SourceID != a.ID || c.Fingerprint != a.Fingerprint || seen[c.Index] || clip.ValidateChunkInput(s.cfg.Analysis, clip.ChunkInput{Source: a, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS}) != nil || clip.ValidateSegments(s.cfg.Analysis, c.Segments, c.OffsetMS, c.OffsetMS+c.DurationMS) != nil {
					continue
				}
				seen[c.Index] = true
				out.Chunks = append(out.Chunks, c)
			}
		}
	}
	slices.SortFunc(out.Chunks, func(a, b clip.ChunkAnalysis) int {
		if a.SourceID == b.SourceID {
			return a.Index - b.Index
		}
		return cmp.Compare(a.SourceID, b.SourceID)
	})
	if len(out.Chunks) == len(r.Chunks) && len(out.Sources) == len(r.Sources) {
		out.PlanDigest, out.Plan, out.PlanReady, out.FlowReady = r.PlanDigest, r.Plan, r.PlanReady, r.FlowReady
	}
	return out
}

func recoveryChunk(r clip.RecoveryState, id string, index int) *clip.ChunkAnalysis {
	for _, c := range r.Chunks {
		if c.SourceID == id && c.Index == index {
			return &c
		}
	}
	return nil
}

func recoverySource(r clip.RecoveryState, id string) (clip.AnalysisSource, bool) {
	for _, a := range r.Sources {
		if a.ID == id {
			return a, true
		}
	}
	return clip.AnalysisSource{}, false
}

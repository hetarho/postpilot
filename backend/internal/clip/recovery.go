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

const AnalysisContractVersion = "clip-observation-v2"

// Records written under the first contract stay READABLE — a finished project
// keeps showing them — but none is ever relabelled v2 or reused for new paid
// generation, because v1 never recorded complete coverage or a scene status
// (CLIP-93).
const LegacyAnalysisContractVersion = "clip-observation-v1"

type RecoveryState struct {
	Language                          string
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
	// The digest is SEMANTIC: it says which candidate plan is still the plan
	// for this input. The observation contract belongs in it because a plan
	// written against v1 scenes was never checked for complete coverage, a
	// contained scene or a scene status; the observations themselves stay
	// reusable under CLIP-93. The owner's instruction is writer input a
	// generation freezes (CLIP-69), so a changed one leaves no plan to reuse.
	// The owner's source-sound setting is deliberately absent — it is
	// render-only and rides the render revision instead.
	raw, _ := json.Marshal(struct {
		Composition                             *ProjectComposition
		Template                                Recipe
		Answers                                 []Answer
		Ratio, Write, Disclosure, CTA, Language string
		Instruction                             string
		Target                                  int
		Hide                                    bool
		Version                                 int
		Analysis                                string
		Sources                                 [][2]string
	}{p.Composition, p.Template, p.Answers, p.Ratio, p.Write, p.Disclosure, p.CTA, p.Language, p.Instruction, p.TargetDurationMS, p.HideDisclosure, CompositionPlanVersion, AnalysisContractVersion, sources})
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

// Legacy attempt evidence was recorded under clip-observation-v1, which never
// proved complete source coverage and never carried a scene status. It is kept
// READABLE for inspection, but it is never converted, relabelled or reused for
// new generation: a successor re-observes instead (CLIP-87, CLIP-93).
func (s *GenerationService) upgradeRecovery(_ context.Context, _, _ string, r *RecoveryState) (*RecoveryState, error) {
	if r == nil || r.Legacy == nil {
		return r, nil
	}
	return nil, nil
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
func (s *GenerationService) selectRecovery(r *RecoveryState, b SourceBatch, observe llm.ModelRef, language string) RecoveryState {
	out := RecoveryState{Version: 1, Contract: AnalysisContractVersion, Observe: observe, Language: language}
	if r == nil || r.Version != 1 || r.Contract != AnalysisContractVersion || r.Observe != observe || r.Language != language || !ValidLanguage(language) {
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

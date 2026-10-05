package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/postpilot/backend/internal/llm"
)

const AnalysisContractVersion = "clip-observation-v2"

// Records written under the first contract stay READABLE — a finished project
// keeps showing them — but none is ever relabelled v2 or reused for new paid
// generation, because v1 never recorded complete coverage or a scene status
// (CLIP-93).
const LegacyAnalysisContractVersion = "clip-observation-v1"

type RecoveryState struct {
	Spoken                            *SpokenDraft `json:",omitempty"`
	Language                          string
	Version                           int
	JobID, Contract, PlanDigest, Plan string
	Observe                           llm.ModelRef
	Pricing                           GenerationPricing
	// PlanReady is a complete plan that resumes at rendering; FlowReady is the
	// written footage flow the narration call has still to write over.
	PlanReady, FlowReady bool
	// The storyline the kept flow opened with, so a continuation that resumes on
	// the flow keeps it (CLIP-178).
	Storyline *Storyline `json:",omitempty"`
	// The intro/outro words the kept flow drafted with that storyline (CLIP-187).
	RegionDrafts []RegionDraft `json:",omitempty"`
	Sources      []AnalysisSource
	Chunks       []ChunkAnalysis
	Legacy       *AttemptCheckpoint
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

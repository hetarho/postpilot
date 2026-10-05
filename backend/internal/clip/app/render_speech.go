package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// No object I/O occurs under a writer transaction. Every requested input is proven
// before either executor or the successful-server-export allowance is admitted.
func (s *GenerationService) renderSpeechAssets(ctx context.Context, user, project string, plan clip.EditPlan, revision int) ([]clip.SpeechAsset, error) {
	if err := clip.OutputNarrationReadiness(plan, s.cfg.Render.FPS, s.cfg.Render.AudioRate); err != nil {
		return nil, err
	}
	if plan.Narration == nil || !plan.Narration.Enabled {
		return nil, nil
	}
	assets, ok := s.store.(speechAssetReader)
	if !ok {
		return nil, clip.ErrCompositionUnavailable
	}
	objects, ok := s.objects.(speechAudioReader)
	if !ok {
		return nil, clip.ErrCompositionUnavailable
	}
	seen := map[string]bool{}
	out := []clip.SpeechAsset{}
	for _, segment := range plan.Narration.Segments {
		ref := segment.Speech
		a, err := assets.GetSpeechAsset(ctx, user, project, ref.AssetID)
		if err != nil {
			return nil, err
		}
		if a.OwnerID != user || a.ProjectID != project || a.ID != ref.AssetID || a.Text != segment.Text || !reflect.DeepEqual(a.Speech, *ref) || a.Bytes <= 0 || a.Bytes > clip.SpeechMaxAssetBytes || !strings.HasPrefix(a.ObjectKey, clip.SpeechAudioPrefix) {
			return nil, clip.ErrInvalidMedia
		}
		if seen[a.ID] {
			continue
		}
		bytes, err := objects.ReadClipSpeechAudio(ctx, a.ObjectKey, a.Bytes)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(bytes)
		if int64(len(bytes)) != a.Bytes || hex.EncodeToString(hash[:]) != ref.AudioHash {
			return nil, clip.ErrInvalidMedia
		}
		seen[a.ID] = true
		out = append(out, a)
	}
	// Deletion or supersession during private object reads cannot reserve new work.
	p, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return nil, err
	}
	if p.Finalized != nil {
		return nil, clip.ErrFinalized
	}
	if p.EditPlanRevision != revision {
		return nil, clip.ErrPlanConflict
	}
	return out, nil
}

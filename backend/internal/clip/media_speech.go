package clip

import "reflect"

func ValidateMediaSpeech(task MediaTask, plan EditPlan) error {
	if err := OutputNarrationReadiness(plan, 30, 48000); err != nil {
		return err
	}
	requested := RequestedSpeech(plan)
	if len(task.Speech) > MaxSpokenSegments {
		return ErrInvalidMedia
	}
	assets := map[string]MediaTaskSpeech{}
	for _, asset := range task.Speech {
		if !ValidMediaLabel(asset.AssetID) || !digest(asset.AudioHash) || asset.Bytes <= 0 || asset.Bytes > SpeechMaxAssetBytes {
			return ErrInvalidMedia
		}
		if _, duplicate := assets[asset.AssetID]; duplicate {
			return ErrInvalidMedia
		}
		assets[asset.AssetID] = asset
	}
	used := map[string]bool{}
	for _, placement := range requested {
		asset, exists := assets[placement.Speech.AssetID]
		if !exists || asset.AudioHash != placement.Speech.AudioHash {
			return ErrInvalidMedia
		}
		used[asset.AssetID] = true
	}
	if len(used) != len(assets) {
		return ErrInvalidMedia
	}
	return nil
}
func VerifyMediaSpeech(task MediaTask, result MediaResult) error {
	plan, err := DecodeEditPlan(task.Plan)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(RequestedSpeech(plan), result.Speech) {
		return ErrInvalidMedia
	}
	return nil
}

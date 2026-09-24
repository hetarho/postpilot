package clip

import "time"

// A render may bind a newly uploaded lease to the retained plan's source ID.
// The fingerprint includes the browser's metadata and file identity; the worker
// additionally measures the original before using that retained identity.
func ResolveMediaSource(op MediaOperation, frozen MediaTaskSource, batch SourceBatch, now time.Time) (SourceLease, error) {
	for _, source := range batch.Sources {
		matches := source.ID == frozen.ID
		if op == MediaRender {
			matches = source.Fingerprint == frozen.Fingerprint
		}
		if !matches {
			continue
		}
		if source.SourceMetadata != frozen.SourceMetadata || source.ActualBytes != source.Bytes || source.State != "ready" || source.CleanupPending || !source.ExpiresAt.After(now) {
			return SourceLease{}, ErrSourceState
		}
		return source, nil
	}
	return SourceLease{}, ErrSourceState
}

func ValidateMediaSourceBinding(op MediaOperation, task MediaTask, batch SourceBatch, now time.Time) error {
	if batch.AccessDenied || op == MediaPrepare && len(batch.Sources) != len(task.Sources) {
		return ErrSourceState
	}
	seen := map[string]bool{}
	for i, frozen := range task.Sources {
		if op == MediaPrepare && batch.Sources[i].ID != frozen.ID {
			return ErrSourceState
		}
		source, err := ResolveMediaSource(op, frozen, batch, now)
		if err != nil {
			return err
		}
		if seen[source.ID] {
			return ErrSourceState
		}
		seen[source.ID] = true
	}
	return nil
}

// Missing legacy cadence/audio metadata cannot authorize a rate, but does not
// prevent a 1x render. Evidence that was recorded must agree with the new probe.
func SameMediaOriginal(expected, actual MediaInfo) bool {
	if expected.DurationMS != actual.DurationMS || expected.Width != actual.Width || expected.Height != actual.Height || expected.HasAudio != actual.HasAudio {
		return false
	}
	if expected.CadenceVerified && !actual.CadenceVerified {
		return false
	}
	if expected.FrameRateNumerator > 0 && expected.FrameRateDenominator > 0 && (actual.FrameRateNumerator <= 0 || actual.FrameRateDenominator <= 0 || int64(expected.FrameRateNumerator)*int64(actual.FrameRateDenominator) != int64(actual.FrameRateNumerator)*int64(expected.FrameRateDenominator)) {
		return false
	}
	return (expected.AudioRate == 0 || expected.AudioRate == actual.AudioRate) && (expected.AudioChannels == 0 || expected.AudioChannels == actual.AudioChannels)
}

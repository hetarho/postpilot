package clip

// FreezeSourceAudio is the render-time snapshot: the LEASES are the owner-facing
// authority, so the payload an executor receives states what the owner has
// chosen right now rather than what a plan was saved with. A source the batch
// does not carry is off, because nothing has authorized its audio.
func FreezeSourceAudio(batch SourceBatch, cuts []Cut) *SourceAudioSettings {
	out := &SourceAudioSettings{}
	for _, key := range cutSourceKeys(cuts) {
		retain := false
		for _, lease := range batch.Sources {
			if lease.ID == key.id && lease.Fingerprint == key.fingerprint {
				retain = lease.RetainOriginalAudio
			}
		}
		out.Values = append(out.Values, SourceAudioSetting{key.id, key.fingerprint, retain})
	}
	return out
}

// ApplySourceAudio returns the plan with ONE source's setting changed and the
// snapshot left complete. It changes nothing else: cut ranges, rates, per-cut
// volume, captions, evidence and the frozen composition are untouched, which is
// what makes the change render-only (CLIP-100).
func ApplySourceAudio(plan EditPlan, sourceID, fingerprint string, retain bool) EditPlan {
	settings := ReconcileSourceAudio(plan.SourceAudio, plan.Cuts)
	for i := range settings.Values {
		if settings.Values[i].SourceID == sourceID && settings.Values[i].Fingerprint == fingerprint {
			settings.Values[i].RetainOriginal = retain
		}
	}
	plan.SourceAudio = settings
	return plan
}

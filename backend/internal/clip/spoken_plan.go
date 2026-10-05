package clip

import "encoding/json"

// Version 7 adds an independent spoken script. Version 6 is decoded by its
// frozen reader; the historical caption narration flag still means only text.
type storedSpokenPlan struct {
	storedAssemblyPlan
	Narration            *storedNarration
	SourceVolumePermille *int
}
type storedNarration struct {
	Enabled                   bool
	VoiceID, BindingDigest    string
	VolumePermille            int
	Segments, RetiredSegments []storedSpokenSegment
}
type storedSpokenSegment struct {
	ID, Text, InputHash          string
	TextRevision, StartMS, EndMS int
	Speech                       *storedSpeech
}
type storedSpeech struct {
	AssetID, VoiceID, BindingDigest, InputHash, SettingsHash, AudioHash, ProfileID string
	ProfileRevision, Samples                                                       int64
	SampleRate, Channels                                                           int
	Timing                                                                         []storedSpeechTiming
}
type storedSpeechTiming struct {
	Text           string
	StartMS, EndMS int
}

func storeSegments(values []SpokenSegment) []storedSpokenSegment {
	var out []storedSpokenSegment
	for _, v := range values {
		s := storedSpokenSegment{ID: v.ID, Text: v.Text, InputHash: v.InputHash, TextRevision: v.TextRevision, StartMS: v.StartMS, EndMS: v.EndMS}
		if a := v.Speech; a != nil {
			s.Speech = &storedSpeech{AssetID: a.AssetID, VoiceID: a.VoiceID, BindingDigest: a.BindingDigest, InputHash: a.InputHash, SettingsHash: a.SettingsHash, AudioHash: a.AudioHash, ProfileID: a.ProfileID, ProfileRevision: a.ProfileRevision, Samples: a.Samples, SampleRate: a.SampleRate, Channels: a.Channels}
			for _, t := range a.Timing {
				s.Speech.Timing = append(s.Speech.Timing, storedSpeechTiming{t.Text, t.StartMS, t.EndMS})
			}
		}
		out = append(out, s)
	}
	return out
}
func readSegments(values []storedSpokenSegment) []SpokenSegment {
	var out []SpokenSegment
	for _, v := range values {
		s := SpokenSegment{ID: v.ID, Text: v.Text, InputHash: v.InputHash, TextRevision: v.TextRevision, StartMS: v.StartMS, EndMS: v.EndMS}
		if a := v.Speech; a != nil {
			s.Speech = &SpeechRef{AssetID: a.AssetID, VoiceID: a.VoiceID, BindingDigest: a.BindingDigest, InputHash: a.InputHash, SettingsHash: a.SettingsHash, AudioHash: a.AudioHash, ProfileID: a.ProfileID, ProfileRevision: a.ProfileRevision, Samples: a.Samples, SampleRate: a.SampleRate, Channels: a.Channels}
			for _, t := range a.Timing {
				s.Speech.Timing = append(s.Speech.Timing, SpeechTiming{t.Text, t.StartMS, t.EndMS})
			}
		}
		out = append(out, s)
	}
	return out
}

// These are persistence boundary helpers shared by the plan envelope and the
// clip-owned private asset row. They never serialize a provider handle or URL.
func EncodeSpeechReference(a SpeechRef) (string, error) {
	values := storeSegments([]SpokenSegment{{Speech: &a}})
	b, err := json.Marshal(values[0].Speech)
	return string(b), err
}
func DecodeSpeechReference(raw string) (SpeechRef, error) {
	var s storedSpeech
	if err := StrictJSON(raw, &s); err != nil {
		return SpeechRef{}, err
	}
	return *readSegments([]storedSpokenSegment{{Speech: &s}})[0].Speech, nil
}
func encodeSpokenPlan(p EditPlan) (string, error) {
	if err := ValidateNarration(p); err != nil {
		return "", err
	}
	raw, err := encodeVersionSixPlan(p)
	if err != nil {
		return "", err
	}
	var base storedAssemblyPlan
	if err := StrictJSON(raw, &base); err != nil {
		return "", err
	}
	base.Version = CompositionPlanVersion
	envelope := storedSpokenPlan{storedAssemblyPlan: base, SourceVolumePermille: p.SourceVolumePermille}
	if n := p.Narration; n != nil {
		envelope.Narration = &storedNarration{n.Enabled, n.VoiceID, n.BindingDigest, n.VolumePermille, storeSegments(n.Segments), storeSegments(n.RetiredSegments)}
	}
	b, err := json.Marshal(envelope)
	return string(b), err
}
func decodeSpokenPlan(raw string) (EditPlan, error) {
	var s storedSpokenPlan
	if StrictJSON(raw, &s) != nil || s.Version != CompositionPlanVersion {
		return EditPlan{}, ErrInvalid
	}
	s.Version = assemblyPlanVersion
	b, err := json.Marshal(s.storedAssemblyPlan)
	if err != nil {
		return EditPlan{}, err
	}
	p, err := decodeAssemblyPlan(string(b))
	if err != nil {
		return EditPlan{}, err
	}
	p.SourceVolumePermille = s.SourceVolumePermille
	if n := s.Narration; n != nil {
		p.Narration = &NarrationPlan{n.Enabled, n.VoiceID, n.BindingDigest, n.VolumePermille, readSegments(n.Segments), readSegments(n.RetiredSegments)}
	}
	return p, ValidateNarration(p)
}

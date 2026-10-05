package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func narration(p *v1.ClipNarration) *clip.NarrationPlan {
	if p == nil {
		return nil
	}
	gain := 1000
	if p.VolumePermille != nil {
		gain = int(*p.VolumePermille)
	}
	out := &clip.NarrationPlan{Enabled: p.Enabled, VoiceID: p.ConfirmedVoiceId, BindingDigest: p.BindingDigest, VolumePermille: gain}
	for _, s := range p.Segments {
		out.Segments = append(out.Segments, clip.SpokenSegment{ID: s.Id, Text: s.Text, TextRevision: int(s.TextRevision), InputHash: s.InputHash, StartMS: int(s.StartMs), EndMS: int(s.EndMs), Speech: speech(s.Speech), Creation: s.Creation})
	}
	return out
}
func speech(a *v1.ClipSpeechRef) *clip.SpeechRef {
	if a == nil {
		return nil
	}
	out := &clip.SpeechRef{AssetID: a.AssetId, VoiceID: a.VoiceId, BindingDigest: a.BindingDigest, InputHash: a.InputHash, SettingsHash: a.SettingsHash, AudioHash: a.AudioHash, ProfileID: a.ProfileId, ProfileRevision: a.ProfileRevision, Samples: a.Samples, SampleRate: int(a.SampleRate), Channels: int(a.Channels)}
	for _, t := range a.Timing {
		out.Timing = append(out.Timing, clip.SpeechTiming{Text: t.Text, StartMS: int(t.StartMs), EndMS: int(t.EndMs)})
	}
	return out
}
func narrationProto(p *clip.NarrationPlan) *v1.ClipNarration {
	if p == nil {
		return nil
	}
	gain := int32(p.VolumePermille)
	out := &v1.ClipNarration{Enabled: p.Enabled, ConfirmedVoiceId: p.VoiceID, BindingDigest: p.BindingDigest, VolumePermille: &gain}
	for _, s := range p.Segments {
		out.Segments = append(out.Segments, &v1.ClipSpokenSegment{Id: s.ID, Text: s.Text, TextRevision: int32(s.TextRevision), InputHash: s.InputHash, StartMs: int32(s.StartMS), EndMs: int32(s.EndMS), Speech: speechProto(s.Speech)})
	}
	return out
}
func speechProto(a *clip.SpeechRef) *v1.ClipSpeechRef {
	if a == nil {
		return nil
	}
	out := &v1.ClipSpeechRef{AssetId: a.AssetID, VoiceId: a.VoiceID, BindingDigest: a.BindingDigest, InputHash: a.InputHash, SettingsHash: a.SettingsHash, AudioHash: a.AudioHash, ProfileId: a.ProfileID, ProfileRevision: a.ProfileRevision, Samples: a.Samples, SampleRate: int32(a.SampleRate), Channels: int32(a.Channels)}
	for _, t := range a.Timing {
		out.Timing = append(out.Timing, &v1.ClipSpeechTiming{Text: t.Text, StartMs: int32(t.StartMS), EndMs: int32(t.EndMS)})
	}
	return out
}
func derived(p *v1.ClipDerivedCaption) *clip.DerivedCaption {
	if p == nil {
		return nil
	}
	return &clip.DerivedCaption{SegmentID: p.SegmentId, TextRevision: int(p.TextRevision), TextEdited: p.TextEdited, TimingEdited: p.TimingEdited}
}
func derivedProto(p *clip.DerivedCaption) *v1.ClipDerivedCaption {
	if p == nil {
		return nil
	}
	return &v1.ClipDerivedCaption{SegmentId: p.SegmentID, TextRevision: int32(p.TextRevision), TextEdited: p.TextEdited, TimingEdited: p.TimingEdited}
}

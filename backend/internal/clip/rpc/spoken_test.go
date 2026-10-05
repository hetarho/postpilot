package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"reflect"
	"testing"
)

func TestSpokenWireDefaultsAndProjection(t *testing.T) {
	old := correctionPlan(&v1.ClipEditPlan{Elements: []*v1.ClipEditableText{{Narration: true}}})
	if old.Narration != nil {
		t.Fatal("legacy caption synthesized")
	}
	n := narration(&v1.ClipNarration{Enabled: true, ConfirmedVoiceId: "voice"})
	if n.VolumePermille != 1000 {
		t.Fatal("missing gain not unity")
	}
	zero := int32(0)
	if narration(&v1.ClipNarration{VolumePermille: &zero}).VolumePermille != 0 {
		t.Fatal("explicit mute lost")
	}
	n.Segments = []clip.SpokenSegment{{ID: "spoken-1", Text: "대본", InputHash: "hash", TextRevision: 2, StartMS: 123, EndMS: 2345, Speech: &clip.SpeechRef{AssetID: "private", Samples: 44100, SampleRate: 44100, Channels: 2, Timing: []clip.SpeechTiming{{Text: "대본", EndMS: 1000}}}}}
	if !reflect.DeepEqual(n, narration(narrationProto(n))) {
		t.Fatal("narration projection lost")
	}
	d := &clip.DerivedCaption{SegmentID: "spoken-1", TextRevision: 2, TextEdited: true, TimingEdited: false}
	if !reflect.DeepEqual(d, derived(derivedProto(d))) {
		t.Fatal("caption link lost")
	}
}

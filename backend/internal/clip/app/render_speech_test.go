package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"strings"
	"testing"
)

type renderSpeechStore struct {
	clip.GenerationStore
	clip.Store
	asset clip.SpeechAsset
	p     clip.Project
	reads int
	err   error
}

func (s *renderSpeechStore) GetSpeechAsset(context.Context, string, string, string) (clip.SpeechAsset, error) {
	s.reads++
	return s.asset, s.err
}
func (s *renderSpeechStore) GetProject(context.Context, string, string) (clip.Project, error) {
	return s.p, nil
}

type renderSpeechObjects struct {
	clip.ProcessingObjects
	bytes []byte
	reads int
	after func()
}

func (o *renderSpeechObjects) ReadClipSpeechAudio(context.Context, string, int64) ([]byte, error) {
	o.reads++
	if o.after != nil {
		o.after()
	}
	return o.bytes, nil
}
func TestRenderSpeechAdmissionRefusesBeforeExportReservation(t *testing.T) {
	data := []byte("immutable fixture")
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	binding := strings.Repeat("a", 64)
	ref := clip.SpeechRef{AssetID: "speech-one", VoiceID: "voice", BindingDigest: binding, InputHash: clip.SpokenInputHash("sentence"), SettingsHash: hash, AudioHash: hash, ProfileID: "profile", ProfileRevision: 1, Samples: 44100, SampleRate: 44100, Channels: 2}
	plan := clip.EditPlan{DurationMS: 15000, Cuts: []clip.Cut{{StartMS: 0, EndMS: 15000}}, Narration: &clip.NarrationPlan{Enabled: true, VoiceID: "voice", BindingDigest: binding, VolumePermille: 0, Segments: []clip.SpokenSegment{{ID: "spoken-1", Text: "sentence", TextRevision: 1, InputHash: ref.InputHash, StartMS: 1000, EndMS: 2000, Speech: &ref}}}}
	for _, name := range []string{"valid-muted", "foreign", "missing", "corrupt", "changed-during-read", "stale"} {
		t.Run(name, func(t *testing.T) {
			st := &renderSpeechStore{p: clip.Project{EditPlanRevision: 2}, asset: clip.SpeechAsset{ID: ref.AssetID, OwnerID: "alice", ProjectID: "project", Text: "sentence", Speech: ref, ObjectKey: clip.SpeechAudioPrefix + "one.mp3", Bytes: int64(len(data))}}
			obj := &renderSpeechObjects{bytes: data}
			p := plan
			n := *p.Narration
			p.Narration = &n
			switch name {
			case "foreign":
				st.asset.OwnerID = "bob"
			case "missing":
				st.err = clip.ErrNotFound
			case "corrupt":
				obj.bytes = []byte("broken")
			case "changed-during-read":
				obj.after = func() { st.p.EditPlanRevision++ }
			case "stale":
				p.Narration.BindingDigest = strings.Repeat("b", 64)
			}
			service := &GenerationService{store: st, projects: &Service{store: st}, objects: obj, cfg: clip.GenerationConfig{Render: clip.DefaultRenderConfig(clip.Environment{})}}
			got, err := service.renderSpeechAssets(t.Context(), "alice", "project", p, 2)
			if name == "valid-muted" {
				if err != nil || len(got) != 1 {
					t.Fatal(got, err)
				}
			} else {
				if err == nil {
					t.Fatal("refusal admitted")
				}
				if name == "changed-during-read" && !errors.Is(err, clip.ErrPlanConflict) {
					t.Fatal(err)
				}
			}
			if (name == "foreign" || name == "missing" || name == "stale") && obj.reads != 0 {
				t.Fatal("invalid input reached private storage")
			}
		})
	}
}

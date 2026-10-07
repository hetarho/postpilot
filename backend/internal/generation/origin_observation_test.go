package generation

import (
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

func TestObservationOriginsUseOnlySameActualMediaWithoutOwnerPromotion(t *testing.T) {
	observation := Observation{File: "a.jpg", Scene: "컵🙂 컵🙂", Mood: "따뜻한 분위기", VisibleText: "가격 5000", Objects: []string{"컵"}, Events: []string{"들어온 뒤 앉았다"}, Speech: "hello"}
	observation.OriginCandidates = []ObservationOriginCandidate{
		{Field: "scene", Quote: "컵🙂", Occurrence: originInt(1), Category: post.OriginPhotoInterpretation, SourceRefs: []string{"photo"}},
		{Field: "mood", Quote: "따뜻한 분위기", Category: post.OriginAIAdded, SourceRefs: []string{"photo"}},
		{Field: "visible_text", Quote: "가격 5000", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}},
		{Field: "object", ItemIndex: originInt(0), Quote: "컵", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"video"}},
		{Field: "event", ItemIndex: originInt(0), Quote: "들어온 뒤 앉았다", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"photo"}},
		{Field: "speech", Quote: "hello", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"photo"}},
	}
	before := observation
	got := ValidateObservationOrigins(observation, originSources(), AttachmentPhoto)
	if len(got.Spans) != 2 || got.Spans[0].Start != 3 || got.Spans[0].End != 5 || got.Spans[1].Category != post.OriginAIAdded || !reflect.DeepEqual(before, observation) {
		t.Fatalf("observation source/category fence changed input: %+v", got)
	}
	legacy := observation
	legacy.OriginCandidates = nil
	if len(ValidateObservationOrigins(legacy, originSources()).Spans) != 0 {
		t.Fatal("missing historical sidecar inferred visual origins")
	}
}

func TestVideoObservationOriginsKeepActualSourceTimeEventsAndHumanFields(t *testing.T) {
	observation := Observation{File: "a.mp4", Scene: "scene", Mood: "mood", VisibleText: "text", Objects: []string{"object"}, Events: []string{"event"}, Speech: "speech"}
	for _, field := range []string{"scene", "mood", "visible_text", "object", "event", "speech"} {
		candidate := ObservationOriginCandidate{Field: field, Quote: field, Category: post.OriginPhotoInterpretation, SourceRefs: []string{"video"}}
		if field == "visible_text" {
			candidate.Quote = "text"
		}
		if field == "object" || field == "event" {
			candidate.ItemIndex = originInt(0)
		}
		observation.OriginCandidates = append(observation.OriginCandidates, candidate)
	}
	got := ValidateObservationOrigins(observation, originSources(), AttachmentVideo)
	if len(got.Spans) != 6 || got.Result.ContentHash == "" {
		t.Fatalf("video human field source-time evidence lost: %+v", got)
	}
	if len(ValidateObservationOrigins(observation, originSources()).Spans) != 4 {
		t.Fatal("unknown delivery was guessed as video evidence")
	}
	invalid := observation
	invalid.OriginCandidates = []ObservationOriginCandidate{{Field: "file", Quote: "a.mp4", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"video"}}, {Field: "object", Quote: "object", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"video"}}, {Field: "scene", ItemIndex: originInt(0), Quote: "scene", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"video"}}}
	if len(ValidateObservationOrigins(invalid, originSources(), AttachmentVideo).Spans) != 0 {
		t.Fatal("filename or malformed human locator gained annotation")
	}
}

func TestStoredObservationOriginsMustMatchActualCurrentObservationAndMedia(t *testing.T) {
	observation := Observation{File: "a.jpg", Scene: "visible cup"}
	observation.OriginCandidates = []ObservationOriginCandidate{{Field: "scene", Quote: "visible cup", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"photo"}}}
	captured := ValidateObservationOrigins(observation, originSources(), AttachmentPhoto)
	got := ValidateStoredObservationOrigins(observation, captured, AttachmentPhoto)
	if len(got.Spans) != 1 {
		t.Fatal("known exact capture lost on reuse")
	}
	changed := observation
	changed.Scene = "new scene"
	if len(ValidateStoredObservationOrigins(changed, captured, AttachmentPhoto).Spans) != 0 {
		t.Fatal("newer observation silently retargeted older evidence")
	}
	changed = observation
	changed.File = "another.jpg"
	if len(ValidateStoredObservationOrigins(changed, captured, AttachmentPhoto).Spans) != 0 {
		t.Fatal("capture moved to another source")
	}
	captured.Sources[4].Available = false
	if len(ValidateStoredObservationOrigins(observation, captured, AttachmentPhoto).Spans) != 0 {
		t.Fatal("withdrawn visual source restored from old quote")
	}
	if len(ValidateStoredObservationOrigins(observation, nil).Spans) != 0 {
		t.Fatal("legacy observation fabricated capture")
	}
}

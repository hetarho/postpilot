package generation

import (
	"testing"

	"github.com/postpilot/backend/internal/post"
)

func TestObservationOriginsSurviveEmptyCollectionPayloadRoundTrip(t *testing.T) {
	observation := Observation{File: "photo.jpg", Scene: "컵🙂", Objects: []string{}, Events: []string{}}
	observation.OriginCandidates = []ObservationOriginCandidate{{Field: "scene", Quote: observation.Scene, Category: post.OriginPhotoInterpretation, SourceRefs: []string{"media.0"}}}
	observation.Origins = ValidateObservationOrigins(observation, observationSources([]string{observation.File}, false, map[string]string{observation.File: "photo-original"}), AttachmentPhoto)
	raw, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, Observations: []Observation{observation}})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeGenerationPayload(raw)
	if err != nil || len(decoded.Observations) != 1 {
		t.Fatal(err)
	}
	after := decoded.Observations[0]
	validated := ValidateStoredObservationOrigins(after, after.Origins, AttachmentPhoto)
	if ObservationOriginIdentity(observation) != ObservationOriginIdentity(after) || len(validated.Spans) != 1 || validated.Sources[0].AttachmentID != "photo-original" {
		t.Fatalf("storage omission lost captured observation identity: %+v", validated)
	}
}

func TestWritingOriginCatalogKeepsCapturedAttachmentIncarnationWithoutBackfill(t *testing.T) {
	observation := Observation{File: "same.jpg", Scene: "원래 컵"}
	observation.OriginCandidates = []ObservationOriginCandidate{{Field: "scene", Quote: observation.Scene, Category: post.OriginPhotoInterpretation, SourceRefs: []string{"media.0"}}}
	for _, capturedID := range []string{"original-photo", ""} {
		observation.Origins = ValidateObservationOrigins(observation, observationSources([]string{observation.File}, false, map[string]string{observation.File: capturedID}), AttachmentPhoto)
		input := WritePromptInput{Photos: []string{observation.File}, Observations: []Observation{observation}}
		catalog := WritingOriginSources(input, false, map[string]string{observation.File: "replacement-photo"})
		if len(catalog) != 1 || catalog[0].AttachmentID != capturedID || catalog[0].Available {
			t.Fatalf("old/unknown source retargeted to replacement: %+v", catalog)
		}
	}
	observation.Origins = ValidateObservationOrigins(observation, observationSources([]string{observation.File}, false, map[string]string{observation.File: "current-photo"}), AttachmentPhoto)
	catalog := WritingOriginSources(WritePromptInput{Photos: []string{observation.File}, Observations: []Observation{observation}}, false, map[string]string{observation.File: "current-photo"})
	if len(catalog) != 1 || !catalog[0].Available || catalog[0].AttachmentID != "current-photo" {
		t.Fatalf("actual current source was rejected: %+v", catalog)
	}
}

package modelcatalog

import (
	"errors"
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestModelPurposesCoverGeneratedEnum(t *testing.T) {
	expected := map[postpilotv1.ModelPurpose]Purpose{
		postpilotv1.ModelPurpose_MODEL_PURPOSE_PHOTO_ANALYSIS:   PurposePhotoAnalysis,
		postpilotv1.ModelPurpose_MODEL_PURPOSE_STYLE_ANALYSIS:   PurposeStyleAnalysis,
		postpilotv1.ModelPurpose_MODEL_PURPOSE_WRITING:          PurposeWriting,
		postpilotv1.ModelPurpose_MODEL_PURPOSE_IMAGE_GENERATION: PurposeImageGeneration,
		postpilotv1.ModelPurpose_MODEL_PURPOSE_VIDEO_GENERATION: PurposeVideoGeneration,
	}
	if got, want := len(postpilotv1.ModelPurpose_name), len(expected)+1; got != want {
		t.Fatalf("generated purposes = %d, want %d; update the closed mirror", got, want)
	}

	seen := make(map[Purpose]bool, len(expected))
	for number, name := range postpilotv1.ModelPurpose_name {
		wire := postpilotv1.ModelPurpose(number)
		t.Run(name, func(t *testing.T) {
			if wire == postpilotv1.ModelPurpose_MODEL_PURPOSE_UNSPECIFIED {
				if _, err := ParsePurpose(""); !errors.Is(err, ErrUnknownPurpose) {
					t.Fatalf("unspecified parsed as a valid purpose: %v", err)
				}
				return
			}
			want, ok := expected[wire]
			if !ok {
				t.Fatalf("generated enum %s has no domain mapping", name)
			}
			got, err := ParsePurpose(string(want))
			if err != nil || got != want {
				t.Fatalf("parse = %q, %v; want %q", got, err, want)
			}
			seen[got] = true
		})
	}
	if len(Purposes) != len(expected) {
		t.Fatalf("domain purposes = %d, want %d", len(Purposes), len(expected))
	}
	for _, purpose := range Purposes {
		if !seen[purpose] {
			t.Errorf("domain purpose %q has no generated enum value", purpose)
		}
	}
}

func TestModelPurposesRejectUnknownValues(t *testing.T) {
	unknown := postpilotv1.ModelPurpose(999)
	if _, ok := postpilotv1.ModelPurpose_name[int32(unknown)]; ok {
		t.Fatal("unknown enum unexpectedly has a generated name")
	}
	for _, value := range []string{"", unknown.String(), "translation"} {
		if got, err := ParsePurpose(value); !errors.Is(err, ErrUnknownPurpose) || got != "" {
			t.Errorf("ParsePurpose(%q) = %q, %v", value, got, err)
		}
	}
}

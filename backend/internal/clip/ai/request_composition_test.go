package ai_test

import (
	"context"
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/llm"
	"io"
	"strings"
	"testing"
)

func assertActualClipComposition(t *testing.T, request llm.Request, wantMode string) llm.RequestInspection {
	t.Helper()
	got, err := llm.PreparedRequestInspection(request)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != wantMode || got.Status != llm.InspectionPrepared || got.IssuedAt != nil {
		t.Fatalf("incorrect prepared mode: %+v", got)
	}
	var system, user strings.Builder
	for _, f := range got.Fragments {
		switch f.Role {
		case llm.InspectionRoleSystem:
			system.WriteString(f.Text)
		case llm.InspectionRoleUser:
			user.WriteString(f.Text)
		}
	}
	var actualUser strings.Builder
	for _, m := range request.Messages {
		for _, p := range m.Parts {
			actualUser.WriteString(p.Text)
		}
	}
	if system.String() != request.System || user.String() != actualUser.String() {
		t.Fatalf("inventory rebuilt different actual text; system match=%t user match=%t", system.String() == request.System, user.String() == actualUser.String())
	}
	return got
}
func TestClipActualCompositionDescribesFrozenInputsWithoutReadingMedia(t *testing.T) {
	service, models := newService(t, raw(observation()), true)
	input := chunk()
	opened := 0
	input.Video.Open = func(_ context.Context) (io.ReadCloser, error) {
		opened++
		return io.NopCloser(strings.NewReader("secret video bytes")), nil
	}
	if _, _, err := service.ObserveChunk(t.Context(), testRef(), input); err != nil {
		t.Fatal(err)
	}
	got := assertActualClipComposition(t, models.calls[0], "observe")
	rawInspection, _ := json.Marshal(got)
	if len(models.calls) != 1 || opened != 0 || strings.Contains(string(rawInspection), "secret video bytes") {
		t.Fatal("inspection read media or admitted an extra call")
	}
	last := got.Fragments[len(got.Fragments)-1]
	if last.SourceRefs[0] != input.Source.ID {
		t.Fatal("safe ordered source identifier omitted")
	}
	service, models = newService(t, raw(flow()), true)
	planning := planningInput()
	planning.Instruction = "owner request"
	planning.Guidelines = clip.VideoGuidelines{Defaults: []string{"stock rule"}, Owner: []string{"owner rule\nsecond line"}}
	if _, _, err := service.Flow(t.Context(), testRef(), planning); err != nil {
		t.Fatal(err)
	}
	got = assertActualClipComposition(t, models.calls[0], "flow")
	ownerSystem := false
	ownerInput := false
	for _, f := range got.Fragments {
		if f.Role == llm.InspectionRoleSystem && f.Authorship == llm.FragmentAuthorshipAccount && f.Text == "owner rule\n  second line" {
			ownerSystem = true
		}
		if f.Role == llm.InspectionRoleUser && f.Authorship == llm.FragmentAuthorshipAccount && strings.Contains(f.Text, "owner request") {
			ownerInput = true
		}
	}
	if !ownerSystem || !ownerInput {
		t.Fatal("System/User role was confused with authorship")
	}
	if len(ai.RequestCompositions()) != 20 {
		t.Fatal("clip mode/correction inventory incomplete")
	}
}
func TestClipCorrectionInspectionKeepsEachActualAttemptAndOmitsRawCandidate(t *testing.T) {
	_, base := newService(t, raw(observation()), true)
	models := &correctionModels{fakeModels: base, validAfter: 2, invalid: "malformed secret candidate"}
	service, err := ai.New(models, ai.DefaultConfig(clip.Environment{}))
	if err != nil {
		t.Fatal(err)
	}
	input := chunk()
	input.Policy.ResponseRetries = 3
	if _, _, err := service.ObserveChunk(t.Context(), testRef(), input); err != nil {
		t.Fatal(err)
	}
	if len(base.calls) != 2 {
		t.Fatal("wrong reserved correction count")
	}
	first := assertActualClipComposition(t, base.calls[0], "observe")
	second := assertActualClipComposition(t, base.calls[1], "observe/correction")
	if len(second.Fragments) != len(first.Fragments)+1 {
		t.Fatal("correction metadata overwrote first attempt or grew duplicate context")
	}
	bytes, _ := json.Marshal(second)
	if strings.Contains(string(bytes), "malformed secret candidate") {
		t.Fatal("raw invalid candidate leaked into correction inspection")
	}
}

func TestClipOwnerRuleMayContainInventoryHeadingAndStockTextVerbatim(t *testing.T) {
	service, models := newService(t, raw(flow()), true)
	in := planningInput()
	owned := []string{"사용자 지침:\n기본 지침:\n- stock rule\n사용자 지침:\n- nested owner rule", "두 번째 원문\n영상 지침은 이 영상의 흐름과 자막을 어떻게 만들지 정합니다."}
	in.Guidelines = clip.VideoGuidelines{Defaults: []string{"stock rule", "사용자 지침:"}, Owner: owned}
	if _, _, err := service.Flow(t.Context(), testRef(), in); err != nil {
		t.Fatal(err)
	}
	got := assertActualClipComposition(t, models.calls[0], "flow")
	var owners []string
	for _, f := range got.Fragments {
		if f.MaterialRole == "frozen-owner-video-rule" {
			if f.Authorship != llm.FragmentAuthorshipAccount || f.Role != llm.InspectionRoleSystem {
				t.Fatal("owned rule inherited code authorship")
			}
			owners = append(owners, f.Text)
		}
	}
	if len(owners) != len(owned) {
		t.Fatalf("owned heading confused section boundary: %+v", owners)
	}
	for i, text := range owned {
		if owners[i] != strings.ReplaceAll(text, "\n", "\n  ") {
			t.Fatalf("owned text %d was split, inferred or reordered", i)
		}
	}
}

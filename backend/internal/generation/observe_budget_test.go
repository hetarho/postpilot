package generation

import (
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// MODEL-100: an observer that reasons gets the observation headroom on photo and video calls;
// one that does not keeps the batch budget.
func TestObservationRequestsGiveReasoningObserversHeadroom(t *testing.T) {
	for _, reasons := range []bool{false, true} {
		models := newFakeModels()
		info := models.infos[observeRef]
		info.Reasons = reasons
		models.infos[observeRef] = info
		s := &Service{budget: testBudget, models: models}
		want := testBudget.Observation(reasons)
		if reasons && want <= testBudget.Observation(false) {
			t.Fatalf("fake budget gives a reasoning observer %d, no more than the batch budget", want)
		}
		photo, _ := s.preparePhotoObservationRequest(nil, []Image{{Filename: "a.jpg"}}, observeRef, OriginProtocolVersion)
		if photo.MaxTokens != want {
			t.Errorf("reasons=%v: photo observation MaxTokens = %d, want %d", reasons, photo.MaxTokens, want)
		}
		video, _ := s.prepareVideoObservationRequest(Image{Filename: "a.mp4", Kind: AttachmentVideo}, "https://example.test/a.mp4", observeRef, OriginProtocolVersion)
		if video.MaxTokens != want {
			t.Errorf("reasons=%v: video observation MaxTokens = %d, want %d", reasons, video.MaxTokens, want)
		}
	}
	// An unresolvable observer is not assumed to reason.
	s := &Service{budget: testBudget, models: newFakeModels()}
	unknown := llm.ModelRef{ProviderID: "stub", ModelID: "gone"}
	if got, _ := s.preparePhotoObservationRequest(nil, []Image{{Filename: "a.jpg"}}, unknown, OriginProtocolVersion); got.MaxTokens != testBudget.Observation(false) {
		t.Errorf("unknown observer MaxTokens = %d, want the batch budget", got.MaxTokens)
	}
}

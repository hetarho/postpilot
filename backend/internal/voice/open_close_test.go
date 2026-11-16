package voice

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

// recountStore is the one voice a recount test reads: its directory row, its current analysis
// and no 학습 글 rows; every other port stays nil.
type recountStore struct {
	Storage
	analysis Analysis
}

func (r recountStore) GetVoice(_ context.Context, userID, voiceID string) (Voice, error) {
	return Voice{ID: voiceID, UserID: userID, Made: true}, nil
}

func (r recountStore) CurrentAnalysis(context.Context, string, string) (*Analysis, error) {
	analysis := r.analysis
	return &analysis, nil
}

func (r recountStore) ListSampleBodies(context.Context, string, string) ([]Sample, error) {
	return nil, nil
}

func acceptedPost(id string, minutes int, body string) AcceptedMaterial {
	return AcceptedMaterial{
		Source: AcceptedSource{SampleID: id, ContentRevision: 1},
		Body:   body, Kind: SampleKindPost, CreatedAt: fingerprintAt.Add(time.Duration(minutes) * time.Minute),
	}
}

func storedAnalysis(origin Origin, openings ...string) Analysis {
	return Analysis{
		Origin: origin, SourceVersionsKnown: true, CreatedAt: fingerprintAt,
		Counted: Fingerprint{OpenClose: OpenClose{Openings: openings, Closings: []string{"그럼 이만."}}, Sentences: 40},
	}
}

// VOICE-24: an analysis stored before ⑤ kept only recurring lines is recounted from its accepted
// bodies when read, so the projection no longer carries the once-seen line.
func TestAStoredAnalysisIsReadWithOnlyRecurringOpenings(t *testing.T) {
	stored := storedAnalysis(OriginPersonal, "오늘 연남동에 다녀왔어요.", "안녕하세요! 오늘은 국수예요.")
	stored.AcceptedMaterials = []AcceptedMaterial{
		acceptedPost("a", 0, "안녕하세요~ 요즘 국수가 좋아요.\n맛있었어요.\n그럼 이만."),
		acceptedPost("b", 1, "안녕하세요! 오늘은 국수예요.\n별로였어요.\n그럼 이만."),
		acceptedPost("c", 2, "오늘 연남동에 다녀왔어요.\n그저 그랬어요.\n다음에 또 만나요."),
	}
	svc := NewService(recountStore{analysis: stored}, nil, nil)
	got, err := svc.analyses.CurrentAnalysis(context.Background(), "u", "v")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"안녕하세요! 오늘은 국수예요."}; !reflect.DeepEqual(got.Counted.OpenClose.Openings, want) {
		t.Fatalf("openings = %q, want %q", got.Counted.OpenClose.Openings, want)
	}
	if got.Counted.Sentences != 40 {
		t.Fatalf("a recount changed another item: %+v", got.Counted)
	}
	profile, err := svc.PromptProfileForTopic(context.Background(), "u", "v", "", LanguageKorean, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(profile.Text, "연남동에 다녀왔어요") || !strings.Contains(profile.Text, "안녕하세요") {
		t.Fatalf("the projection carries the once-seen line or drops the recurring one:\n%s", profile.Text)
	}
}

// VOICE-69: a synthetic analysis has one sample, so it holds no recurring line.
func TestASyntheticAnalysisHasNoOpenClose(t *testing.T) {
	stored := storedAnalysis(OriginSynthetic, "안녕하세요! 오늘은 국수예요.")
	stored.SyntheticSample = "안녕하세요! 오늘은 국수예요.\n맛있었어요.\n그럼 이만."
	got := withRecurringOpenClose(stored)
	if len(got.Counted.OpenClose.Openings) != 0 || len(got.Counted.OpenClose.Closings) != 0 || !got.Counted.OpenClose.Unknown {
		t.Fatalf("synthetic ⑤ = %+v", got.Counted.OpenClose)
	}
}

// VOICE-74: with no accepted bodies the stored lines stay as they were.
func TestAnAnalysisWithoutAcceptedBodiesKeepsItsStoredLines(t *testing.T) {
	stored := storedAnalysis(OriginPersonal, "오늘 연남동에 다녀왔어요.")
	stored.SourceVersionsKnown = false
	if got := withRecurringOpenClose(stored); !reflect.DeepEqual(got, stored) {
		t.Fatalf("an analysis without bodies changed: %+v", got.Counted.OpenClose)
	}
}

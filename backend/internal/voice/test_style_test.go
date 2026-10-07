package voice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func testedSynthetic(t *testing.T) Analysis {
	t.Helper()
	analysis, err := (&Authoring{}).PrepareWritingStyle(WritingStyleDraft{Name: "담백한 말투", Description: "짧고 따뜻하게 경험을 이야기하는 말투예요.", Sample: strings.Repeat("산책하다 작은 가게에서 차를 마셨어요. 창가에 앉아 조용한 풍경을 보니 마음이 편안해졌어요. ", 5)}, "p/w", time.Date(2026, 10, 7, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}
func TestStyleFactoryIsSyntheticAndRevisionUsesOnlyImmutableAcceptedData(t *testing.T) {
	analysis := testedSynthetic(t)
	if analysis.Origin != OriginSynthetic || len(analysis.MaterialIDs) != 0 || len(analysis.AcceptedSources) != 0 || len(analysis.AcceptedMaterials) != 0 || analysis.SyntheticSample == "" {
		t.Fatalf("profile=%+v", analysis)
	}
	baseline := AcceptedAnalysisRevision(analysis)
	copy := analysis
	copy.CreatedAt = copy.CreatedAt.In(time.FixedZone("KST", 9*3600))
	copy.AcceptedSources = []AcceptedSource{}
	copy.AcceptedMaterials = []AcceptedMaterial{}
	if AcceptedAnalysisRevision(copy) != baseline {
		t.Fatal("equivalent storage/time forms changed revision")
	}
	copy.AI.Impression += " 달라졌어요."
	if AcceptedAnalysisRevision(copy) == baseline {
		t.Fatal("changed profile kept revision")
	}
	personal := Analysis{Origin: OriginPersonal, CreatedAt: analysis.CreatedAt, MaterialIDs: []string{"sample"}, AcceptedSources: []AcceptedSource{{SampleID: "sample", ContentRevision: 1}}, AcceptedMaterials: []AcceptedMaterial{{Source: AcceptedSource{SampleID: "sample", ContentRevision: 1}, Body: "받아들인 이전 글", CreatedAt: analysis.CreatedAt}}, SourceVersionsKnown: true}
	rev := AcceptedAnalysisRevision(personal)
	personal.AcceptedSources[0].ContentRevision = 2
	if AcceptedAnalysisRevision(personal) == rev {
		t.Fatal("changed accepted source kept revision")
	}
}

type testedStoreFake struct {
	calls       int
	publication TestStylePublication
	created     Voice
}

func (s *testedStoreFake) PublishTestStyle(_ context.Context, in TestStylePublication, v Voice) (TestStyleReceipt, error) {
	s.calls++
	s.publication = in
	s.created = v
	return TestStyleReceipt{VoiceID: v.ID, RequestKey: in.RequestKey}, nil
}
func TestStylePublisherAcceptsExactSyntheticProfileWithoutRegenerationAndCannotCopyPersonalEvidence(t *testing.T) {
	analysis := testedSynthetic(t)
	store := &testedStoreFake{}
	publisher := NewTestStylePublisher(store)
	in := TestStylePublication{UserID: "alice", TestID: "test", WinnerID: "winner", Action: "save_setting", RequestKey: "key", Fingerprint: "frozen-input", Name: "My tested style", Analysis: analysis, MakeDefault: true}
	if _, err := publisher.PublishTestWinner(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 || AcceptedAnalysisRevision(store.publication.Analysis) != AcceptedAnalysisRevision(analysis) || store.created.Name != in.Name {
		t.Fatal("tested profile changed")
	}
	for _, invalid := range []TestStylePublication{
		{UserID: "alice", TestID: "test", WinnerID: "winner", Action: "save_setting", RequestKey: "key", Fingerprint: "frozen", Name: "copy", Analysis: Analysis{Origin: OriginPersonal}},
		{UserID: "alice", TestID: "test", WinnerID: "winner", Action: "save_setting", RequestKey: "key", Fingerprint: "frozen", Name: "copy", Analysis: Analysis{Origin: OriginSynthetic, MaterialIDs: []string{"personal"}}},
	} {
		if _, err := publisher.PublishTestWinner(context.Background(), invalid); !errors.Is(err, ErrTestStylePublicationConflict) {
			t.Fatalf("invalid err=%v", err)
		}
	}
	if store.calls != 1 {
		t.Fatal("personal copy reached store")
	}
	personal := Analysis{Origin: OriginPersonal, CreatedAt: analysis.CreatedAt, MaterialIDs: []string{"owned-source"}}
	in.Action = "use_setting"
	in.SourceVoiceID = "owned"
	in.Analysis = personal
	in.AcceptedRevision = AcceptedAnalysisRevision(personal)
	if _, err := publisher.PublishTestWinner(context.Background(), in); err != nil {
		t.Fatal(err)
	}
}

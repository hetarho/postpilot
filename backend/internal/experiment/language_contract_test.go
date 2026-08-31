package experiment

import (
	"context"
	"testing"
)

func experimentLanguagePointer(value Language) *Language { return &value }

func TestFreezeSnapshotIncludesTargetLanguageInHashAndDetail(t *testing.T) {
	raw := []byte(`{"same":true}`)
	ko, koHash, err := FreezeSnapshot(Snapshot{
		Content: raw, PromptVersion: "write-v3", VoiceID: "voice", PurposeName: "purpose",
		TargetLanguage: experimentLanguagePointer(LanguageKorean),
	})
	if err != nil {
		t.Fatal(err)
	}
	en, enHash, err := FreezeSnapshot(Snapshot{
		Content: raw, PromptVersion: "write-v3", VoiceID: "voice", PurposeName: "purpose",
		TargetLanguage: experimentLanguagePointer(LanguageEnglish),
	})
	if err != nil {
		t.Fatal(err)
	}
	if koHash == enHash {
		t.Fatal("changing only the target language did not change the input hash")
	}
	if ko.TargetLanguage == nil || *ko.TargetLanguage != LanguageKorean || en.TargetLanguage == nil || *en.TargetLanguage != LanguageEnglish {
		t.Fatalf("frozen targets = %v / %v", ko.TargetLanguage, en.TargetLanguage)
	}
	if ko.VoiceID != "voice" || ko.PurposeName != "purpose" {
		t.Fatalf("snapshot detail was dropped: %+v", ko)
	}
	*ko.TargetLanguage = LanguageEnglish
	if en.TargetLanguage == ko.TargetLanguage {
		t.Fatal("frozen snapshots share target-language pointer storage")
	}
}

func TestWriteExperimentRetainsFrozenTargetThroughHandleAndRetry(t *testing.T) {
	svc, store, _, jobs, runner := newTestService()
	runner.snapshotTarget = LanguageEnglish
	started, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", Stage: StageWrite,
		ModelA: ModelRef{ProviderID: "p", ModelID: "a"}, ModelB: ModelRef{ProviderID: "p", ModelID: "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	found, err := store.Get(context.Background(), started.ExperimentID)
	if err != nil {
		t.Fatal(err)
	}
	if found.TargetLanguage == nil || *found.TargetLanguage != LanguageEnglish {
		t.Fatalf("stored target = %v", found.TargetLanguage)
	}
	runner.fail["b"] = context.DeadlineExceeded
	if err := svc.Handle(context.Background(), found.ID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	partial, _ := store.Get(context.Background(), found.ID)
	if partial.TargetLanguage == nil || *partial.TargetLanguage != LanguageEnglish {
		t.Fatalf("target after prepare/handle = %v", partial.TargetLanguage)
	}
	if _, err := svc.Retry(context.Background(), "alice", found.ID); err != nil {
		t.Fatal(err)
	}
	retried, _ := store.Get(context.Background(), found.ID)
	if retried.TargetLanguage == nil || *retried.TargetLanguage != LanguageEnglish {
		t.Fatalf("target after retry = %v", retried.TargetLanguage)
	}
	if len(jobs.requests) != 2 {
		t.Fatalf("job requests = %+v", jobs.requests)
	}
	for i, request := range jobs.requests {
		if request.TargetLanguage == nil || *request.TargetLanguage != LanguageEnglish {
			t.Fatalf("job request %d target = %v", i+1, request.TargetLanguage)
		}
	}
}

func TestWriteExperimentRejectsMissingOrUnsupportedRunnerTarget(t *testing.T) {
	for name, target := range map[string]Language{"missing": "", "unsupported": "fr"} {
		t.Run(name, func(t *testing.T) {
			svc, store, _, jobs, runner := newTestService()
			runner.snapshotTarget = target
			if target == "" {
				runner.omitSnapshotTarget = true
			}
			_, err := svc.Start(context.Background(), StartRequest{
				UserID: "alice", PostSlug: "post", Stage: StageWrite,
				ModelA: ModelRef{ProviderID: "p", ModelID: "a"}, ModelB: ModelRef{ProviderID: "p", ModelID: "b"},
			})
			if err != ErrLanguageRequired || len(store.rows) != 0 || len(jobs.ids) != 0 {
				t.Fatalf("error/store/jobs = %v / %v / %v", err, store.rows, jobs.ids)
			}
		})
	}
}

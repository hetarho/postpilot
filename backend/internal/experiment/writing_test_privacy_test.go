package experiment

import (
	"reflect"
	"testing"
)

func TestWritingTestProjectionBlindsEveryNonrevealedStateAndAlwaysOmitsSupplierCost(t *testing.T) {
	for _, factor := range []TestFactor{FactorModel, FactorVoice, FactorTemplate, FactorGuideline} {
		for _, status := range []TestStatus{TestQueued, TestRunning, TestPartial, TestReview, TestFailed, TestCompleted, TestCancelled} {
			t.Run(string(factor)+"/"+string(status), func(t *testing.T) {
				ref := TestEntrantRef{SourceKind: "setting", SettingKind: string(factor), SettingID: "private-source", SettingRevision: "private-version"}
				original := WritingTest{ID: "test", UserID: "owner", Factor: factor, Status: status, SourcePostSlug: "source", CommonSnapshot: []byte("private instructions"), Input: TestInput{TargetLanguage: "en", Fictional: true, Material: "private material", VoiceID: "private-voice", TemplateID: "private-template"}, ConfirmedCredits: 4, ReservedCredits: 9,
					Failure:      &Failure{Reason: "MODEL_RATE_LIMITED", TechnicalDetail: "private provider", Params: map[string]string{"private": "secret"}},
					Candidates:   []TestCandidate{{ID: "candidate", SeedPosition: 7, SnapshotIndex: 3, Ref: ref, SourceRevision: "private-version", SemanticKey: "private-semantics", FrozenVariant: []byte("private setting"), Output: []byte("readable complete answer"), Accounting: []byte("private cost"), Identity: &TestCandidateIdentity{Label: "Private setting", Ref: ref, Synthetic: true}, Usage: &Usage{PromptTokens: 10, CompletionTokens: 20, LatencyMS: 30, CostMicrousd: 123, CostSource: CostReported}, Failure: &Failure{Reason: "MODEL_RATE_LIMITED", TechnicalDetail: "private model", Params: map[string]string{"model": "private"}}}},
					Publications: []TestPublication{{ID: "publication", Fingerprint: "private digest"}},
				}
				got := ProjectWritingTest(original)
				if len(got.CommonSnapshot) != 0 || got.Input.Material != "" || got.Input.VoiceID != "" || got.Input.TemplateID != "" || got.Input.TargetLanguage != "en" || !got.Input.Fictional {
					t.Fatalf("private common material leaked: %#v", got.Input)
				}
				if got.Failure.TechnicalDetail != "" || len(got.Failure.Params) != 0 {
					t.Fatal("private aggregate failure leaked")
				}
				c := got.Candidates[0]
				if c.Ref != (TestEntrantRef{}) || c.SeedPosition != 0 || c.SnapshotIndex != 0 || c.SourceRevision != "" || c.SemanticKey != "" || len(c.FrozenVariant) != 0 || len(c.Accounting) != 0 {
					t.Fatalf("private candidate snapshot leaked: %#v", c)
				}
				if c.Failure.TechnicalDetail != "" || len(c.Failure.Params) != 0 || string(c.Output) != "readable complete answer" {
					t.Fatal("projection changed output or leaked diagnostic")
				}
				revealed := status == TestCompleted || status == TestCancelled
				if !revealed {
					if c.Identity != nil || c.Usage != nil || got.SourcePostSlug != "" || got.ConfirmedCredits != 0 || got.ReservedCredits != 0 {
						t.Fatal("blind result reveals identity/accounting/source")
					}
				} else {
					if c.Identity == nil || c.Identity.Ref != ref || c.Usage == nil || c.Usage.PromptTokens != 10 || c.Usage.CompletionTokens != 20 || c.Usage.LatencyMS != 30 || c.Usage.CostMicrousd != 0 || c.Usage.CostSource != "" {
						t.Fatal("revealed result loses useful identity/usage or exposes supplier cost")
					}
					c.Identity.Label = "manual change"
				}
				c.Output[0] = 'x'
				c.Failure.Reason = "changed"
				if got.Publications[0].Fingerprint != "" {
					t.Fatal("private publication digest leaked")
				}
				got.Publications[0].ID = "changed"
				if original.Publications[0].ID != "publication" || original.Publications[0].Fingerprint != "private digest" || original.Candidates[0].SeedPosition != 7 || original.Candidates[0].SnapshotIndex != 3 {
					t.Fatal("private execution metadata changed")
				}
				if string(original.Candidates[0].Output) != "readable complete answer" || !reflect.DeepEqual(original.Candidates[0].Identity.Ref, ref) || original.Candidates[0].Identity.Label != "Private setting" || original.Candidates[0].Failure.Reason != "MODEL_RATE_LIMITED" {
					t.Fatal("projection aliases stored private state")
				}
			})
		}
	}
}

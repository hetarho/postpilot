package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

// These tests cross the real generation factory and its public snapshot codec.
// Ordinary mutation collaborators panic if an independent test reaches them.
type adapterCanonicalPosts struct{ reads, writes int }

func (p *adapterCanonicalPosts) AttachedImages(context.Context, string, string) (generation.PostInput, error) {
	p.reads++
	return generation.PostInput{}, errors.New("ordinary source lookup is forbidden")
}
func (p *adapterCanonicalPosts) SetObservations(context.Context, string, string, []generation.Observation) error {
	p.writes++
	panic("test wrote canonical observations")
}
func (p *adapterCanonicalPosts) SetGeneratedContent(context.Context, string, string, generation.PostContent, generation.Language, *generation.WriteAnnotations) error {
	p.writes++
	panic("test wrote canonical content")
}
func (p *adapterCanonicalPosts) SetStoryline(context.Context, string, string, generation.Storyline) error {
	p.writes++
	panic("test wrote canonical storyline")
}

type adapterUnusedReads struct {
	generation.Profiles
	generation.ImageReader
	generation.Jobs
	generation.TemplateBriefs
	generation.GuidelinesForPrompt
	generation.MemoriesForPrompt
	generation.GuidelineCandidates
	generation.QualityRulesForPrompt
	generation.VideoLinker
	generation.WritingTestTemplates
	generation.WritingTestCandidates
}
type adapterBudget struct{}

func (adapterBudget) Write(*int, bool) int       { return 2048 }
func (adapterBudget) Revise(int, *int, bool) int { return 2048 }
func (adapterBudget) Storyline(bool) int         { return 512 }
func (adapterBudget) Observation() int           { return 512 }

type adapterFactoryPorts struct {
	sourceReads, rightsChecks, voiceReads int
	blocked                               bool
}

func (p *adapterFactoryPorts) ResolveWritingTestSource(_ context.Context, user, slug string, input, content int64, m generation.WritingTestMaterialRequest) (generation.WritingTestSource, error) {
	p.sourceReads++
	if user != "alice" || slug != "source" || input != 4 || content != 7 {
		return generation.WritingTestSource{}, generation.ErrWritingTestRevision
	}
	return generation.WritingTestSource{Post: generation.PostInput{UserID: user, Slug: slug, Title: "Shared title", Memo: m.Material, Content: &generation.PostContent{Title: "PRIVATE PRIOR CANONICAL"}, Storyline: &generation.Storyline{Paragraphs: []generation.StorylineParagraph{{Text: "PRIOR CANONICAL STORYLINE"}}}}, InputRevision: input, ContentRevision: content, Revision: "source-v4", AssignmentsHash: "frozen-assignments"}, nil
}
func adapterModel(ref llm.ModelRef) llm.ModelInfo {
	return llm.ModelInfo{Ref: ref, Stages: []string{"observe", "write"}, Vision: true, StructuredOutput: true, ContextTokens: 131072}
}
func (p *adapterFactoryPorts) PrepareWritingTestModel(_ context.Context, user, stage string, ref llm.ModelRef) (generation.WritingTestModel, error) {
	p.rightsChecks++
	if p.blocked || user != "alice" || ref.ProviderID != "approved" || ref.ModelID == "" || (stage != "write" && stage != "observe") {
		return generation.WritingTestModel{}, llm.ErrModelUnavailable
	}
	return generation.WritingTestModel{Info: adapterModel(ref), PromptTokens: 30000}, nil
}
func (p *adapterFactoryPorts) ResolveWritingTestProfile(_ context.Context, user string, ref generation.WritingTestReference, _ *generation.WritingTestPreparedSetting, _ generation.Language, _ string) (generation.WritingTestVoice, error) {
	p.voiceReads++
	if user != "alice" || ref.SourceKind != "setting" || ref.SettingKind != "voice" || ref.SettingRevision != "accepted-v1" {
		return generation.WritingTestVoice{}, generation.ErrWritingTestReference
	}
	return generation.WritingTestVoice{Voice: generation.VoiceRef{ID: ref.SettingID, Name: "Approved " + ref.SettingID, Made: true}, Profile: generation.Profile{Text: "Frozen expression " + ref.SettingID}, Revision: ref.SettingRevision, Synthetic: true}, nil
}
func (*adapterFactoryPorts) FreezeWritingTestGuidelines(_ context.Context, user, _, _ string, _ generation.Language, _ bool) (generation.WritingTestRules, error) {
	if user != "alice" {
		return generation.WritingTestRules{}, generation.ErrWritingTestReference
	}
	return generation.WritingTestRules{Defaults: []string{"Approved common direction"}}, nil
}
func (*adapterFactoryPorts) ResolveWritingTestGuideline(context.Context, string, generation.WritingTestReference, *generation.WritingTestPreparedSetting) (generation.WritingTestRule, error) {
	panic("unused guideline contender")
}

type adapterLLMCall struct {
	ref     llm.ModelRef
	request llm.Request
}
type adapterLLM struct {
	calls  []adapterLLMCall
	before func(llm.ModelRef, llm.Request)
}

func (*adapterLLM) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return adapterModel(ref), ref.ProviderID == "approved" && ref.ModelID != ""
}
func (m *adapterLLM) Complete(_ context.Context, ref llm.ModelRef, r llm.Request) (llm.Response, error) {
	if m.before != nil {
		m.before(ref, r)
	}
	m.calls = append(m.calls, adapterLLMCall{ref, r})
	return llm.Response{Text: fmt.Sprintf(`{"title":"Complete %s","summary":"Complete summary","tags":["one","two","three","extra"],"blocks":[{"type":"TEXT","content":"A full validated post based on the approved material."},{"type":"LIST","items":["first","second"]}],"storyline":[{"text":"A complete generated plan","files":[]}],"nouns":["material","post"]}`, ref.ModelID), Usage: llm.Usage{PromptTokens: 23, CompletionTokens: 17, CostMicrousd: 9, CostReported: true}}, nil
}

type adapterWorkStore struct{ work experiment.TestExecutionWork }

func (s *adapterWorkStore) GetTest(_ context.Context, user, id string) (experiment.WritingTest, error) {
	if user != s.work.Test.UserID || id != s.work.Test.ID {
		return experiment.WritingTest{}, experiment.ErrTestNotFound
	}
	return s.work.Test, nil
}
func (s *adapterWorkStore) PreparedTestWork(_ context.Context, user, id string) (experiment.TestExecutionWork, error) {
	if user != s.work.Test.UserID || id != s.work.Test.ID {
		return experiment.TestExecutionWork{}, experiment.ErrTestNotFound
	}
	return s.work, nil
}
func realFactoryAdapter(t *testing.T, factor experiment.TestFactor, count int) (*WritingTestGeneration, *adapterWorkStore, *adapterFactoryPorts, *adapterLLM, *adapterCanonicalPosts) {
	t.Helper()
	ports := &adapterFactoryPorts{}
	models := &adapterLLM{}
	canonical := &adapterCanonicalPosts{}
	unused := adapterUnusedReads{}
	service := generation.NewService(canonical, unused, models, unused, unused, 2, generation.DefaultReasoningPolicy(), adapterBudget{}, generation.Deps{Templates: unused, Guidelines: unused, Memories: unused, Candidates: unused, QualityRules: unused, Videos: unused, VideoURLTTL: time.Minute})
	factory := generation.NewWritingTestFactory(service, generation.WritingTestFactoryDeps{Sources: ports, Models: ports, Profiles: ports, Templates: unused, Guidelines: ports, Candidates: unused})
	store := &adapterWorkStore{}
	adapter := NewWritingTestGeneration(factory, store)
	start := experiment.TestStart{UserID: "alice", RequestKey: "start", Factor: factor, Count: count, Input: experiment.TestInput{SourcePostSlug: "source", InputRevision: 4, ContentRevision: 7, Material: "Explicit approved owner material", TargetLanguage: "en", TargetLength: 1500, TagCount: 3, ObserveModel: experiment.ModelRef{ProviderID: "approved", ModelID: "observer"}, WriteModel: experiment.ModelRef{ProviderID: "approved", ModelID: "fixed-writer"}}}
	if factor == experiment.FactorModel {
		start.ModelStage = experiment.StageWrite
	}
	var variants []experiment.FrozenTestVariant
	for i := 0; i < count; i++ {
		ref := experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ProviderID: "approved", ModelID: fmt.Sprintf("writer-%d", i)}}
		if factor == experiment.FactorVoice {
			ref = experiment.TestEntrantRef{SourceKind: "setting", SettingKind: "voice", SettingID: fmt.Sprintf("voice-%d", i), SettingRevision: "accepted-v1"}
		}
		start.Entrants = append(start.Entrants, ref)
		variants = append(variants, experiment.FrozenTestVariant{Reference: ref, Label: fmt.Sprintf("Frozen entrant %d", i)})
	}
	plan, err := adapter.PrepareWritingTest(context.Background(), start, variants)
	if err != nil {
		t.Fatal(err)
	}
	test, err := experiment.BuildWritingTest(start, plan, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(test.Candidates)
	work := experiment.TestExecutionWork{Test: test, Plan: plan, CandidateIndices: map[string]int{}, CandidateCheckpoints: map[string][]byte{}}
	for _, c := range test.Candidates {
		work.CandidateIDs = append(work.CandidateIDs, c.ID)
		work.CandidateIndices[c.ID] = c.SnapshotIndex
	}
	store.work = work
	return adapter, store, ports, models, canonical
}
func saveAdapterCheckpoint(store *adapterWorkStore, id string) func(context.Context, []byte) error {
	return func(_ context.Context, raw []byte) error {
		if id == "" {
			store.work.SharedCheckpoint = slices.Clone(raw)
		} else {
			store.work.CandidateCheckpoints[id] = slices.Clone(raw)
		}
		return nil
	}
}
func prepareAdapterInput(t *testing.T, a *WritingTestGeneration, s *adapterWorkStore) []byte {
	t.Helper()
	shared, err := a.PrepareTestInput(context.Background(), s.work, saveAdapterCheckpoint(s, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	return shared
}
func TestActualFactoryAdapterProducesEveryFrozenEntrantAndCompleteStorylineWithoutCanonicalWrites(t *testing.T) {
	for _, scenario := range []struct {
		factor experiment.TestFactor
		count  int
	}{{experiment.FactorModel, 2}, {experiment.FactorModel, 16}, {experiment.FactorVoice, 2}} {
		t.Run(fmt.Sprintf("%s/%d", scenario.factor, scenario.count), func(t *testing.T) {
			a, s, ports, models, canonical := realFactoryAdapter(t, scenario.factor, scenario.count)
			shared := prepareAdapterInput(t, a, s)
			planned := 0
			for _, call := range s.work.Plan.Calls {
				planned += call.Count
				if call.Stage != experiment.StageWrite || call.PromptTokens != 30000 || call.CompletionTokens != 2048 {
					t.Fatalf("bounded plan=%+v", call)
				}
			}
			if planned != scenario.count || len(models.calls) != 0 {
				t.Fatalf("plan=%d preparation calls=%d", planned, len(models.calls))
			}
			for _, c := range s.work.Test.Candidates {
				id := c.ID
				models.before = func(_ llm.ModelRef, r llm.Request) {
					cp, err := decodeTestCheckpoint(s.work.CandidateCheckpoints[id])
					if err != nil || cp == nil || cp.Index != c.SnapshotIndex || cp.InFlightStage != "write" || r.Stage != "write" {
						t.Fatalf("call was not durably fenced checkpoint=%+v err=%v", cp, err)
					}
				}
				result, err := a.RunTestCandidate(context.Background(), s.work, id, shared, saveAdapterCheckpoint(s, id), nil)
				if err != nil {
					t.Fatal(err)
				}
				output, err := experiment.DecodeTestOutput(result.Output)
				if err != nil || output.ContentLanguage != "en" || output.Storyline == nil || len(output.Storyline.Paragraphs) != 1 || output.Storyline.Paragraphs[0].Text != "A complete generated plan" || len(output.Content.Blocks) != 2 || len(output.Content.Tags) != 3 || !slices.Equal(output.Nouns, []string{"material", "post"}) {
					t.Fatalf("complete output=%+v err=%v", output, err)
				}
				expected := "fixed-writer"
				if scenario.factor == experiment.FactorModel {
					expected = fmt.Sprintf("writer-%d", c.SnapshotIndex)
				}
				if output.Content.Title != "Complete "+expected || models.calls[len(models.calls)-1].ref.ModelID != expected {
					t.Fatalf("shuffled contestant ran wrong original snapshot index=%d output=%s", c.SnapshotIndex, output.Content.Title)
				}
				if bytes.Contains(result.Output, []byte("PRIOR CANONICAL")) {
					t.Fatal("source result was reused")
				}
			}
			if len(models.calls) != scenario.count || ports.sourceReads != 1 || canonical.reads != 0 || canonical.writes != 0 {
				t.Fatalf("call/source accounting calls=%d sourcefreeze=%d canonical=%+v", len(models.calls), ports.sourceReads, canonical)
			}
			// The publication adapter sees seed/display order, while the frozen
			// generation hash and descriptor preserve the original variant order.
			for i := range s.work.Test.Candidates {
				s.work.Test.Candidates[i].Status = "succeeded"
			}
			s.work.Test.Status = experiment.TestCompleted
			s.work.Test.WinnerID = s.work.Test.Candidates[0].ID
			description, winner, err := (GenerationPublicationSnapshots{}).DescribePublication(s.work.Test, s.work.Test.WinnerID)
			if err != nil || description.UserID != "alice" || description.AssignmentsHash != "frozen-assignments" || len(description.Variants) != scenario.count || winner.Reference != toGenerationTestReference(s.work.Test.Candidates[0].Ref) {
				t.Fatalf("shuffled publication metadata lost exact winner=%+v description=%+v err=%v", winner, description, err)
			}
			for _, call := range models.calls {
				if strings.Contains(call.request.System, "verification") || len(call.request.JSONSchema) == 0 || call.request.MaxTokens != 2048 {
					t.Fatalf("full writing contract lost=%+v", call.request)
				}
			}
		})
	}
}
func TestActualFactoryAdapterPlansOnlyFailedOriginalSnapshotAndReplaysPaidAnswerWithZeroCalls(t *testing.T) {
	a, s, ports, models, canonical := realFactoryAdapter(t, experiment.FactorModel, 2)
	shared := prepareAdapterInput(t, a, s)
	candidate := s.work.Test.Candidates[0]
	other := s.work.Test.Candidates[1]
	s.work.Test.Revision = 9
	s.work.Test.Status = experiment.TestPartial
	for i := range s.work.Test.Candidates {
		if s.work.Test.Candidates[i].ID == candidate.ID {
			s.work.Test.Candidates[i].Status = "failed"
		} else {
			s.work.Test.Candidates[i].Status = "succeeded"
		}
	}
	request := experiment.TestRetryQuoteRequest{UserID: "alice", TestID: s.work.Test.ID, ExpectedRevision: 9, CandidateIDs: []string{candidate.ID}}
	plan, err := a.PrepareFailedTestCandidates(context.Background(), request)
	if err != nil || len(plan.Calls) != 1 || plan.Calls[0].Count != 1 || plan.Calls[0].Ref != candidate.Ref.Model || plan.Snapshot.Hash != s.work.Plan.Snapshot.Hash {
		t.Fatalf("failed-only exact plan=%+v err=%v", plan, err)
	}
	bad := request
	bad.CandidateIDs = []string{other.ID}
	if _, err := a.PrepareFailedTestCandidates(context.Background(), bad); !errors.Is(err, experiment.ErrTestEntrant) {
		t.Fatalf("successful contestant entered retry=%v", err)
	}
	result, err := a.RunTestCandidate(context.Background(), s.work, candidate.ID, shared, saveAdapterCheckpoint(s, candidate.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	issued := len(models.calls)
	rights := ports.rightsChecks
	ports.blocked = true
	recovery, err := a.PrepareFailedTestCandidates(context.Background(), request)
	if err != nil || len(recovery.Calls) != 0 || recovery.Snapshot.Hash != s.work.Plan.Snapshot.Hash || ports.rightsChecks != rights {
		t.Fatalf("paid checkpoint requoted calls=%+v rights=%d/%d err=%v", recovery.Calls, ports.rightsChecks, rights, err)
	}
	replayed, err := a.RunTestCandidate(context.Background(), s.work, candidate.ID, shared, saveAdapterCheckpoint(s, candidate.ID), nil)
	if err != nil || !bytes.Equal(replayed.Output, result.Output) || len(replayed.Accounting) != 0 || len(models.calls) != issued || ports.sourceReads != 1 || canonical.writes != 0 {
		t.Fatalf("paid answer replay charged/replaced output result=%+v calls=%d err=%v", replayed, len(models.calls), err)
	}
}
func TestActualFactoryAdapterRefusesIssuedUnconfirmedCheckpointWithoutReplayOrFreshQuote(t *testing.T) {
	a, s, _, models, canonical := realFactoryAdapter(t, experiment.FactorModel, 2)
	shared := prepareAdapterInput(t, a, s)
	candidate := s.work.Test.Candidates[0]
	lost := errors.New("settled checkpoint write response unavailable")
	save := func(ctx context.Context, raw []byte) error {
		cp, err := decodeTestCheckpoint(raw)
		if err != nil {
			return err
		}
		if cp.Answer != nil {
			return lost
		}
		return saveAdapterCheckpoint(s, candidate.ID)(ctx, raw)
	}
	if _, err := a.RunTestCandidate(context.Background(), s.work, candidate.ID, shared, save, nil); !errors.Is(err, lost) {
		t.Fatalf("uncertainty was not injected=%v", err)
	}
	if len(models.calls) != 1 {
		t.Fatalf("issued calls=%d", len(models.calls))
	}
	s.work.Test.Revision = 5
	for i := range s.work.Test.Candidates {
		if s.work.Test.Candidates[i].ID == candidate.ID {
			s.work.Test.Candidates[i].Status = "failed"
		}
	}
	request := experiment.TestRetryQuoteRequest{UserID: "alice", TestID: s.work.Test.ID, ExpectedRevision: 5, CandidateIDs: []string{candidate.ID}}
	if _, err := a.PrepareFailedTestCandidates(context.Background(), request); !errors.Is(err, experiment.ErrTestStateInvalid) {
		t.Fatalf("uncertain call admitted fresh quote=%v", err)
	}
	if _, err := a.RunTestCandidate(context.Background(), s.work, candidate.ID, shared, saveAdapterCheckpoint(s, candidate.ID), nil); !errors.Is(err, generation.ErrWritingTestExecutionUncertain) {
		t.Fatalf("uncertain call replayed=%v", err)
	}
	if len(models.calls) != 1 || canonical.reads != 0 || canonical.writes != 0 {
		t.Fatal("uncertain call spent or mutated a source twice")
	}
}

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

type publicationTestStore struct {
	test              experiment.WritingTest
	publication       experiment.TestPublication
	confirmLost       bool
	confirmationCalls int
}

func (s *publicationTestStore) GetTest(_ context.Context, user, id string) (experiment.WritingTest, error) {
	if user != s.test.UserID || id != s.test.ID {
		return experiment.WritingTest{}, experiment.ErrTestNotFound
	}
	found := s.test
	if s.publication.ID != "" {
		found.Publications = []experiment.TestPublication{s.publication}
	}
	return found, nil
}
func (s *publicationTestStore) BeginPublication(_ context.Context, in experiment.WinnerPublication) (experiment.TestPublication, error) {
	if in.UserID != s.test.UserID || in.TestID != s.test.ID {
		return experiment.TestPublication{}, experiment.ErrTestNotFound
	}
	if in.WinnerID != s.test.WinnerID {
		return experiment.TestPublication{}, experiment.ErrTestOperation
	}
	if s.publication.ID != "" {
		return s.publication, nil
	}
	if s.test.PurgeFence != 0 || s.test.Status != experiment.TestCompleted || in.ExpectedRevision != s.test.Revision {
		return experiment.TestPublication{}, experiment.ErrTestRevisionConflict
	}
	s.publication = experiment.TestPublication{ID: "pub", UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey, Fingerprint: "frozen-choice", Status: "pending"}
	return s.publication, nil
}
func (s *publicationTestStore) ConfirmPublication(_ context.Context, p experiment.TestPublication, r experiment.PublicationReceipt) (experiment.TestPublication, error) {
	s.confirmationCalls++
	if p.UserID != r.UserID || p.TestID != r.TestID || p.WinnerID != r.WinnerID || p.Action != r.Action || p.RequestKey != r.RequestKey {
		return p, experiment.ErrTestPublicationConflict
	}
	if s.confirmLost {
		s.confirmLost = false
		return p, errors.New("experiment receipt temporarily unavailable")
	}
	s.publication.TargetID = r.TargetID
	s.publication.Status = "confirmed"
	return s.publication, nil
}

type publicationSnapshotFake struct {
	calls       int
	description generation.WritingTestDescription
	variant     generation.WritingTestVariantDescription
}

func (s *publicationSnapshotFake) DescribePublication(t experiment.WritingTest, winner string) (generation.WritingTestDescription, generation.WritingTestVariantDescription, error) {
	s.calls++
	if t.UserID != s.description.UserID || winner != t.WinnerID {
		return generation.WritingTestDescription{}, generation.WritingTestVariantDescription{}, experiment.ErrTestPublicationConflict
	}
	return s.description, s.variant, nil
}

type publicationTarget struct {
	validationFailure error
	validations       int

	calls, reads int
	key, target  string
	model        provider.TestModelAdoption
	voice        voice.TestStylePublication
	template     template.TestedPublication
	guideline    guideline.TestedPublication
	post         post.TestOutputPublication
	failure      error
}
type modelPublicationFake struct{ h *publicationTarget }

func (f modelPublicationFake) AdoptTestModel(_ context.Context, in provider.TestModelAdoption) (provider.TestModelReceipt, error) {
	f.h.calls++
	f.h.model = in
	if f.h.failure != nil {
		return provider.TestModelReceipt{}, f.h.failure
	}
	f.h.key = in.RequestKey
	f.h.target = in.Ref.String()
	return provider.TestModelReceipt{RequestKey: in.RequestKey, Stage: in.Stage, Ref: in.Ref}, nil
}
func (f modelPublicationFake) ReadTestPublicationReceipt(context.Context, string, string, string) (provider.TestModelReceipt, bool, error) {
	f.h.reads++
	return provider.TestModelReceipt{RequestKey: f.h.key, Stage: f.h.model.Stage, Ref: f.h.model.Ref}, f.h.key != "", nil
}

type templatePublicationFake struct{ h *publicationTarget }

func (f templatePublicationFake) PublishTestWinner(_ context.Context, in template.TestedPublication) (template.TestedPublicationReceipt, error) {
	f.h.calls++
	f.h.template = in
	if f.h.failure != nil {
		return template.TestedPublicationReceipt{}, f.h.failure
	}
	f.h.key = in.RequestKey
	f.h.target = "saved-template"
	return template.TestedPublicationReceipt{RequestKey: in.RequestKey, TargetID: f.h.target}, nil
}
func (f templatePublicationFake) ReadTestPublicationReceipt(context.Context, string, string, string, string) (template.TestedPublicationReceipt, bool, error) {
	f.h.reads++
	return template.TestedPublicationReceipt{RequestKey: f.h.key, TargetID: f.h.target}, f.h.key != "", nil
}

type guidelinePublicationFake struct{ h *publicationTarget }

func (f guidelinePublicationFake) PublishTestWinner(_ context.Context, in guideline.TestedPublication) (guideline.TestedPublicationReceipt, error) {
	f.h.calls++
	f.h.guideline = in
	if f.h.failure != nil {
		return guideline.TestedPublicationReceipt{}, f.h.failure
	}
	f.h.key = in.RequestKey
	f.h.target = "saved-guideline"
	return guideline.TestedPublicationReceipt{RequestKey: in.RequestKey, TargetID: f.h.target}, nil
}
func (f guidelinePublicationFake) ReadTestPublicationReceipt(context.Context, string, string, string, string) (guideline.TestedPublicationReceipt, bool, error) {
	f.h.reads++
	return guideline.TestedPublicationReceipt{RequestKey: f.h.key, TargetID: f.h.target}, f.h.key != "", nil
}

type voicePublicationFake struct{ h *publicationTarget }

func (f voicePublicationFake) PublishTestWinner(_ context.Context, in voice.TestStylePublication) (voice.TestStyleReceipt, error) {
	f.h.calls++
	f.h.voice = in
	if f.h.failure != nil {
		return voice.TestStyleReceipt{}, f.h.failure
	}
	f.h.key = in.RequestKey
	f.h.target = "saved-voice"
	return voice.TestStyleReceipt{RequestKey: in.RequestKey, VoiceID: f.h.target}, nil
}
func (f voicePublicationFake) ReadTestPublicationReceipt(context.Context, string, string, string, string) (voice.TestStyleReceipt, bool, error) {
	f.h.reads++
	return voice.TestStyleReceipt{RequestKey: f.h.key, VoiceID: f.h.target}, f.h.key != "", nil
}

type postPublicationFake struct{ h *publicationTarget }

func (f postPublicationFake) ApplyTestResult(_ context.Context, in post.TestOutputPublication) (post.TestOutputReceipt, error) {
	f.h.calls++
	f.h.post = in
	if f.h.failure != nil {
		return post.TestOutputReceipt{}, f.h.failure
	}
	f.h.key = in.RequestKey
	f.h.target = in.PostSlug
	return post.TestOutputReceipt{UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID, Action: "apply_output", RequestKey: in.RequestKey, TargetID: in.PostSlug, ResultingRevision: 8}, nil
}
func (f postPublicationFake) ReadTestPublicationReceipt(_ context.Context, user, test, winner string) (post.TestOutputReceipt, bool, error) {
	f.h.reads++
	return post.TestOutputReceipt{UserID: user, TestID: test, WinnerID: winner, Action: "apply_output", RequestKey: f.h.key, TargetID: f.h.target, ResultingRevision: 8}, f.h.key != "", nil
}
func publicationFixture(t *testing.T, factor experiment.TestFactor) (*Publications, *publicationTestStore, *publicationSnapshotFake, *publicationTarget, experiment.WinnerPublication) {
	t.Helper()
	stage := experiment.Stage("")
	if factor == experiment.FactorModel {
		stage = experiment.StageWrite
	}
	raw, err := experiment.EncodeTestOutput(experiment.TestOutput{ContentLanguage: "en", Content: experiment.TestOutputContent{Title: "paid winner", Tags: []string{"tag"}, Blocks: []experiment.TestOutputBlock{{Type: "TEXT", Content: "full canonical output"}, {Type: "GALLERY", Files: []string{"one.jpg", "two.jpg"}, Layout: "SLIDE"}}}, Storyline: &experiment.TestOutputStoryline{MadeWith: []string{"one.jpg", "two.jpg"}, Paragraphs: []experiment.TestOutputParagraph{{Text: "complete storyline", Files: []string{"one.jpg"}}}}, Nouns: []string{"winner"}})
	if err != nil {
		t.Fatal(err)
	}
	store := &publicationTestStore{test: experiment.WritingTest{ID: "test", UserID: "alice", Factor: factor, ModelStage: stage, Count: 2, Status: experiment.TestCompleted, Revision: 4, WinnerID: "winner", SourcePostSlug: "source", CommonSnapshot: []byte("frozen common"), Candidates: []experiment.TestCandidate{{ID: "winner", Status: "succeeded", Output: raw}}}}
	snap := &publicationSnapshotFake{description: generation.WritingTestDescription{UserID: "alice", Factor: string(factor), ModelStage: string(stage), SourcePostSlug: "source", AssignmentsHash: "assignments", InputRevision: 6, ContentRevision: 7, TargetLanguage: generation.LanguageEnglish}, variant: generation.WritingTestVariantDescription{Revision: "accepted", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "winner-model"}, ObserveModel: llm.ModelRef{ProviderID: "p", ModelID: "winner-observer"}, Payload: []byte("exact owned snapshot"), Reference: generation.WritingTestReference{SourceKind: "setting", SettingKind: string(factor), SettingID: "owned-setting", SettingRevision: "accepted"}}}
	if factor == experiment.FactorVoice {
		snap.variant.Payload, _ = json.Marshal(voice.FrozenWritingStyle{Revision: "accepted", Analysis: voice.Analysis{Origin: voice.OriginSynthetic, SyntheticSample: "frozen fictional example"}})
	}
	target := &publicationTarget{}
	service := NewPublications(PublicationDependencies{Store: store, Snapshots: snap, Models: modelPublicationFake{target}, Templates: templatePublicationFake{target}, Guidelines: guidelinePublicationFake{target}, Voices: voicePublicationFake{target}, Posts: postPublicationFake{target}})
	in := experiment.WinnerPublication{TestMutation: experiment.TestMutation{UserID: "alice", TestID: "test", ExpectedRevision: 4, RequestKey: "original-key"}, WinnerID: "winner", Action: "save_setting", Name: "Explicit copy", Scope: ""}
	if factor == experiment.FactorGuideline {
		in.Scope = "global"
	}
	if factor == experiment.FactorModel {
		in.Action = "adopt_model"
		in.Name = ""
		in.Scope = ""
	}
	return service, store, snap, target, in
}
func TestChampionPublicationLostResponseRecoversReceiptBeforePurgedSnapshotAndNeverRepeatsLaterChoices(t *testing.T) {
	for _, factor := range []experiment.TestFactor{experiment.FactorModel, experiment.FactorVoice, experiment.FactorTemplate, experiment.FactorGuideline} {
		t.Run(string(factor), func(t *testing.T) {
			p, store, snap, target, in := publicationFixture(t, factor)
			store.confirmLost = true
			_, _, err := p.SaveWinner(context.Background(), in)
			if err == nil || target.calls != 1 || store.publication.Status != "pending" {
				t.Fatalf("lost response err=%v calls=%d receipt=%+v", err, target.calls, store.publication)
			}
			store.test.PurgeFence = 1
			store.test.CommonSnapshot = nil
			store.test.Candidates = nil
			store.test.Revision = 99
			target.target = "later retained target"
			in.RequestKey = "lost-key-new-handle"
			in.ExpectedRevision = 0
			receipt, _, err := p.SaveWinner(context.Background(), in)
			if err != nil || receipt.RequestKey != "original-key" || receipt.Status != "confirmed" || target.calls != 1 || snap.calls != 1 {
				t.Fatalf("recovery receipt=%+v err=%v calls=%d decode=%d", receipt, err, target.calls, snap.calls)
			}
			_, _, err = p.SaveWinner(context.Background(), in)
			if err != nil || target.calls != 1 || snap.calls != 1 {
				t.Fatalf("confirmed replay mutated %v", err)
			}
		})
	}
}
func TestUncommittedPurgedPublicationCannotRestorePayloadOrCreateTarget(t *testing.T) {
	p, store, snap, target, in := publicationFixture(t, experiment.FactorTemplate)
	store.publication = experiment.TestPublication{ID: "pending", UserID: "alice", TestID: "test", WinnerID: "winner", Action: "save_setting", RequestKey: "original-key", Fingerprint: "frozen-choice", Status: "pending"}
	store.test.PurgeFence = 2
	store.test.CommonSnapshot = nil
	_, _, err := p.SaveWinner(context.Background(), in)
	if !errors.Is(err, experiment.ErrTestStateInvalid) || target.calls != 0 || snap.calls != 0 || store.confirmationCalls != 0 {
		t.Fatalf("purge recovery err=%v target=%d decoded=%d", err, target.calls, snap.calls)
	}
}
func TestWinnerPublicationPassesOnlyExactFrozenDomainPayloadAndExplicitChoices(t *testing.T) {
	for _, factor := range []experiment.TestFactor{experiment.FactorVoice, experiment.FactorTemplate, experiment.FactorGuideline} {
		t.Run(string(factor), func(t *testing.T) {
			p, _, snap, target, in := publicationFixture(t, factor)
			in.MakeDefault = factor == experiment.FactorVoice
			if factor == experiment.FactorGuideline {
				in.Scope = "fields"
				in.ScopeIDs = []string{"food"}
			}
			_, _, err := p.SaveWinner(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			switch factor {
			case experiment.FactorVoice:
				if target.voice.Analysis.SyntheticSample != "frozen fictional example" || !target.voice.MakeDefault || target.voice.AcceptedRevision != "accepted" || target.voice.SourceVoiceID != "owned-setting" {
					t.Fatalf("voice=%+v", target.voice)
				}
			case experiment.FactorTemplate:
				if string(target.template.FrozenContent) != string(snap.variant.Payload) || target.template.Name != in.Name {
					t.Fatalf("template=%+v", target.template)
				}
			case experiment.FactorGuideline:
				if string(target.guideline.FrozenContent) != string(snap.variant.Payload) || target.guideline.Scope != in.Scope || !reflect.DeepEqual(target.guideline.ScopeIDs, in.ScopeIDs) {
					t.Fatalf("guideline=%+v", target.guideline)
				}
			}
		})
	}
}
func TestModelAdoptionUsesOnlyItsFrozenStageAndDomainRechecksLiveEligibility(t *testing.T) {
	for _, stage := range []experiment.Stage{experiment.StageObserve, experiment.StageWrite} {
		t.Run(string(stage), func(t *testing.T) {
			p, store, snap, target, in := publicationFixture(t, experiment.FactorModel)
			store.test.ModelStage = stage
			snap.description.ModelStage = string(stage)
			target.failure = provider.ErrTestPublicationConflict
			_, _, err := p.SaveWinner(context.Background(), in)
			if !errors.Is(err, experiment.ErrTestPublicationConflict) {
				t.Fatalf("live gate error=%v", err)
			}
			want := "winner-model"
			if stage == experiment.StageObserve {
				want = "winner-observer"
			}
			if target.model.Stage != provider.Stage(stage) || target.model.Ref.ModelID != want || store.publication.Status != "pending" {
				t.Fatalf("adoption=%+v receipt=%+v", target.model, store.publication)
			}
		})
	}
}
func TestSourceApplicationUsesFrozenCompleteWinnerAndSameMachineBaseline(t *testing.T) {
	p, store, _, target, in := publicationFixture(t, experiment.FactorModel)
	request := experiment.OutputApplication{TestMutation: in.TestMutation, WinnerID: in.WinnerID, InputRevision: 6, ContentRevision: 7, Output: []byte("client replacement forbidden"), PostSlug: "foreign", AssignmentsHash: "client fake"}
	_, _, err := p.ApplyOutput(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	applied := target.post
	if applied.PostSlug != "source" || applied.AssignmentsHash != "assignments" || applied.Content.Title != "paid winner" || !reflect.DeepEqual(applied.Content, applied.Baseline) || applied.ContentLanguage != post.LanguageEnglish || applied.Storyline.Paragraphs[0].Text != "complete storyline" || !reflect.DeepEqual(applied.Nouns, []string{"winner"}) {
		t.Fatalf("source application=%+v", applied)
	}
	store.test.PurgeFence = 1
	store.test.CommonSnapshot = nil
	request.RequestKey = "another"
	request.ExpectedRevision = 0
	_, _, err = p.ApplyOutput(context.Background(), request)
	if err != nil || target.calls != 1 {
		t.Fatalf("apply replay=%v calls=%d", err, target.calls)
	}
}
func TestSourceApplicationRefusesOtherFactorsOrDifferentFrozenRevisionsBeforeTargetMutation(t *testing.T) {
	for _, factor := range []experiment.TestFactor{experiment.FactorModel, experiment.FactorVoice, experiment.FactorTemplate, experiment.FactorGuideline} {
		for _, change := range []string{"input", "content"} {
			t.Run(fmt.Sprintf("%s/%s", factor, change), func(t *testing.T) {
				p, _, _, target, in := publicationFixture(t, factor)
				request := experiment.OutputApplication{TestMutation: in.TestMutation, WinnerID: in.WinnerID, InputRevision: 6, ContentRevision: 7}
				if change == "input" {
					request.InputRevision++
				} else {
					request.ContentRevision++
				}
				_, _, err := p.ApplyOutput(context.Background(), request)
				if !errors.Is(err, experiment.ErrTestOutputIncompatible) || target.calls != 0 {
					t.Fatalf("incompatible applied err=%v calls=%d", err, target.calls)
				}
			})
		}
	}
}

func (f templatePublicationFake) ValidateTestPublicationChoices(context.Context, template.TestedPublication) error {
	f.h.validations++
	return f.h.validationFailure
}

func (f guidelinePublicationFake) ValidateTestPublicationChoices(context.Context, guideline.TestedPublication) error {
	f.h.validations++
	return f.h.validationFailure
}

func (f voicePublicationFake) ValidateTestPublicationChoices(context.Context, voice.TestStylePublication) error {
	f.h.validations++
	return f.h.validationFailure
}

func TestFixableInitialWinnerChoicesAreRefusedBeforeImmutableIntentAndCanBeCorrected(t *testing.T) {
	for _, factor := range []experiment.TestFactor{experiment.FactorModel, experiment.FactorVoice, experiment.FactorTemplate, experiment.FactorGuideline} {
		t.Run(string(factor), func(t *testing.T) {
			p, store, _, target, in := publicationFixture(t, factor)
			switch factor {
			case experiment.FactorModel:
				in.MakeDefault = true
			case experiment.FactorVoice:
				in.Name = ""
				target.validationFailure = &voice.VoiceNameError{Chars: 0}
			case experiment.FactorTemplate:
				in.Name = ""
				target.validationFailure = template.ErrNameRequired
			case experiment.FactorGuideline:
				in.Scope = ""
				target.validationFailure = guideline.ErrScopeShape
			}
			if _, _, err := p.SaveWinner(context.Background(), in); err == nil || store.publication.ID != "" || target.calls != 0 {
				t.Fatalf("bad choice created intent=%+v calls=%d err=%v", store.publication, target.calls, err)
			}
			in.MakeDefault = false
			if factor != experiment.FactorModel {
				in.Name = "Corrected name"
			}
			if factor == experiment.FactorGuideline {
				in.Scope = "global"
			}
			target.validationFailure = nil
			receipt, _, err := p.SaveWinner(context.Background(), in)
			if err != nil || receipt.Status != "confirmed" || target.calls != 1 {
				t.Fatalf("corrected choice failed=%+v err=%v", receipt, err)
			}
			validations := target.validations
			target.validationFailure = errors.New("later unavailable live source")
			store.test.PurgeFence = 1
			store.test.CommonSnapshot = nil
			in.ExpectedRevision = 0
			if replay, _, err := p.SaveWinner(context.Background(), in); err != nil || replay != receipt || target.calls != 1 || target.validations != validations {
				t.Fatalf("confirmed replay revalidated live choices=%+v err=%v", replay, err)
			}
		})
	}
}
func TestInvalidSnapshotIndicesAreRefusedWithoutDecodingOrTargetMutation(t *testing.T) {
	for _, indices := range [][]int{{0, 0}, {0, 2}, {-1, 1}} {
		t.Run(fmt.Sprint(indices), func(t *testing.T) {
			found := experiment.WritingTest{Count: 2, Status: experiment.TestCompleted, WinnerID: "winner", Candidates: []experiment.TestCandidate{{ID: "winner", SnapshotIndex: indices[0], Status: "succeeded"}, {ID: "other", SnapshotIndex: indices[1], Status: "succeeded"}}}
			if _, _, err := publicationDescription(found, "winner"); !errors.Is(err, experiment.ErrTestPublicationConflict) {
				t.Fatalf("invalid index accepted=%v", err)
			}
		})
	}
}

// Package app coordinates tested champion publication through target-owned ports.
package app

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

type PublicationStore interface {
	GetTest(context.Context, string, string) (experiment.WritingTest, error)
	BeginPublication(context.Context, experiment.WinnerPublication) (experiment.TestPublication, error)
	ConfirmPublication(context.Context, experiment.TestPublication, experiment.PublicationReceipt) (experiment.TestPublication, error)
}
type ModelPublications interface {
	AdoptTestModel(context.Context, provider.TestModelAdoption) (provider.TestModelReceipt, error)
	ReadTestPublicationReceipt(context.Context, string, string, string) (provider.TestModelReceipt, bool, error)
}
type TemplatePublications interface {
	ValidateTestPublicationChoices(context.Context, template.TestedPublication) error
	PublishTestWinner(context.Context, template.TestedPublication) (template.TestedPublicationReceipt, error)
	ReadTestPublicationReceipt(context.Context, string, string, string, string) (template.TestedPublicationReceipt, bool, error)
}
type GuidelinePublications interface {
	ValidateTestPublicationChoices(context.Context, guideline.TestedPublication) error
	PublishTestWinner(context.Context, guideline.TestedPublication) (guideline.TestedPublicationReceipt, error)
	ReadTestPublicationReceipt(context.Context, string, string, string, string) (guideline.TestedPublicationReceipt, bool, error)
}
type VoicePublications interface {
	ValidateTestPublicationChoices(context.Context, voice.TestStylePublication) error
	PublishTestWinner(context.Context, voice.TestStylePublication) (voice.TestStyleReceipt, error)
	ReadTestPublicationReceipt(context.Context, string, string, string, string) (voice.TestStyleReceipt, bool, error)
}
type PostPublications interface {
	ApplyTestResult(context.Context, post.TestOutputPublication) (post.TestOutputReceipt, error)
	ReadTestPublicationReceipt(context.Context, string, string, string) (post.TestOutputReceipt, bool, error)
}

// The snapshot owner decodes private publication metadata. It makes no live
// lookup or provider call and is never a customer inspection projection.
type PublicationSnapshots interface {
	DescribePublication(experiment.WritingTest, string) (generation.WritingTestDescription, generation.WritingTestVariantDescription, error)
}
type GenerationPublicationSnapshots struct{}

func (GenerationPublicationSnapshots) DescribePublication(t experiment.WritingTest, winner string) (generation.WritingTestDescription, generation.WritingTestVariantDescription, error) {
	return publicationDescription(t, winner)
}

type PublicationDependencies struct {
	Store      PublicationStore
	Snapshots  PublicationSnapshots
	Models     ModelPublications
	Templates  TemplatePublications
	Guidelines GuidelinePublications
	Voices     VoicePublications
	Posts      PostPublications
}
type Publications struct{ dependencies PublicationDependencies }

func NewPublications(in PublicationDependencies) *Publications {
	if in.Store == nil || in.Snapshots == nil || in.Models == nil || in.Templates == nil || in.Guidelines == nil || in.Voices == nil || in.Posts == nil {
		panic("experiment: all target publication and receipt ports are required")
	}
	return &Publications{dependencies: in}
}

func (p *Publications) SaveWinner(ctx context.Context, in experiment.WinnerPublication) (experiment.TestPublication, experiment.WritingTest, error) {
	action := experiment.TestPublicationAction(in.Action)
	if action != experiment.TestAdoptModel && action != experiment.TestSaveSetting && action != experiment.TestUseSetting {
		return experiment.TestPublication{}, experiment.WritingTest{}, experiment.ErrTestOperation
	}
	return p.publish(ctx, in, nil)
}
func (p *Publications) ApplyOutput(ctx context.Context, in experiment.OutputApplication) (experiment.TestPublication, experiment.WritingTest, error) {
	request := experiment.WinnerPublication{TestMutation: in.TestMutation, WinnerID: in.WinnerID, Action: string(experiment.TestApplyOutput), InputRevision: in.InputRevision, ContentRevision: in.ContentRevision}
	return p.publish(ctx, request, &in)
}
func (p *Publications) publish(ctx context.Context, in experiment.WinnerPublication, output *experiment.OutputApplication) (experiment.TestPublication, experiment.WritingTest, error) {
	var empty experiment.WritingTest
	initial, err := p.dependencies.Store.GetTest(ctx, in.UserID, in.TestID)
	if err != nil {
		return experiment.TestPublication{}, empty, err
	}
	existing := false
	for _, prior := range initial.Publications {
		if prior.WinnerID == in.WinnerID && prior.Action == in.Action {
			existing = true
			break
		}
	}
	if !existing {
		if err := p.preflight(ctx, initial, in, output); err != nil {
			return experiment.TestPublication{}, initial, targetPublicationError(err)
		}
	}
	// First persist the precise action. A recovery always uses its original key.
	publication, err := p.dependencies.Store.BeginPublication(ctx, in)
	if err != nil {
		return publication, empty, err
	}
	found, err := p.dependencies.Store.GetTest(ctx, in.UserID, in.TestID)
	if err != nil {
		return publication, empty, err
	}
	if publication.Status == string(experiment.TestPublicationConfirmed) {
		return publication, found, nil
	}
	in.RequestKey = publication.RequestKey
	receipt, committed, err := p.readReceipt(ctx, found, publication)
	if err != nil {
		return publication, found, err
	}
	if !committed {
		if found.PurgeFence != 0 || len(found.CommonSnapshot) == 0 {
			return publication, found, experiment.ErrTestStateInvalid
		}
		description, winner, err := p.dependencies.Snapshots.DescribePublication(found, in.WinnerID)
		if err != nil {
			return publication, found, err
		}
		if description.UserID != in.UserID || description.Factor != string(found.Factor) || description.ModelStage != string(found.ModelStage) {
			return publication, found, experiment.ErrTestPublicationConflict
		}
		receipt, err = p.commitTarget(ctx, found, publication, in, output, description, winner)
		if err != nil {
			return publication, found, targetPublicationError(err)
		}
	}
	publication, err = p.dependencies.Store.ConfirmPublication(ctx, publication, receipt)
	if err != nil {
		return publication, found, err
	}
	found, err = p.dependencies.Store.GetTest(ctx, in.UserID, in.TestID)
	return publication, found, err
}
func (p *Publications) preflight(ctx context.Context, t experiment.WritingTest, in experiment.WinnerPublication, output *experiment.OutputApplication) error {
	switch experiment.TestPublicationAction(in.Action) {
	case experiment.TestAdoptModel:
		if t.Factor != experiment.FactorModel || in.MakeDefault || in.Name != "" || in.Scope != "" || len(in.ScopeIDs) != 0 {
			return experiment.ErrTestOperation
		}
		return nil
	case experiment.TestApplyOutput:
		if output == nil || t.Factor != experiment.FactorModel || t.SourcePostSlug == "" {
			return experiment.ErrTestOutputIncompatible
		}
		d, _, err := p.dependencies.Snapshots.DescribePublication(t, in.WinnerID)
		if err != nil {
			return err
		}
		if d.UserID != in.UserID || d.SourcePostSlug != t.SourcePostSlug || d.InputRevision != output.InputRevision || d.ContentRevision != output.ContentRevision {
			return experiment.ErrTestOutputIncompatible
		}
		return nil
	case experiment.TestSaveSetting, experiment.TestUseSetting:
		switch t.Factor {
		case experiment.FactorTemplate:
			return p.dependencies.Templates.ValidateTestPublicationChoices(ctx, template.TestedPublication{UserID: in.UserID, Action: in.Action, Name: in.Name, Scope: in.Scope, ScopeIDs: in.ScopeIDs, MakeDefault: in.MakeDefault})
		case experiment.FactorGuideline:
			return p.dependencies.Guidelines.ValidateTestPublicationChoices(ctx, guideline.TestedPublication{UserID: in.UserID, Action: in.Action, Name: in.Name, Scope: in.Scope, ScopeIDs: in.ScopeIDs, MakeDefault: in.MakeDefault})
		case experiment.FactorVoice:
			if in.Scope != "" || len(in.ScopeIDs) != 0 {
				return experiment.ErrTestOperation
			}
			return p.dependencies.Voices.ValidateTestPublicationChoices(ctx, voice.TestStylePublication{UserID: in.UserID, Action: in.Action, Name: in.Name, MakeDefault: in.MakeDefault})
		}
		return experiment.ErrTestOperation
	}
	return experiment.ErrTestOperation
}
func publicationDescription(found experiment.WritingTest, winnerID string) (generation.WritingTestDescription, generation.WritingTestVariantDescription, error) {
	contents := make([][]byte, len(found.Candidates))
	seen := make([]bool, len(found.Candidates))
	index := -1
	var winner experiment.TestCandidate
	for _, c := range found.Candidates {
		if c.SnapshotIndex < 0 || c.SnapshotIndex >= len(contents) || seen[c.SnapshotIndex] {
			return generation.WritingTestDescription{}, generation.WritingTestVariantDescription{}, experiment.ErrTestPublicationConflict
		}
		seen[c.SnapshotIndex] = true
		contents[c.SnapshotIndex] = c.FrozenVariant
		if c.ID == winnerID {
			index = c.SnapshotIndex
			winner = c
		}
	}
	if len(contents) != found.Count || found.Status != experiment.TestCompleted || found.WinnerID != winnerID || index < 0 || winner.Status != string(experiment.TestCandidateSucceeded) {
		return generation.WritingTestDescription{}, generation.WritingTestVariantDescription{}, experiment.ErrTestStateInvalid
	}
	snapshot, err := generation.RestoreWritingTestSnapshot(found.CommonSnapshot, contents, found.CommonHash, found.PromptVersion)
	if err != nil {
		return generation.WritingTestDescription{}, generation.WritingTestVariantDescription{}, experiment.ErrTestPublicationConflict
	}
	description, err := generation.DescribeWritingTestSnapshot(snapshot)
	if err != nil || index >= len(description.Variants) {
		return description, generation.WritingTestVariantDescription{}, experiment.ErrTestPublicationConflict
	}
	return description, description.Variants[index], nil
}
func (p *Publications) readReceipt(ctx context.Context, t experiment.WritingTest, pub experiment.TestPublication) (experiment.PublicationReceipt, bool, error) {
	result := experiment.PublicationReceipt{UserID: pub.UserID, TestID: pub.TestID, WinnerID: pub.WinnerID, Action: pub.Action, RequestKey: pub.RequestKey}
	switch experiment.TestPublicationAction(pub.Action) {
	case experiment.TestAdoptModel:
		r, found, err := p.dependencies.Models.ReadTestPublicationReceipt(ctx, pub.UserID, pub.TestID, pub.WinnerID)
		result.RequestKey = r.RequestKey
		result.TargetID = string(r.Stage) + ":" + r.Ref.String()
		return result, found, err
	case experiment.TestApplyOutput:
		r, found, err := p.dependencies.Posts.ReadTestPublicationReceipt(ctx, pub.UserID, pub.TestID, pub.WinnerID)
		result.RequestKey = r.RequestKey
		result.TargetID = r.TargetID
		result.ResultingRevision = r.ResultingRevision
		return result, found, err
	case experiment.TestSaveSetting, experiment.TestUseSetting:
		switch t.Factor {
		case experiment.FactorVoice:
			r, found, err := p.dependencies.Voices.ReadTestPublicationReceipt(ctx, pub.UserID, pub.TestID, pub.WinnerID, pub.Action)
			result.RequestKey = r.RequestKey
			result.TargetID = r.VoiceID
			return result, found, err
		case experiment.FactorTemplate:
			r, found, err := p.dependencies.Templates.ReadTestPublicationReceipt(ctx, pub.UserID, pub.TestID, pub.WinnerID, pub.Action)
			result.RequestKey = r.RequestKey
			result.TargetID = r.TargetID
			return result, found, err
		case experiment.FactorGuideline:
			r, found, err := p.dependencies.Guidelines.ReadTestPublicationReceipt(ctx, pub.UserID, pub.TestID, pub.WinnerID, pub.Action)
			result.RequestKey = r.RequestKey
			result.TargetID = r.TargetID
			return result, found, err
		}
	}
	return result, false, experiment.ErrTestOperation
}
func (p *Publications) commitTarget(ctx context.Context, t experiment.WritingTest, pub experiment.TestPublication, in experiment.WinnerPublication, application *experiment.OutputApplication, d generation.WritingTestDescription, v generation.WritingTestVariantDescription) (experiment.PublicationReceipt, error) {
	result := experiment.PublicationReceipt{UserID: pub.UserID, TestID: pub.TestID, WinnerID: pub.WinnerID, Action: pub.Action, RequestKey: pub.RequestKey}
	switch experiment.TestPublicationAction(pub.Action) {
	case experiment.TestAdoptModel:
		if t.Factor != experiment.FactorModel || in.MakeDefault || in.Name != "" || in.Scope != "" || len(in.ScopeIDs) != 0 {
			return result, experiment.ErrTestOperation
		}
		ref := v.WriteModel
		if t.ModelStage == experiment.StageObserve {
			ref = v.ObserveModel
		}
		r, err := p.dependencies.Models.AdoptTestModel(ctx, provider.TestModelAdoption{UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID, RequestKey: pub.RequestKey, Fingerprint: pub.Fingerprint, Stage: provider.Stage(t.ModelStage), Ref: ref})
		result.TargetID = string(r.Stage) + ":" + r.Ref.String()
		result.RequestKey = r.RequestKey
		return result, err
	case experiment.TestApplyOutput:
		if application == nil || t.Factor != experiment.FactorModel || t.SourcePostSlug == "" || d.SourcePostSlug != t.SourcePostSlug || application.InputRevision != d.InputRevision || application.ContentRevision != d.ContentRevision {
			return result, experiment.ErrTestOutputIncompatible
		}
		var raw []byte
		for _, candidate := range t.Candidates {
			if candidate.ID == in.WinnerID {
				raw = candidate.Output
			}
		}
		output, err := experiment.DecodeTestOutput(raw)
		if err != nil {
			return result, err
		}
		if output.ContentLanguage != d.TargetLanguage.String() {
			return result, experiment.ErrTestOutputIncompatible
		}
		content, story := postOutput(output)
		r, err := p.dependencies.Posts.ApplyTestResult(ctx, post.TestOutputPublication{UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID, RequestKey: pub.RequestKey, PostSlug: d.SourcePostSlug, AssignmentsHash: d.AssignmentsHash, InputRevision: d.InputRevision, ContentRevision: d.ContentRevision, Content: content, Baseline: content, Origins: output.Origins, ContentLanguage: post.Language(output.ContentLanguage), Storyline: story, Nouns: output.Nouns, PrivatePayloadFence: &t.PurgeFence})
		result.TargetID = r.TargetID
		result.RequestKey = r.RequestKey
		result.ResultingRevision = r.ResultingRevision
		return result, err
	case experiment.TestSaveSetting, experiment.TestUseSetting:
		switch t.Factor {
		case experiment.FactorTemplate:
			r, err := p.dependencies.Templates.PublishTestWinner(ctx, template.TestedPublication{UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID, Action: pub.Action, RequestKey: pub.RequestKey, Fingerprint: pub.Fingerprint, Name: in.Name, Scope: in.Scope, ScopeIDs: in.ScopeIDs, MakeDefault: in.MakeDefault, FrozenContent: v.Payload})
			result.TargetID = r.TargetID
			result.RequestKey = r.RequestKey
			return result, err
		case experiment.FactorGuideline:
			r, err := p.dependencies.Guidelines.PublishTestWinner(ctx, guideline.TestedPublication{UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID, Action: pub.Action, RequestKey: pub.RequestKey, Fingerprint: pub.Fingerprint, Name: in.Name, Scope: in.Scope, ScopeIDs: in.ScopeIDs, MakeDefault: in.MakeDefault, FrozenContent: v.Payload})
			result.TargetID = r.TargetID
			result.RequestKey = r.RequestKey
			return result, err
		case experiment.FactorVoice:
			var frozen voice.FrozenWritingStyle
			if err := json.Unmarshal(v.Payload, &frozen); err != nil {
				return result, experiment.ErrTestPublicationConflict
			}
			source := v.Reference.SettingID
			r, err := p.dependencies.Voices.PublishTestWinner(ctx, voice.TestStylePublication{UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID, Action: pub.Action, RequestKey: pub.RequestKey, Fingerprint: pub.Fingerprint, SourceVoiceID: source, AcceptedRevision: frozen.Revision, Name: in.Name, MakeDefault: in.MakeDefault, Analysis: frozen.Analysis})
			result.TargetID = r.VoiceID
			result.RequestKey = r.RequestKey
			return result, err
		}
	}
	return result, experiment.ErrTestOperation
}
func postOutput(value experiment.TestOutput) (post.PostContent, *post.Storyline) {
	content := post.PostContent{Title: value.Content.Title, Summary: value.Content.Summary, Tags: value.Content.Tags}
	for _, b := range value.Content.Blocks {
		content.Blocks = append(content.Blocks, post.Block{Type: post.BlockType(b.Type), Content: b.Content, Level: b.Level, File: b.File, Alt: b.Alt, Caption: b.Caption, Items: b.Items, Files: b.Files, Layout: post.GalleryLayout(b.Layout)})
	}
	var story *post.Storyline
	if value.Storyline != nil {
		story = &post.Storyline{MadeWith: value.Storyline.MadeWith, Origins: value.Storyline.Origins}
		for _, item := range value.Storyline.Paragraphs {
			story.Paragraphs = append(story.Paragraphs, post.StorylineParagraph{Text: item.Text, Files: item.Files})
		}
	}
	return content, story
}
func targetPublicationError(err error) error {
	if errors.Is(err, provider.ErrTestPublicationConflict) || errors.Is(err, template.ErrTestPublicationConflict) || errors.Is(err, guideline.ErrTestPublicationConflict) || errors.Is(err, voice.ErrTestStylePublicationConflict) || errors.Is(err, post.ErrTestPublicationConflict) {
		return experiment.ErrTestPublicationConflict
	}
	return err
}

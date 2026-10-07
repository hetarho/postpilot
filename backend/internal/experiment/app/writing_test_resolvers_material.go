package app

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/usage"
)

func (r *WritingTestResolvers) PrepareWritingTestModel(ctx context.Context, user, stage string, ref llm.ModelRef) (generation.WritingTestModel, error) {
	if stage != llm.StageNameObserve && stage != llm.StageNameWrite {
		return generation.WritingTestModel{}, experiment.ErrTestFactor
	}
	info, err := r.dependencies.Models.PrepareTestModel(ctx, user, provider.Stage(stage), ref)
	if err != nil {
		return generation.WritingTestModel{}, err
	}
	return generation.WritingTestModel{Info: info, PromptTokens: int(usage.HoldPromptTokenBound(0))}, nil
}
func (r *WritingTestResolvers) ResolveWritingTestSource(ctx context.Context, user, slug string, input, content int64, m generation.WritingTestMaterialRequest) (result generation.WritingTestSource, err error) {
	defer func() { err = WritingTestPreparationError(err) }()
	answers := make([]post.TemplateAnswer, len(m.TemplateAnswers))
	for i, a := range m.TemplateAnswers {
		answers[i] = post.TemplateAnswer{Label: a.Label, Text: a.Text, Enabled: a.Enabled}
	}
	if err := r.dependencies.Posts.ValidateWritingTestMaterial(post.WritingTestMaterial{Language: post.Language(m.TargetLanguage), TargetLength: m.TargetLength, TagCount: m.TagCount, Answers: answers, QualityRules: m.QualityRules}); err != nil {
		return generation.WritingTestSource{}, err
	}
	if user == "" || input < 0 || content < 0 {
		return generation.WritingTestSource{}, experiment.ErrTestMaterialInvalid
	}
	value := generation.WritingTestSource{Post: generation.PostInput{UserID: user, Slug: slug, Memo: m.Material, TargetLanguage: m.TargetLanguage, TargetLength: m.TargetLength, TagCount: m.TagCount, TemplateID: m.TemplateID, TemplateAnswers: slices.Clone(m.TemplateAnswers), UseMemory: m.UseMemory, QualityRuleIDs: slices.Clone(m.QualityRules)}}
	var attachments []post.WritingTestAttachment
	if slug == "" {
		if input != 0 || content != 0 {
			return value, experiment.ErrTestMaterialInvalid
		}
		found, err := r.dependencies.Posts.WritingTestAttachments(ctx, user, m.AttachmentIDs)
		if err != nil {
			return value, materialSourceError(err)
		}
		attachments = found
		value.Revision = "explicit-owned-material-v1"
	} else {
		source, err := r.dependencies.Posts.AttachedImages(ctx, user, slug)
		if err != nil {
			return value, materialSourceError(err)
		}
		if source.InputRevision != input || source.ContentRevision != content {
			return value, experiment.ErrTestRevisionConflict
		}
		value.Post.Title, value.Post.Field = source.Title, source.Field
		value.InputRevision, value.ContentRevision = input, content
		value.Revision = fmt.Sprintf("post:%s:%d:%d", slug, input, content)
		// The target's input revision fences source material. This fingerprint binds
		// the requested common writing choices, including deliberate test overrides.
		effective := source
		effective.VoiceID = m.VoiceID
		effective.TemplateID = m.TemplateID
		effective.TargetLanguage = post.Language(m.TargetLanguage)
		effective.TargetLength = m.TargetLength
		effective.TagCount = m.TagCount
		effective.UseMemory = m.UseMemory
		effective.QualityRules = slices.Clone(m.QualityRules)
		value.AssignmentsHash = post.TestAssignmentsHash(effective)
		allowed := map[string]post.WritingTestAttachment{}
		for _, p := range source.Images {
			allowed[p.ID] = post.WritingTestAttachment{ID: p.ID, Filename: p.Filename, Key: p.Key, Kind: post.AttachmentPhoto, ContentType: "image/jpeg", Bytes: p.Bytes, Width: int(p.Width), Height: int(p.Height), Rotation: int(p.Rotation), RotationByOwner: p.RotationByOwner}
		}
		for _, v := range source.Videos {
			allowed[v.ID] = post.WritingTestAttachment{ID: v.ID, Filename: v.Filename, Key: v.Key, Kind: post.AttachmentVideo, ContentType: v.ContentType, Bytes: v.Bytes, DurationMs: v.DurationMs, Width: int(v.Width), Height: int(v.Height)}
		}
		seen := map[string]bool{}
		for _, id := range m.AttachmentIDs {
			attachment, ok := allowed[id]
			if !ok || seen[id] {
				return value, experiment.ErrTestMaterialInvalid
			}
			seen[id] = true
			attachment.Fingerprint = post.WritingTestAttachmentFingerprint(attachment)
			attachments = append(attachments, attachment)
		}
		// Reuse only observation rows for exactly the selected source attachments.
		names := map[string]bool{}
		for _, a := range attachments {
			names[a.Filename] = true
		}
		for _, o := range source.Observations {
			if names[o.File] {
				value.Post.Observations = append(value.Post.Observations, generation.Observation{File: o.File, Scene: o.Scene, Mood: o.Mood, VisibleText: o.VisibleText, Objects: slices.Clone(o.Objects), PeoplePresent: o.PeoplePresent, Model: o.Model, Events: slices.Clone(o.Events), Speech: o.Speech, Rotation: o.Rotation})
			}
		}
	}
	names := map[string]bool{}
	for _, a := range attachments {
		if a.Filename == "" || a.Key == "" || a.Fingerprint == "" || names[a.Filename] {
			return value, experiment.ErrTestMaterialInvalid
		}
		names[a.Filename] = true
		kind := generation.AttachmentPhoto
		if a.Kind == post.AttachmentVideo {
			kind = generation.AttachmentVideo
		}
		image := generation.Image{Filename: a.Filename, Key: a.Key, Kind: kind, ContentType: a.ContentType, DurationMs: a.DurationMs, Width: int32(a.Width), Height: int32(a.Height), Rotation: int32(a.Rotation), RotationByOwner: a.RotationByOwner}
		value.Post.Images = append(value.Post.Images, image)
		value.Attachments = append(value.Attachments, generation.WritingTestAttachment{ID: a.ID, Fingerprint: a.Fingerprint, Image: image})
	}
	return value, nil
}
func materialSourceError(err error) error {
	if errors.Is(err, post.ErrNotFound) || errors.Is(err, post.ErrForbidden) {
		return experiment.ErrTestNotFound
	}
	return err
}

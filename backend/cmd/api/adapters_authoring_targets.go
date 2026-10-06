package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

// Each publisher validates and commits only its domain's entity and durable receipt.
// This adapter maps the common authoring DTO; it opens no SQL transaction and calls no AI.
type authoringTargets struct {
	postTemplates  *template.Authoring
	videoTemplates *clipapp.Authoring
	guidelines     *guideline.Authoring
	voices         *voice.Authoring
}

func (a authoringTargets) Seed(ctx context.Context, user string, kind authoring.Kind, id string) (authoring.Seed, error) {
	if !kind.Valid() {
		return authoring.Seed{}, authoring.ErrInvalidKind
	}
	if id == "" {
		return authoring.Seed{}, nil
	}
	var out authoring.Seed
	var err error
	switch kind {
	case authoring.PostTemplate:
		var value template.Draft
		value, out.TargetVersion, err = a.postTemplates.Seed(ctx, user, id)
		out.Artifact = &authoring.Artifact{ID: id, Name: value.Name, Description: value.Description, Body: value.Body, TitleArea: value.TitleArea}
	case authoring.VideoTemplate:
		var value clip.Recipe
		value, out.TargetVersion, err = a.videoTemplates.Seed(ctx, user, id)
		out.Artifact = &authoring.Artifact{ID: id, Name: value.Name, Body: value.CompositionBody}
	case authoring.PostGuideline, authoring.VideoGuideline:
		var value guideline.AuthoringDraft
		value, out.TargetVersion, err = a.guidelines.Seed(ctx, user, guidelineKind(kind), id)
		out.Artifact = &authoring.Artifact{ID: id, Name: value.Name, Body: value.Body}
	case authoring.WritingVoice:
		var value voice.WritingStyleDraft
		value, out.TargetVersion, out.SourceContext, err = a.voices.Seed(ctx, user, id)
		out.Artifact = &authoring.Artifact{ID: id, Name: value.Name, Description: value.Description, Body: value.Sample}
		out.ForkVoice = true
	}
	if err != nil {
		return authoring.Seed{}, authoringTargetError(err)
	}
	return out, nil
}
func (a authoringTargets) CanStart(ctx context.Context, user string, kind authoring.Kind, id string) error {
	var err error
	switch kind {
	case authoring.PostTemplate:
		err = a.postTemplates.CanStart(ctx, user, id)
	case authoring.VideoTemplate:
		err = a.videoTemplates.CanStart(ctx, user, id)
	case authoring.PostGuideline, authoring.VideoGuideline:
		err = a.guidelines.CanStart(ctx, user, guidelineKind(kind), id)
	case authoring.WritingVoice:
		err = a.voices.CanStart(ctx, user, id)
	default:
		return authoring.ErrInvalidKind
	}
	return authoringTargetError(err)
}
func (a authoringTargets) Validate(kind authoring.Kind, value authoring.Artifact) error {
	if !kind.Valid() {
		return authoring.ErrInvalidKind
	}
	if utf8.RuneCountInString(value.Description) > 200 || kind != authoring.PostTemplate && value.TitleArea != "" {
		return authoring.ErrOutput
	}
	var err error
	switch kind {
	case authoring.PostTemplate:
		_, err = a.postTemplates.Validate(template.Draft{Name: value.Name, Description: value.Description, Body: value.Body, TitleArea: value.TitleArea})
	case authoring.VideoTemplate:
		_, err = a.videoTemplates.Validate(clip.Recipe{Name: value.Name, CompositionBody: value.Body})
	case authoring.PostGuideline, authoring.VideoGuideline:
		_, err = a.guidelines.Validate(guideline.AuthoringDraft{Name: value.Name, Body: value.Body})
	case authoring.WritingVoice:
		_, err = a.voices.Validate(voice.WritingStyleDraft{Name: value.Name, Description: value.Description, Sample: value.Body})
	}
	return err
}
func (a authoringTargets) Guide(kind authoring.Kind) string {
	switch kind {
	case authoring.PostTemplate:
		return a.postTemplates.Guide()
	case authoring.VideoTemplate:
		return a.videoTemplates.Guide()
	case authoring.PostGuideline, authoring.VideoGuideline:
		return a.guidelines.Guide(guidelineKind(kind))
	case authoring.WritingVoice:
		return a.voices.Guide()
	default:
		return ""
	}
}
func (a authoringTargets) Publish(ctx context.Context, in authoring.Publication) (authoring.SavedRef, error) {
	var id, name string
	var err error
	switch in.Kind {
	case authoring.PostTemplate:
		var value template.Template
		value, err = a.postTemplates.Publish(ctx, in.UserID, template.AuthoringPublication{Key: template.AuthoringKey{Key: in.Key, SessionID: in.SessionID, Revision: in.Revision}, TargetID: in.TargetID, TargetVersion: in.TargetVersion, Draft: template.Draft{Name: in.Artifact.Name, Description: in.Artifact.Description, Body: in.Artifact.Body, TitleArea: in.Artifact.TitleArea}})
		id, name = value.ID, value.Name
	case authoring.VideoTemplate:
		var value clip.VideoTemplate
		value, err = a.videoTemplates.Publish(ctx, in.UserID, clip.AuthoringPublication{Key: clip.AuthoringKey{Key: in.Key, SessionID: in.SessionID, Revision: in.Revision}, TargetID: in.TargetID, TargetVersion: in.TargetVersion, Recipe: clip.Recipe{Name: in.Artifact.Name, CompositionBody: in.Artifact.Body}})
		id, name = value.ID, value.Name
	case authoring.PostGuideline, authoring.VideoGuideline:
		var value guideline.Guideline
		value, err = a.guidelines.Publish(ctx, in.UserID, guideline.AuthoringPublication{Key: guideline.AuthoringKey{Key: in.Key, SessionID: in.SessionID, Revision: in.Revision}, Kind: guidelineKind(in.Kind), TargetID: in.TargetID, TargetVersion: in.TargetVersion, Draft: guideline.AuthoringDraft{Name: in.Artifact.Name, Body: in.Artifact.Body}})
		id, name = value.ID, value.Title
	case authoring.WritingVoice:
		var value voice.Voice
		value, err = a.voices.Publish(ctx, in.UserID, voice.AuthoringPublication{Key: voice.AuthoringKey{Key: in.Key, SessionID: in.SessionID, Revision: in.Revision}, SourceID: in.TargetID, Draft: voice.WritingStyleDraft{Name: in.Artifact.Name, Description: in.Artifact.Description, Sample: in.Artifact.Body}, MakeDefault: in.MakeDefault, WriteModel: in.WriteModel})
		id, name = value.ID, value.Name
	default:
		return authoring.SavedRef{}, authoring.ErrInvalidKind
	}
	if err != nil {
		return authoring.SavedRef{}, authoringTargetError(err)
	}
	return authoring.SavedRef{Kind: in.Kind, ID: id, Name: name}, nil
}
func guidelineKind(kind authoring.Kind) guideline.Kind {
	if kind == authoring.VideoGuideline {
		return guideline.KindClip
	}
	return guideline.KindPost
}
func authoringTargetError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, template.ErrAuthoringConflict) || errors.Is(err, clip.ErrAuthoringConflict) || errors.Is(err, guideline.ErrAuthoringConflict) || errors.Is(err, voice.ErrAuthoringConflict) || errors.Is(err, voice.ErrVoiceDeleted) || errors.Is(err, voice.ErrVoiceNotMade) {
		return fmt.Errorf("%w: %w", authoring.ErrTargetConflict, err)
	}
	if errors.Is(err, template.ErrNotFound) || errors.Is(err, clip.ErrNotFound) || errors.Is(err, guideline.ErrNotFound) || errors.Is(err, voice.ErrVoiceNotFound) {
		return fmt.Errorf("%w: %w", authoring.ErrNotFound, err)
	}
	var cap *guideline.AccountCapError
	if errors.As(err, &cap) {
		return &authoringTargetFailure{reason: "GUIDELINE_LIMIT_REACHED", cause: err, params: map[string]string{"max": strconv.Itoa(cap.Max)}}
	}
	var failure *authoringTargetFailure
	if errors.As(err, &failure) {
		return err
	}
	if errors.Is(err, template.ErrTooMany) {
		return &authoringTargetFailure{reason: "TEMPLATE_LIMIT_REACHED", cause: err}
	}
	if errors.Is(err, template.ErrDuplicateName) {
		return &authoringTargetFailure{reason: "TEMPLATE_NAME_TAKEN", cause: err}
	}
	if errors.Is(err, guideline.ErrDuplicateText) {
		return &authoringTargetFailure{reason: "GUIDELINE_TEXT_TAKEN", cause: err}
	}
	if errors.Is(err, clip.ErrDuplicateName) {
		return &authoringTargetFailure{reason: "CLIP_TEMPLATE_NAME_TAKEN", cause: err}
	}
	return err
}

type authoringTargetFailure struct {
	reason string
	cause  error
	params map[string]string
}

func (e *authoringTargetFailure) Error() string             { return "configuration authoring target refused" }
func (e *authoringTargetFailure) Unwrap() error             { return e.cause }
func (e *authoringTargetFailure) Reason() string            { return e.reason }
func (e *authoringTargetFailure) Params() map[string]string { return e.params }

var _ rpcserver.AppFailure = (*authoringTargetFailure)(nil)

var _ authoring.Targets = authoringTargets{}

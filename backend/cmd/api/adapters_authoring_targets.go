package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
		if kind == authoring.VideoTemplate {
			draft := a.videoTemplates.NewDraft()
			return authoring.Seed{WorkingSource: &authoring.Artifact{Body: draft.CompositionBody}}, nil
		}
		return authoring.Seed{}, nil
	}
	var out authoring.Seed
	var err error
	switch kind {
	case authoring.PostTemplate:
		var value template.Draft
		var numbers template.Numbers
		value, numbers, out.TargetVersion, err = a.postTemplates.SeedWithNumbers(ctx, user, id)
		out.Artifact = &authoring.Artifact{ID: id, Name: value.Name, Description: value.Description, Body: value.Body, TitleArea: value.TitleArea, TargetLength: authoringNumberString(numbers.TargetLength), TagCount: authoringNumberString(numbers.TagCount)}
	case authoring.VideoTemplate:
		var value clip.Recipe
		value, out.TargetVersion, err = a.videoTemplates.Seed(ctx, user, id)
		out.Artifact = &authoring.Artifact{ID: id, Name: value.Name, Body: value.CompositionBody}
	case authoring.PostGuideline, authoring.VideoGuideline:
		var value guideline.AuthoringDraft
		var scope guideline.ScopePatch
		value, scope, out.TargetVersion, err = a.guidelines.SeedWithScope(ctx, user, guidelineKind(kind), id)
		scopeName := string(scope.Scope)
		out.Artifact = &authoring.Artifact{ID: id, Name: value.Name, Body: value.Body, Scope: &scopeName, TemplateIDs: scope.TemplateIDs, Fields: scope.Fields}
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
	if kind != authoring.PostTemplate && (value.TargetLength != nil || value.TagCount != nil || value.BuilderState != "") {
		return authoring.ErrOutput
	}
	if kind != authoring.PostGuideline && kind != authoring.VideoGuideline && (value.Scope != nil || len(value.TemplateIDs) > 0 || len(value.Fields) > 0) {
		return authoring.ErrOutput
	}
	var err error
	switch kind {
	case authoring.PostTemplate:
		numbers, e := authoringNumbers(value)
		if e != nil {
			return e
		}
		if numbers != nil {
			if e = a.postTemplates.ValidateNumbers(*numbers); e != nil {
				return e
			}
		}
		_, err = a.postTemplates.Validate(template.Draft{Name: value.Name, Description: value.Description, Body: value.Body, TitleArea: value.TitleArea})
	case authoring.VideoTemplate:
		_, err = a.videoTemplates.Validate(clip.Recipe{Name: value.Name, CompositionBody: value.Body})
	case authoring.PostGuideline, authoring.VideoGuideline:
		if _, e := a.scope(value, kind); e != nil {
			return e
		}
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
		numbers, e := authoringNumbers(in.Artifact)
		if e != nil {
			return authoring.SavedRef{}, e
		}
		var value template.Template
		value, err = a.postTemplates.Publish(ctx, in.UserID, template.AuthoringPublication{Key: template.AuthoringKey{Key: in.Key, SessionID: in.SessionID, Revision: in.Revision}, TargetID: in.TargetID, TargetVersion: in.TargetVersion, Draft: template.Draft{Name: in.Artifact.Name, Description: in.Artifact.Description, Body: in.Artifact.Body, TitleArea: in.Artifact.TitleArea}, Numbers: numbers})
		id, name = value.ID, value.Name
	case authoring.VideoTemplate:
		var value clip.VideoTemplate
		value, err = a.videoTemplates.Publish(ctx, in.UserID, clip.AuthoringPublication{Key: clip.AuthoringKey{Key: in.Key, SessionID: in.SessionID, Revision: in.Revision}, TargetID: in.TargetID, TargetVersion: in.TargetVersion, Recipe: clip.Recipe{Name: in.Artifact.Name, CompositionBody: in.Artifact.Body}})
		id, name = value.ID, value.Name
	case authoring.PostGuideline, authoring.VideoGuideline:
		scope, e := a.scope(in.Artifact, in.Kind)
		if e != nil {
			return authoring.SavedRef{}, e
		}
		if in.TargetID == "" && scope == nil {
			return authoring.SavedRef{}, authoring.ErrDraftInvalid
		}
		var value guideline.Guideline
		value, err = a.guidelines.Publish(ctx, in.UserID, guideline.AuthoringPublication{Key: guideline.AuthoringKey{Key: in.Key, SessionID: in.SessionID, Revision: in.Revision}, Kind: guidelineKind(in.Kind), TargetID: in.TargetID, TargetVersion: in.TargetVersion, Draft: guideline.AuthoringDraft{Name: in.Artifact.Name, Body: in.Artifact.Body}, Scope: scope})
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

func authoringNumberString(value *int) *string {
	out := ""
	if value != nil {
		out = strconv.Itoa(*value)
	}
	return &out
}
func authoringNumbers(a authoring.Artifact) (*template.Numbers, error) {
	if a.TargetLength == nil && a.TagCount == nil {
		return nil, nil
	}
	if a.TargetLength == nil || a.TagCount == nil {
		return nil, authoring.ErrDraftInvalid
	}
	parse := func(raw *string) (*int, error) {
		if *raw == "" {
			return nil, nil
		}
		if strings.TrimSpace(*raw) != *raw {
			return nil, authoring.ErrDraftInvalid
		}
		n, e := strconv.Atoi(*raw)
		if e != nil {
			return nil, authoring.ErrDraftInvalid
		}
		return &n, nil
	}
	length, e := parse(a.TargetLength)
	if e != nil {
		return nil, e
	}
	tags, e := parse(a.TagCount)
	if e != nil {
		return nil, e
	}
	return &template.Numbers{TargetLength: length, TagCount: tags}, nil
}
func (targets authoringTargets) scope(a authoring.Artifact, kind authoring.Kind) (*guideline.ScopePatch, error) {
	if a.Scope == nil {
		if len(a.TemplateIDs) > 0 || len(a.Fields) > 0 {
			return nil, authoring.ErrDraftInvalid
		}
		return nil, nil
	}
	p := guideline.ScopePatch{Scope: guideline.Scope(*a.Scope), TemplateIDs: a.TemplateIDs, Fields: a.Fields}
	if err := targets.guidelines.ValidateScopeShape(guidelineKind(kind), p); err != nil {
		return nil, authoring.ErrDraftInvalid
	}

	return &p, nil
}

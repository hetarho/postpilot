package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
)

// Map the authoring port into the real domain adapters and SQLite publishers.
// These domains intentionally have no description field in their saved content.
type domainTargets struct {
	guides *guideline.Authoring
	video  *clipapp.Authoring
	drift  string
}

func guideKind(kind authoring.Kind) guideline.Kind {
	if kind == authoring.VideoGuideline {
		return guideline.KindClip
	}
	return guideline.KindPost
}
func (d *domainTargets) Seed(ctx context.Context, user string, kind authoring.Kind, id string) (authoring.Seed, error) {
	if id == "" {
		return authoring.Seed{}, nil
	}
	var a authoring.Artifact
	var version string
	var err error
	if kind == authoring.VideoTemplate {
		var recipe clip.Recipe
		recipe, version, err = d.video.Seed(ctx, user, id)
		a = authoring.Artifact{ID: id, Name: recipe.Name, Body: recipe.CompositionBody}
	} else {
		var draft guideline.AuthoringDraft
		draft, version, err = d.guides.Seed(ctx, user, guideKind(kind), id)
		a = authoring.Artifact{ID: id, Name: draft.Name, Body: draft.Body}
	}
	return authoring.Seed{Artifact: &a, TargetVersion: version}, err
}
func (d *domainTargets) CanStart(ctx context.Context, user string, kind authoring.Kind, id string) error {
	if kind == authoring.VideoTemplate {
		return d.video.CanStart(ctx, user, id)
	}
	return d.guides.CanStart(ctx, user, guideKind(kind), id)
}
func (d *domainTargets) Validate(kind authoring.Kind, a authoring.Artifact) error {
	if kind == authoring.VideoTemplate {
		_, err := d.video.Validate(clip.Recipe{Name: a.Name, CompositionBody: a.Body})
		return err
	}
	_, err := d.guides.Validate(guideline.AuthoringDraft{Name: a.Name, Body: a.Body})
	return err
}
func (d *domainTargets) Guide(kind authoring.Kind) string {
	if kind == authoring.VideoTemplate {
		return d.video.Guide()
	}
	return d.guides.Guide(guideKind(kind))
}
func (d *domainTargets) Publish(ctx context.Context, p authoring.Publication) (authoring.SavedRef, error) {
	var id, name string
	var err error
	if p.Kind == authoring.VideoTemplate {
		var value clip.VideoTemplate
		value, err = d.video.Publish(ctx, p.UserID, clip.AuthoringPublication{Key: clip.AuthoringKey{Key: p.Key, SessionID: p.SessionID, Revision: p.Revision}, TargetID: p.TargetID, TargetVersion: p.TargetVersion, Recipe: clip.Recipe{Name: p.Artifact.Name, CompositionBody: p.Artifact.Body}})
		id, name = value.ID, value.Name
	} else {
		var value guideline.Guideline
		value, err = d.guides.Publish(ctx, p.UserID, guideline.AuthoringPublication{Key: guideline.AuthoringKey{Key: p.Key, SessionID: p.SessionID, Revision: p.Revision}, Kind: guideKind(p.Kind), TargetID: p.TargetID, TargetVersion: p.TargetVersion, Draft: guideline.AuthoringDraft{Name: p.Artifact.Name, Body: p.Artifact.Body}})
		id, name = value.ID, value.Title
	}
	if errors.Is(err, clip.ErrAuthoringConflict) || errors.Is(err, guideline.ErrAuthoringConflict) {
		return authoring.SavedRef{}, fmt.Errorf("%w: %w", authoring.ErrTargetConflict, err)
	}
	if err == nil && d.drift != "" {
		// Commit a real later edit between publication and authoring confirmation.
		seed, e := d.Seed(ctx, p.UserID, p.Kind, id)
		if e != nil {
			return authoring.SavedRef{}, e
		}
		later := authoring.Publication{Key: p.Key + ":outside", UserID: p.UserID, SessionID: p.SessionID + ":outside", Kind: p.Kind, Revision: 1, TargetID: id, TargetVersion: seed.TargetVersion, Artifact: *seed.Artifact}
		if d.drift == "name" {
			later.Artifact.Name = "Later owner name"
		} else if p.Kind == authoring.VideoTemplate {
			later.Artifact.Body = videoBody("Later owner body")
		} else {
			later.Artifact.Body = "Later owner body"
		}
		d.drift = ""
		if _, e = d.Publish(ctx, later); e != nil {
			return authoring.SavedRef{}, e
		}
	}
	return authoring.SavedRef{Kind: p.Kind, ID: id, Name: name}, err
}

type authoringFields struct{}

func (authoringFields) Known(string) bool { return false }

type authoringObjects struct{ clip.ObjectStore }
type authoringFinalizer struct{ clip.ProjectFinalizer }

func realTargets(h harness) *domainTargets {
	guides := guidelinestore.New(h.db.Writer, h.db.Reader)
	guideService := guideline.NewService(guides, authoringFields{}, guideline.Limits{TextMaxChars: 300, TitleMaxChars: 40, MaxPerAccount: 50}, 30)
	videos := clipstore.New(h.db.Writer, h.db.Reader)
	sources := clipapp.NewSourceService(videos, authoringObjects{}, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: 6 * time.Hour, PutTTL: time.Minute}))
	videoService := clipapp.NewService(videos, clip.DefaultLimits(), sources, authoringFinalizer{})
	return &domainTargets{guides: guideline.NewAuthoring(guideService, guides), video: clipapp.NewAuthoring(videoService, videos)}
}
func videoBody(text string) string {
	return `<clip version="1"><stage name="Visit">` + text + `</stage><text id="hook" kind="ai" role="hook"><row kind="ai">Opening</row></text><text id="ending" kind="ai" role="ending"><row kind="ai">Closing</row></text></clip>`
}
func TestRealDomainPublicationRefreshesOnlyItsOwnVersionAndContinuesAfterConflict(t *testing.T) {
	for _, kind := range []authoring.Kind{authoring.PostGuideline, authoring.VideoGuideline, authoring.VideoTemplate} {
		for _, drift := range []string{"", "name", "body"} {
			t.Run(string(kind)+"/"+drift, func(t *testing.T) {
				h := fixture(t)
				ctx := context.Background()
				domains := realTargets(h)
				h.svc = authoring.NewService(h.store, h.models, h.jobs, domains, budget{}, estimates{})
				body := "Original saved direction"
				if kind == authoring.VideoTemplate {
					body = videoBody("Original scene")
				}
				ref, err := domains.Publish(ctx, authoring.Publication{Key: "initial", UserID: "alice", SessionID: "initial", Kind: kind, Revision: 1, Artifact: authoring.Artifact{Name: "Original", Body: body}})
				if err != nil {
					t.Fatal(err)
				}
				s, err := h.svc.Create(ctx, "alice", kind, ref.ID, "open")
				if err != nil {
					t.Fatal(err)
				}
				source := *s.WorkingSource
				source.Name = "Edited once"
				source.Description = "Unsaved authoring description"
				s, err = h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "edit", WorkingSource: source})
				if err != nil {
					t.Fatal(err)
				}
				domains.drift = drift
				saved, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "first-save"}, false)
				if err != nil || saved.Phase != "saved" {
					t.Fatal("first save failed", err)
				}
				seed, err := domains.Seed(ctx, "alice", kind, ref.ID)
				if err != nil || seed.Artifact.Description != "" {
					t.Fatal("domain seed unexpectedly preserved authoring metadata", err)
				}
				if (saved.TargetVersion == seed.TargetVersion) != (drift == "") {
					t.Fatal("confirmation did not distinguish its publication from a later edit")
				}
				// Continue without resetting any conversation and save a second edit.
				s = patch(t, h, saved, "continue", body)
				source = *s.WorkingSource
				source.Name = "Edited twice"
				s, err = h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "second-edit", WorkingSource: source})
				if err != nil {
					t.Fatal(err)
				}
				save := authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "second-save"}
				second, err := h.svc.SaveWithKey(ctx, save, false)
				if drift == "" {
					if err != nil || second.Saved.ID != ref.ID {
						t.Fatal("normal consecutive save conflicted", err)
					}
					return
				}
				if !errors.Is(err, authoring.ErrTargetConflict) {
					t.Fatal("later external edit was overwritten", err)
				}
				reopened, err := secondServiceWithTargets(h, domains).Get(ctx, "alice", s.ID)
				if err != nil || reopened.FailureReason != "AUTHORING_SAVE_CONFLICT" || reopened.Publication == nil {
					t.Fatal("conflict receipt was not durable", err)
				}
				p := *reopened.Publication
				continued := patch(t, h, reopened, "continue-conflict", body)
				if continued.Phase != "editing" || continued.Publication == nil || *continued.Publication != p || continued.Saved == nil || *continued.Saved != *saved.Saved {
					t.Fatal("continuation lost current work or publication receipts")
				}
				if _, err = h.svc.SaveWithKey(ctx, save, false); !errors.Is(err, authoring.ErrTargetConflict) {
					t.Fatal("old conflicting receipt bypassed the version fence", err)
				}
				current, _ := h.svc.Get(ctx, "alice", s.ID)
				if current.Revision != continued.Revision || current.WorkingSource.Name != "Edited twice" {
					t.Fatal("receipt replay rewound continued editing")
				}
				final, _ := domains.Seed(ctx, "alice", kind, ref.ID)
				if *final.Artifact != *seed.Artifact || final.TargetVersion != seed.TargetVersion {
					t.Fatal("conflict recovery changed the external edit")
				}
			})
		}
	}
}

func secondServiceWithTargets(h harness, targets authoring.Targets) *authoring.Service {
	return authoring.NewService(h.store, h.models, h.jobs, targets, budget{}, estimates{})
}

func TestNewPublishedSettingRecoversContinuationByTargetWithoutChangingReceipts(t *testing.T) {
	for _, kind := range []authoring.Kind{authoring.PostGuideline, authoring.VideoGuideline, authoring.VideoTemplate} {
		t.Run(string(kind), func(t *testing.T) {
			h := fixture(t)
			ctx := context.Background()
			domains := realTargets(h)
			h.svc = secondServiceWithTargets(h, domains)
			s, err := h.svc.Create(ctx, "alice", kind, "", "new-setting")
			if err != nil {
				t.Fatal(err)
			}
			body := "First valid direction"
			if kind == authoring.VideoTemplate {
				body = videoBody("First scene")
			}
			s, err = h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "manual-new", WorkingSource: authoring.Artifact{Name: "My setting", Body: body}})
			if err != nil {
				t.Fatal(err)
			}
			save := authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "first-save"}
			saved, err := h.svc.SaveWithKey(ctx, save, false)
			if err != nil {
				t.Fatal(err)
			}
			published := *saved.Publication
			if published.TargetID != "" || saved.TargetVersion == "" {
				t.Fatal("first creation receipt or confirmed version is invalid")
			}
			continuedBody := "Unpublished continuation"
			if kind == authoring.VideoTemplate {
				continuedBody = videoBody("Unpublished scene")
			}
			continued := patch(t, h, saved, "continue-new", continuedBody)
			latest, err := h.svc.Latest(ctx, "alice", kind, saved.Saved.ID)
			if err != nil || latest == nil || latest.ID != continued.ID || latest.WorkingSource.Body != continuedBody || !latest.HasUnpublishedChanges {
				t.Fatal("published target lost its continuing draft", err)
			}
			if latest.TargetID != "" || latest.TargetVersion != saved.TargetVersion {
				t.Fatal("recovery changed the immutable source target or version fence")
			}
			rows, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: kind})
			if err != nil || len(rows) != 1 || rows[0].TargetID != saved.Saved.ID || !rows[0].SavedAvailable || !rows[0].HasUnpublishedChanges {
				t.Fatal("summary cannot associate the draft with its saved target", rows, err)
			}
			unsaved, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: kind, UnsavedOnly: true})
			if err != nil || len(unsaved) != 0 {
				t.Fatal("a published setting was mixed into new creations", unsaved, err)
			}
			foreign, err := h.svc.Latest(ctx, "bob", kind, saved.Saved.ID)
			if err != nil || foreign != nil {
				t.Fatal("published-id recovery crossed owners", err)
			}
			replay, err := h.svc.SaveWithKey(ctx, save, false)
			if err != nil || *replay.Publication != published || replay.TargetID != "" || replay.Saved.ID != saved.Saved.ID {
				t.Fatal("creation receipt changed after target-based recovery", err)
			}
			current, _ := h.svc.Get(ctx, "alice", saved.ID)
			if current.Revision != continued.Revision || current.WorkingSource.Body != continuedBody {
				t.Fatal("receipt replay rewound continuing work")
			}
			second, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: current.ID, ExpectedRevision: current.Revision, OperationKey: "second-save"}, false)
			if err != nil || second.Saved.ID != saved.Saved.ID || second.Publication.TargetID != saved.Saved.ID {
				t.Fatal("continued publication did not update the same target", err)
			}
			newest, err := h.svc.Create(ctx, "alice", kind, saved.Saved.ID, "another-target-session")
			if err != nil {
				t.Fatal(err)
			}
			rows, _, err = h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: kind})
			if err != nil || len(rows) != 1 || rows[0].SessionID != newest.ID {
				t.Fatal("published-id alias duplicated a named target summary", rows, err)
			}
		})
	}
}

func TestPublishedVoiceCopyKeepsItsSourceSessionScope(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := selected(t, h, authoring.WritingVoice)
	saved, err := h.svc.Save(ctx, "alice", s.ID, s.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	continued := patch(t, h, saved, "continue-style", saved.WorkingSource.Body+" 새 방향")
	latest, err := h.svc.Latest(ctx, "alice", authoring.WritingVoice, saved.Saved.ID)
	if err != nil || latest != nil {
		t.Fatal("synthetic copy rebound its originating session", err)
	}
	rows, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.WritingVoice})
	if err != nil || len(rows) != 1 || rows[0].TargetID != continued.TargetID {
		t.Fatal("synthetic copy changed summary source semantics", rows, err)
	}
}

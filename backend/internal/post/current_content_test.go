package post

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// Collaborators that fail on any call, so a read that reaches one is caught.
type (
	failingJobs        struct{}
	failingExperiments struct{}
	failingVoices      struct{}
	failingTemplates   struct{}
)

var errCollaborator = errors.New("a collaborator was asked")

func (failingJobs) ActiveForPost(context.Context, string) (*ActiveJob, error) {
	return nil, errCollaborator
}
func (failingExperiments) PendingForPost(context.Context, string, string) (string, error) {
	return "", errCollaborator
}
func (failingVoices) Voices(context.Context, string) ([]VoiceRef, error) {
	return nil, errCollaborator
}
func (failingTemplates) Templates(context.Context, string) ([]TemplateRef, error) {
	return nil, errCollaborator
}

// Review F8: the quality context measures content, so its read is the post row alone — no
// listing, presign, job, experiment, voice or template read, which the screen's Get pays for.
func TestCurrentContentReadsThePostRowAlone(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	found := generatedWith(t, svc, alice, &annotations)
	attachPhoto(t, svc, blobs, found.Slug, "IMG_1.jpg")
	mustAttachVideo(t, svc, blobs, alice, found.Slug, "clip.mp4", 5_000)

	svc.jobs = failingJobs{}
	svc.experiments = failingExperiments{}
	svc.voices = failingVoices{}
	svc.templates = failingTemplates{}
	blobs.failPresign = true

	// The collaborators are live: the screen's read reaches them and fails.
	if _, err := svc.Get(ctx, alice, found.Slug); err == nil {
		t.Fatal("Get succeeded with every collaborator failing")
	}

	got, err := svc.CurrentContent(ctx, alice, found.Slug)
	if err != nil {
		t.Fatalf("CurrentContent: %v", err)
	}
	row := store.posts[found.Slug]
	want := ContentSnapshot{
		Slug: row.Slug, ContentRevision: row.ContentRevision, Content: row.Content,
		ContentLanguage: row.ContentLanguage, TargetLanguage: row.TargetLanguage, Nouns: row.ContentNouns,
	}
	if !reflect.DeepEqual(got, want) || got.Content == nil || len(got.Nouns) == 0 {
		t.Fatalf("CurrentContent = %+v, want the stored row %+v", got, want)
	}

	if _, err := svc.CurrentContent(ctx, alice, "no-such-post"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown slug = %v, want ErrNotFound", err)
	}
	if _, err := svc.CurrentContent(ctx, bob, found.Slug); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign slug = %v, want ErrForbidden", err)
	}
}

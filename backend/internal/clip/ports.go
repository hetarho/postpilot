package clip

import (
	"context"
	"time"
)

// Store scopes every access to the actor. Patches and their readback are atomic.
type Store interface {
	ListTemplates(context.Context, string) ([]VideoTemplate, error)
	GetTemplate(context.Context, string, string) (VideoTemplate, error)
	InsertTemplate(context.Context, VideoTemplate) error
	UpdateTemplate(context.Context, string, string, TemplatePatch, time.Time) (VideoTemplate, error)
	DeleteTemplate(context.Context, string, string) (int, error)
	ListProjects(context.Context, string) ([]Project, error)
	GetProject(context.Context, string, string) (Project, error)
	InsertProject(context.Context, Project) error
	UpdateProject(context.Context, string, string, ProjectPatch, time.Time) (Project, error)
	DeleteProject(context.Context, string, string) error
}

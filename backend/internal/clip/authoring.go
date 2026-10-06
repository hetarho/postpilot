package clip

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

var ErrAuthoringConflict = errors.New("video template changed since the authoring draft was opened")

type AuthoringKey struct {
	Key, SessionID string
	Revision       uint32
}
type AuthoringPublication struct {
	Key                     AuthoringKey
	TargetID, TargetVersion string
	Recipe                  Recipe
}
type AuthoringStore interface {
	PublishAuthoringTemplate(context.Context, string, AuthoringPublication, string, time.Time) (VideoTemplate, error)
}

func AuthoringTemplateVersion(t VideoTemplate) string {
	fields := []string{t.ID, t.Name, t.CompositionBody, t.Design.IntroPreset, t.Design.OutroPreset, fmt.Sprintf("%q", t.Design.CaptionStyles), t.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	return fmt.Sprintf("video-template:%x", sha256.Sum256([]byte(fmt.Sprintf("%q", fields))))
}

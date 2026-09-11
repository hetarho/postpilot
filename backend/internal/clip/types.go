// Package clip owns independent video projects and their reusable recipes.
package clip

import (
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("clip or video template not found")
	ErrDuplicateName = errors.New("video template name already exists")
	ErrInvalid       = errors.New("invalid clip input")
	// The owner has not chosen a campaign type, so the clip has no disclosure
	// phrase to show — and a clip without one may not be rendered (CDS-5).
	ErrDisclosureRequired = errors.New("clip disclosure required")
)

type Limits struct {
	NameChars, GuidanceChars, FieldCount, LabelChars, PromptChars int
	TitleChars, AnswerChars, MinDurationMS, MaxDurationMS         int
}

type InformationField struct{ Label, Prompt string }
type Recipe struct {
	Name              string
	InformationFields []InformationField
	CutGuidance       string
	CopyStyles        []string
	Accent            string
	// One of the five CDS category presets. Empty is a template written before
	// presets existed and reads as the shared defaults, never as a category.
	Preset string
}
type VideoTemplate struct {
	ID, UserID string
	Recipe
	ProjectCount         int
	CreatedAt, UpdatedAt time.Time
}
type TemplatePatch struct {
	Name, CutGuidance, Accent, Preset *string
	InformationFields                 *[]InformationField
	CopyStyles                        *[]string
}
type Answer struct{ Label, Text string }
type Result struct {
	Key, ContentType     string
	ViewURL, DownloadURL string
	Bytes                int64
	DurationMS           int
	CreatedAt            time.Time
}
type Project struct {
	ID, UserID, Title, VideoTemplateID, Ratio string
	// The owner's campaign type and closing call to action, as fixed ids: the
	// phrases are code-owned so the renderer can never be handed an edited
	// disclosure. Empty disclosure is what the generation gate refuses; an empty
	// CTA falls back to the template's preset.
	Disclosure, CTA                        string
	TargetDurationMS                       int
	Answers                                []Answer
	Analysis, EditPlan                     string
	Result                                 *Result
	EditPlanRevision, RenderedPlanRevision int
	CreatedAt, UpdatedAt                   time.Time
}
type ProjectInput struct {
	Title, VideoTemplateID, Ratio string
	Disclosure, CTA               string
	TargetDurationMS              int
	Answers                       []Answer
}

// Ratio deliberately has no update representation.
type ProjectPatch struct {
	Title, VideoTemplateID, Disclosure, CTA *string
	TargetDurationMS                        *int
	Answers                                 []Answer
}

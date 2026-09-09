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
}
type VideoTemplate struct {
	ID, UserID string
	Recipe
	ProjectCount         int
	CreatedAt, UpdatedAt time.Time
}
type TemplatePatch struct {
	Name, CutGuidance, Accent *string
	InformationFields         *[]InformationField
	CopyStyles                *[]string
}
type Answer struct{ Label, Text string }
type Result struct {
	Key, ContentType string
	Bytes            int64
	DurationMS       int
	CreatedAt        time.Time
}
type Project struct {
	ID, UserID, Title, VideoTemplateID, Ratio string
	TargetDurationMS                          int
	Answers                                   []Answer
	Analysis, EditPlan                        string
	Result                                    *Result
	EditPlanRevision, RenderedPlanRevision    int
	CreatedAt, UpdatedAt                      time.Time
}
type ProjectInput struct {
	Title, VideoTemplateID, Ratio string
	TargetDurationMS              int
	Answers                       []Answer
}

// Ratio deliberately has no update representation.
type ProjectPatch struct {
	Title, VideoTemplateID *string
	TargetDurationMS       *int
	Answers                []Answer
}

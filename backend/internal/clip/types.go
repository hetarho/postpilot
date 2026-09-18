// Package clip owns independent video projects and their reusable recipes.
package clip

import (
	"errors"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip/composition"
)

var (
	ErrNotFound      = errors.New("clip or video template not found")
	ErrDuplicateName = errors.New("video template name already exists")
	ErrInvalid       = errors.New("invalid clip input")
	// The owner has not chosen a campaign type, so the clip has no disclosure
	// phrase to show when disclosure visibility is enabled (CDS-5).
	ErrDisclosureRequired = errors.New("clip disclosure required")
	// The owner has not chosen how long the clip should be. The choice belongs
	// beside the sources it measures (CLIP-130), so the project is minted
	// without it and generation is what refuses it (CLIP-7).
	ErrTargetDurationRequired = errors.New("clip target duration required")
)

type Limits struct {
	Composition                                                   composition.Limits
	NameChars, GuidanceChars, FieldCount, LabelChars, PromptChars int
	TitleChars, AnswerChars, MinDurationMS, MaxDurationMS         int
	// The project instruction's own maximum (CLIP-121), counted CDS-20's way
	// like every other bounded text.
	InstructionChars int
}

type InformationField struct{ Label, Prompt string }
type Recipe struct {
	CompositionBody string
	// Legacy is a server-owned conversion marker, never accepted from a client.
	CompositionLegacy bool
	CaptionPace       string
	Name              string
	InformationFields []InformationField
	CutGuidance       string
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
	// True only on a read projection whose CompositionBody has been converted
	// from the old grammar (CLIP-140); the stored body is untouched until the
	// owner saves. Never set on a template read for generation.
	CompositionConverted bool
}
type TemplatePatch struct {
	CompositionBody                                *string
	Name, CutGuidance, Accent, Preset, CaptionPace *string
	InformationFields                              *[]InformationField
}
type Answer struct{ Label, Text string }
type RenderKind string

const (
	RenderServer  RenderKind = "server"
	RenderBrowser RenderKind = "browser"
)

var ErrRenderUnavailable = errors.New("browser rendering is not implemented")

type Result struct {
	Kind                 RenderKind
	ID                   string
	Key, ContentType     string
	ViewURL, DownloadURL string
	Bytes                int64
	DurationMS           int
	CreatedAt            time.Time
}

// RenderKind reads results written before the kind was recorded as server work.
func (r Result) RenderKind() RenderKind {
	if r.Kind == "" {
		return RenderServer
	}
	return r.Kind
}

type Project struct {
	Language                                  string
	Finalized                                 *Finalization
	Composition                               *ProjectComposition
	HideDisclosure                            bool
	ID, UserID, Title, VideoTemplateID, Ratio string
	// The owner's campaign type and closing call to action, as fixed ids: the
	// phrases are code-owned so the renderer can never be handed an edited
	// disclosure. Empty disclosure is what the generation gate refuses; an empty
	// CTA falls back to the template's preset.
	Disclosure, CTA string
	// The owner's own free-text instruction for this project (CLIP-121), empty
	// when none was written. It belongs to the project, not to the answers the
	// template declared.
	Instruction string
	// The owner's caption pace and accent for this clip (CLIP-139). Empty is
	// "not chosen": the render falls back to what the frozen document said, so
	// a project made before they moved renders exactly as it did.
	CaptionPace, Accent string
	// The owner's design selection for this clip (CLIP-139, CLIP-142): the two
	// region presets and the caption styles the clip may use. Seeded from the
	// template at creation and the project's to change afterwards; an empty
	// style selection is the default style alone (CDS-25).
	IntroPreset, OutroPreset               string
	CaptionStyles                          []string
	TargetDurationMS                       int
	Answers                                []Answer
	Analysis, EditPlan                     string
	Result                                 *Result
	EditPlanRevision, RenderedPlanRevision int
	CreatedAt, UpdatedAt                   time.Time
	// What the owner asked the AI for, newest first (CLIP-133). Read only where
	// the owner reads the project; the run paths take the project without it.
	Requests []ProjectRequest
}

// ProjectRequest is one accepted request, kept verbatim: the instruction a
// generation froze, or the words of a revision and the document it named
// (CLIP-133). It is owner content — never a diagnostic, never deduplicated, and
// never written by a save.
type ProjectRequest struct {
	Kind      string
	Body      string
	CreatedAt time.Time
}

// RequestInstruction is the kind of the instruction a generation froze; a
// revision's kind names the target it was about. An empty body under
// RequestInstruction is the record of a generation run WITHOUT an instruction.
const RequestInstruction = "instruction"

func RevisionRequestKind(target string) string { return "revision:" + target }

// ValidRequestKind is the same set the table's CHECK holds.
func ValidRequestKind(kind string) bool {
	return kind == RequestInstruction ||
		(strings.HasPrefix(kind, "revision:") && ValidRevisionTarget(strings.TrimPrefix(kind, "revision:")))
}

type ProjectInput struct {
	Language                      string
	CompositionInputs             *CompositionInputs
	HideDisclosure                bool
	Title, VideoTemplateID, Ratio string
	Disclosure, CTA               string
	Instruction                   string
	// Absent seeds both from the selected template; an explicit empty string is
	// the steady pace and no accent.
	CaptionPace, Accent *string
	// Absent seeds all three from the selected template, or from the shared
	// defaults where no template is attached (CLIP-139).
	IntroPreset, OutroPreset *string
	CaptionStyles            *[]string
	TargetDurationMS         int
	Answers                  []Answer
}

// Ratio deliberately has no update representation.
type ProjectPatch struct {
	ExpectedCompositionRevision *int
	CompositionInputs           *CompositionInputs
	// The service creates the snapshot; the caller cannot replace frozen content.
	Composition                             *ProjectComposition
	HideDisclosure                          *bool
	Title, VideoTemplateID, Disclosure, CTA *string
	Instruction                             *string
	CaptionPace, Accent                     *string
	// Presence-aware like the pace and the accent: changing any of the three
	// marks the result stale and invalidates no observation and no plan
	// (CLIP-139).
	IntroPreset, OutroPreset *string
	CaptionStyles            *[]string
	TargetDurationMS         *int
	Answers                  []Answer
}

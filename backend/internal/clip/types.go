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
	Composition                                           composition.Limits
	NameChars, FieldCount, LabelChars, PromptChars        int
	TitleChars, AnswerChars, MinDurationMS, MaxDurationMS int
	// The project instruction's own maximum (CLIP-121), counted CDS-20's way
	// like every other BoundedText text.
	InstructionChars int
}

// Recipe is what a template carries: its name and its outline body (CLIP-4).
type Recipe struct {
	Name, CompositionBody string
}
type VideoTemplate struct {
	ID, UserID string
	Recipe
	ProjectCount         int
	CreatedAt, UpdatedAt time.Time
}
type TemplatePatch struct {
	Name, CompositionBody *string
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
	// The owner's campaign type, as a fixed id: the phrase is code-owned so the
	// renderer can never be handed an edited disclosure. Empty is what the
	// generation gate refuses.
	Disclosure string
	// The owner's own free-text instruction for this project (CLIP-121), empty
	// when none was written. It belongs to the project, not to the values the
	// template declared.
	Instruction string
	// The owner's caption pace and accent for this clip (CLIP-139). Empty is the
	// shared default: the steady pace and no accent.
	CaptionPace, Accent string
	// The owner's design selection for this clip (CLIP-139, CLIP-142): the two
	// region presets and the caption styles the clip may use; an empty style
	// selection is the default style alone (CDS-25).
	IntroPreset, OutroPreset               string
	CaptionStyles                          []string
	TargetDurationMS                       int
	Analysis, EditPlan                     string
	Result                                 *Result
	EditPlanRevision, RenderedPlanRevision int
	CreatedAt, UpdatedAt                   time.Time
	// What the owner asked the AI for, newest first (CLIP-133). Read only where
	// the owner reads the project; the run paths take the project without it.
	Requests []ProjectRequest
	// The clip's storyline (CLIP-178); nil when it has none.
	Storyline *Storyline
	// The plan revision a generation or a revision last wrote (CLIP-180). An owner edit moves
	// EditPlanRevision past it, which is what "edited by hand" means.
	GeneratedPlanRevision int
}

// PlanEditedByHand is whether the owner changed the plan since a writer last wrote it: the
// confirmation 이 스토리로 만들기 asks before replacing it (CLIP-180).
func (p Project) PlanEditedByHand() bool {
	return p.EditPlan != "" && p.EditPlanRevision != p.GeneratedPlanRevision
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

// RequestStoryline is the kind of a storyline request's words (CLIP-133, CLIP-181).
const RequestStoryline = "storyline"

func RevisionRequestKind(target string) string { return "revision:" + target }

// ValidRequestKind is the same set the table's CHECK holds.
func ValidRequestKind(kind string) bool {
	return kind == RequestInstruction || kind == RequestStoryline ||
		(strings.HasPrefix(kind, "revision:") && ValidRevisionTarget(strings.TrimPrefix(kind, "revision:")))
}

type ProjectInput struct {
	Language                      string
	CompositionInputs             *CompositionInputs
	HideDisclosure                bool
	Title, VideoTemplateID, Ratio string
	Disclosure                    string
	Instruction                   string
	// Absent or empty is the shared default: the steady pace and no accent.
	CaptionPace, Accent *string
	// Absent seeds all three from the selected template, or from the shared
	// defaults where no template is attached (CLIP-139).
	IntroPreset, OutroPreset *string
	CaptionStyles            *[]string
	TargetDurationMS         int
}

// Ratio deliberately has no update representation.
type ProjectPatch struct {
	ExpectedCompositionRevision *int
	CompositionInputs           *CompositionInputs
	// The service creates the snapshot; the caller cannot replace frozen content.
	Composition                        *ProjectComposition
	HideDisclosure                     *bool
	Title, VideoTemplateID, Disclosure *string
	Instruction                        *string
	CaptionPace, Accent                *string
	// Presence-aware like the pace and the accent: changing any of the three
	// marks the result stale and invalidates no observation and no plan
	// (CLIP-139).
	IntroPreset, OutroPreset *string
	CaptionStyles            *[]string
	TargetDurationMS         *int
	// The owner's storyline edit (CLIP-178): the same paragraphs with their texts and scenes
	// replaced. Nil leaves the storyline as it is.
	Storyline *[]StorylineParagraph
}

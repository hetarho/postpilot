package generation

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

type ImageReader interface {
	Read(ctx context.Context, key string) ([]byte, error)
}

// VideoLinker mints the short-lived URL a video reaches a model by. It is a LINK and never
// bytes: a clip is up to 200 MiB, and the largest thing this process carries stays a memo
// (VIDEO-10, POST-34). Declared here by its consumer and satisfied by the object store.
type VideoLinker interface {
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type Posts interface {
	AttachedImages(ctx context.Context, userID, slug string) (PostInput, error)
	SetObservations(ctx context.Context, userID, slug string, observations []Observation) error
	// SetGeneratedContent stores a machine write. annotations nil keeps the post's nouns (a
	// revision); non-nil replaces them, where empty clears.
	SetGeneratedContent(ctx context.Context, userID, slug string, content PostContent, language Language, annotations *WriteAnnotations) error
	// SetStoryline replaces the post's storyline with a storyline job's answer and touches
	// nothing else; a published post refuses it (GEN-68, GEN-69, POST-74).
	SetStoryline(ctx context.Context, userID, slug string, storyline Storyline) error
}

// Profiles projects exactly the post's voice; the voice context never falls back to a
// sibling voice, so an empty voice prompts as empty.
type Profiles interface {
	ProfileForPrompt(ctx context.Context, userID, voiceID string, target Language) (Profile, error)
}

type TopicProfiles interface {
	ProfileForPromptForTopic(ctx context.Context, userID, voiceID string, target Language, topic string, tags []string) (Profile, error)
}

type LLM interface {
	Resolve(ref llm.ModelRef) (llm.ModelInfo, bool)
	Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error)
}

type Jobs interface {
	// EnqueueGeneration stores payload verbatim and reads only request, for the row, the guards
	// and the hold.
	EnqueueGeneration(ctx context.Context, request StartRequest, payload []byte) (string, error)
	EnqueueRevision(ctx context.Context, request StartRevisionRequest, payload []byte) (string, error)
	EnqueueStoryline(ctx context.Context, request StartStorylineRequest, payload []byte) (string, error)
	EnqueueStorylineRevision(ctx context.Context, request StartStorylineRevisionRequest, payload []byte) (string, error)
	GetGeneration(ctx context.Context, id, userID string) (*JobSummary, error)
}

// PendingExperiments is the experiment context's published post guard. Generation asks
// only for the editor write comparison that holds the post (GEN-23); it never reads
// experiment rows.
type PendingExperiments interface {
	BlockingWriteForPost(ctx context.Context, userID, postSlug string) (string, error)
}

// TemplateBriefs is the template context's published render, consumed only at enqueue time.
// `ok` false means the post has no template or it was deleted between the save and the start
// — an ordinary case, not an error, because a prompt without a template is a valid one.
//
// Whether the post has a photo and the post's answers are passed IN rather than looked up on
// the other side: the freeze has to see exactly what this enqueue decided on, and a lookup
// there could observe a photo attached or a field edited in between. The brief names no
// attachment (TMPL-21). An error is a real failure and stops the start.
type TemplateBriefs interface {
	RenderedFor(ctx context.Context, userID, templateID string, hasPhotos bool, answers []TemplateAnswer) (TemplateBrief, bool, error)
	RenderedForNewWrite(ctx context.Context, userID, templateID string, hasPhotos bool, answers []TemplateAnswer) (TemplateBrief, bool, error)
}

// FrozenGuidelines are the 지침 texts one run is given, in injection order (GUIDE-14): the
// enabled 기본 지침 in the product's order and the run's target language, then the owner's.
type FrozenGuidelines struct {
	Defaults []string
	Owner    []string
}

// GuidelinesForPrompt is the guideline context's published resolution, consumed only at
// enqueue time: the enabled 기본 지침 of a post, then the owner's texts — the global group, the
// template group, then the 분야 group, each by created_at then id (GUIDE-14). templateID and
// field are nil for a post with none, which yields only the groups the post has — an empty
// result is the ordinary case, not an error. withMemories says whether the run's prompt
// carries a [기억] section, which a memories-only 기본 지침 needs to be sent at all (GEN-73).
type GuidelinesForPrompt interface {
	ForPrompt(ctx context.Context, userID string, templateID, field *string, target Language, withMemories bool) (FrozenGuidelines, error)
}

// MemoriesForPrompt is the memory context's published retrieval, consumed only at enqueue
// time and only for a post that opted in (MEM-18, MEM-19). What crosses is the post's own
// words — its memo, its 가제, its template answers and the `objects` and `visible_text` of
// its observations (MEM-7) — and what comes back is TEXTS. A memory row never reaches this
// context, so nothing here learns the kind, the tags or the id that selected it (ARCH-7).
//
// An empty result is the ordinary case, not an error: a post whose key matched no memory
// builds the same prompt as a post with the option off.
type MemoriesForPrompt interface {
	ForPost(ctx context.Context, userID string, keyParts []string) ([]string, error)
}

// GuidelineCandidates records a completed revision's instruction so the correction accrues
// instead of vanishing with the tab (GUIDE-7). Declared here beside GuidelinesForPrompt
// for the same reason: the generation context must not learn the guideline context's tables.
//
// A candidate is the user's own sentence, recorded verbatim — nothing on either side of this
// port reads it with a model, and recording adds no queue and no provider call ([I5]).
type GuidelineCandidates interface {
	Record(ctx context.Context, userID, postSlug, instruction string) error
}

type Progress func(stage string, done, total int)

// QualityRulesForPrompt is the quality context's published rendering of the ticked rules,
// consumed only at enqueue: the texts of the ticked metrics the post's account is still over
// band on, in the target language (GEN-51, POST-81). What crosses in is the post and the ASCII
// metric ids; what comes back is TEXTS, so no metric, band or measurement reaches this context
// (ARCH-7).
type QualityRulesForPrompt interface {
	RulesFor(ctx context.Context, userID, slug string, ticked []string, language Language) ([]string, error)
}

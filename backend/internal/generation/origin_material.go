package generation

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

const OriginProtocolVersion = 1

// These are admission allowances, never additions to a dispatched frozen cap.
// Completion uses the existing owner's budget/cap policy with additional demand.
const OriginPromptTokenOverhead = 4096
const ObserveOriginPromptTokenOverhead = 1024
const OriginCompletionExtraChars = 1024

func OriginBudgetTarget(target *int) *int {
	value := post.TargetLengthMin
	if target != nil {
		value = *target
	}
	value += OriginCompletionExtraChars
	return &value
}

type originCatalog struct {
	sources []post.OriginSource
	chars   int
}

func (c *originCatalog) add(id string, kind post.OriginSourceKind, text, filename string, available bool, attachmentIDs ...string) {
	attachmentID := ""
	if len(attachmentIDs) == 1 {
		attachmentID = attachmentIDs[0]
	}
	chars := utf8.RuneCountInString(text)
	if !utf8.ValidString(text) || !utf8.ValidString(id) || !utf8.ValidString(attachmentID) || utf8.RuneCountInString(attachmentID) > post.OriginMaxSourceIDChars || strings.TrimSpace(text) == "" || len(c.sources) >= post.OriginMaxSources || utf8.RuneCountInString(id) > post.OriginMaxSourceIDChars || c.chars+chars > post.OriginMaxSourceTextChars {
		return
	}
	c.sources = append(c.sources, post.OriginSource{ID: id, Kind: kind, Text: text, AttachmentFilename: filename, AttachmentID: attachmentID, Available: available})
	c.chars += chars
}

// WritingOriginSources describes material actually present in this request. It
// does not reread sources, classify arbitrary prose or treat style/form as facts.
func WritingOriginSources(input WritePromptInput, fictional bool, attachmentIDs ...map[string]string) []post.OriginSource {
	currentIDs := map[string]string{}
	if len(attachmentIDs) == 1 {
		currentIDs = attachmentIDs[0]
	}
	var catalog originCatalog
	kind := post.OriginSourceMemo
	if fictional {
		kind = post.OriginSourceAIProposal
	}
	catalog.add("current.title", kind, input.Title, "", true)
	catalog.add("current.memo", kind, input.Memo, "", true)
	if input.Template != nil {
		var walk func(string, []TemplateMaterialPart)
		walk = func(path string, parts []TemplateMaterialPart) {
			for i, part := range parts {
				id := fmt.Sprintf("%s.%d", path, i)
				if part.Kind == "fact" {
					catalog.add(id, post.OriginSourceTemplateAnswer, part.Text, "", true)
				}
				walk(id, part.Parts)
			}
		}
		walk("current.answer.body", input.Template.BodyParts)
		walk("current.answer.title", input.Template.TitleParts)
		// Retained renderer facts are explicit field values, not a reparse of the
		// plain historical body or the template's instruction topics.
		if input.Template.BodyParts == nil && input.Template.TitleParts == nil {
			for i, fact := range input.Template.Facts {
				catalog.add(fmt.Sprintf("current.answer.%d", i), post.OriginSourceTemplateAnswer, fact.Value, "", true)
			}
		}
	}
	for i, memory := range input.Memories {
		catalog.add(fmt.Sprintf("current.memory.%d", i), post.OriginSourceMemory, memory, "", true)
	}
	shown := make(map[string]bool, len(input.Photos)+len(input.Videos))
	for _, file := range append(append([]string(nil), input.Photos...), input.Videos...) {
		shown[file] = true
	}
	for i, observation := range input.Observations {
		if !shown[observation.File] {
			continue
		}
		kind := AttachmentPhoto
		for _, video := range input.Videos {
			if observation.File == video {
				kind = AttachmentVideo
				break
			}
		}
		validated := ValidateStoredObservationOrigins(observation, observation.Origins, kind)
		byID := map[string]post.OriginSource{}
		for _, source := range validated.Sources {
			byID[source.ID] = source
		}
		for j, span := range validated.Spans {
			sourceKind := post.OriginSourceAIProposal
			if span.Category == post.OriginPhotoInterpretation {
				sourceKind = post.OriginSourceVisualObservation
			}
			attachmentID := inheritedOriginAttachmentID(span.SourceRefs, byID)
			available := true
			if currentID := currentIDs[observation.File]; currentID != "" && currentID != attachmentID {
				available = false
			}
			catalog.add(fmt.Sprintf("current.visual.%d.%d", i, j), sourceKind, span.Quote, observation.File, available, attachmentID)
		}
	}
	return catalog.sources
}

const originResponseContract = `Semantic-origin metadata is optional and follows all complete canonical members in a final origins array. Use exact meaningful quotes, not Unicode offsets; split different meanings within one sentence. Each candidate has field, quote, optional zero-based occurrence, category and source_refs. Categories are owner_input, photo_interpretation and ai_added. owner_input requires an actual supplied memo/answer/edit/memory source; photo_interpretation requires an identified visual observation. Style examples, generic rules, template topics/form, saved ownership and plan approval do not supply owner facts. Preserve prior meaning's origins; unknown historical evidence stays unconfirmed. Source labels are not factual certification. A flavor, texture, aroma or explanation beyond supplied meaning is ai_added, not a paraphrase. Never invent events or chronology merely by labeling it AI-added. Omit uncertain or unsupported annotations rather than guessing. Refer only to catalog IDs supplied below; do not fabricate live references, URLs, filenames as evidence IDs or new origin categories. For post fields, field.kind is title, summary, tag, block_content, block_item, block_alt or block_caption with the applicable zero-based tag_index/block_index/item_index. For returned storyline text only, field.kind is storyline_paragraph with paragraph_index. Do not annotate filenames, JSON keys or format enums. A frozen-story write returns no new plan, and a revision returns no plan or memories. Never add a provider pass to finish metadata.`

func scopedOriginContract(request llm.Request) string {
	common, _, _ := strings.Cut(originResponseContract, " For post fields,")
	postFields := " For post text only, field.kind is title, summary, tag, block_content, block_item, block_alt or block_caption with the applicable zero-based tag_index/block_index/item_index. Do not annotate filenames, keys or format enums."
	planFields := " For returned storyline text only, field.kind is storyline_paragraph with zero-based paragraph_index."
	mode := "direct"
	if request.Composition != nil {
		mode = request.Composition.Mode
	}
	switch mode {
	case "storyline-create", "storyline-rewrite":
		return common + planFields + " Return only plan-paragraph origins, no final-post title/tag/block origin fields."
	case "frozen-storyline":
		return common + postFields + " Return no replacement plan or plan origins; preserve the supplied plan's meaning and origin context."
	case "revision":
		return common + postFields + " Return no plan origins and read no memory text; preserve supplied current origins for untouched meaning."
	default:
		return common + postFields + planFields
	}
}

func appendOriginRequest(request llm.Request, sources []post.OriginSource, prior any) llm.Request {
	return appendOriginContract(request, sources, prior, scopedOriginContract(request))
}

func appendOriginContract(request llm.Request, sources []post.OriginSource, prior any, contract string) llm.Request {
	request.System += "\n" + contract
	data := struct {
		Sources []originSourceJSON `json:"sources"`
		Prior   any                `json:"prior_meaning,omitempty"`
	}{encodeOriginSources(sources), prior}
	raw, _ := json.Marshal(data)
	text := "\n\n[Result-local supplied origin material]\n" + string(raw)
	request.Messages[len(request.Messages)-1].Parts = append(request.Messages[len(request.Messages)-1].Parts, llm.TextPart(text))
	if request.Composition != nil {
		composition := *request.Composition
		composition.Fragments = append([]llm.RequestFragment(nil), composition.Fragments...)
		composition.PromptVersion = "post-origin-contracts-v1"
		systemFragment := llm.RequestFragment{ID: "origin-response-contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "optional-semantic-origin-output-contract", Text: "\n" + contract, SourceFiles: []string{"internal/generation/origin_material.go"}, Activation: "admitted origin protocol v1"}
		index := 0
		for index < len(composition.Fragments) && composition.Fragments[index].Role == llm.InspectionRoleSystem {
			index++
		}
		composition.Fragments = append(composition.Fragments[:index], append([]llm.RequestFragment{systemFragment}, composition.Fragments[index:]...)...)
		composition.Fragments = append(composition.Fragments, llm.RequestFragment{ID: "origin-supplied-catalog", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "bounded-actual-stage-source-catalog-and-prior-origins", Text: text, SourceFiles: []string{"internal/generation/origin_material.go"}, Activation: "actual supplied stage material only; no inferred historical source"})
		composition.SourceFiles = append(append([]string(nil), composition.SourceFiles...), "internal/generation/origin_material.go")
		request.Composition = &composition
	}
	return request
}

func setOriginOutput(request *llm.Request, name string, schema []byte, protocol int) {
	if request.Composition == nil {
		return
	}
	request.Composition.Output = schemaInspection(name, schema)
	request.Composition.SchemaVersion = request.Composition.Output.Version
	if protocol == 0 {
		request.Composition.PromptVersion = postPromptCompositionVersion
	}
}

func observationSources(filenames []string, video bool, attachmentIDs ...map[string]string) []post.OriginSource {
	var catalog originCatalog
	label := "Supplied photo media"
	if video {
		label = "Supplied video media; source time is visual evidence"
	}
	for i, file := range filenames {
		id := ""
		if len(attachmentIDs) == 1 {
			id = attachmentIDs[0][file]
		}
		catalog.add(fmt.Sprintf("media.%d", i), post.OriginSourceVisualObservation, label, file, true, id)
	}
	return catalog.sources
}

const observationOriginContract = `Return optional semantic-origin candidates only in a final top-level origins array after the complete observations array. Each candidate names the exact returned file and field.kind observation_scene, observation_mood, observation_visible_text, observation_object, observation_event or observation_speech; object/event uses zero-based item_index. Quote exact meaningful text and use zero-based occurrence only when repeated. Use photo_interpretation only for meaning tied to an actual supplied media ID; additional interpretation, mood or explanation unsupported by its visible/audible evidence is ai_added. There is no owner factual input in this pass, so never return owner_input. Source IDs are media.0, media.1 etc from the supplied catalog, bound to their exact files. Photos cannot establish taste, smell, events outside the frame, speech or event chronology. Never derive chronology from upload, file-name, stored or capture order. An actual observed source-time video event or speech is distinct evidence. Missing or uncertain origin candidates remain unconfirmed; never add a call to finish them. Do not annotate filenames, rotation, booleans, keys or format enums. Return no post content, storyline, title, tags or prose-revision annotations.`

// Prior context contains current annotated phrases and their retained categories,
// never the full older memory/memo/source text. It widens no revision material.
func priorOriginProjection(review *post.OriginReview) any {
	if review == nil {
		return nil
	}
	return encodeOriginReview(&post.OriginReview{Version: review.Version, Result: review.Result, Spans: review.Spans})
}

func priorPlanProjection(review *PlanOriginReview) any {
	if review == nil {
		return nil
	}
	return encodePlanOrigins(&PlanOriginReview{Version: review.Version, Result: review.Result, Spans: review.Spans})
}

func cloneContentOriginCandidates(values []post.OriginCandidate) []post.OriginCandidate {
	if values == nil {
		return nil
	}
	out := make([]post.OriginCandidate, len(values))
	for i, value := range values {
		out[i] = value
		out[i].Field.TagIndex = cloneOriginIndex(value.Field.TagIndex)
		out[i].Field.BlockIndex = cloneOriginIndex(value.Field.BlockIndex)
		out[i].Field.ItemIndex = cloneOriginIndex(value.Field.ItemIndex)
		out[i].Occurrence = cloneOriginIndex(value.Occurrence)
		out[i].SourceRefs = append([]string(nil), value.SourceRefs...)
	}
	return out
}

func clonePlanOriginCandidates(values []PlanOriginCandidate) []PlanOriginCandidate {
	if values == nil {
		return nil
	}
	out := make([]PlanOriginCandidate, len(values))
	for i, value := range values {
		out[i] = value
		out[i].Occurrence = cloneOriginIndex(value.Occurrence)
		out[i].SourceRefs = append([]string(nil), value.SourceRefs...)
	}
	return out
}

func catalogWithPriorPlan(sources []post.OriginSource, paragraphs []StorylineParagraph, prior *PlanOriginReview, photos, videos []string) []post.OriginSource {
	validated := ValidateStoredPlanOrigins(paragraphs, prior)
	if len(validated.Spans) == 0 {
		return sources
	}
	prior = validated
	catalog := originCatalog{sources: sources}
	for _, source := range sources {
		catalog.chars += utf8.RuneCountInString(source.Text)
	}
	priorSources := OriginSourcesWithAttachments(prior.Sources, photos, videos)
	byID := map[string]post.OriginSource{}
	for _, source := range priorSources {
		byID[source.ID] = source
	}
	for i, span := range prior.Spans {
		if span.ParagraphIndex < 0 || span.ParagraphIndex >= len(paragraphs) {
			continue
		}
		text := []rune(paragraphs[span.ParagraphIndex].Text)
		if span.Start < 0 || span.End <= span.Start || span.End > len(text) || string(text[span.Start:span.End]) != span.Quote {
			continue
		}
		kind, filename, available := inheritedOriginSource(span.Category, span.SourceRefs, byID)
		catalog.add(fmt.Sprintf("prior.plan.%d", i), kind, span.Quote, filename, available, inheritedOriginAttachmentID(span.SourceRefs, byID))
	}
	return catalog.sources
}

func catalogWithPriorContent(sources []post.OriginSource, content PostContent, prior *post.OriginReview, identity *post.OriginResultIdentity, photos, videos []string) []post.OriginSource {
	validated := validatedContentOriginContext(content, prior, identity, photos, videos)
	if validated == nil {
		return sources
	}
	catalog := originCatalog{sources: sources}
	for _, source := range sources {
		catalog.chars += utf8.RuneCountInString(source.Text)
	}
	byID := map[string]post.OriginSource{}
	for _, source := range validated.Sources {
		byID[source.ID] = source
	}
	for i, span := range validated.Spans {
		kind, filename, available := inheritedOriginSource(span.Category, span.SourceRefs, byID)
		catalog.add(fmt.Sprintf("prior.content.%d", i), kind, span.Quote, filename, available, inheritedOriginAttachmentID(span.SourceRefs, byID))
	}
	return catalog.sources
}

func validatedContentOriginContext(content PostContent, prior *post.OriginReview, identity *post.OriginResultIdentity, photos, videos []string) *post.OriginReview {
	if prior == nil {
		return nil
	}
	current := OriginContentIdentity(content)
	if identity != nil {
		current = *identity
	} else if prior.Result.ContentRevision != 0 {
		return nil
	}
	copy := *prior
	copy.Sources = OriginSourcesWithAttachments(copy.Sources, photos, videos)
	validated := post.ValidateOriginReview(originPostContent(content), current, &copy).Review
	byID := make(map[string]post.OriginSource, len(validated.Sources))
	for _, source := range validated.Sources {
		byID[source.ID] = source
	}
	kept := validated.Spans[:0]
	for _, span := range validated.Spans {
		if originSourceCategoryCompatible(span.Category, span.SourceRefs, byID) {
			kept = append(kept, span)
		}
	}
	validated.Spans = kept
	if len(kept) == 0 {
		return nil
	}
	return &validated
}

func inheritedOriginSource(category post.OriginCategory, refs []string, catalog map[string]post.OriginSource) (post.OriginSourceKind, string, bool) {
	if !originSourceCategoryCompatible(category, refs, catalog) {
		return post.OriginSourceAIProposal, "", false
	}
	if category == post.OriginAIAdded {
		filename := ""
		if len(refs) > 0 {
			filename = catalog[refs[0]].AttachmentFilename
		}
		return post.OriginSourceAIProposal, filename, true
	}
	if len(refs) == 0 {
		return post.OriginSourceAIProposal, "", false
	}
	source := catalog[refs[0]]
	return source.Kind, source.AttachmentFilename, source.Available
}

func inheritedOriginAttachmentID(refs []string, catalog map[string]post.OriginSource) string {
	id := ""
	for i, ref := range refs {
		value := catalog[ref].AttachmentID
		if i == 0 {
			id = value
		} else if id != value {
			return ""
		}
	}
	return id
}

func originAttachmentIDs(images []Image) map[string]string {
	ids := make(map[string]string, len(images))
	for _, image := range images {
		ids[image.Filename] = image.ID
	}
	return ids
}

func originExpectedPlanFingerprint(input PostInput) *string {
	if input.StorylineFingerprint == "" {
		return nil
	}
	fingerprint := input.StorylineFingerprint
	return &fingerprint
}

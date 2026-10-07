package post

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// OriginVersion versions result-local origin annotations independently of canonical
// PostContent. None of these checks certifies a meaning's factual or visual support.
const OriginVersion = 1

// Sources are bounded separately from usable content. The text allowance reuses the
// post's largest existing writing-material allowance; a catalog can contain all
// ordinary attachments and authored material without becoming a source history.
const (
	OriginMaxSources         = 128
	OriginMaxSourceTextChars = TargetLengthMax
	OriginMaxSourceIDChars   = 128
	OriginMaxRefsPerSpan     = OriginMaxSources
)

type OriginCategory string

const (
	OriginOwnerInput          OriginCategory = "owner_input"
	OriginPhotoInterpretation OriginCategory = "photo_interpretation"
	OriginAIAdded             OriginCategory = "ai_added"
)

func (c OriginCategory) Valid() bool {
	return c == OriginOwnerInput || c == OriginPhotoInterpretation || c == OriginAIAdded
}

// OriginReviewState is independent of semantic origin. Confirmation records a
// review action, never an assertion that an AI claim became owner-supplied meaning.
type OriginReviewState string

const (
	OriginUnconfirmed OriginReviewState = "unconfirmed"
	OriginUnreviewed  OriginReviewState = "unreviewed"
	OriginConfirmed   OriginReviewState = "confirmed"
)

func (s OriginReviewState) Valid() bool {
	return s == OriginUnconfirmed || s == OriginUnreviewed || s == OriginConfirmed
}

type OriginFieldKind string

const (
	OriginFieldTitle        OriginFieldKind = "title"
	OriginFieldSummary      OriginFieldKind = "summary"
	OriginFieldTag          OriginFieldKind = "tag"
	OriginFieldBlockContent OriginFieldKind = "block_content"
	OriginFieldBlockItem    OriginFieldKind = "block_item"
	OriginFieldBlockAlt     OriginFieldKind = "block_alt"
	OriginFieldBlockCaption OriginFieldKind = "block_caption"
)

// OriginFieldLocator names only human-readable canonical fields. Optional indices
// distinguish omitted members from index zero without adding stable block IDs.
type OriginFieldLocator struct {
	Kind       OriginFieldKind
	TagIndex   *int
	BlockIndex *int
	ItemIndex  *int
}

type OriginResultIdentity struct {
	ContentRevision int64
	ContentHash     string
}

type OriginSourceKind string

const (
	OriginSourceMemo              OriginSourceKind = "memo"
	OriginSourceTemplateAnswer    OriginSourceKind = "template_answer"
	OriginSourceOwnerEdit         OriginSourceKind = "owner_edit"
	OriginSourceMemory            OriginSourceKind = "memory"
	OriginSourceVisualObservation OriginSourceKind = "visual_observation"
	OriginSourceAIProposal        OriginSourceKind = "ai_proposal"
	OriginSourceLiteralText       OriginSourceKind = "literal_text"
)

func (k OriginSourceKind) Valid() bool {
	switch k {
	case OriginSourceMemo, OriginSourceTemplateAnswer, OriginSourceOwnerEdit,
		OriginSourceMemory, OriginSourceVisualObservation, OriginSourceAIProposal, OriginSourceLiteralText:
		return true
	}
	return false
}

// OriginSource is frozen evidence for this result, identified by a result-local
// ID. It contains neither a live source lookup nor a storage key or signed URL.
type OriginSource struct {
	ID                 string
	Kind               OriginSourceKind
	Text               string
	AttachmentFilename string
	Available          bool
}

// OriginSpan uses half-open Unicode scalar offsets into exactly one final field.
type OriginSpan struct {
	Field       OriginFieldLocator
	Start       int
	End         int
	Quote       string
	Category    OriginCategory
	SourceRefs  []string
	ReviewState OriginReviewState
}

// OriginCandidate is model-facing: code resolves exact quotes into offsets.
// Occurrence is zero-based among left-to-right, nonoverlapping exact matches.
type OriginCandidate struct {
	Field      OriginFieldLocator
	Quote      string
	Occurrence *int
	Category   OriginCategory
	SourceRefs []string
}

type OriginReview struct {
	Version int
	Result  OriginResultIdentity
	Sources []OriginSource
	Spans   []OriginSpan
}

type OriginIssueCode string

const (
	OriginIssueMissing       OriginIssueCode = "missing_origin"
	OriginIssueVersion       OriginIssueCode = "unsupported_version"
	OriginIssueStale         OriginIssueCode = "stale_result"
	OriginIssueLocator       OriginIssueCode = "invalid_locator"
	OriginIssueText          OriginIssueCode = "invalid_text"
	OriginIssueEmptyQuote    OriginIssueCode = "empty_quote"
	OriginIssueQuote         OriginIssueCode = "quote_mismatch"
	OriginIssueRange         OriginIssueCode = "invalid_range"
	OriginIssueOccurrence    OriginIssueCode = "invalid_occurrence"
	OriginIssueAmbiguous     OriginIssueCode = "ambiguous_quote"
	OriginIssueCategory      OriginIssueCode = "unsupported_category"
	OriginIssueUnknownSource OriginIssueCode = "unknown_source"
	OriginIssueUnavailable   OriginIssueCode = "unavailable_source"
	OriginIssueReviewState   OriginIssueCode = "invalid_review_state"
	OriginIssueOverlap       OriginIssueCode = "overlapping_span"
	OriginIssueMetadataLimit OriginIssueCode = "metadata_limit"
	OriginIssueSourceCatalog OriginIssueCode = "invalid_source_catalog"
)

type OriginIssue struct {
	// Index is the original candidate/span position; -1 identifies a whole-review issue.
	Index int
	Code  OriginIssueCode
}

// OriginResolution contains only structurally validated, nonconflicting spans.
// Uncovered text and rejected annotations remain unconfirmed; canonical content
// is never changed or rejected by origin validation.
type OriginResolution struct {
	Review OriginReview
	Issues []OriginIssue
}

// OriginFieldText resolves a named field without normalization. Fields belonging
// to another block type, extraneous indices and filenames are never exposed.
func OriginFieldText(content PostContent, field OriginFieldLocator) (string, bool) {
	switch field.Kind {
	case OriginFieldTitle, OriginFieldSummary:
		if field.TagIndex != nil || field.BlockIndex != nil || field.ItemIndex != nil {
			return "", false
		}
		if field.Kind == OriginFieldTitle {
			return content.Title, true
		}
		return content.Summary, true
	case OriginFieldTag:
		if field.BlockIndex != nil || field.ItemIndex != nil || !originIndex(field.TagIndex, len(content.Tags)) {
			return "", false
		}
		return content.Tags[*field.TagIndex], true
	case OriginFieldBlockContent, OriginFieldBlockItem, OriginFieldBlockAlt, OriginFieldBlockCaption:
		if field.TagIndex != nil || !originIndex(field.BlockIndex, len(content.Blocks)) {
			return "", false
		}
		block := content.Blocks[*field.BlockIndex]
		switch field.Kind {
		case OriginFieldBlockContent:
			if field.ItemIndex == nil && (block.Type == BlockText || block.Type == BlockHeading || block.Type == BlockQuote) {
				return block.Content, true
			}
		case OriginFieldBlockItem:
			if block.Type == BlockList && originIndex(field.ItemIndex, len(block.Items)) {
				return block.Items[*field.ItemIndex], true
			}
		case OriginFieldBlockAlt, OriginFieldBlockCaption:
			if field.ItemIndex == nil && (block.Type == BlockImage || block.Type == BlockGallery || block.Type == BlockVideo) {
				if field.Kind == OriginFieldBlockAlt {
					return block.Alt, true
				}
				return block.Caption, true
			}
		}
	}
	return "", false
}

func originIndex(index *int, length int) bool {
	return index != nil && *index >= 0 && *index < length
}

// OriginAnnotationLimit derives the maximum number of nonempty, nonoverlapping
// annotations from this result's readable scalar count, rather than its byte count.
func OriginAnnotationLimit(content PostContent) int {
	count := utf8.RuneCountInString(content.Title) + utf8.RuneCountInString(content.Summary)
	for _, tag := range content.Tags {
		count += utf8.RuneCountInString(tag)
	}
	for _, block := range content.Blocks {
		switch block.Type {
		case BlockText, BlockHeading, BlockQuote:
			count += utf8.RuneCountInString(block.Content)
		case BlockList:
			for _, item := range block.Items {
				count += utf8.RuneCountInString(item)
			}
		case BlockImage, BlockGallery, BlockVideo:
			count += utf8.RuneCountInString(block.Alt) + utf8.RuneCountInString(block.Caption)
		}
	}
	return count
}

// ResolveOriginCandidates matches exact quotes within named final fields. An
// omitted occurrence is accepted only for one match; no first-match fallback exists.
func ResolveOriginCandidates(content PostContent, result OriginResultIdentity, sources []OriginSource, candidates []OriginCandidate) OriginResolution {
	resolved := OriginResolution{Review: OriginReview{Version: OriginVersion, Result: result}}
	if result.ContentHash == "" || result.ContentRevision < 0 {
		return originGlobalIssue(resolved, OriginIssueStale)
	}
	if len(candidates) > OriginAnnotationLimit(content) {
		return originGlobalIssue(resolved, OriginIssueMetadataLimit)
	}
	catalog, issue := originCatalog(sources)
	if issue != "" {
		return originGlobalIssue(resolved, issue)
	}
	resolved.Review.Sources = slices.Clone(sources)
	indexes := make([]int, 0, len(candidates))
	for index, candidate := range candidates {
		span := OriginSpan{Field: candidate.Field, Quote: candidate.Quote, Category: candidate.Category,
			SourceRefs: slices.Clone(candidate.SourceRefs), ReviewState: OriginUnreviewed}
		text, valid := OriginFieldText(content, candidate.Field)
		code := originSpanMetadata(span, text, valid, catalog)
		if code == "" {
			span.Start, span.End, code = originExactRange(text, candidate.Quote, candidate.Occurrence)
		}
		if code != "" {
			resolved.Issues = append(resolved.Issues, OriginIssue{Index: index, Code: code})
			continue
		}
		resolved.Review.Spans = append(resolved.Review.Spans, cloneOriginSpan(span))
		indexes = append(indexes, index)
	}
	return originWithoutConflicts(resolved, indexes)
}

// ValidateOriginReview validates persisted scalar ranges against the current
// result, preserving valid review states without changing semantic categories.
func ValidateOriginReview(content PostContent, current OriginResultIdentity, review *OriginReview) OriginResolution {
	resolved := OriginResolution{Review: OriginReview{Version: OriginVersion, Result: current}}
	if review == nil {
		return originGlobalIssue(resolved, OriginIssueMissing)
	}
	if review.Version != OriginVersion {
		return originGlobalIssue(resolved, OriginIssueVersion)
	}
	if current.ContentHash == "" || current.ContentRevision < 0 || current != review.Result {
		return originGlobalIssue(resolved, OriginIssueStale)
	}
	if len(review.Spans) > OriginAnnotationLimit(content) {
		return originGlobalIssue(resolved, OriginIssueMetadataLimit)
	}
	catalog, issue := originCatalog(review.Sources)
	if issue != "" {
		return originGlobalIssue(resolved, issue)
	}
	resolved.Review.Sources = slices.Clone(review.Sources)
	indexes := make([]int, 0, len(review.Spans))
	for index, span := range review.Spans {
		text, valid := OriginFieldText(content, span.Field)
		code := originSpanMetadata(span, text, valid, catalog)
		if code == "" && !span.ReviewState.Valid() {
			code = OriginIssueReviewState
		}
		if code == "" {
			runes := []rune(text)
			if span.Start < 0 || span.End <= span.Start || span.End > len(runes) {
				code = OriginIssueRange
			} else if string(runes[span.Start:span.End]) != span.Quote {
				code = OriginIssueQuote
			}
		}
		if code != "" {
			resolved.Issues = append(resolved.Issues, OriginIssue{Index: index, Code: code})
			continue
		}
		resolved.Review.Spans = append(resolved.Review.Spans, cloneOriginSpan(span))
		indexes = append(indexes, index)
	}
	return originWithoutConflicts(resolved, indexes)
}

func originGlobalIssue(resolved OriginResolution, code OriginIssueCode) OriginResolution {
	resolved.Issues = []OriginIssue{{Index: -1, Code: code}}
	return resolved
}

func originCatalog(sources []OriginSource) (map[string]OriginSource, OriginIssueCode) {
	if len(sources) > OriginMaxSources {
		return nil, OriginIssueMetadataLimit
	}
	catalog := make(map[string]OriginSource, len(sources))
	for _, source := range sources {
		if utf8.RuneCountInString(source.ID) > OriginMaxSourceIDChars ||
			utf8.RuneCountInString(source.Text) > OriginMaxSourceTextChars ||
			utf8.RuneCountInString(source.AttachmentFilename) > OriginMaxSourceTextChars {
			return nil, OriginIssueMetadataLimit
		}
		if source.ID == "" || !source.Kind.Valid() || !utf8.ValidString(source.ID) ||
			!utf8.ValidString(source.Text) || !utf8.ValidString(source.AttachmentFilename) {
			return nil, OriginIssueSourceCatalog
		}
		if _, duplicate := catalog[source.ID]; duplicate {
			return nil, OriginIssueSourceCatalog
		}
		catalog[source.ID] = source
	}
	return catalog, ""
}

func originSpanMetadata(span OriginSpan, text string, valid bool, catalog map[string]OriginSource) OriginIssueCode {
	if !valid {
		return OriginIssueLocator
	}
	if !utf8.ValidString(text) || !utf8.ValidString(span.Quote) {
		return OriginIssueText
	}
	if span.Quote == "" {
		return OriginIssueEmptyQuote
	}
	if !span.Category.Valid() {
		return OriginIssueCategory
	}
	if len(span.SourceRefs) > OriginMaxRefsPerSpan {
		return OriginIssueMetadataLimit
	}
	if len(span.SourceRefs) == 0 && span.Category != OriginAIAdded {
		return OriginIssueUnknownSource
	}
	seen := make(map[string]struct{}, len(span.SourceRefs))
	for _, ref := range span.SourceRefs {
		source, known := catalog[ref]
		if _, duplicate := seen[ref]; duplicate || !known {
			return OriginIssueUnknownSource
		}
		if !source.Available {
			return OriginIssueUnavailable
		}
		seen[ref] = struct{}{}
	}
	return ""
}

func originExactRange(text, quote string, occurrence *int) (int, int, OriginIssueCode) {
	if occurrence != nil && *occurrence < 0 {
		return 0, 0, OriginIssueOccurrence
	}
	quoteScalars := utf8.RuneCountInString(quote)
	bytePosition, scalarPosition, matches, first := 0, 0, 0, 0
	for bytePosition < len(text) {
		relative := strings.Index(text[bytePosition:], quote)
		if relative < 0 {
			break
		}
		matchByte := bytePosition + relative
		start := scalarPosition + utf8.RuneCountInString(text[bytePosition:matchByte])
		if occurrence != nil && matches == *occurrence {
			return start, start + quoteScalars, ""
		}
		matches++
		if occurrence == nil {
			if matches > 1 {
				return 0, 0, OriginIssueAmbiguous
			}
			first = start
		}
		bytePosition, scalarPosition = matchByte+len(quote), start+quoteScalars
	}
	if matches == 0 {
		if occurrence != nil {
			return 0, 0, OriginIssueOccurrence
		}
		return 0, 0, OriginIssueQuote
	}
	if occurrence == nil {
		return first, first + quoteScalars, ""
	}
	return 0, 0, OriginIssueOccurrence
}

func cloneOriginSpan(span OriginSpan) OriginSpan {
	span.SourceRefs = slices.Clone(span.SourceRefs)
	clone := func(index *int) *int {
		if index == nil {
			return nil
		}
		value := *index
		return &value
	}
	span.Field.TagIndex = clone(span.Field.TagIndex)
	span.Field.BlockIndex = clone(span.Field.BlockIndex)
	span.Field.ItemIndex = clone(span.Field.ItemIndex)
	return span
}

type originFieldKey struct {
	kind             OriginFieldKind
	tag, block, item int
}

func originKey(field OriginFieldLocator) originFieldKey {
	value := func(index *int) int {
		if index == nil {
			return -1
		}
		return *index
	}
	return originFieldKey{field.Kind, value(field.TagIndex), value(field.BlockIndex), value(field.ItemIndex)}
}

func originWithoutConflicts(resolved OriginResolution, indexes []int) OriginResolution {
	fields := make(map[originFieldKey][]int)
	for index, span := range resolved.Review.Spans {
		key := originKey(span.Field)
		fields[key] = append(fields[key], index)
	}
	conflicting := make(map[int]bool)
	for _, group := range fields {
		sort.Slice(group, func(i, j int) bool {
			left, right := resolved.Review.Spans[group[i]], resolved.Review.Spans[group[j]]
			if left.Start != right.Start {
				return left.Start < right.Start
			}
			return left.End < right.End
		})
		// A connected overlap cluster invalidates every member, including a long
		// span that contains several disjoint short spans. Adjacent spans are valid.
		for begin := 0; begin < len(group); {
			end, furthest := begin+1, resolved.Review.Spans[group[begin]].End
			for end < len(group) && resolved.Review.Spans[group[end]].Start < furthest {
				furthest = max(furthest, resolved.Review.Spans[group[end]].End)
				end++
			}
			if end-begin > 1 {
				for _, index := range group[begin:end] {
					conflicting[index] = true
				}
			}
			begin = end
		}
	}
	spans := make([]OriginSpan, 0, len(resolved.Review.Spans))
	for index, span := range resolved.Review.Spans {
		if conflicting[index] {
			resolved.Issues = append(resolved.Issues, OriginIssue{Index: indexes[index], Code: OriginIssueOverlap})
		} else {
			spans = append(spans, span)
		}
	}
	resolved.Review.Spans = spans
	sort.SliceStable(resolved.Issues, func(i, j int) bool { return resolved.Issues[i].Index < resolved.Issues[j].Index })
	return resolved
}

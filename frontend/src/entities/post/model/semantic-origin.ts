import {
  ORIGIN_MAX_SOURCES,
  ORIGIN_MAX_REFS_PER_SPAN,
  ORIGIN_REVIEW_VERSION,
  ORIGIN_SOURCE_ID_MAX_SCALARS,
  ORIGIN_SOURCE_TEXT_MAX_SCALARS,
} from '../config/semantic-origin'

export const SEMANTIC_ORIGIN_CATEGORIES = [
  'owner_input',
  'photo_interpretation',
  'ai_added',
] as const
export type SemanticOriginCategory = (typeof SEMANTIC_ORIGIN_CATEGORIES)[number]

/** Review is independent of the source of meaning; confirmation never changes its category. */
export const ORIGIN_REVIEW_STATES = ['unconfirmed', 'unreviewed', 'confirmed'] as const
export type OriginReviewState = (typeof ORIGIN_REVIEW_STATES)[number]

export const ORIGIN_SOURCE_KINDS = [
  'memo',
  'template_answer',
  'owner_edit',
  'memory',
  'visual_observation',
  'ai_proposal',
  'literal_text',
] as const
export type OriginSourceKind = (typeof ORIGIN_SOURCE_KINDS)[number]

export type OriginFieldLocator =
  | { kind: 'title' }
  | { kind: 'summary' }
  | { kind: 'tag'; tagIndex: number }
  | { kind: 'block_content'; blockIndex: number }
  | { kind: 'block_item'; blockIndex: number; itemIndex: number }
  | { kind: 'block_alt'; blockIndex: number }
  | { kind: 'block_caption'; blockIndex: number }

export interface OriginResultIdentity {
  contentRevision: bigint
  contentHash: string
}

/** Frozen evidence belongs to this result, rather than a live setting or attachment lookup. */
export interface OriginSource {
  id: string
  kind: OriginSourceKind
  text: string
  attachmentFilename: string
  available: boolean
}

/** Persisted half-open offsets count Unicode scalars, never UTF-16 code units. */
export interface OriginSpan {
  field: OriginFieldLocator
  start: number
  end: number
  quote: string
  category: SemanticOriginCategory
  sourceRefs: string[]
  reviewState: OriginReviewState
}

export interface OriginCandidate {
  field: OriginFieldLocator
  quote: string
  /** Zero-based, left-to-right, nonoverlapping exact matches. Omitted repeats are ambiguous. */
  occurrence?: number
  category: SemanticOriginCategory
  sourceRefs: string[]
}

export interface OriginReview {
  version: number
  result: OriginResultIdentity
  sources: OriginSource[]
  spans: OriginSpan[]
}

export type OriginIssueCode =
  | 'missing_origin'
  | 'unsupported_version'
  | 'stale_result'
  | 'invalid_locator'
  | 'invalid_text'
  | 'empty_quote'
  | 'quote_mismatch'
  | 'invalid_range'
  | 'invalid_occurrence'
  | 'ambiguous_quote'
  | 'unsupported_category'
  | 'unknown_source'
  | 'unavailable_source'
  | 'invalid_review_state'
  | 'overlapping_span'
  | 'metadata_limit'
  | 'invalid_source_catalog'

export interface OriginIssue {
  /** Original candidate/span index; -1 means the result or catalog as a whole. */
  index: number
  code: OriginIssueCode
}

export interface OriginValidation {
  /** Only text-aligned, supported, nonconflicting spans. Uncovered text stays unconfirmed. */
  spans: OriginSpan[]
  issues: OriginIssue[]
}

/** Numeric canonical vocabulary is pinned against the generated enum at the API boundary. */
export const ORIGIN_BLOCK_TYPES = {
  text: 1,
  heading: 2,
  image: 3,
  quote: 4,
  list: 5,
  video: 6,
  gallery: 7,
} as const

/** Structural view of the existing flat canonical content, with no annotation markers. */
export interface OriginContent {
  title: string
  summary: string
  tags: readonly string[]
  blocks: readonly {
    type: number
    content?: string
    items?: readonly string[]
    alt?: string
    caption?: string
  }[]
}

export interface OriginRange {
  start: number
  end: number
}

function isIndex(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

/** Reject lone surrogates instead of silently counting the replacement character as evidence. */
function scalarBoundaries(text: string): number[] | undefined {
  const offsets = [0]
  for (let index = 0; index < text.length;) {
    const unit = text.charCodeAt(index)
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = text.charCodeAt(index + 1)
      if (!(next >= 0xdc00 && next <= 0xdfff)) return undefined
      index += 2
    } else {
      if (unit >= 0xdc00 && unit <= 0xdfff) return undefined
      index += 1
    }
    offsets.push(index)
  }
  return offsets
}

export function scalarRangeToUtf16(text: string, range: OriginRange): OriginRange | undefined {
  const offsets = scalarBoundaries(text)
  if (
    !offsets ||
    !isIndex(range.start) ||
    !isIndex(range.end) ||
    range.start > range.end ||
    range.end >= offsets.length
  )
    return undefined
  return { start: offsets[range.start]!, end: offsets[range.end]! }
}

export function utf16RangeToScalar(text: string, range: OriginRange): OriginRange | undefined {
  const offsets = scalarBoundaries(text)
  if (!offsets || !isIndex(range.start) || !isIndex(range.end) || range.start > range.end)
    return undefined
  const start = offsets.indexOf(range.start)
  const end = offsets.indexOf(range.end)
  return start < 0 || end < 0 ? undefined : { start, end }
}

/** Only the readable members declared by the canonical block type can hold an origin. */
export function originFieldText(
  content: OriginContent,
  field: OriginFieldLocator,
): string | undefined {
  if (!field || typeof field !== 'object') return undefined
  const members = Object.keys(field)
  const hasOnly = (...keys: string[]) => members.every((key) => keys.includes(key))
  switch (field.kind) {
    case 'title':
      return hasOnly('kind') ? content.title : undefined
    case 'summary':
      return hasOnly('kind') ? content.summary : undefined
    case 'tag':
      return hasOnly('kind', 'tagIndex') && isIndex(field.tagIndex)
        ? content.tags[field.tagIndex]
        : undefined
    case 'block_content':
    case 'block_item':
    case 'block_alt':
    case 'block_caption': {
      const keys =
        field.kind === 'block_item' ? ['kind', 'blockIndex', 'itemIndex'] : ['kind', 'blockIndex']
      if (!hasOnly(...keys) || !isIndex(field.blockIndex)) return undefined
      const block = content.blocks[field.blockIndex]
      if (!block) return undefined
      if (field.kind === 'block_content') {
        return [ORIGIN_BLOCK_TYPES.text, ORIGIN_BLOCK_TYPES.heading, ORIGIN_BLOCK_TYPES.quote].some(
          (type) => type === block.type,
        )
          ? (block.content ?? '')
          : undefined
      }
      if (field.kind === 'block_item') {
        return block.type === ORIGIN_BLOCK_TYPES.list && isIndex(field.itemIndex)
          ? block.items?.[field.itemIndex]
          : undefined
      }
      if (
        ![ORIGIN_BLOCK_TYPES.image, ORIGIN_BLOCK_TYPES.gallery, ORIGIN_BLOCK_TYPES.video].some(
          (type) => type === block.type,
        )
      )
        return undefined
      return (field.kind === 'block_alt' ? block.alt : block.caption) ?? ''
    }
  }
}

function fieldKey(field: OriginFieldLocator): string {
  switch (field.kind) {
    case 'title':
    case 'summary':
      return field.kind
    case 'tag':
      return `tag:${field.tagIndex}`
    case 'block_item':
      return `block_item:${field.blockIndex}:${field.itemIndex}`
    case 'block_content':
    case 'block_alt':
    case 'block_caption':
      return `${field.kind}:${field.blockIndex}`
  }
}

function readableScalarCount(content: OriginContent): number {
  const fields = [content.title, content.summary, ...content.tags]
  for (const block of content.blocks) {
    switch (block.type) {
      case ORIGIN_BLOCK_TYPES.text:
      case ORIGIN_BLOCK_TYPES.heading:
      case ORIGIN_BLOCK_TYPES.quote:
        fields.push(block.content ?? '')
        break
      case ORIGIN_BLOCK_TYPES.list:
        fields.push(...(block.items ?? []))
        break
      case ORIGIN_BLOCK_TYPES.image:
      case ORIGIN_BLOCK_TYPES.gallery:
      case ORIGIN_BLOCK_TYPES.video:
        fields.push(block.alt ?? '', block.caption ?? '')
        break
    }
  }
  return fields.reduce((total, text) => total + (scalarBoundaries(text)?.length ?? 1) - 1, 0)
}

function catalogIssue(sources: readonly OriginSource[]): OriginIssueCode | undefined {
  if (sources.length > ORIGIN_MAX_SOURCES) return 'metadata_limit'
  const seen = new Set<string>()
  for (const source of sources) {
    const id = typeof source.id === 'string' ? scalarBoundaries(source.id) : undefined
    const text = typeof source.text === 'string' ? scalarBoundaries(source.text) : undefined
    const filename =
      typeof source.attachmentFilename === 'string'
        ? scalarBoundaries(source.attachmentFilename)
        : undefined
    if (
      !id ||
      id.length <= 1 ||
      !text ||
      !filename ||
      seen.has(source.id) ||
      !(ORIGIN_SOURCE_KINDS as readonly unknown[]).includes(source.kind) ||
      typeof source.available !== 'boolean'
    )
      return 'invalid_source_catalog'
    if (
      id.length - 1 > ORIGIN_SOURCE_ID_MAX_SCALARS ||
      text.length - 1 > ORIGIN_SOURCE_TEXT_MAX_SCALARS ||
      filename.length - 1 > ORIGIN_SOURCE_TEXT_MAX_SCALARS
    )
      return 'metadata_limit'
    seen.add(source.id)
  }
}

function annotationIssue(
  annotation: Pick<OriginCandidate, 'category' | 'sourceRefs'>,
  sources: readonly OriginSource[],
): OriginIssueCode | undefined {
  if (!(SEMANTIC_ORIGIN_CATEGORIES as readonly unknown[]).includes(annotation.category))
    return 'unsupported_category'
  if (!Array.isArray(annotation.sourceRefs)) return 'unknown_source'
  if (annotation.sourceRefs.length > ORIGIN_MAX_REFS_PER_SPAN) return 'metadata_limit'
  if (annotation.category !== 'ai_added' && annotation.sourceRefs.length === 0)
    return 'unknown_source'
  const seen = new Set<string>()
  for (const ref of annotation.sourceRefs) {
    const source = sources.find((entry) => entry.id === ref)
    if (!source || seen.has(ref)) return 'unknown_source'
    if (!source.available) return 'unavailable_source'
    seen.add(ref)
  }
}

function globalIssue(code: OriginIssueCode): OriginValidation {
  return { spans: [], issues: [{ index: -1, code }] }
}

/** Overlaps invalidate every involved span; array order never decides which origin wins. */
function withoutConflicts(
  entries: { index: number; span: OriginSpan }[],
  issues: OriginIssue[],
): OriginValidation {
  const conflicting = new Set<number>()
  const fields = new Map<string, typeof entries>()
  for (const entry of entries) {
    const key = fieldKey(entry.span.field)
    const field = fields.get(key) ?? []
    field.push(entry)
    fields.set(key, field)
  }
  for (const field of fields.values()) {
    field.sort(
      (left, right) => left.span.start - right.span.start || left.span.end - right.span.end,
    )
    let component: number[] = []
    let end = -1
    const flush = () => {
      if (component.length > 1) for (const index of component) conflicting.add(index)
      component = []
    }
    for (const entry of field) {
      if (entry.span.start >= end) flush()
      component.push(entry.index)
      end = Math.max(end, entry.span.end)
    }
    flush()
  }
  for (const index of conflicting) issues.push({ index, code: 'overlapping_span' })
  issues.sort((left, right) => left.index - right.index)
  return {
    spans: entries.filter((entry) => !conflicting.has(entry.index)).map((entry) => entry.span),
    issues,
  }
}

/** Resolve exact quotes in the final named field; no normalization or guessed first match. */
export function resolveOriginCandidates(
  content: OriginContent,
  sources: readonly OriginSource[],
  candidates: readonly OriginCandidate[],
): OriginValidation {
  if (candidates.length > readableScalarCount(content)) return globalIssue('metadata_limit')
  const catalog = catalogIssue(sources)
  if (catalog) return globalIssue(catalog)
  const entries: { index: number; span: OriginSpan }[] = []
  const issues: OriginIssue[] = []
  candidates.forEach((candidate, index) => {
    const reject = (code: OriginIssueCode) => issues.push({ index, code })
    const text = originFieldText(content, candidate.field)
    if (text === undefined) return reject('invalid_locator')
    if (
      !scalarBoundaries(text) ||
      typeof candidate.quote !== 'string' ||
      !scalarBoundaries(candidate.quote)
    )
      return reject('invalid_text')
    if (!candidate.quote) return reject('empty_quote')
    const annotation = annotationIssue(candidate, sources)
    if (annotation) return reject(annotation)
    if (candidate.occurrence !== undefined && !isIndex(candidate.occurrence))
      return reject('invalid_occurrence')
    const matches: number[] = []
    for (let from = 0; ;) {
      const match = text.indexOf(candidate.quote, from)
      if (match < 0) break
      matches.push(match)
      from = match + candidate.quote.length
    }
    if (matches.length === 0)
      return reject(candidate.occurrence === undefined ? 'quote_mismatch' : 'invalid_occurrence')
    if (candidate.occurrence === undefined && matches.length !== 1) return reject('ambiguous_quote')
    const start = matches[candidate.occurrence ?? 0]
    if (start === undefined) return reject('invalid_occurrence')
    const range = utf16RangeToScalar(text, { start, end: start + candidate.quote.length })!
    entries.push({
      index,
      span: {
        ...range,
        field: { ...candidate.field },
        quote: candidate.quote,
        category: candidate.category,
        sourceRefs: [...candidate.sourceRefs],
        reviewState: 'unreviewed',
      },
    })
  })
  return withoutConflicts(entries, issues)
}

/** Mechanical correspondence checks do not certify a model's semantic or visual claim. */
export function validateOriginSpans(
  content: OriginContent,
  sources: readonly OriginSource[],
  spans: readonly OriginSpan[],
): OriginValidation {
  if (spans.length > readableScalarCount(content)) return globalIssue('metadata_limit')
  const catalog = catalogIssue(sources)
  if (catalog) return globalIssue(catalog)
  const entries: { index: number; span: OriginSpan }[] = []
  const issues: OriginIssue[] = []
  spans.forEach((span, index) => {
    const reject = (code: OriginIssueCode) => issues.push({ index, code })
    const text = originFieldText(content, span.field)
    if (text === undefined) return reject('invalid_locator')
    if (!scalarBoundaries(text) || typeof span.quote !== 'string' || !scalarBoundaries(span.quote))
      return reject('invalid_text')
    if (!span.quote) return reject('empty_quote')
    const annotation = annotationIssue(span, sources)
    if (annotation) return reject(annotation)
    if (!(ORIGIN_REVIEW_STATES as readonly unknown[]).includes(span.reviewState))
      return reject('invalid_review_state')
    const range = scalarRangeToUtf16(text, span)
    if (!range || span.start === span.end) return reject('invalid_range')
    if (text.slice(range.start, range.end) !== span.quote) return reject('quote_mismatch')
    entries.push({
      index,
      span: { ...span, field: { ...span.field }, sourceRefs: [...span.sourceRefs] },
    })
  })
  return withoutConflicts(entries, issues)
}

/** Absent legacy annotations and a different revision/hash never assert historical origins. */
export function validateOriginReview(
  content: OriginContent,
  current: OriginResultIdentity,
  review?: OriginReview,
): OriginValidation {
  if (!review) return globalIssue('missing_origin')
  if (review.version !== ORIGIN_REVIEW_VERSION) return globalIssue('unsupported_version')
  if (
    !current.contentHash ||
    current.contentRevision < 0n ||
    current.contentRevision !== review.result.contentRevision ||
    current.contentHash !== review.result.contentHash
  )
    return globalIssue('stale_result')
  return validateOriginSpans(content, review.sources, review.spans)
}

import {
  ORIGIN_BLOCK_TYPES,
  originFieldText,
  scalarRangeToUtf16,
  validateOriginReview,
  type OriginContent,
  type OriginFieldLocator,
  type OriginIssueCode,
  type OriginResultIdentity,
  type OriginReview,
  type OriginReviewState,
  type OriginSpan,
  type SemanticOriginCategory,
} from './semantic-origin'

/** Nonempty named canonical fields in reading order; excludes filenames and format metadata. */
export function originReadableFields(content: OriginContent): OriginFieldLocator[] {
  const fields: OriginFieldLocator[] = [
    { kind: 'title' },
    { kind: 'summary' },
    ...content.tags.map((_tag, tagIndex) => ({ kind: 'tag' as const, tagIndex })),
  ]
  content.blocks.forEach((block, blockIndex) => {
    switch (block.type) {
      case ORIGIN_BLOCK_TYPES.text:
      case ORIGIN_BLOCK_TYPES.heading:
      case ORIGIN_BLOCK_TYPES.quote:
        fields.push({ kind: 'block_content', blockIndex })
        break
      case ORIGIN_BLOCK_TYPES.list:
        for (let itemIndex = 0; itemIndex < (block.items?.length ?? 0); itemIndex += 1)
          fields.push({ kind: 'block_item', blockIndex, itemIndex })
        break
      case ORIGIN_BLOCK_TYPES.image:
      case ORIGIN_BLOCK_TYPES.gallery:
      case ORIGIN_BLOCK_TYPES.video:
        fields.push({ kind: 'block_alt', blockIndex }, { kind: 'block_caption', blockIndex })
        break
    }
  })
  return fields.filter((field) => Boolean(originFieldText(content, field)))
}

export interface OriginFieldSegment {
  /** Exact canonical text. Offsets count Unicode scalars, rather than UTF-16 units. */
  text: string
  start: number
  end: number
  state: 'supported' | 'unconfirmed'
  /** Present only when current, independently validated evidence supports this phrase. */
  category?: SemanticOriginCategory
  sourceRefs: string[]
  reviewState: OriginReviewState
  reason?: OriginIssueCode | 'pending'
}

type CanonicalBlock = OriginContent['blocks'][number] & {
  file?: string
  files?: readonly string[]
  level?: number
  layout?: number
}

function stringsMatch(left: readonly string[] = [], right: readonly string[] = []): boolean {
  return left.length === right.length && left.every((text, index) => text === right[index])
}

/** Compare the canonical document, including media identity and ordering, without annotations. */
export function originContentMatches(
  left: OriginContent | undefined,
  right: OriginContent | undefined,
): boolean {
  if (left === right) return true
  if (!left || !right) return false
  return (
    left.title === right.title &&
    left.summary === right.summary &&
    stringsMatch(left.tags, right.tags) &&
    left.blocks.length === right.blocks.length &&
    left.blocks.every((value, index) => {
      const block = value as CanonicalBlock
      const other = right.blocks[index] as CanonicalBlock
      return (
        block.type === other.type &&
        (block.content ?? '') === (other.content ?? '') &&
        stringsMatch(block.items, other.items) &&
        (block.alt ?? '') === (other.alt ?? '') &&
        (block.caption ?? '') === (other.caption ?? '') &&
        (block.file ?? '') === (other.file ?? '') &&
        stringsMatch(block.files, other.files) &&
        (block.level ?? 0) === (other.level ?? 0) &&
        (block.layout ?? 0) === (other.layout ?? 0)
      )
    })
  )
}

function sameField(left: OriginFieldLocator, right: OriginFieldLocator): boolean {
  if (!left || left.kind !== right.kind) return false
  switch (left.kind) {
    case 'title':
    case 'summary':
      return true
    case 'tag':
      return right.kind === 'tag' && left.tagIndex === right.tagIndex
    case 'block_item':
      return (
        right.kind === 'block_item' &&
        left.blockIndex === right.blockIndex &&
        left.itemIndex === right.itemIndex
      )
    default:
      return 'blockIndex' in right && left.blockIndex === right.blockIndex
  }
}

/** Project evidence over the exact displayed field; this never aligns or classifies new prose. */
export function originFieldSegments(
  content: OriginContent,
  current: OriginResultIdentity,
  review: OriginReview | undefined,
  field: OriginFieldLocator,
  options: { pending?: boolean; canonicalContent?: OriginContent } = {},
): OriginFieldSegment[] {
  const text = originFieldText(content, field)
  if (typeof text !== 'string' || !text) return []
  const length = Array.from(text).length
  const unconfirmed = (start: number, end: number, reason?: OriginFieldSegment['reason']) => {
    const range = scalarRangeToUtf16(text, { start, end })
    return {
      text: text.slice(range?.start ?? 0, range?.end ?? text.length),
      start,
      end,
      state: 'unconfirmed' as const,
      sourceRefs: [],
      reviewState: 'unconfirmed' as const,
      reason,
    }
  }
  if (
    options.pending ||
    (options.canonicalContent && !originContentMatches(content, options.canonicalContent))
  )
    return [unconfirmed(0, length, 'pending')]

  // A malformed sidecar must never interrupt rendering of the usable canonical document.
  let validated: ReturnType<typeof validateOriginReview>
  try {
    validated = validateOriginReview(content, current, review)
  } catch {
    return [unconfirmed(0, length, 'invalid_source_catalog')]
  }
  const globalIssue = validated.issues.find((issue) => issue.index === -1)
  if (globalIssue) return [unconfirmed(0, length, globalIssue.code)]

  const spans = validated.spans
    .filter((span) => sameField(span.field, field))
    .sort((left, right) => left.start - right.start)
  const explanations: { span: OriginSpan; reason: OriginIssueCode }[] = []
  for (const issue of validated.issues) {
    const span = review?.spans[issue.index]
    if (!span || !sameField(span.field, field)) continue
    const range = scalarRangeToUtf16(text, span)
    if (range && span.start < span.end && text.slice(range.start, range.end) === span.quote)
      explanations.push({ span, reason: issue.code })
  }

  const result: OriginFieldSegment[] = []
  const appendGap = (start: number, end: number) => {
    const points = new Set([start, end])
    for (const { span } of explanations) {
      if (span.start > start && span.start < end) points.add(span.start)
      if (span.end > start && span.end < end) points.add(span.end)
    }
    const boundaries = [...points].sort((left, right) => left - right)
    for (let index = 1; index < boundaries.length; index += 1) {
      const from = boundaries[index - 1]!
      const to = boundaries[index]!
      const reason = explanations.find(({ span }) => span.start <= from && span.end >= to)?.reason
      result.push(unconfirmed(from, to, reason ?? 'missing_origin'))
    }
  }
  let cursor = 0
  for (const span of spans) {
    if (cursor < span.start) appendGap(cursor, span.start)
    if (span.reviewState === 'unconfirmed') {
      result.push(unconfirmed(span.start, span.end, 'missing_origin'))
    } else {
      const range = scalarRangeToUtf16(text, span)!
      result.push({
        text: text.slice(range.start, range.end),
        start: span.start,
        end: span.end,
        state: 'supported',
        category: span.category,
        sourceRefs: [...span.sourceRefs],
        reviewState: span.reviewState,
      })
    }
    cursor = span.end
  }
  if (cursor < length) appendGap(cursor, length)
  return result
}

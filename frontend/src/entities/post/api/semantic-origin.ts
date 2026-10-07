import {
  originFieldKindFromProto,
  originReviewStateFromProto,
  semanticOriginCategoryFromProto,
  type OriginReview as ProtoOriginReview,
} from '@/shared/api'
import {
  validateOriginReview,
  type OriginContent,
  type OriginFieldLocator,
  type OriginResultIdentity,
  type OriginReview,
  type OriginReviewState,
  type OriginSourceKind,
  type SemanticOriginCategory,
} from '../model/semantic-origin'

/** Keep unknown metadata invalid for independent validation; do not guess its origin. */
export function originReviewFromProto(value?: ProtoOriginReview): OriginReview | undefined {
  if (!value) return undefined
  return {
    version: value.version,
    result: {
      contentRevision: value.result?.contentRevision ?? -1n,
      contentHash: value.result?.contentHash ?? '',
    },
    sources: value.sources.map((source) => ({
      id: source.id,
      kind: source.kind as OriginSourceKind,
      text: source.text,
      attachmentFilename: source.attachmentFilename,
      ...(source.attachmentId ? { attachmentId: source.attachmentId } : {}),
      available: source.available,
    })),
    spans: value.spans.map((span) => ({
      field: {
        kind: span.field ? originFieldKindFromProto(span.field.kind) : undefined,
        ...(span.field?.tagIndex !== undefined ? { tagIndex: span.field.tagIndex } : {}),
        ...(span.field?.blockIndex !== undefined ? { blockIndex: span.field.blockIndex } : {}),
        ...(span.field?.itemIndex !== undefined ? { itemIndex: span.field.itemIndex } : {}),
      } as OriginFieldLocator,
      start: span.start,
      end: span.end,
      quote: span.quote,
      category: semanticOriginCategoryFromProto(span.category) as SemanticOriginCategory,
      sourceRefs: [...span.sourceRefs],
      reviewState: originReviewStateFromProto(span.reviewState) as OriginReviewState,
    })),
  }
}

export function validateOriginReviewFromProto(
  content: OriginContent,
  current: OriginResultIdentity,
  value?: ProtoOriginReview,
) {
  return validateOriginReview(content, current, originReviewFromProto(value))
}

/** Discard another result's metadata while preserving usable canonical content. */
export function alignedOriginReviewFromProto(
  content: OriginContent | undefined,
  current: OriginResultIdentity,
  value?: ProtoOriginReview,
): OriginReview | undefined {
  const review = originReviewFromProto(value)
  if (!content || !review) return undefined
  const validated = validateOriginReview(content, current, review)
  if (validated.issues.some((issue) => issue.index === -1)) return undefined
  return { ...review, spans: validated.spans }
}

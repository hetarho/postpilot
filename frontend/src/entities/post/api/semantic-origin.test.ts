import { create, fromBinary, toBinary } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  BlockType,
  OriginFieldKind,
  OriginReviewSchema,
  OriginReviewState as ProtoReviewState,
  SemanticOriginCategory as ProtoCategory,
  type OriginReview as ProtoOriginReview,
} from '@/shared/api'
import {
  ORIGIN_BLOCK_TYPES,
  scalarRangeToUtf16,
  type OriginContent,
  type OriginResultIdentity,
} from '../model/semantic-origin'
import {
  alignedOriginReviewFromProto,
  originReviewFromProto,
  validateOriginReviewFromProto,
} from './semantic-origin'

const content: OriginContent = {
  title: '가😀끝',
  summary: '',
  tags: [],
  blocks: [],
}
const current: OriginResultIdentity = { contentRevision: 7n, contentHash: 'current-hash' }

function annotated(): ProtoOriginReview {
  return create(OriginReviewSchema, {
    version: 1,
    result: current,
    sources: [
      {
        id: 'visual-1',
        kind: 'visual_observation',
        text: '웃는 얼굴',
        attachmentFilename: 'face.jpg',
        available: true,
      },
    ],
    spans: [
      {
        field: { kind: OriginFieldKind.TITLE },
        start: 1,
        end: 2,
        quote: '😀',
        category: ProtoCategory.PHOTO_INTERPRETATION,
        sourceRefs: ['visual-1'],
        reviewState: ProtoReviewState.CONFIRMED,
      },
    ],
  })
}

describe('semantic origin API boundary', () => {
  it('returns only the current result review and retains validated scalar spans', () => {
    const review = alignedOriginReviewFromProto(content, current, annotated())
    expect(review?.result).toEqual(current)
    expect(review?.spans[0]?.quote).toBe('😀')
    expect(review?.spans[0]?.reviewState).toBe('confirmed')
    const stale = annotated()
    stale.result!.contentRevision++
    expect(alignedOriginReviewFromProto(content, current, stale)).toBeUndefined()
    expect(alignedOriginReviewFromProto(content, current)).toBeUndefined()
    expect(alignedOriginReviewFromProto(undefined, current, annotated())).toBeUndefined()
  })

  it('drops invalid annotations without discarding valid current content evidence', () => {
    const malformed = annotated()
    malformed.spans[0]!.quote = 'another result'
    const review = alignedOriginReviewFromProto(content, current, malformed)
    expect(review?.result).toEqual(current)
    expect(review?.spans).toEqual([])
    malformed.sources[0]!.kind = 'voice_example'
    expect(alignedOriginReviewFromProto(content, current, malformed)).toBeUndefined()
  })

  it('pins every canonical block kind used by the pure field locator against the generated enum', () => {
    const known = Object.fromEntries(
      Object.entries(BlockType)
        .filter(([, value]) => typeof value === 'number' && value !== 0)
        .map(([name, value]) => [name.toLowerCase(), value]),
    )
    expect(ORIGIN_BLOCK_TYPES).toEqual(known)
  })

  it('round-trips scalar ranges and keeps semantic category independent of owner confirmation', () => {
    const wire = fromBinary(OriginReviewSchema, toBinary(OriginReviewSchema, annotated()))
    const domain = originReviewFromProto(wire)!
    expect(domain.result).toEqual(current)
    expect(domain.sources[0]?.attachmentFilename).toBe('face.jpg')
    expect(domain.spans[0]).toEqual({
      field: { kind: 'title' },
      start: 1,
      end: 2,
      quote: '😀',
      category: 'photo_interpretation',
      sourceRefs: ['visual-1'],
      reviewState: 'confirmed',
    })
    const validated = validateOriginReviewFromProto(content, current, wire)
    expect(validated.issues).toEqual([])
    expect(scalarRangeToUtf16(content.title, validated.spans[0]!)).toEqual({ start: 1, end: 3 })
  })

  it('treats legacy absence as unavailable origin evidence with unconfirmed text', () => {
    expect(originReviewFromProto()).toBeUndefined()
    expect(validateOriginReviewFromProto(content, current)).toEqual({
      spans: [],
      issues: [{ index: -1, code: 'missing_origin' }],
    })
  })

  it.each([ProtoCategory.UNSPECIFIED, 999 as ProtoCategory])(
    'rejects unsupported category %s without guessing a category',
    (category) => {
      const value = annotated()
      value.spans[0]!.category = category
      expect(validateOriginReviewFromProto(content, current, value)).toEqual({
        spans: [],
        issues: [{ index: 0, code: 'unsupported_category' }],
      })
    },
  )

  it('rejects unsupported source kinds and unknown source references', () => {
    const value = annotated()
    value.sources[0]!.kind = 'voice_example'
    expect(validateOriginReviewFromProto(content, current, value).issues).toEqual([
      { index: -1, code: 'invalid_source_catalog' },
    ])
    const unknown = annotated()
    unknown.spans[0]!.sourceRefs = ['different-result']
    expect(validateOriginReviewFromProto(content, current, unknown).issues).toEqual([
      { index: 0, code: 'unknown_source' },
    ])
  })

  it('preserves unexpected locator members for validation rather than dropping them', () => {
    const value = annotated()
    value.spans[0]!.field!.blockIndex = 0
    const wire = fromBinary(OriginReviewSchema, toBinary(OriginReviewSchema, value))
    expect(originReviewFromProto(wire)?.spans[0]?.field).toEqual({ kind: 'title', blockIndex: 0 })
    expect(validateOriginReviewFromProto(content, current, wire).issues).toEqual([
      { index: 0, code: 'invalid_locator' },
    ])
  })

  it('leaves malformed current identities, unsupported review states and missing field locators unconfirmed', () => {
    const value = annotated()
    value.result = undefined
    expect(validateOriginReviewFromProto(content, current, value).issues).toEqual([
      { index: -1, code: 'stale_result' },
    ])
    const invalidState = annotated()
    invalidState.spans[0]!.reviewState = ProtoReviewState.UNSPECIFIED
    expect(validateOriginReviewFromProto(content, current, invalidState).issues).toEqual([
      { index: 0, code: 'invalid_review_state' },
    ])
    const missingField = annotated()
    missingField.spans[0]!.field = undefined
    expect(validateOriginReviewFromProto(content, current, missingField).issues).toEqual([
      { index: 0, code: 'invalid_locator' },
    ])
  })
})

import { describe, expect, it } from 'vitest'
import { ORIGIN_REVIEW_VERSION } from '../config/semantic-origin'
import {
  originContentMatches,
  originFieldSegments,
  originReadableFields,
} from './origin-projection'
import {
  ORIGIN_BLOCK_TYPES,
  originFieldText,
  type OriginContent,
  type OriginFieldLocator,
  type OriginResultIdentity,
  type OriginReview,
  type OriginSource,
  type OriginSpan,
} from './semantic-origin'

const current: OriginResultIdentity = { contentRevision: 7n, contentHash: 'current-result' }
const source: OriginSource = {
  id: 'memo',
  kind: 'memo',
  text: '직접 적은 내용',
  attachmentFilename: '',
  available: true,
}
const visual: OriginSource = {
  ...source,
  id: 'observation',
  kind: 'visual_observation',
  text: '빨간 접시',
  attachmentFilename: 'plate.jpg',
}
const canonical: OriginContent = {
  title: '맛있다😀맛있다, 기대!',
  summary: '',
  tags: [],
  blocks: [],
}
const field: OriginFieldLocator = { kind: 'title' }
const spans: OriginSpan[] = [
  {
    field,
    start: 0,
    end: 3,
    quote: '맛있다',
    category: 'owner_input',
    sourceRefs: ['memo'],
    reviewState: 'unreviewed',
  },
  {
    field,
    start: 4,
    end: 7,
    quote: '맛있다',
    category: 'photo_interpretation',
    sourceRefs: ['observation'],
    reviewState: 'unreviewed',
  },
  {
    field,
    start: 9,
    end: 12,
    quote: '기대!',
    category: 'ai_added',
    sourceRefs: [],
    reviewState: 'confirmed',
  },
]
const evidence = (patch: Partial<OriginReview> = {}): OriginReview => ({
  version: ORIGIN_REVIEW_VERSION,
  result: current,
  sources: [source, visual],
  spans,
  ...patch,
})

describe('canonical field origin projection', () => {
  it('enumerates only nonempty declared canonical fields in reading order for accessible inspection', () => {
    const content: OriginContent = {
      title: '제목😀',
      summary: '',
      tags: ['', '태그😀'],
      blocks: [
        { type: ORIGIN_BLOCK_TYPES.text, content: '본문😀', alt: '제외', items: ['제외'] },
        { type: ORIGIN_BLOCK_TYPES.heading, content: '' },
        { type: ORIGIN_BLOCK_TYPES.quote, content: '인용😀' },
        { type: ORIGIN_BLOCK_TYPES.list, items: ['항목😀', '', '항목😀'], content: '제외' },
        { type: ORIGIN_BLOCK_TYPES.image, alt: '사진😀', caption: '' },
        { type: ORIGIN_BLOCK_TYPES.gallery, alt: '묶음😀', caption: '묶음 설명😀' },
        { type: ORIGIN_BLOCK_TYPES.video, alt: '', caption: '영상 설명😀' },
        { type: 999, content: '제외', alt: '제외', caption: '제외' },
      ],
    }
    const before = structuredClone(content)
    expect(originReadableFields(content)).toEqual([
      { kind: 'title' },
      { kind: 'tag', tagIndex: 1 },
      { kind: 'block_content', blockIndex: 0 },
      { kind: 'block_content', blockIndex: 2 },
      { kind: 'block_item', blockIndex: 3, itemIndex: 0 },
      { kind: 'block_item', blockIndex: 3, itemIndex: 2 },
      { kind: 'block_alt', blockIndex: 4 },
      { kind: 'block_alt', blockIndex: 5 },
      { kind: 'block_caption', blockIndex: 5 },
      { kind: 'block_caption', blockIndex: 6 },
    ])
    expect(content).toEqual(before)
    expect(originReadableFields({ title: '', summary: '', tags: [], blocks: [] })).toEqual([])
  })

  it('preserves repeated Korean, emoji, punctuation and mixed origins at exact scalar ranges', () => {
    const review = evidence()
    const before = structuredClone({ canonical, review })
    const segments = originFieldSegments(canonical, current, review, field)
    expect(
      segments.map(({ text, start, end, state, category }) => ({
        text,
        start,
        end,
        state,
        category,
      })),
    ).toEqual([
      { text: '맛있다', start: 0, end: 3, state: 'supported', category: 'owner_input' },
      { text: '😀', start: 3, end: 4, state: 'unconfirmed', category: undefined },
      { text: '맛있다', start: 4, end: 7, state: 'supported', category: 'photo_interpretation' },
      { text: ', ', start: 7, end: 9, state: 'unconfirmed', category: undefined },
      { text: '기대!', start: 9, end: 12, state: 'supported', category: 'ai_added' },
    ])
    expect(segments.map(({ text }) => text).join('')).toBe(canonical.title)
    expect(segments[4]).toMatchObject({ category: 'ai_added', reviewState: 'confirmed' })
    segments[0]!.sourceRefs.push('client-change')
    expect({ canonical, review }).toEqual(before)
  })

  it('covers every named readable field and keeps distinct indices independent', () => {
    const content: OriginContent = {
      title: '제목😀',
      summary: '요약😀',
      tags: ['태그😀', '태그😀'],
      blocks: [
        { type: ORIGIN_BLOCK_TYPES.text, content: '본문😀' },
        { type: ORIGIN_BLOCK_TYPES.heading, content: '소제목😀' },
        { type: ORIGIN_BLOCK_TYPES.quote, content: '인용😀' },
        { type: ORIGIN_BLOCK_TYPES.list, items: ['항목😀', '항목😀'] },
        { type: ORIGIN_BLOCK_TYPES.image, alt: '사진😀', caption: '사진 설명😀' },
        { type: ORIGIN_BLOCK_TYPES.gallery, alt: '묶음😀', caption: '묶음 설명😀' },
        { type: ORIGIN_BLOCK_TYPES.video, alt: '영상😀', caption: '영상 설명😀' },
      ],
    }
    const fields: OriginFieldLocator[] = [
      { kind: 'title' },
      { kind: 'summary' },
      { kind: 'tag', tagIndex: 0 },
      { kind: 'tag', tagIndex: 1 },
      ...[0, 1, 2].map((blockIndex) => ({ kind: 'block_content' as const, blockIndex })),
      ...[0, 1].map((itemIndex) => ({ kind: 'block_item' as const, blockIndex: 3, itemIndex })),
      ...[4, 5, 6].flatMap((blockIndex) => [
        { kind: 'block_alt' as const, blockIndex },
        { kind: 'block_caption' as const, blockIndex },
      ]),
    ]
    const review = evidence({
      spans: fields.map((field) => {
        const quote = originFieldText(content, field)!
        return {
          field,
          quote,
          start: 0,
          end: Array.from(quote).length,
          category: 'owner_input',
          sourceRefs: [source.id],
          reviewState: 'unreviewed',
        }
      }),
    })
    for (const field of fields) {
      const text = originFieldText(content, field)!
      expect(originFieldSegments(content, current, review, field)).toEqual([
        {
          text,
          start: 0,
          end: Array.from(text).length,
          category: 'owner_input',
          state: 'supported',
          sourceRefs: ['memo'],
          reviewState: 'unreviewed',
        },
      ])
    }
    expect(
      originFieldSegments(content, current, review, { kind: 'block_alt', blockIndex: 0 }),
    ).toEqual([])
    expect(originFieldSegments(content, current, review, { kind: 'tag', tagIndex: 2 })).toEqual([])
  })

  it.each([
    [undefined, 'missing_origin'],
    [evidence({ version: 999 }), 'unsupported_version'],
    [evidence({ result: { ...current, contentRevision: 8n } }), 'stale_result'],
    [evidence({ result: { ...current, contentHash: 'different' } }), 'stale_result'],
  ] as const)(
    'keeps legacy, unsupported and stale metadata readable and unconfirmed',
    (review, reason) => {
      expect(originFieldSegments(canonical, current, review, field)).toEqual([
        {
          text: canonical.title,
          start: 0,
          end: 12,
          state: 'unconfirmed',
          sourceRefs: [],
          reviewState: 'unconfirmed',
          reason,
        },
      ])
    },
  )

  it('withholds every category while saving or when any displayed canonical field differs', () => {
    for (const options of [
      { pending: true },
      { canonicalContent: { ...canonical, summary: '서버에만 있는 요약' } },
    ]) {
      expect(originFieldSegments(canonical, current, evidence(), field, options)).toEqual([
        {
          text: canonical.title,
          start: 0,
          end: 12,
          state: 'unconfirmed',
          sourceRefs: [],
          reviewState: 'unconfirmed',
          reason: 'pending',
        },
      ])
    }
    expect(
      originFieldSegments(canonical, current, evidence(), field, {
        canonicalContent: structuredClone(canonical),
      })[0]?.state,
    ).toBe('supported')
  })

  it('does not promote unconfirmed review or invent support for inserted text', () => {
    const review = evidence({ spans: [{ ...spans[0]!, reviewState: 'unconfirmed' }] })
    const pieces = originFieldSegments(canonical, current, review, field)
    expect(
      pieces.every(
        (piece) =>
          piece.state === 'unconfirmed' && !piece.category && piece.sourceRefs.length === 0,
      ),
    ).toBe(true)
    const edited = { ...canonical, title: `새 문장 ${canonical.title}` }
    expect(
      originFieldSegments(edited, current, evidence(), field).every(
        (piece) => piece.state === 'unconfirmed',
      ),
    ).toBe(true)
  })

  it('renders removed or unavailable evidence unconfirmed without disturbing other supported phrases', () => {
    const review = evidence({ sources: [source, { ...visual, available: false }] })
    const segments = originFieldSegments(canonical, current, review, field)
    expect(segments.find(({ start }) => start === 4)).toMatchObject({
      text: '맛있다',
      state: 'unconfirmed',
      reason: 'unavailable_source',
      sourceRefs: [],
    })
    expect(segments[0]).toMatchObject({ state: 'supported', category: 'owner_input' })
    expect(segments.at(-1)).toMatchObject({ state: 'supported', category: 'ai_added' })
    const missing = originFieldSegments(canonical, current, evidence({ sources: [source] }), field)
    expect(missing.find(({ start }) => start === 4)).toMatchObject({
      state: 'unconfirmed',
      reason: 'unknown_source',
    })
    expect(segments.map(({ text }) => text).join('')).toBe(canonical.title)
  })

  it.each([
    { result: undefined },
    { sources: undefined },
    { spans: undefined },
    { sources: [null] },
    { spans: [null] },
  ])('tolerates malformed runtime sidecars %j', (patch) => {
    const review = { ...evidence(), ...patch } as unknown as OriginReview
    const segments = originFieldSegments(canonical, current, review, field)
    expect(segments.every(({ state, category }) => state === 'unconfirmed' && !category)).toBe(true)
    expect(segments.map(({ text }) => text).join('')).toBe(canonical.title)
  })

  it.each([
    { end: 99 },
    { quote: '다른 말' },
    { category: 'fourth' },
    { sourceRefs: ['missing'] },
    { reviewState: 'promoted' },
    { start: 0.5 },
  ])('never asserts a malformed phrase %j', (patch) => {
    const review = evidence({ spans: [{ ...spans[0]!, ...patch } as OriginSpan] })
    const segments = originFieldSegments(canonical, current, review, field)
    expect(segments.every(({ state, category }) => state === 'unconfirmed' && !category)).toBe(true)
    expect(segments.map(({ text }) => text).join('')).toBe(canonical.title)
  })

  it('keeps malformed Unicode visible without giving it an evidence range', () => {
    const content = { ...canonical, title: '앞\ud800뒤' }
    expect(
      originFieldSegments(content, current, evidence(), field)
        .map(({ text }) => text)
        .join(''),
    ).toBe(content.title)
    expect(
      originFieldSegments(content, current, evidence(), field).every(
        ({ state }) => state === 'unconfirmed',
      ),
    ).toBe(true)
  })
})

describe('canonical content comparison', () => {
  const content = {
    title: '제목',
    summary: '요약',
    tags: ['가', '나'],
    blocks: [
      { type: ORIGIN_BLOCK_TYPES.heading, content: '제목', level: 2 },
      { type: ORIGIN_BLOCK_TYPES.image, file: 'a.jpg', alt: '사진', caption: '설명' },
      {
        type: ORIGIN_BLOCK_TYPES.gallery,
        files: ['a.jpg', 'b.jpg'],
        layout: 1,
        alt: '묶음',
        caption: '설명',
      },
      { type: ORIGIN_BLOCK_TYPES.list, items: ['가', '나'] },
    ],
  }
  it('recognizes a saved or cancelled exact snapshot independent of object identity', () => {
    expect(originContentMatches(content, structuredClone(content))).toBe(true)
    expect(
      originContentMatches(content, { ...content, $typeName: 'ignored' } as typeof content),
    ).toBe(true)
    expect(originContentMatches(content, undefined)).toBe(false)
  })
  it('detects text, tag/item ordering, media identity, layout, heading level, reorder and delete changes', () => {
    const changes = [
      { ...content, title: '변경' },
      { ...content, summary: '변경' },
      { ...content, tags: ['나', '가'] },
      { ...content, blocks: [...content.blocks].reverse() },
      { ...content, blocks: content.blocks.slice(1) },
      ...[
        [0, { content: '변경' }],
        [0, { level: 3 }],
        [1, { file: 'b.jpg' }],
        [1, { alt: '변경' }],
        [1, { caption: '변경' }],
        [2, { files: ['b.jpg', 'a.jpg'] }],
        [2, { layout: 2 }],
        [3, { items: ['나', '가'] }],
      ].map(([index, patch]) => ({
        ...content,
        blocks: content.blocks.map((block, blockIndex) =>
          blockIndex === index ? { ...block, ...(patch as object) } : block,
        ),
      })),
    ]
    for (const changed of changes) expect(originContentMatches(content, changed)).toBe(false)
  })
})

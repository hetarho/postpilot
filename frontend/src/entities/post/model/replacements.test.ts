import { clone, create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import {
  applyReplacement,
  canonicalTag,
  spansAt,
  visibleSpans,
  type ReplacementCandidate,
} from './replacements'

const CONTENT = create(PostContentSchema, {
  title: '성수 카페 투어',
  summary: '성수에서 보낸 하루',
  tags: ['성수', '카페 투어', '#주말'],
  blocks: [
    create(BlockSchema, { type: BlockType.TEXT, content: '성수 카페에 갔다. 성수는 붐볐다.' }),
    create(BlockSchema, {
      type: BlockType.TEXT,
      content: '{{slot:1}} 성수',
      slot: { kind: 'place', label: '장소' },
    }),
    create(BlockSchema, { type: BlockType.HEADING, level: 2, content: '성수의 오후' }),
    create(BlockSchema, { type: BlockType.QUOTE, content: '성수는 늘 새롭다.' }),
    create(BlockSchema, { type: BlockType.LIST, items: ['라떼', '성수 산책', '성수 빵'] }),
    create(BlockSchema, { type: BlockType.IMAGE, file: 'IMG_1.jpg', alt: '성수 거리' }),
  ],
})

const candidate = (
  surface: ReplacementCandidate['surface'],
  index: number,
  source: string,
  phrases: string[] = ['성수동', '서울 성수'],
): ReplacementCandidate => ({ surface, index, source, phrases })

describe('the placement rules', () => {
  it('marks the title, a tag by its index, and TEXT, HEADING and QUOTE blocks', () => {
    const spans = visibleSpans(CONTENT, [
      candidate('title', 0, '카페'),
      candidate('tag', 1, '투어'),
      candidate('body', 0, '갔다'),
      candidate('body', 2, '오후'),
      candidate('body', 3, '새롭다'),
    ])
    expect(spans.map((span) => [span.at, span.start, span.end])).toEqual([
      [{ surface: 'title' }, 3, 5],
      [{ surface: 'tag', index: 1 }, 3, 5],
      [{ surface: 'body', index: 0 }, 7, 9],
      [{ surface: 'body', index: 2 }, 4, 6],
      [{ surface: 'body', index: 3 }, 6, 9],
    ])
  })

  it('marks the lowest-numbered LIST item that contains the source', () => {
    const [span] = visibleSpans(CONTENT, [candidate('body', 4, '성수')])
    expect(span.at).toEqual({ surface: 'body', index: 4, item: 1 })
    expect([span.start, span.end]).toEqual([0, 2])
  })

  it('never marks a slot TEXT block, an image, a title index other than 0 or an index out of range', () => {
    expect(
      visibleSpans(CONTENT, [
        candidate('body', 1, '성수'),
        candidate('body', 5, '성수'),
        candidate('title', 1, '카페'),
        candidate('tag', 3, '성수'),
        candidate('tag', -1, '성수'),
        candidate('body', 9, '성수'),
      ]),
    ).toEqual([])
  })

  it('drops a candidate whose source no longer stands there, or is blank', () => {
    expect(
      visibleSpans(CONTENT, [
        candidate('title', 0, '빵집'),
        candidate('body', 0, '홍대'),
        candidate('body', 4, '커피'),
        // A blank source stands everywhere, so it would mark nothing or a bare space.
        candidate('title', 0, ''),
        candidate('title', 0, ' '),
        candidate('body', 4, ' '),
      ]),
    ).toEqual([])
  })

  it('keeps two candidates that only touch', () => {
    const spans = visibleSpans(CONTENT, [candidate('body', 0, '카페'), candidate('body', 0, '에')])
    expect(spans.map((span) => [span.start, span.end])).toEqual([
      [3, 5],
      [5, 6],
    ])
  })

  it('marks only the first occurrence, case-sensitively', () => {
    const [span] = visibleSpans(CONTENT, [candidate('body', 0, '성수')])
    expect([span.start, span.end]).toEqual([0, 2])
    const english = create(PostContentSchema, { title: 'Cafe cafe', tags: [], blocks: [] })
    expect(visibleSpans(english, [candidate('title', 0, 'cafe')])[0]?.start).toBe(5)
  })

  it('keeps the first of two overlapping candidates in answer order, and one elsewhere', () => {
    const spans = visibleSpans(CONTENT, [
      candidate('body', 0, '카페에 갔다'),
      candidate('body', 0, '성수 카페'),
      candidate('body', 0, '붐볐다'),
      candidate('title', 0, '성수 카페'),
    ])
    expect(spans.map((span) => [span.at, span.source])).toEqual([
      [{ surface: 'body', index: 0 }, '카페에 갔다'],
      [{ surface: 'body', index: 0 }, '붐볐다'],
      [{ surface: 'title' }, '성수 카페'],
    ])
  })
})

describe('the phrases a mark offers', () => {
  it('offers at most three, none blank, repeated or equal to the source', () => {
    const [span] = visibleSpans(CONTENT, [
      candidate('title', 0, '카페', [
        '카페',
        ' ',
        '브런치 카페',
        '브런치 카페',
        '디저트 카페',
        '루프탑 카페',
        '북카페',
      ]),
    ])
    expect(span.phrases).toEqual(['브런치 카페', '디저트 카페', '루프탑 카페'])
  })

  it('does not offer a tag phrase that would canonically duplicate another tag or empty it', () => {
    expect(canonicalTag('  #카페   투어 ')).toBe('카페 투어')
    const [span] = visibleSpans(CONTENT, [
      candidate('tag', 0, '성수', ['주말', '## 주말', '#', '성수동']),
    ])
    expect(span.phrases).toEqual(['성수동'])
  })

  it('judges a tag phrase by the whole tag it would leave', () => {
    // '성수' alone would duplicate tag 0, but '카페 성수' and '성수 투어' duplicate nothing.
    expect(visibleSpans(CONTENT, [candidate('tag', 1, '투어', ['성수'])])[0]?.phrases).toEqual([
      '성수',
    ])
    expect(visibleSpans(CONTENT, [candidate('tag', 1, '카페', ['성수'])])[0]?.phrases).toEqual([
      '성수',
    ])
  })

  it('compares a tag phrase against the other tags only', () => {
    // A phrase that leaves the tag canonically as it is duplicates nothing else.
    const [span] = visibleSpans(CONTENT, [candidate('tag', 1, '카페', ['카페 '])])
    expect(span.phrases).toEqual(['카페 '])
  })

  it('marks nothing for a candidate left with no phrase', () => {
    expect(visibleSpans(CONTENT, [candidate('tag', 0, '성수', ['#주말', '카페   투어'])])).toEqual(
      [],
    )
  })
})

describe('spans at one place', () => {
  it('are the spans of that place alone, in reading order', () => {
    const spans = visibleSpans(CONTENT, [
      candidate('body', 0, '붐볐다'),
      candidate('title', 0, '투어'),
      candidate('body', 0, '갔다'),
    ])
    expect(spansAt(spans, { surface: 'body', index: 0 }).map((span) => span.source)).toEqual([
      '갔다',
      '붐볐다',
    ])
    expect(spansAt(spans, { surface: 'body', index: 4, item: 0 })).toEqual([])
  })

  it('keeps two items of one LIST apart', () => {
    const spans = visibleSpans(CONTENT, [
      candidate('body', 4, '산책'),
      candidate('body', 4, '라떼'),
    ])
    expect(
      spansAt(spans, { surface: 'body', index: 4, item: 0 }).map((span) => span.source),
    ).toEqual(['라떼'])
    expect(
      spansAt(spans, { surface: 'body', index: 4, item: 1 }).map((span) => span.source),
    ).toEqual(['산책'])
  })
})

describe('taking a phrase', () => {
  it.each([
    [
      'the title',
      candidate('title', 0, '카페'),
      (c: typeof CONTENT) => c.title,
      '성수 브런치 투어',
    ],
    ['a tag', candidate('tag', 1, '투어'), (c: typeof CONTENT) => c.tags[1], '카페 브런치'],
    [
      'a TEXT block',
      candidate('body', 0, '갔다'),
      (c: typeof CONTENT) => c.blocks[0].content,
      '성수 카페에 브런치. 성수는 붐볐다.',
    ],
    [
      'a LIST item',
      candidate('body', 4, '산책'),
      (c: typeof CONTENT) => c.blocks[4].items[1],
      '성수 브런치',
    ],
  ])(
    'splices the phrase over the span in %s, leaving the input as it was',
    (_label, found, read, want) => {
      const before = clone(PostContentSchema, CONTENT)
      const [span] = visibleSpans(CONTENT, [found])
      const taken = applyReplacement(CONTENT, span, '브런치')
      expect(read(taken)).toBe(want)
      expect(CONTENT).toEqual(before)
      // Every other place is left as it was.
      expect(taken.summary).toBe(CONTENT.summary)
      expect(taken.blocks).toHaveLength(CONTENT.blocks.length)
    },
  )

  it('leaves the other tags and blocks as they were', () => {
    const [tag] = visibleSpans(CONTENT, [candidate('tag', 1, '투어')])
    expect(applyReplacement(CONTENT, tag, '브런치').tags).toEqual(['성수', '카페 브런치', '#주말'])
    const [text] = visibleSpans(CONTENT, [candidate('body', 0, '갔다')])
    expect(applyReplacement(CONTENT, text, '브런치').blocks.slice(1)).toEqual(
      CONTENT.blocks.slice(1),
    )
  })

  it('leaves the other items of a LIST as they were', () => {
    const [span] = visibleSpans(CONTENT, [candidate('body', 4, '산책')])
    expect(applyReplacement(CONTENT, span, '브런치').blocks[4].items).toEqual([
      '라떼',
      '성수 브런치',
      '성수 빵',
    ])
  })

  it('drops the taken mark, because its source no longer stands there', () => {
    const [span] = visibleSpans(CONTENT, [candidate('title', 0, '카페')])
    const taken = applyReplacement(CONTENT, span, '브런치')
    expect(visibleSpans(taken, [candidate('title', 0, '카페')])).toEqual([])
  })
})

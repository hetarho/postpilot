import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  ORIGIN_MAX_SOURCES,
  ORIGIN_MAX_REFS_PER_SPAN,
  ORIGIN_REVIEW_VERSION,
  ORIGIN_SOURCE_ID_MAX_SCALARS,
  ORIGIN_SOURCE_TEXT_MAX_SCALARS,
} from '../config/semantic-origin'
import {
  ORIGIN_BLOCK_TYPES,
  originFieldText,
  resolveOriginCandidates,
  scalarRangeToUtf16,
  utf16RangeToScalar,
  validateOriginReview,
  validateOriginSpans,
  type OriginCandidate,
  type OriginContent,
  type OriginFieldLocator,
  type OriginIssueCode,
  type OriginResultIdentity,
  type OriginReview,
  type OriginSource,
  type OriginSpan,
} from './semantic-origin'

const current: OriginResultIdentity = { contentRevision: 7n, contentHash: 'fixture-current-hash' }
const source: OriginSource = {
  id: 'memo-1',
  kind: 'memo',
  text: '맛있었다',
  attachmentFilename: '',
  available: true,
}

function content(title = '맛있었다'): OriginContent {
  return { title, summary: '', tags: [], blocks: [] }
}

function candidate(patch: Partial<OriginCandidate> = {}): OriginCandidate {
  return {
    field: { kind: 'title' },
    quote: '맛있었다',
    category: 'owner_input',
    sourceRefs: [source.id],
    ...patch,
  }
}

function span(patch: Partial<OriginSpan> = {}): OriginSpan {
  return {
    field: { kind: 'title' },
    start: 0,
    end: 4,
    quote: '맛있었다',
    category: 'owner_input',
    sourceRefs: [source.id],
    reviewState: 'unreviewed',
    ...patch,
  }
}

function review(patch: Partial<OriginReview> = {}): OriginReview {
  return {
    version: ORIGIN_REVIEW_VERSION,
    result: current,
    sources: [source],
    spans: [span()],
    ...patch,
  }
}

describe('Unicode scalar and UTF-16 ranges', () => {
  const text = '가😀e\u0301\r\n끝'

  it('round-trips every scalar boundary, including astral, combining and CRLF characters', () => {
    const boundaries = [0, 1, 3, 4, 5, 6, 7, 8]
    for (let start = 0; start < boundaries.length; start += 1) {
      for (let end = start; end < boundaries.length; end += 1) {
        const utf16 = { start: boundaries[start]!, end: boundaries[end]! }
        expect(scalarRangeToUtf16(text, { start, end })).toEqual(utf16)
        expect(utf16RangeToScalar(text, utf16)).toEqual({ start, end })
      }
    }
  })

  it.each([
    { start: -1, end: 1 },
    { start: 1, end: 0 },
    { start: 0.5, end: 1 },
    { start: 0, end: Infinity },
    { start: 0, end: NaN },
    { start: 0, end: Number.MAX_SAFE_INTEGER + 1 },
    { start: 0, end: 8 },
  ])('rejects invalid scalar offsets %j', (range) => {
    expect(scalarRangeToUtf16(text, range)).toBeUndefined()
  })

  it('refuses UTF-16 offsets inside an astral surrogate pair', () => {
    expect(utf16RangeToScalar(text, { start: 1, end: 2 })).toBeUndefined()
    expect(utf16RangeToScalar(text, { start: 2, end: 3 })).toBeUndefined()
    expect(utf16RangeToScalar(text, { start: 0, end: 9 })).toBeUndefined()
  })

  it.each(['\ud800', '\udc00', 'good\ud800text', '\ud800\ud800', '\udc00\ud800'])(
    'rejects malformed surrogate text %j even when the requested range avoids it',
    (text) => {
      expect(scalarRangeToUtf16(text, { start: 0, end: 0 })).toBeUndefined()
      expect(utf16RangeToScalar(text, { start: 0, end: 0 })).toBeUndefined()
    },
  )
})

describe('exact quote resolution', () => {
  it('keeps mixed meaning categories and uncovered text separate within one sentence', () => {
    const visual: OriginSource = {
      ...source,
      id: 'photo-1',
      kind: 'visual_observation',
      text: '빨간 접시',
      attachmentFilename: 'plate.jpg',
    }
    const result = resolveOriginCandidates(
      content('맛있었다. 빨간 접시에 달콤한 향.'),
      [source, visual],
      [
        candidate(),
        candidate({
          quote: '빨간 접시',
          category: 'photo_interpretation',
          sourceRefs: [visual.id],
        }),
        candidate({ quote: '달콤한 향', category: 'ai_added', sourceRefs: [] }),
      ],
    )
    expect(result.issues).toEqual([])
    expect(result.spans.map(({ category, reviewState }) => ({ category, reviewState }))).toEqual([
      { category: 'owner_input', reviewState: 'unreviewed' },
      { category: 'photo_interpretation', reviewState: 'unreviewed' },
      { category: 'ai_added', reviewState: 'unreviewed' },
    ])
    expect(result.spans.map(({ start, end }) => ({ start, end }))).toEqual([
      { start: 0, end: 4 },
      { start: 6, end: 11 },
      { start: 13, end: 18 },
    ])
  })

  it('uses explicit zero-based nonoverlapping occurrences and never silently chooses a repeat', () => {
    expect(
      resolveOriginCandidates(content('aaaaa'), [source], [candidate({ quote: 'aa' })]).issues,
    ).toEqual([{ index: 0, code: 'ambiguous_quote' }])
    const result = resolveOriginCandidates(
      content('aaaaa'),
      [source],
      [candidate({ quote: 'aa', occurrence: 1 })],
    )
    expect(result.spans).toEqual([span({ quote: 'aa', start: 2, end: 4 })])
    expect(result.spans[0]).not.toHaveProperty('occurrence')
    expect(
      resolveOriginCandidates(
        content('aaaaa'),
        [source],
        [candidate({ quote: 'aa', occurrence: 2 })],
      ).issues,
    ).toEqual([{ index: 0, code: 'invalid_occurrence' }])
  })

  it('rejects explicit occurrences when the quote is absent', () => {
    expect(
      resolveOriginCandidates(
        content('aaaaa'),
        [source],
        [candidate({ quote: 'absent', occurrence: 0 })],
      ).issues,
    ).toEqual([{ index: 0, code: 'invalid_occurrence' }])
  })

  it.each([-1, 0.5, NaN, Infinity])('rejects occurrence %s', (occurrence) => {
    expect(
      resolveOriginCandidates(content(), [source], [candidate({ occurrence })]).issues,
    ).toEqual([{ index: 0, code: 'invalid_occurrence' }])
  })

  it.each([
    ['Hello world', 'hello world'],
    ['Hello  world', 'Hello world'],
    ['é', 'e\u0301'],
    ['line\r\nnext', 'line\nnext'],
  ])('does not normalize %j to match %j', (text, quote) => {
    expect(resolveOriginCandidates(content(text), [source], [candidate({ quote })]).issues).toEqual(
      [{ index: 0, code: 'quote_mismatch' }],
    )
  })

  it('rejects every overlapping span while keeping adjacent spans and other fields', () => {
    const result = resolveOriginCandidates(
      { ...content('abcdef'), summary: 'abcdef' },
      [source],
      [
        candidate({ quote: 'abcd' }),
        candidate({ quote: 'bc' }),
        candidate({ quote: 'def' }),
        candidate({ quote: 'a', field: { kind: 'summary' } }),
        candidate({ quote: 'bcdef', field: { kind: 'summary' } }),
      ],
    )
    expect(result.issues).toEqual([0, 1, 2].map((index) => ({ index, code: 'overlapping_span' })))
    expect(result.spans.map(({ field, quote }) => ({ field, quote }))).toEqual([
      { field: { kind: 'summary' }, quote: 'a' },
      { field: { kind: 'summary' }, quote: 'bcdef' },
    ])
  })
})

describe('origin validation remains independent of canonical content and review', () => {
  it('preserves confirmed AI meaning without promoting it to owner input', () => {
    const result = validateOriginSpans(
      content(),
      [],
      [span({ category: 'ai_added', sourceRefs: [], reviewState: 'confirmed' })],
    )
    expect(result.spans[0]?.category).toBe('ai_added')
    expect(result.spans[0]?.reviewState).toBe('confirmed')
  })

  it('rejects empty quotes, invalid persisted ranges and exact-quote mismatches', () => {
    const result = validateOriginSpans(
      content(),
      [source],
      [span({ quote: '' }), span({ end: 8 }), span({ quote: '맛있었어' })],
    )
    expect(result.issues.map(({ code }) => code)).toEqual([
      'empty_quote',
      'invalid_range',
      'quote_mismatch',
    ])
    expect(result.spans).toEqual([])
  })

  it('leaves absent, unsupported and stale result annotations unconfirmed', () => {
    expect(validateOriginReview(content(), current).issues).toEqual([
      { index: -1, code: 'missing_origin' },
    ])
    expect(validateOriginReview(content(), current, review({ version: 99 })).issues).toEqual([
      { index: -1, code: 'unsupported_version' },
    ])
    for (const result of [
      { ...current, contentRevision: 8n },
      { ...current, contentHash: 'another-result' },
      { ...current, contentRevision: -1n },
    ]) {
      expect(validateOriginReview(content(), current, review({ result })).issues).toEqual([
        { index: -1, code: 'stale_result' },
      ])
    }
    expect(
      validateOriginReview(content(), { ...current, contentHash: '' }, review()).spans,
    ).toEqual([])
  })

  it('leaves unknown, withdrawn and duplicated source references unconfirmed', () => {
    const result = resolveOriginCandidates(
      content(),
      [source, { ...source, id: 'withdrawn', available: false }],
      [
        candidate({ sourceRefs: ['unknown'] }),
        candidate({ sourceRefs: ['withdrawn'] }),
        candidate({ sourceRefs: [source.id, source.id] }),
        candidate({ sourceRefs: [] }),
      ],
    )
    expect(result.issues.map(({ code }) => code)).toEqual([
      'unknown_source',
      'unavailable_source',
      'unknown_source',
      'unknown_source',
    ])
    expect(result.spans).toEqual([])
  })

  it('rejects excessive reference metadata independently of usable content', () => {
    expect(
      resolveOriginCandidates(
        content(),
        [source],
        [
          candidate({
            sourceRefs: Array.from({ length: ORIGIN_MAX_REFS_PER_SPAN + 1 }, () => source.id),
          }),
        ],
      ).issues,
    ).toEqual([{ index: 0, code: 'metadata_limit' }])
  })

  it('rejects unsupported categories, review states and malformed source catalogs', () => {
    expect(
      resolveOriginCandidates(
        content(),
        [source],
        [candidate({ category: 'historical' as OriginCandidate['category'] })],
      ).issues,
    ).toEqual([{ index: 0, code: 'unsupported_category' }])
    expect(
      validateOriginSpans(
        content(),
        [source],
        [span({ reviewState: 'promoted' as OriginSpan['reviewState'] })],
      ).issues,
    ).toEqual([{ index: 0, code: 'invalid_review_state' }])
    for (const sources of [
      [source, source],
      [{ ...source, kind: 'voice_example' as OriginSource['kind'] }],
      [{ ...source, id: '' }],
    ]) {
      expect(resolveOriginCandidates(content(), sources, [candidate()]).issues).toEqual([
        { index: -1, code: 'invalid_source_catalog' },
      ])
    }
  })

  it('rejects excessive metadata without mutating usable canonical content', () => {
    const canonical = content()
    const before = structuredClone(canonical)
    const catalogs = [
      Array.from({ length: ORIGIN_MAX_SOURCES + 1 }, (_, index) => ({
        ...source,
        id: String(index),
      })),
      [{ ...source, id: '😀'.repeat(ORIGIN_SOURCE_ID_MAX_SCALARS + 1) }],
      [{ ...source, text: '😀'.repeat(ORIGIN_SOURCE_TEXT_MAX_SCALARS + 1) }],
      [{ ...source, attachmentFilename: 'a'.repeat(ORIGIN_SOURCE_TEXT_MAX_SCALARS + 1) }],
    ]
    for (const sources of catalogs)
      expect(resolveOriginCandidates(canonical, sources, []).issues).toEqual([
        { index: -1, code: 'metadata_limit' },
      ])
    expect(
      resolveOriginCandidates(
        canonical,
        [source],
        Array.from({ length: 5 }, () => candidate()),
      ).issues,
    ).toEqual([{ index: -1, code: 'metadata_limit' }])
    expect(canonical).toEqual(before)
    expect(
      resolveOriginCandidates(
        canonical,
        [{ ...source, text: '😀'.repeat(ORIGIN_SOURCE_TEXT_MAX_SCALARS) }],
        [candidate()],
      ).spans,
    ).toHaveLength(1)
  })

  it('returns independent span/source-reference arrays so review cannot mutate caller metadata', () => {
    const original = span()
    const result = validateOriginSpans(content(), [source], [original])
    result.spans[0]!.sourceRefs.push('new')
    result.spans[0]!.field.kind = 'summary'
    expect(original.sourceRefs).toEqual([source.id])
    expect(original.field).toEqual({ kind: 'title' })
  })
})

describe('field locator boundaries', () => {
  const canonical: OriginContent = {
    ...content('title'),
    summary: 'summary',
    tags: ['tag'],
    blocks: [
      { type: ORIGIN_BLOCK_TYPES.text, content: 'text', alt: 'ignored', items: ['ignored'] },
      { type: ORIGIN_BLOCK_TYPES.list, items: ['item'], content: 'ignored' },
      { type: ORIGIN_BLOCK_TYPES.image, alt: 'alt', caption: 'caption', content: 'ignored' },
    ],
  }

  it.each<OriginFieldLocator>([
    { kind: 'tag', tagIndex: -1 },
    { kind: 'tag', tagIndex: 1 },
    { kind: 'block_content', blockIndex: 1 },
    { kind: 'block_content', blockIndex: 2 },
    { kind: 'block_content', blockIndex: 3 },
    { kind: 'block_item', blockIndex: 0, itemIndex: 0 },
    { kind: 'block_item', blockIndex: 1, itemIndex: 1 },
    { kind: 'block_alt', blockIndex: 0 },
    { kind: 'block_caption', blockIndex: 0 },
  ])('does not classify irrelevant or missing fields %j', (field) => {
    expect(originFieldText(canonical, field)).toBeUndefined()
  })

  it('rejects filenames, layout enums and extra index members', () => {
    for (const field of [
      { kind: 'block_file', blockIndex: 2 },
      { kind: 'layout', blockIndex: 2 },
      { kind: 'title', blockIndex: 0 },
    ])
      expect(originFieldText(canonical, field as OriginFieldLocator)).toBeUndefined()
  })
})

interface SharedCase {
  name: string
  content: Omit<OriginContent, 'blocks'> & {
    blocks: { type: string; content?: string; items?: string[]; alt?: string; caption?: string }[]
  }
  sources: OriginSource[]
  candidates?: OriginCandidate[]
  spans?: OriginSpan[]
  expectedSpans: OriginSpan[]
  expectedIssueCodes: OriginIssueCode[]
  invalidUnicode?: boolean
  result?: { contentRevision: number; contentHash: string }
  version?: number
}

it('runs the same origin contract fixtures as Go against every readable field and failure', () => {
  const fixture = JSON.parse(
    readFileSync(resolve('../backend/internal/post/testdata/semantic-origin.json'), 'utf8'),
  ) as {
    version: number
    current: { contentRevision: number; contentHash: string }
    cases: SharedCase[]
  }
  expect(fixture.cases.length).toBeGreaterThan(0)
  const current = { ...fixture.current, contentRevision: BigInt(fixture.current.contentRevision) }
  for (const scenario of fixture.cases) {
    const canonical: OriginContent = {
      ...scenario.content,
      blocks: scenario.content.blocks.map((block) => ({
        ...block,
        type: ORIGIN_BLOCK_TYPES[block.type.toLowerCase() as keyof typeof ORIGIN_BLOCK_TYPES],
      })),
    }
    const candidates = scenario.candidates?.map((entry) => ({ ...entry }))
    const sources = scenario.sources.map((entry) => ({
      ...entry,
      attachmentFilename: entry.attachmentFilename ?? '',
    }))
    if (scenario.invalidUnicode) {
      canonical.title = '\ud800'
      for (const entry of candidates ?? []) entry.quote = '\ud800'
    }
    const result = candidates
      ? resolveOriginCandidates(canonical, sources, candidates)
      : validateOriginReview(canonical, current, {
          version: scenario.version ?? fixture.version,
          result: scenario.result
            ? { ...scenario.result, contentRevision: BigInt(scenario.result.contentRevision) }
            : current,
          sources,
          spans: scenario.spans ?? [],
        })
    expect(result.spans, scenario.name).toEqual(scenario.expectedSpans)
    expect(
      result.issues.map(({ code }) => code),
      scenario.name,
    ).toEqual(scenario.expectedIssueCodes)
  }
})

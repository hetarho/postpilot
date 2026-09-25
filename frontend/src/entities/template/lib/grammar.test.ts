/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  askFields,
  decode,
  parse,
  parseTemplate,
  serialize,
  templateAskFields,
  type TemplateNode,
} from './grammar'

// The node reference is local to this file on purpose: the app tsconfig deliberately does not
// expose node types, so app code cannot reach the filesystem. Reading the shared fixture is a
// test-only need, and copying the fixture into the frontend would defeat the point of it.

/** The SAME fixture file the Go parser's suite reads. Two implementations of one grammar stay
 *  honest only if a new rule has one place to land (spec/legacy/tech/post-template-grammar.md §4). */
interface FixtureNode {
  t: string
  raw?: string
  text?: string
  kind?: string
  label?: string
  count?: number
  each?: string
  children?: FixtureNode[]
}
interface FixtureCase {
  name: string
  /** "" when the case has no title area (TMPL-50). */
  titleArea?: string
  body: string
  titleNodes?: FixtureNode[]
  nodes?: FixtureNode[]
  /** `area` is absent for a refusal in the body. */
  error?: { line: number; reason: string; area?: string }
}

const fixturePath = resolve(
  import.meta.dirname,
  '../../../../../backend/internal/template/testdata/grammar/cases.json',
)
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8'))
const cases: FixtureCase[] = fixture.cases
/** The ceilings the FIXTURE declares, never this deployment's own: a `count` or a data-field
 *  case has to mean the same thing on both sides, which is exactly what this file guarantees. */
const options = {
  photoRowMax: fixture.photoRowMax as number,
  askMaxPerBody: fixture.askMaxPerBody as number,
}

function expectNodes(got: readonly TemplateNode[], want: readonly FixtureNode[], path: string) {
  expect(
    got.map((n) => n.kind),
    `${path}: kinds`,
  ).toEqual(want.map((n) => n.t))
  want.forEach((expected, i) => {
    const actual = got[i]
    const at = `${path}[${i}]`
    if (expected.t === 'literal') expect(actual.text, at).toBe(expected.raw)
    if (expected.t === 'write' || expected.t === 'note') {
      expect(decode(actual.text ?? ''), at).toBe(expected.text)
    }
    if (expected.t === 'slot') {
      expect(actual.slotKind, at).toBe(expected.kind)
      expect(decode(actual.label ?? ''), at).toBe(expected.label)
      expect(actual.count, at).toBe(expected.count)
    }
    if (expected.t === 'ask') {
      expect(decode(actual.label ?? ''), at).toBe(expected.label)
      expect(decode(actual.text ?? ''), at).toBe(expected.text)
    }
    if (expected.t === 'repeat') {
      expect(actual.each, at).toBe(expected.each)
      expectNodes(actual.children ?? [], expected.children ?? [], at)
    }
  })
}

describe('template grammar against the shared fixtures', () => {
  it('runs the ceilings the fixture declares', () => {
    expect(options.photoRowMax).toBeGreaterThan(0)
    expect(options.askMaxPerBody).toBeGreaterThan(0)
  })

  it('reads at least one accepted and one refused case', () => {
    expect(cases.filter((c) => c.nodes).length).toBeGreaterThan(0)
    expect(cases.filter((c) => c.error).length).toBeGreaterThan(0)
  })

  it('reads at least one title-area case', () => {
    expect(cases.filter((c) => (c.titleArea ?? '') !== '').length).toBeGreaterThan(0)
  })

  // Every case is a (title area, body) pair parsed as one template. One with no title area must
  // also give the body-only parse's identical verdict, which keeps every template saved before
  // title areas existed exactly where it was.
  cases.forEach((testCase) => {
    it(testCase.name, () => {
      const titleArea = testCase.titleArea ?? ''
      const result = parseTemplate(titleArea, testCase.body, options)
      if (titleArea === '') {
        const bodyOnly = parse(testCase.body, options)
        expect(bodyOnly.ok).toBe(result.ok)
        if (!bodyOnly.ok && !result.ok) expect(bodyOnly.failure).toEqual(result.failure)
        if (bodyOnly.ok && result.ok)
          expect(serialize(bodyOnly.nodes)).toBe(serialize(result.nodes))
      }
      if (testCase.error) {
        expect(result.ok, 'expected a parse failure').toBe(false)
        if (result.ok) return
        expect(result.failure).toEqual({ area: 'body', ...testCase.error })
        return
      }
      expect(result.ok, `unexpected failure: ${JSON.stringify(result)}`).toBe(true)
      if (!result.ok) return
      expectNodes(result.titleNodes, testCase.titleNodes ?? [], 'titleNodes')
      expectNodes(result.nodes, testCase.nodes ?? [], 'nodes')
      // Every accepted area must serialize back byte for byte: this is the round-trip
      // guarantee the builder's source toggle rests on (change 25 AC8).
      expect(serialize(result.titleNodes)).toBe(titleArea)
      expect(serialize(result.nodes)).toBe(testCase.body)
    })
  })

  it('names the area a failure sits in, a body on its own included', () => {
    const body = parse('<writer>', options)
    expect(body.ok ? null : body.failure.area).toBe('body')
    const title = parse('<note>톤</note>', { ...options, titleArea: true })
    expect(title.ok ? null : title.failure).toEqual({
      line: 1,
      reason: 'not_in_title',
      area: 'title_area',
    })
    expect(parse('<slot kind="photo"/>', options).ok).toBe(true)
  })
})

/** The differential corpus: 600 machine-generated bodies plus the GO parser's verdict on each.
 *
 *  The hand-written fixtures above pin the rules anyone thought to write down. This pins the
 *  shapes nobody did — the combinations a mistake in either implementation actually falls into.
 *  Regenerate with `TEMPLATE_CORPUS_REGEN=1 go test ./internal/template/ -run TestCorpus`. */
interface CorpusCase {
  body: string
  ok: boolean
  line?: number
  reason?: string
}

interface TitleCorpusCase {
  titleArea: string
  body: string
  ok: boolean
  line?: number
  reason?: string
  area?: string
}

const corpusFile = JSON.parse(
  readFileSync(
    resolve(
      import.meta.dirname,
      '../../../../../backend/internal/template/testdata/grammar/corpus.json',
    ),
    'utf8',
  ),
)
const corpus: CorpusCase[] = corpusFile.cases
/** 200 generated (title area, body) pairs with the Go parser's verdict (TMPL-50). */
const titleCorpus: TitleCorpusCase[] = corpusFile.titleCases

describe('the TypeScript parser agrees with the Go parser', () => {
  it('reaches the same verdict, line and reason on every corpus body', () => {
    const disagreements: string[] = []
    for (const testCase of corpus) {
      const result = parse(testCase.body, options)
      if (result.ok !== testCase.ok) {
        disagreements.push(
          `${JSON.stringify(testCase.body)}: go ${testCase.ok ? 'accepted' : 'refused'}, ts ${
            result.ok ? 'accepted' : 'refused'
          }`,
        )
        continue
      }
      if (!result.ok) {
        if (result.failure.line !== testCase.line || result.failure.reason !== testCase.reason) {
          disagreements.push(
            `${JSON.stringify(testCase.body)}: go ${testCase.reason}@${testCase.line}, ts ${
              result.failure.reason
            }@${result.failure.line}`,
          )
        }
        continue
      }
      if (serialize(result.nodes) !== testCase.body) {
        disagreements.push(`${JSON.stringify(testCase.body)}: ts round trip changed the body`)
      }
    }
    expect(disagreements.slice(0, 10)).toEqual([])
  })

  it('reaches the same verdict, line, reason and area on every title-area pair', () => {
    expect(titleCorpus.length).toBe(200)
    const disagreements: string[] = []
    for (const testCase of titleCorpus) {
      const pair = `${JSON.stringify(testCase.titleArea)} | ${JSON.stringify(testCase.body)}`
      const result = parseTemplate(testCase.titleArea, testCase.body, options)
      if (result.ok !== testCase.ok) {
        disagreements.push(
          `${pair}: go ${testCase.ok ? 'accepted' : 'refused'}, ts ${result.ok ? 'accepted' : 'refused'}`,
        )
        continue
      }
      if (!result.ok) {
        const { line, reason, area } = result.failure
        if (line !== testCase.line || reason !== testCase.reason || area !== testCase.area) {
          disagreements.push(
            `${pair}: go ${testCase.reason}@${testCase.area}:${testCase.line}, ts ${reason}@${area}:${line}`,
          )
        }
        continue
      }
      if (
        serialize(result.titleNodes) !== testCase.titleArea ||
        serialize(result.nodes) !== testCase.body
      ) {
        disagreements.push(`${pair}: ts round trip changed the pair`)
      }
    }
    expect(disagreements.slice(0, 10)).toEqual([])
  })
})

/** What the write screen reads off a body (TEMPLATE-43). It is the one place ① learns which
 *  fields exist, so it has to answer for a body nobody can fix from there. */
describe('the data fields a body asks for', () => {
  it('lists them in body order with the flavor each one feeds', () => {
    expect(
      askFields(
        '오늘의 기록\n<ask label="방문일"/>\n<slot kind="photo"/>\n<ask label="총평">총평을 쓰세요</ask>',
        options,
      ),
    ).toEqual([
      { label: '방문일', flavor: 'verbatim' },
      { label: '총평', flavor: 'write' },
    ])
  })

  it('decodes a title the way the builder shows it', () => {
    expect(askFields('<ask label="네이버 &quot;별점&quot;"/>', options)).toEqual([
      { label: '네이버 "별점"', flavor: 'verbatim' },
    ])
  })

  it('asks for nothing when the body has no field', () => {
    expect(askFields('<write>인트로</write>', options)).toEqual([])
  })

  // ① is not where a broken template is fixed: it renders no fields rather than an error.
  it('asks for nothing when the body does not parse', () => {
    expect(askFields('<ask label="총평"/>\n<ask label="총평"/>', options)).toEqual([])
    expect(askFields('<writer>', options)).toEqual([])
  })
})

/** What ① will read off a template once it has a title area: one namespace, title first. */
describe('the data fields a template asks for', () => {
  it('lists the fields of the title area first, then those of the body', () => {
    expect(
      templateAskFields(
        '맛집 | <ask label="가게 이름"/>',
        '<write>인트로</write>\n<ask label="총평">총평을 쓰세요</ask>',
        options,
      ),
    ).toEqual([
      { label: '가게 이름', flavor: 'verbatim' },
      { label: '총평', flavor: 'write' },
    ])
  })

  it('asks for nothing when the title area does not parse', () => {
    expect(templateAskFields('<note>톤</note>', '<ask label="총평"/>', options)).toEqual([])
  })

  it('asks for nothing when the body does not parse', () => {
    expect(templateAskFields('<ask label="가게"/>', '<writer>', options)).toEqual([])
  })

  it('asks for nothing when the two areas reuse a title', () => {
    expect(templateAskFields('<ask label="가게"/>', '<ask label="가게"/>', options)).toEqual([])
  })

  it('reads an empty title area as the body alone', () => {
    const body = '오늘의 기록\n<ask label="방문일"/>\n<ask label="총평">총평을 쓰세요</ask>'
    expect(templateAskFields('', body, options)).toEqual(askFields(body, options))
  })
})

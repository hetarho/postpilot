/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { decode, parse, serialize, type TemplateNode } from './grammar'

// The node reference is local to this file on purpose: the app tsconfig deliberately does not
// expose node types, so app code cannot reach the filesystem. Reading the shared fixture is a
// test-only need, and copying the fixture into the frontend would defeat the point of it.

/** The SAME fixture file the Go parser's suite reads. Two implementations of one grammar stay
 *  honest only if a new rule has one place to land (spec/tech/post-template-grammar.md §4). */
interface FixtureNode {
  t: string
  raw?: string
  text?: string
  kind?: string
  label?: string
  each?: string
  children?: FixtureNode[]
}
interface FixtureCase {
  name: string
  body: string
  nodes?: FixtureNode[]
  error?: { line: number; reason: string }
}

const fixturePath = resolve(
  import.meta.dirname,
  '../../../../../backend/internal/template/testdata/grammar/cases.json',
)
const cases: FixtureCase[] = JSON.parse(readFileSync(fixturePath, 'utf8')).cases

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
    }
    if (expected.t === 'repeat') {
      expect(actual.each, at).toBe(expected.each)
      expectNodes(actual.children ?? [], expected.children ?? [], at)
    }
  })
}

describe('template grammar against the shared fixtures', () => {
  it('reads at least one accepted and one refused case', () => {
    expect(cases.filter((c) => c.nodes).length).toBeGreaterThan(0)
    expect(cases.filter((c) => c.error).length).toBeGreaterThan(0)
  })

  cases.forEach((testCase) => {
    it(testCase.name, () => {
      const result = parse(testCase.body)
      if (testCase.error) {
        expect(result.ok, 'expected a parse failure').toBe(false)
        if (result.ok) return
        expect(result.failure).toEqual(testCase.error)
        return
      }
      expect(result.ok, `unexpected failure: ${JSON.stringify(result)}`).toBe(true)
      if (!result.ok) return
      expectNodes(result.nodes, testCase.nodes ?? [], 'nodes')
      // Every accepted body must serialize back byte for byte: this is the round-trip
      // guarantee the builder's source toggle rests on (change 25 AC8).
      expect(serialize(result.nodes)).toBe(testCase.body)
    })
  })
})

/** The differential corpus: 600 machine-generated bodies plus the GO parser's verdict on each.
 *
 *  The hand-written fixtures above pin the rules anyone thought to write down. This pins the
 *  shapes nobody did — the combinations a mistake in either implementation actually falls into.
 *  Regenerate with `go test ./internal/template/ -run TestRegenerateCorpus`. */
interface CorpusCase {
  body: string
  ok: boolean
  line?: number
  reason?: string
}

const corpus: CorpusCase[] = JSON.parse(
  readFileSync(
    resolve(
      import.meta.dirname,
      '../../../../../backend/internal/template/testdata/grammar/corpus.json',
    ),
    'utf8',
  ),
).cases

describe('the TypeScript parser agrees with the Go parser', () => {
  it('reaches the same verdict, line and reason on every corpus body', () => {
    const disagreements: string[] = []
    for (const testCase of corpus) {
      const result = parse(testCase.body)
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
})

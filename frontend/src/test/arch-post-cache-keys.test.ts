import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

/** ARCH-14's cross-noun rule as a source-tree pin: "a post embeds its voice and template" is a
 *  property of `entities/post`, declared once and exposed as one invalidation entry that the verbs
 *  call. Before this, eight slices restated it with `listPostsQueryKey` + `postDetailQueriesKey`,
 *  so the next voice verb had to remember the rule and a key-shape change edited all eight.
 *  The quality cache follows the same rule, owned by `entities/quality`.
 *  No page, widget or feature test reads a query key by position either: the key's shape is the
 *  entity's, and a test that indexes into it breaks on a change no behavior sees. */

const src = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const POST_KEYS = ['postDetailQueriesKey', 'listPostsQueryKey', 'getPostQueryKey']

/** The post entity itself. Test harnesses fake a backend rather than a cache, so they are held to
 *  the same rule — nothing outside the entity names a post key. */
const OWNERS = ['entities/post/']

const QUALITY_KEYS = [
  'qualityQueriesKey',
  'accountQualityQueryKey',
  'accountQualityQuery',
  'postMeasurementQueryKey',
]
const QUALITY_OWNERS = ['entities/quality/']

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) return sourceFiles(full)
    if (!/\.tsx?$/.test(entry.name) || /\.test\.tsx?$/.test(entry.name)) return []
    return [full]
  })
}

function testFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) return testFiles(full)
    return /\.test\.tsx?$/.test(entry.name) ? [full] : []
  })
}

/** Every source file outside `owners` that names one of `keys`, as `file: key`. */
function keyOffenders(keys: readonly string[], owners: readonly string[]): string[] {
  return sourceFiles(src)
    .map((file) => path.relative(src, file))
    .filter((file) => !owners.some((owner) => file.startsWith(owner)))
    .flatMap((file) => {
      const source = readFileSync(path.join(src, file), 'utf8')
      return keys
        .filter((key) => new RegExp(`\\b${key}\\b`).test(source))
        .map((key) => `${file}: ${key}`)
    })
}

describe('ARCH-14: the post cache declares its own dependents', () => {
  it('lets nothing outside entities/post name a post query key', () => {
    expect(keyOffenders(POST_KEYS, OWNERS)).toEqual([])
  })

  it('lets nothing outside entities/quality name a quality query key', () => {
    expect(keyOffenders(QUALITY_KEYS, QUALITY_OWNERS)).toEqual([])
  })

  it('lets no page, widget or feature test read a query key by position', () => {
    const offenders = ['pages', 'widgets', 'features']
      .flatMap((layer) => testFiles(path.join(src, layer)))
      .filter((file) => /\bqueryKey(\[\d|\.at\()/.test(readFileSync(file, 'utf8')))
      .map((file) => path.relative(src, file))

    expect(offenders).toEqual([])
  })
})

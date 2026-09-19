import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

/** ARCH-14's cross-noun rule as a source-tree pin: "a post embeds its voice and template" is a
 *  property of `entities/post`, declared once and exposed as one invalidation entry that the verbs
 *  call. Before this, eight slices restated it with `listPostsQueryKey` + `postDetailQueriesKey`,
 *  so the next voice verb had to remember the rule and a key-shape change edited all eight. */

const src = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const POST_KEYS = ['postDetailQueriesKey', 'listPostsQueryKey', 'getPostQueryKey']

/** The post entity itself. Test harnesses fake a backend rather than a cache, so they are held to
 *  the same rule — nothing outside the entity names a post key. */
const OWNERS = ['entities/post/']

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) return sourceFiles(full)
    if (!/\.tsx?$/.test(entry.name) || /\.test\.tsx?$/.test(entry.name)) return []
    return [full]
  })
}

describe('ARCH-14: the post cache declares its own dependents', () => {
  it('lets nothing outside entities/post name a post query key', () => {
    const offenders = sourceFiles(src)
      .map((file) => path.relative(src, file))
      .filter((file) => !OWNERS.some((owner) => file.startsWith(owner)))
      .flatMap((file) => {
        const source = readFileSync(path.join(src, file), 'utf8')
        return POST_KEYS.filter((key) => new RegExp(`\\b${key}\\b`).test(source)).map(
          (key) => `${file}: ${key}`,
        )
      })

    expect(offenders).toEqual([])
  })
})

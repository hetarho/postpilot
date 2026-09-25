import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

/** ARCH-17 as a source-tree pin, beside the ESLint rule that reports it while you type.
 *
 *  The rule is: a proto service descriptor, a message schema and `@connectrpc/connect-query` are
 *  named only in `shared/api` and `entities/<noun>/api`. Pages, widgets and features consume the hooks
 *  and domain types an entity exports, so a proto rename stops at one directory and a message
 *  rename at one entity. The same holds for an entity's wire mappers (`*ToProto`, `*FromProto`):
 *  the entity maps inside itself and hands out hooks that take domain values. ESLint enforces it
 *  per file; this test states it once over the whole tree — since T262 with no exception left in
 *  it. */

const src = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const LAYERS = ['pages', 'widgets', 'features']

const PROTO_SYMBOL = /^(Proto[A-Z]|[A-Za-z]+(Service|Schema)$)/
const CONNECT_MODULES = ['@connectrpc/connect-query', '@connectrpc/connect']

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) return sourceFiles(full)
    if (!/\.tsx?$/.test(entry.name) || /\.test\.tsx?$/.test(entry.name)) return []
    return [full]
  })
}

function slicesUnderTest(): string[] {
  return LAYERS.flatMap((layer) => sourceFiles(path.join(src, layer))).map((file) =>
    path.relative(src, file),
  )
}

function importedNames(source: string, module: string): string[] {
  const pattern = new RegExp(`import\\s+(?:type\\s+)?\\{([^}]*)\\}\\s+from\\s+'${module}'`, 'g')
  return [...source.matchAll(pattern)].flatMap((match) =>
    match[1]
      .split(',')
      .map((name) =>
        name
          .replace(/^type\s+/, '')
          .split(/\s+as\s+/)[0]
          .trim(),
      )
      .filter(Boolean),
  )
}

describe('ARCH-17: proto symbols stop at entities', () => {
  const files = slicesUnderTest()

  it('covers the whole page/widget/feature tree, clip slices included', () => {
    expect(files.length).toBeGreaterThan(200)
    expect(files.filter((file) => file.includes('clip')).length).toBeGreaterThan(20)
  })

  it('names no proto service or message schema outside shared/api and entities/*/api', () => {
    const offenders = files.flatMap((file) => {
      const source = readFileSync(path.join(src, file), 'utf8')
      return importedNames(source, '@/shared/api')
        .filter((name) => PROTO_SYMBOL.test(name))
        .map((name) => `${file}: ${name}`)
    })
    expect(offenders).toEqual([])
  })

  it('imports no entity wire mapper into a page, widget or feature', () => {
    const offenders = files.flatMap((file) => {
      const source = readFileSync(path.join(src, file), 'utf8')
      return importedNames(source, '@/entities/[a-z-]+')
        .filter((name) => /(ToProto|FromProto)$/.test(name))
        .map((name) => `${file}: ${name}`)
    })
    expect(offenders).toEqual([])
  })

  it('leaves the Connect transport and connect-query to shared/api and entities/*/api', () => {
    const offenders = files.filter((file) => {
      const source = readFileSync(path.join(src, file), 'utf8')
      return CONNECT_MODULES.some((module) => source.includes(`from '${module}'`))
    })
    expect(offenders).toEqual([])
  })
})

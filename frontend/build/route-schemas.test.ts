import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { afterEach, expect, it } from 'vitest'
import { routeSchemasPastPageIndex } from './route-schemas'

const directories: string[] = []
afterEach(() => {
  for (const directory of directories.splice(0)) rmSync(directory, { recursive: true })
})

function fixture() {
  const pages = mkdtempSync(path.join(tmpdir(), 'postpilot-route-schemas-'))
  directories.push(pages)
  mkdirSync(path.join(pages, 'fixture'))
  writeFileSync(
    path.join(pages, 'fixture', 'index.ts'),
    [
      "export { internalSchema as searchSchema } from './model/search'",
      "export { Page } from './ui/Page'",
      "export type { Search } from './model/search'",
    ].join('\n'),
  )
  return routeSchemasPastPageIndex(pages)
}

const route = '/project/frontend/src/app/routes/models.ts'

it('splits mixed value/type imports and preserves renamed bindings through both boundaries', () => {
  const transformed = fixture().transform(
    "import { searchSchema as schema, type Search as Params } from '@/pages/fixture'",
    route,
  )
  expect(transformed).toBe(
    [
      "import { internalSchema as schema } from '@/pages/fixture/model/search'",
      "import type { Search as Params } from '@/pages/fixture'",
    ].join('\n'),
  )
})

it('does not convert a type-only binding into a runtime dependency', () => {
  expect(fixture().transform("import { type Search } from '@/pages/fixture'", route)).toBe(
    "import type { Search } from '@/pages/fixture'",
  )
})

it('preserves real UI-bearing and unsupported imports and ignores other source files', () => {
  const plugin = fixture()
  for (const source of [
    "import { searchSchema, Page } from '@/pages/fixture'",
    "import DefaultPage, { searchSchema } from '@/pages/fixture'",
    "import * as page from '@/pages/fixture'",
    "import type { Search } from '@/pages/fixture'",
  ])
    expect(plugin.transform(source, route)).toBe(source)
  expect(
    plugin.transform("import { searchSchema } from '@/pages/fixture'", '/src/features/action.ts'),
  ).toBeNull()
})

it('reads import and export bindings through the TypeScript grammar, including comments', () => {
  const plugin = fixture()
  const transformed = plugin.transform(
    "import { /* model binding */ searchSchema as schema, type Search } from '@/pages/fixture'",
    route,
  )
  expect(transformed).toContain('internalSchema as schema')
  expect(transformed).toContain("from '@/pages/fixture/model/search'")
  expect(transformed).toContain("import type { Search } from '@/pages/fixture'")
})

it('rewrites the actual mixed writing-test route while its page UI stays a lazy import', () => {
  const frontend = path.resolve(import.meta.dirname, '..')
  const source = readFileSync(path.join(frontend, 'src/app/routes/models.ts'), 'utf8')
  const transformed = routeSchemasPastPageIndex(path.join(frontend, 'src/pages')).transform(
    source,
    route,
  )!
  expect(transformed).toContain("from '@/pages/writing-tests/model/search'")
  expect(transformed).toContain(
    "import type { WritingTestSearch, WritingTestHistorySearch } from '@/pages/writing-tests'",
  )
  const writingRoute = readFileSync(path.join(frontend, 'src/app/routes/writing-tests.ts'), 'utf8')
  const lazy = routeSchemasPastPageIndex(path.join(frontend, 'src/pages')).transform(
    writingRoute,
    route.replace('models.ts', 'writing-tests.ts'),
  )!
  expect(lazy).toContain("from '@/pages/writing-tests/model/search'")
  expect(lazy).toContain("import('@/pages/writing-tests')")
})

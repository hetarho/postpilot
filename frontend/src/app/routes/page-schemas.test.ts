import { readdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, it } from 'vitest'

const routes = import.meta.dirname
const pages = resolve(import.meta.dirname, '../../pages')

/** Every search schema a route file imports, with the page whose index it is imported from. */
function schemaImports() {
  const found: { file: string; page: string; name: string }[] = []
  for (const file of readdirSync(routes).filter((f) => /^[a-z-]+\.ts$/.test(f))) {
    const code = readFileSync(resolve(routes, file), 'utf8')
    for (const [, list, page] of code.matchAll(/import \{([^}]+)\} from '@\/pages\/([a-z-]+)'/g))
      for (const name of list!.split(',').map((one) => one.trim()))
        if (name.endsWith('SearchSchema')) found.push({ file, page: page!, name })
  }
  return found
}

// The build takes a route's schema import straight from the page's `model` segment, past the
// page's index (`routeSchemasPastPageIndex` in vite.config.ts), so the index stays the lazy
// import's alone and the page stays out of the entry chunk. That holds only while each schema
// is re-exported by its index from `model`, in an export naming nothing from `ui`.
it('re-exports every search schema a route imports from its page’s model segment', () => {
  const imports = schemaImports()
  expect(imports.length).toBeGreaterThan(10)
  for (const { file, page, name } of imports) {
    const index = readFileSync(resolve(pages, page, 'index.ts'), 'utf8')
    const line = index.split('\n').find((one) => new RegExp(`\\b${name}\\b`).test(one))
    expect(line, `${file} imports ${name} from @/pages/${page}`).toMatch(
      /^export \{[^}]*\} from '\.\/model\/[^']+'$/,
    )
  }
})

import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { createRequire } from 'node:module'
import { expect, it } from 'vitest'
import fonts from './clip-ink-fonts.json'
import catalog from './clip-caption-ink.json'
import identity from './clip-ink-identity.json'
import { CLIP_BROWSER_COMPOSITION } from './browser-composition'

const canonical = (value: unknown) =>
  JSON.stringify(value, (_key, item: unknown) =>
    item && typeof item === 'object' && !Array.isArray(item)
      ? Object.fromEntries(Object.entries(item).sort(([a], [b]) => a.localeCompare(b)))
      : item,
  )
const digest = (value: unknown) => createHash('sha256').update(canonical(value)).digest('hex')
it('binds actual source/derived assets, native style data and the distributed WASM to one frozen profile', () => {
  expect(identity.fontsManifestSHA256).toBe(digest(fonts))
  expect(identity.catalogSHA256).toBe(digest(catalog))
  const { assetVersion, ...material } = identity
  expect(assetVersion).toBe(`clip-design-assets-v1-ink-${digest(material)}`)
  expect(CLIP_BROWSER_COMPOSITION.assets).toBe(assetVersion)
  const directory = resolve(import.meta.dirname, '../../../../public/fonts/clip')
  for (const resource of fonts.resources) {
    const bytes = readFileSync(resolve(directory, resource.file))
    expect(bytes.byteLength).toBe(resource.bytes)
    expect(createHash('sha256').update(bytes).digest('hex')).toBe(resource.sha256)
    expect(resource.family.split(' ').some((part) => /^\d/u.test(part))).toBe(false)
  }
  const require = createRequire(import.meta.url)
  expect(
    createHash('sha256')
      .update(readFileSync(require.resolve('@resvg/resvg-wasm/index_bg.wasm')))
      .digest('hex'),
  ).toBe(identity.wasmSHA256)
  const changed = structuredClone(fonts)
  changed.resources[0]!.sha256 = 'a'.repeat(64)
  expect(digest(changed)).not.toBe(identity.fontsManifestSHA256)
})

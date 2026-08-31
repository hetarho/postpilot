import { afterEach, describe, expect, it } from 'vitest'
import { applyDocumentLocale } from './metadata'

afterEach(() => applyDocumentLocale('ko'))

describe('document locale metadata', () => {
  it('sets lang, title, and description before the first React tree is needed', () => {
    applyDocumentLocale('en')

    expect(document.documentElement.lang).toBe('en')
    expect(document.title).toBe('Postpilot — Turn photos and experiences into blog posts')
    expect(document.querySelector<HTMLMetaElement>('meta[name="description"]')?.content).toBe(
      'Turn your photos and experiences into blog posts in your own voice.',
    )
  })
})

import { beforeEach, describe, expect, it } from 'vitest'
import { applyDocumentMetadata } from './document-metadata'

/** The head without its `<title>`, which `document.title = …` materializes on its own and which
 *  the assertions below check separately. */
function head(): string {
  return [...document.head.children]
    .filter((child) => child.tagName !== 'TITLE')
    .map((child) => child.outerHTML)
    .join('')
}

describe('applyDocumentMetadata', () => {
  beforeEach(() => {
    document.head.innerHTML = ''
    document.title = ''
  })

  it('creates absent tags and removes exactly those on restore', () => {
    document.title = 'Base title'
    const restore = applyDocumentMetadata({
      title: 'Route title',
      meta: { description: 'Route description' },
      property: { 'og:title': 'OG title', 'og:locale': 'ko_KR' },
      link: { canonical: 'https://example.test/about' },
    })

    expect(document.title).toBe('Route title')
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toBe(
      'Route description',
    )
    expect(document.head.querySelector('meta[property="og:title"]')?.getAttribute('content')).toBe(
      'OG title',
    )
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
      'https://example.test/about',
    )

    restore()
    expect(document.title).toBe('Base title')
    // Every tag this call created is gone, so nothing leaks into the next route.
    expect(head()).toBe('')
  })

  it('writes back the previous value of a tag that already existed', () => {
    document.head.innerHTML =
      '<meta name="description" content="App description">' +
      '<link rel="canonical" href="https://example.test/">'
    document.title = 'App title'

    const restore = applyDocumentMetadata({
      title: 'Route title',
      meta: { description: 'Route description' },
      link: { canonical: 'https://example.test/about' },
    })
    restore()

    expect(document.title).toBe('App title')
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toBe(
      'App description',
    )
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
      'https://example.test/',
    )
    // Restored in place: an existing tag is never removed and re-created, so anything else
    // holding a reference to it still points at the live element.
    expect(document.head.querySelectorAll('meta[name="description"]')).toHaveLength(1)
    expect(document.head.querySelectorAll('link[rel="canonical"]')).toHaveLength(1)
  })

  it('unwinds nested transactions like a stack', () => {
    document.title = 'Base'
    const outer = applyDocumentMetadata({ title: 'Outer', property: { 'og:title': 'Outer OG' } })
    const inner = applyDocumentMetadata({ title: 'Inner', property: { 'og:title': 'Inner OG' } })

    inner()
    expect(document.title).toBe('Outer')
    expect(document.head.querySelector('meta[property="og:title"]')?.getAttribute('content')).toBe(
      'Outer OG',
    )
    outer()
    expect(document.title).toBe('Base')
    expect(head()).toBe('')
  })

  it('touches only the keys it was given', () => {
    document.head.innerHTML = '<meta name="robots" content="noindex">'
    const restore = applyDocumentMetadata({ meta: { description: 'Route' } })
    expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe(
      'noindex',
    )
    restore()
    // The tag it never wrote is still there, untouched by the cleanup.
    expect(head()).toBe('<meta name="robots" content="noindex">')
  })

  it('leaves the document alone for empty metadata', () => {
    document.title = 'Base'
    const restore = applyDocumentMetadata({})
    expect(document.title).toBe('Base')
    expect(head()).toBe('')
    restore()
    expect(document.title).toBe('Base')
  })
})

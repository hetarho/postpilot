import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'

function content(selector: string): string | null {
  return document.head.querySelector(selector)?.getAttribute('content') ?? null
}
function href(selector: string): string | null {
  return document.head.querySelector(selector)?.getAttribute('href') ?? null
}

const APP_TITLE_KO = 'Postpilot — 사진과 경험을 블로그 글로'
const APP_TITLE_EN = 'Postpilot — Turn photos and experiences into blog posts'

beforeEach(() => {
  document.head.innerHTML = ''
  initializeI18n('ko')
})
afterEach(() => {
  initializeI18n('ko')
})

describe('/about document metadata', () => {
  // A10: the complete localized set, with canonical and og:url derived from the current origin.
  it('applies the complete localized set with origin-derived URLs', async () => {
    renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    const url = `${window.location.origin}/about`
    expect(document.title).toBe('Postpilot이란? — 사진과 메모로 블로그 글 초안 만들기')
    expect(content('meta[name="description"]')).toMatch(/사진과 거친 메모를/)
    expect(content('meta[property="og:type"]')).toBe('website')
    expect(content('meta[property="og:title"]')).toBe(document.title)
    expect(content('meta[property="og:description"]')).toBe(content('meta[name="description"]'))
    expect(content('meta[property="og:locale"]')).toBe('ko_KR')
    expect(content('meta[property="og:url"]')).toBe(url)
    expect(href('link[rel="canonical"]')).toBe(url)
    // A11: no invented og:image.
    expect(document.head.querySelector('meta[property="og:image"]')).toBeNull()
  })

  // A6/A10: switching locale while mounted reapplies the WHOLE set, and the URL stays /about.
  it('reapplies every tag when the locale changes, without moving the URL', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    await user.click(screen.getByRole('button', { name: '언어' }))
    await user.click(screen.getByRole('menuitemradio', { name: 'English' }))

    await waitFor(() => expect(document.title).toMatch(/^What is Postpilot\?/))
    expect(content('meta[property="og:title"]')).toBe(document.title)
    expect(content('meta[property="og:locale"]')).toBe('en_US')
    expect(content('meta[name="description"]')).toMatch(/Turn photos and rough notes/)
    expect(href('link[rel="canonical"]')).toBe(`${window.location.origin}/about`)
    expect(router.state.location.pathname).toBe('/about')
  })

  // A10: leaving the route removes what About created and hands the title back to the APP default
  // for the locale that is current now — not the one that was current when About mounted.
  it('restores the app default on navigation away, in the current locale', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    await user.click(screen.getByRole('button', { name: '언어' }))
    await user.click(screen.getByRole('menuitemradio', { name: 'English' }))
    await waitFor(() => expect(document.title).toMatch(/^What is Postpilot\?/))

    await router.navigate({ to: '/login' })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))

    await waitFor(() => expect(document.title).toBe(APP_TITLE_EN))
    // Every tag About created is gone; nothing marketing-specific leaks into /login.
    for (const property of ['og:type', 'og:title', 'og:description', 'og:locale', 'og:url']) {
      expect(document.head.querySelector(`meta[property="${property}"]`)).toBeNull()
    }
    expect(document.head.querySelector('link[rel="canonical"]')).toBeNull()
  })

  // The cleanup must not delete another route's metadata: a description that existed before About
  // mounted is written back rather than removed.
  it('writes back a description that existed before it mounted', async () => {
    document.head.innerHTML = '<meta name="description" content="Existing app description">'
    const { router } = renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })
    expect(content('meta[name="description"]')).toMatch(/사진과 거친 메모를/)

    await router.navigate({ to: '/login' })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))

    // Back to the app default for the current locale — one element, not a duplicate.
    await waitFor(() => expect(document.title).toBe(APP_TITLE_KO))
    expect(document.head.querySelectorAll('meta[name="description"]')).toHaveLength(1)
  })

  // A10: reaching About from another route replaces that route's metadata and gives it back.
  it('replaces and restores metadata when entered from another route', async () => {
    const { router } = renderAppAt('/login')
    await screen.findByRole('button', { name: '로그인' })
    expect(document.title).toBe(APP_TITLE_KO)

    await router.navigate({ to: '/about' })
    await screen.findByRole('heading', { level: 1 })
    expect(document.title).toBe('Postpilot이란? — 사진과 메모로 블로그 글 초안 만들기')

    await router.navigate({ to: '/login' })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    await waitFor(() => expect(document.title).toBe(APP_TITLE_KO))
  })
})

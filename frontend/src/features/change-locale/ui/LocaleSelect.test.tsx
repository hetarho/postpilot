import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient } from '@tanstack/react-query'
import { initializeI18n, resolveLocale } from '@/app/providers/i18n'
import { LOCALE_STORAGE_KEY } from '@/shared/lib/localization'
import { LocaleSelect } from './LocaleSelect'

afterEach(() => {
  initializeI18n('ko')
  localStorage.clear()
  history.replaceState(null, '', '/')
  vi.restoreAllMocks()
})

describe('LocaleSelect', () => {
  it('uses autonyms, persists a canonical override, and preserves focus and URL', async () => {
    history.replaceState(null, '', '/posts?view=recent#draft')
    render(<LocaleSelect />)
    const tabs = screen.getByRole('tablist', { name: '언어' })
    expect(tabs).toBeInTheDocument()

    const english = screen.getByRole('tab', { name: 'English' })
    await userEvent.click(english)

    expect(await screen.findByRole('tab', { name: 'English', selected: true })).toHaveFocus()
    expect(screen.getByRole('tab', { name: '한국어', selected: false })).toBeInTheDocument()
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en')
    expect(document.documentElement.lang).toBe('en')
    expect(location.pathname + location.search + location.hash).toBe('/posts?view=recent#draft')
    expect(await screen.findByText('Interface language changed to English.')).toBeInTheDocument()
    expect(resolveLocale({ navigatorLanguages: ['ko-KR'] })).toBe('en')
  })

  it('is keyboard reachable and still changes in memory when storage is denied', async () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('denied')
    })
    render(<LocaleSelect />)

    await userEvent.tab()
    const korean = screen.getByRole('tab', { name: '한국어' })
    expect(korean).toHaveFocus()
    await userEvent.keyboard('{ArrowRight}')

    expect(await screen.findByRole('tab', { name: 'English', selected: true })).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('en')
  })

  it('switches back to Korean without provider traffic or query invalidation and survives reload bootstrap', async () => {
    initializeI18n('en')
    const invalidate = vi.spyOn(QueryClient.prototype, 'invalidateQueries')
    const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('unexpected request'))
    history.replaceState(null, '', '/voices?tab=profile#current')

    const view = render(<LocaleSelect />)
    expect(screen.getByRole('tablist', { name: 'Language' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: '한국어' }))

    expect(await screen.findByRole('tab', { name: '한국어', selected: true })).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('ko')
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('ko')
    expect(invalidate).not.toHaveBeenCalled()
    expect(fetch).not.toHaveBeenCalled()
    expect(location.pathname + location.search + location.hash).toBe('/voices?tab=profile#current')

    view.unmount()
    initializeI18n(resolveLocale({ navigatorLanguages: ['en-US'] }))
    render(<LocaleSelect />)
    expect(screen.getByRole('tablist', { name: '언어' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: '한국어', selected: true })).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('ko')
  })
})

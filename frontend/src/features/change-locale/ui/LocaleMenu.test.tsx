import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient } from '@tanstack/react-query'
import { initializeI18n, resolveLocale } from '@/app/providers/i18n'
import { LOCALE_STORAGE_KEY } from '@/shared/lib/localization'
import { LocaleMenu } from './LocaleMenu'

afterEach(() => {
  initializeI18n('ko')
  localStorage.clear()
  history.replaceState(null, '', '/')
  vi.restoreAllMocks()
})

describe('LocaleMenu', () => {
  it('uses autonyms, persists a canonical override, and preserves focus and URL', async () => {
    history.replaceState(null, '', '/posts?view=recent#draft')
    render(<LocaleMenu />)
    const trigger = screen.getByRole('button', { name: '언어' })
    expect(trigger.querySelector('svg')).toHaveClass('lucide-languages')

    await userEvent.click(trigger)
    expect(screen.getAllByRole('menuitemradio').map((option) => option.textContent)).toEqual([
      '한국어',
      'English',
    ])
    await userEvent.click(screen.getByRole('menuitemradio', { name: 'English' }))

    // The menu closes and focus returns to the trigger, now named in the new language.
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Language' })).toHaveFocus()
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en')
    expect(document.documentElement.lang).toBe('en')
    expect(location.pathname + location.search + location.hash).toBe('/posts?view=recent#draft')
    expect(await screen.findByText('Interface language changed to English.')).toBeInTheDocument()
    expect(resolveLocale({ navigatorLanguages: ['ko-KR'] })).toBe('en')
  })

  it('is keyboard operable and still changes in memory when storage is denied', async () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('denied')
    })
    render(<LocaleMenu />)

    await userEvent.tab()
    expect(screen.getByRole('button', { name: '언어' })).toHaveFocus()
    await userEvent.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitemradio', { name: '한국어' })).toHaveFocus()
    await userEvent.keyboard('{ArrowDown}{Enter}')

    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(document.documentElement.lang).toBe('en')
  })

  it('switches back to Korean without provider traffic or query invalidation and survives reload bootstrap', async () => {
    initializeI18n('en')
    const invalidate = vi.spyOn(QueryClient.prototype, 'invalidateQueries')
    const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('unexpected request'))
    history.replaceState(null, '', '/voices?tab=profile#current')

    const view = render(<LocaleMenu />)
    await userEvent.click(screen.getByRole('button', { name: 'Language' }))
    await userEvent.click(screen.getByRole('menuitemradio', { name: '한국어' }))

    expect(await screen.findByRole('button', { name: '언어' })).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('ko')
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('ko')
    expect(invalidate).not.toHaveBeenCalled()
    expect(fetch).not.toHaveBeenCalled()
    expect(location.pathname + location.search + location.hash).toBe('/voices?tab=profile#current')

    view.unmount()
    initializeI18n(resolveLocale({ navigatorLanguages: ['en-US'] }))
    render(<LocaleMenu />)
    const trigger = screen.getByRole('button', { name: '언어' })
    await userEvent.click(trigger)
    expect(screen.getByRole('menuitemradio', { name: '한국어', checked: true })).toBeInTheDocument()
  })
})

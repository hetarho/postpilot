import { useState } from 'react'
import i18next from 'i18next'
import { QueryClient } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { resolveEffectiveTheme, type ThemePreference } from '@/shared/lib'
import { ThemeControllerProvider } from '../model/theme-controller'
import { ThemeSelector } from './ThemeSelector'

const koLabels: Record<ThemePreference, string> = {
  system: '시스템',
  light: '밝게',
  dark: '어둡게',
}

function Harness({
  initial = 'system',
  onChange,
}: {
  initial?: ThemePreference
  onChange?: (value: ThemePreference) => void
}) {
  const [preference, setPreference] = useState<ThemePreference>(initial)
  return (
    <ThemeControllerProvider
      value={{
        preference,
        effectiveTheme: resolveEffectiveTheme(preference, false),
        setPreference: (next) => {
          setPreference(next)
          onChange?.(next)
        },
      }}
    >
      <ThemeSelector />
    </ThemeControllerProvider>
  )
}

afterEach(async () => {
  cleanup()
  await i18next.changeLanguage('ko')
  history.replaceState(null, '', '/')
  vi.restoreAllMocks()
})

describe('ThemeSelector', () => {
  it.each([
    {
      locale: 'ko',
      label: '테마',
      options: ['시스템', '밝게', '어둡게'],
    },
    {
      locale: 'en',
      label: 'Theme',
      options: ['System', 'Light', 'Dark'],
    },
  ])(
    'exposes the selected preference and translated choices in $locale',
    async ({ locale, label, options }) => {
      await i18next.changeLanguage(locale)
      render(<Harness initial="dark" />)

      const tabs = screen.getByRole('tablist', { name: label })
      expect(tabs).toHaveClass('min-h-11')
      expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(options)
      expect(screen.getByRole('tab', { selected: true })).toHaveTextContent(options[2])
    },
  )

  it.each([
    ['light', 'dark'],
    ['dark', 'system'],
    ['system', 'light'],
  ] as const)('changes %s to %s through the injected controller only', async (initial, next) => {
    history.replaceState(null, '', '/posts?view=recent#draft')
    const invalidate = vi.spyOn(QueryClient.prototype, 'invalidateQueries')
    const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('unexpected request'))
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<Harness initial={initial} onChange={onChange} />)

    await user.tab()
    expect(screen.getByRole('tab', { name: koLabels[initial] })).toHaveFocus()
    await user.click(screen.getByRole('tab', { name: koLabels[next] }))

    expect(screen.getByRole('tab', { name: koLabels[next], selected: true })).toHaveFocus()
    expect(onChange).toHaveBeenCalledWith(next)
    expect(document.documentElement.lang).toBe('ko')
    expect(location.pathname + location.search + location.hash).toBe('/posts?view=recent#draft')
    expect(invalidate).not.toHaveBeenCalled()
    expect(fetch).not.toHaveBeenCalled()
  })
})

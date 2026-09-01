import { useState } from 'react'
import i18next from 'i18next'
import { QueryClient } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { resolveEffectiveTheme, type ThemePreference } from '@/shared/lib'
import { ThemeControllerProvider } from '../model/theme-controller'
import { ThemeMenu } from './ThemeMenu'

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
      <ThemeMenu />
    </ThemeControllerProvider>
  )
}

afterEach(async () => {
  cleanup()
  await i18next.changeLanguage('ko')
  history.replaceState(null, '', '/')
  vi.restoreAllMocks()
})

describe('ThemeMenu', () => {
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
    'opens translated choices from an icon trigger and marks the stored one in $locale',
    async ({ locale, label, options }) => {
      await i18next.changeLanguage(locale)
      const user = userEvent.setup()
      render(<Harness initial="dark" />)

      const trigger = screen.getByRole('button', { name: label })
      expect(trigger).toHaveClass('size-11')
      expect(trigger).toHaveAccessibleDescription(
        locale === 'ko' ? '현재 테마 설정: 어둡게' : 'Current theme preference: Dark',
      )
      expect(trigger.querySelector('svg')).toHaveClass('lucide-moon')
      await user.click(trigger)

      const menu = screen.getByRole('menu', { name: label })
      expect(menu).toHaveClass('bg-surface-highest')
      expect(screen.getAllByRole('menuitemradio').map((option) => option.textContent)).toEqual(
        options,
      )
      expect(screen.getByRole('menuitemradio', { checked: true })).toHaveTextContent(options[2])
    },
  )

  it.each([
    ['light', 'dark', 'lucide-moon'],
    ['dark', 'system', 'lucide-monitor'],
    ['system', 'light', 'lucide-sun'],
  ] as const)(
    'changes %s to %s through the injected controller only, and the trigger icon follows',
    async (initial, next, iconClass) => {
      history.replaceState(null, '', '/posts?view=recent#draft')
      const invalidate = vi.spyOn(QueryClient.prototype, 'invalidateQueries')
      const fetch = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('unexpected request'))
      const onChange = vi.fn()
      const user = userEvent.setup()
      render(<Harness initial={initial} onChange={onChange} />)

      const trigger = screen.getByRole('button', { name: '테마' })
      await user.click(trigger)
      await user.click(screen.getByRole('menuitemradio', { name: koLabels[next] }))

      expect(onChange).toHaveBeenCalledWith(next)
      // The menu closed, focus came home, and the trigger now wears the new preference.
      expect(screen.queryByRole('menu')).not.toBeInTheDocument()
      expect(trigger).toHaveFocus()
      expect(trigger.querySelector('svg')).toHaveClass(iconClass)
      expect(trigger).toHaveAccessibleDescription(`현재 테마 설정: ${koLabels[next]}`)
      expect(document.documentElement.lang).toBe('ko')
      expect(location.pathname + location.search + location.hash).toBe('/posts?view=recent#draft')
      expect(invalidate).not.toHaveBeenCalled()
      expect(fetch).not.toHaveBeenCalled()
    },
  )

  it('reselecting the current preference closes without a controller call', async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<Harness initial="system" onChange={onChange} />)

    await user.click(screen.getByRole('button', { name: '테마' }))
    await user.click(screen.getByRole('menuitemradio', { name: '시스템' }))

    expect(onChange).not.toHaveBeenCalled()
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })
})

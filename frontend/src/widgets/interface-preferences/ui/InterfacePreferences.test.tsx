import { useState } from 'react'
import i18next from 'i18next'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { ThemeControllerProvider } from '@/features/change-theme'
import { resolveEffectiveTheme, type ThemePreference } from '@/shared/lib'
import * as publicApi from '../index'

function Harness() {
  const [preference, setPreference] = useState<ThemePreference>('system')
  return (
    <ThemeControllerProvider
      value={{
        preference,
        effectiveTheme: resolveEffectiveTheme(preference, false),
        setPreference,
      }}
    >
      <publicApi.InterfacePreferences />
    </ThemeControllerProvider>
  )
}

afterEach(async () => {
  cleanup()
  await i18next.changeLanguage('ko')
  localStorage.clear()
})

describe('InterfacePreferences', () => {
  it('keeps one stable public component export', () => {
    expect(Object.keys(publicApi)).toEqual(['InterfacePreferences'])
  })

  it.each([
    { locale: 'ko', group: '인터페이스 환경설정', theme: '테마', language: '언어' },
    { locale: 'en', group: 'Interface preferences', theme: 'Theme', language: 'Language' },
  ])(
    'exposes both preferences as translated icon menus in one group in $locale',
    async ({ locale, group, theme, language }) => {
      await i18next.changeLanguage(locale)
      render(<Harness />)

      const container = screen.getByRole('group', { name: group })
      const themeTrigger = within(container).getByRole('button', { name: theme })
      const localeTrigger = within(container).getByRole('button', { name: language })
      // Icon-only 44px triggers; the theme one wears the stored preference (System = monitor).
      for (const trigger of [themeTrigger, localeTrigger]) {
        expect(trigger).toHaveClass('size-11')
        expect(trigger).toHaveAttribute('aria-haspopup', 'menu')
      }
      expect(themeTrigger.querySelector('svg')).toHaveClass('lucide-monitor')
      expect(localeTrigger.querySelector('svg')).toHaveClass('lucide-languages')
    },
  )

  it('changes both preferences through their own menus, one open at a time', async () => {
    const user = userEvent.setup()
    render(<Harness />)

    await user.click(screen.getByRole('button', { name: '테마' }))
    await user.click(screen.getByRole('menuitemradio', { name: '어둡게' }))
    expect(screen.getByRole('button', { name: '테마' }).querySelector('svg')).toHaveClass(
      'lucide-moon',
    )

    await user.click(screen.getByRole('button', { name: '언어' }))
    const menu = screen.getByRole('menu', { name: '언어' })
    expect(menu).toHaveClass('bg-surface-highest')
    expect(menu).not.toHaveClass('border')
    await user.click(screen.getByRole('menuitemradio', { name: 'English' }))

    // Locale changed while the theme choice stayed.
    expect(await screen.findByRole('button', { name: 'Theme' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Theme' }).querySelector('svg')).toHaveClass(
      'lucide-moon',
    )
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('returns focus to the trigger when a menu closes on Escape', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const trigger = screen.getByRole('button', { name: '테마' })
    await user.click(trigger)
    expect(screen.getByRole('menu', { name: '테마' })).toBeInTheDocument()

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })
})

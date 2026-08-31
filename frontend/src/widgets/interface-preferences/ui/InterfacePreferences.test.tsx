import { useState } from 'react'
import i18next from 'i18next'
import { cleanup, render, screen } from '@testing-library/react'
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
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 })
})

describe('InterfacePreferences', () => {
  it('keeps one stable public component export', () => {
    expect(Object.keys(publicApi)).toEqual(['InterfacePreferences'])
  })

  it.each([
    {
      locale: 'ko',
      trigger: '인터페이스 환경설정',
      dialog: '인터페이스 환경설정',
      language: '언어',
      theme: '테마',
    },
    {
      locale: 'en',
      trigger: 'Interface preferences',
      dialog: 'Interface preferences',
      language: 'Language',
      theme: 'Theme',
    },
  ])(
    'opens a compact, translated preference surface in $locale',
    async ({ locale, trigger, dialog, language, theme }) => {
      await i18next.changeLanguage(locale)
      const user = userEvent.setup()
      render(<Harness />)

      const button = screen.getByRole('button', { name: trigger })
      expect(button).toHaveClass('min-h-11')
      const wideLabel = [...button.querySelectorAll('span')].find((value) =>
        value.classList.contains('sm:inline'),
      )
      expect(wideLabel).toHaveTextContent(locale === 'ko' ? '환경설정' : 'Preferences')
      await user.click(button)

      const panel = screen.getByRole('dialog', { name: dialog })
      expect(panel).toHaveClass('top-full', 'w-72', 'bg-surface-highest')
      expect(panel).not.toHaveClass('border')
      const languageSelect = screen.getByRole('combobox', { name: language })
      const themeSelect = screen.getByRole('combobox', { name: theme })
      expect(languageSelect).toHaveFocus()
      expect(languageSelect).toHaveClass('min-h-11')
      expect(themeSelect).toHaveClass('min-h-11')
    },
  )

  it('keeps the dialog open across both preferences and returns focus on Escape', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const trigger = screen.getByRole('button', { name: '인터페이스 환경설정' })
    await user.click(trigger)

    await user.selectOptions(screen.getByRole('combobox', { name: '테마' }), 'dark')
    expect(screen.getByRole('combobox', { name: '테마' })).toHaveValue('dark')
    await user.selectOptions(screen.getByRole('combobox', { name: '언어' }), 'en')
    expect(screen.getByRole('dialog', { name: 'Interface preferences' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Theme' })).toHaveValue('dark')

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Interface preferences' })).toHaveFocus()
  })

  it('keeps a compact trigger and 44px controls at 320px', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 320 })
    window.dispatchEvent(new Event('resize'))
    await i18next.changeLanguage('en')
    const user = userEvent.setup()
    render(<Harness />)

    const trigger = screen.getByRole('button', { name: 'Interface preferences' })
    expect(trigger).toHaveClass('min-h-11', 'px-3', 'sm:px-4')
    expect(trigger.querySelector('svg')).toHaveClass('size-5')
    const responsiveLabel = [...trigger.querySelectorAll('span')].find((value) =>
      value.classList.contains('sm:inline'),
    )
    expect(responsiveLabel).toHaveClass('hidden', 'sm:inline')

    await user.click(trigger)
    expect(screen.getByRole('dialog', { name: 'Interface preferences' })).toHaveClass('w-72')
    expect(screen.getByRole('combobox', { name: 'Language' })).toHaveClass('min-h-11')
    expect(screen.getByRole('combobox', { name: 'Theme' })).toHaveClass('min-h-11')
  })
})

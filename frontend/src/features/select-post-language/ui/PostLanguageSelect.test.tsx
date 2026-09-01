import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code } from '@connectrpc/connect'
import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { connectAppError } from '@/test/app-error'
import { PostLanguageSelect } from './PostLanguageSelect'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

it.each([
  ['ko', '글 언어', '지원하지 않는 글 언어예요.'],
  ['en', 'Post language', 'That post language is not supported.'],
] as const)(
  'renders a structured %s failure and marks the select invalid',
  async (locale, label, copy) => {
    initializeI18n(locale)
    const user = userEvent.setup()
    render(
      <PostLanguageSelect
        value="ko"
        onSelect={() =>
          Promise.reject(connectAppError('POST_TARGET_LANGUAGE_UNSUPPORTED', Code.InvalidArgument))
        }
      />,
    )

    // A WAI-APG select-only combobox names itself "<label> <current value>", so the label is a
    // prefix rather than the whole name.
    const select = screen.getByRole('combobox', { name: new RegExp(label) })
    await user.click(select)
    await user.click(screen.getByRole('option', { name: locale === 'ko' ? '영어' : 'English' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(copy)
    expect(select).toHaveAttribute('aria-invalid', 'true')
    expect(document.body).not.toHaveTextContent('private backend prose')
  },
)

import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it } from 'vitest'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'

beforeEach(() => sessionStorage.clear())

/** 홈 changes a piece's default in place: the piece opens a sheet holding the piece screen's own
 *  control, a change is saved there, and the sheet's state line follows the saved data. */
it('picks the writing AI in a sheet on 홈 and keeps 템플릿 a link to its screen', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  renderAppAt('/', {
    user: { id: 'alice' },
    providers: {
      calls,
      models: [
        { providerId: 'openrouter', modelId: 'writer-a', label: 'Writer A' },
        { providerId: 'openrouter', modelId: 'writer-b', label: 'Writer B' },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer-a' }],
    },
  })
  const piece = await screen.findByRole('button', { name: /^글 쓰는 AI: 준비됨, Writer A/ })
  expect(piece).toHaveAttribute('aria-haspopup', 'dialog')
  // 템플릿 has no account default (TMPL-54): it stays the way to its screen.
  expect(screen.getByRole('link', { name: /^템플릿:/ })).toHaveAttribute('href', '/templates')

  await user.click(piece)
  const sheet = within(await screen.findByRole('dialog', { name: '글 쓰는 AI' }))
  expect(sheet.getByRole('link', { name: /AI 토너먼트로 가기/ })).toHaveAttribute(
    'href',
    '/ai-models#ai-in-use',
  )
  // The tournament row's own picker, under the row rather than in a second sheet.
  expect(sheet.queryByRole('button', { name: /^바꾸기 · / })).toBeNull()
  await user.click(await sheet.findByRole('radio', { name: /Writer B/ }))
  await waitFor(() => expect(calls.filter((call) => call === 'SaveSelection')).toHaveLength(1))
  await waitFor(() => expect(sheet.getAllByRole('status')[0]).toHaveTextContent('Writer B'))
  expect(screen.getByRole('button', { name: /^글 쓰는 AI: 준비됨, Writer B/ })).toBeInTheDocument()
})

it('makes another finished voice the default from 홈, and offers none to one still being made', async () => {
  const user = userEvent.setup()
  renderAppAt('/', {
    user: { id: 'alice' },
    voice: {
      voices: [
        { id: 'blog', name: '블로그 말투', isDefault: true },
        { id: 'diary', name: '일기 말투' },
        { id: 'draft', name: '만드는 말투', made: false },
      ],
    },
  })
  await user.click(await screen.findByRole('button', { name: /^말투: 준비됨, 블로그 말투/ }))
  const sheet = within(await screen.findByRole('dialog', { name: '말투' }))
  const list = within(await sheet.findByRole('list', { name: '내 말투' }))
  const row = (name: string) => within(list.getByRole('link', { name }).closest('li')!)
  expect(row('만드는 말투').queryByRole('button')).toBeNull()
  await user.click(row('일기 말투').getByRole('button', { name: '기본으로 설정' }))
  await waitFor(() => expect(sheet.getAllByRole('status')[0]).toHaveTextContent('일기 말투'))
  expect(row('일기 말투').getByRole('button', { name: '기본 해제' })).toBeInTheDocument()
})

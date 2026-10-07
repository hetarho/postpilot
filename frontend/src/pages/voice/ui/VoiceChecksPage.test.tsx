import { afterEach, expect, it } from 'vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Transport } from '@connectrpc/connect'
import { initializeI18n } from '@/app/providers/i18n'
import { WritingTestService, ProtoVoiceCheckStatus } from '@/shared/api'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { createFakeAuthTransport, type FakeAuthOptions } from '@/test/session'
import { renderAppAt } from '@/test/app'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
  sessionStorage.clear()
})
function renderHistory(at: string, options: FakeAuthOptions = {}) {
  const calls: string[] = []
  const base = createFakeAuthTransport({ existingSetup: true, user: { id: 'alice' }, ...options })
  const fixture = createWritingTestStudioFixture({ readableTest: true })
  const transport: Transport = {
    ...base,
    unary: async (...args) => {
      const method = args[0]
      calls.push(method.name)
      return (
        method.parent.typeName === WritingTestService.typeName ? fixture.transport : base
      ).unary(...args)
    },
  }
  return { ...renderAppAt(at, { transport }), calls, fixture }
}
it('aliases old check addresses into named common history without admitting or retrying checks', async () => {
  const view = renderHistory('/voices/voice-default/checks')
  await screen.findByRole('heading', { name: '테스트 기록', level: 1 })
  expect(view.router.state.location.pathname).toBe('/tests/history')
  expect(view.router.state.location.search).toMatchObject({
    stage: 'voice',
    voiceId: 'voice-default',
    entry: '/voices',
  })
  expect(screen.queryByRole('button', { name: /검증하기|다시 검증/ })).toBeNull()
  expect(view.calls.filter((name) => /^(Start|Retry|Analyze|Update|Create)/.test(name))).toEqual([])
})
it('retains paid answers, generated writing, withdrawal and previous-analysis markers as read only', async () => {
  const view = renderHistory('/voices/voice-default/checks', {
    voice: {
      checks: [
        {
          id: 'paid-check',
          status: ProtoVoiceCheckStatus.DONE,
          piece: '예전에 결제한 완성 글',
          answer: '이전의 내 답',
          stale: true,
          createdAt: '2026-10-01T00:00:00Z',
        },
        {
          id: 'withdrawn',
          status: ProtoVoiceCheckStatus.DONE,
          piece: '답 삭제 후 남은 결제 결과',
          answerDeleted: true,
        },
      ],
    },
  })
  expect(await screen.findByText('예전에 결제한 완성 글')).toBeVisible()
  expect(screen.getByText('이전의 내 답')).toBeVisible()
  expect(screen.getByText('이전 분석으로 만든 결과')).toBeVisible()
  expect(screen.getByText('이 답은 학습 데이터에서 삭제했어요.')).toBeVisible()
  expect(screen.getByText('답 삭제 후 남은 결제 결과')).toBeVisible()
  await userEvent.click(screen.getAllByRole('button', { name: '현재 결과 확인' })[0]!)
  expect(
    view.calls.filter((name) => /^(Start|RetryVoiceCheck|Analyze|Update|Create)/.test(name)),
  ).toEqual([])
})
it('uses two local panels and explicit common writing test/history entries for a made voice', async () => {
  const view = renderHistory('/voices/voice-default')
  await screen.findByRole('heading', { name: '기본 말투', level: 1 })
  const nav = screen.getByRole('navigation', { name: '말투 설정' })
  expect(within(nav).getAllByRole('link')).toHaveLength(2)
  const link = screen.getByRole('link', { name: '글쓰기 테스트' })
  expect(link).toHaveAttribute('href', expect.stringContaining('factor=voice'))
  await userEvent.click(link)
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/tests'))
  expect(view.router.state.location.search).toMatchObject({
    factor: 'voice',
    count: 2,
    voiceId: 'voice-default',
    entry: '/voices',
  })
  expect(view.calls.filter((name) => /^(Start|Analyze|Update|Create)/.test(name))).toEqual([])
  await act(() =>
    view.router.navigate({
      to: '/tests/history',
      search: { stage: 'voice', voiceId: 'voice-default' },
    }),
  )
  await screen.findByRole('heading', { name: '테스트 기록', level: 1 })
})

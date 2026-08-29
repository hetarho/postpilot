import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { VoiceValueSource } from '@/shared/api'
import { voiceProfileQueryKey } from '@/entities/voice-profile'
import { renderAppAt } from '@/test/app'

/** A learned profile whose axes are partly unanswered — the state the analysis produces once it
 *  stops fabricating a neutral 0 for an axis the model never addressed. */
const LEARNED = {
  empty: false,
  meta: { version: 3n, sourceCount: 2 },
  lexical: {
    description: { value: '담백한 어휘', source: VoiceValueSource.ANALYZED, unknown: false },
  },
  endings: {
    baseRegister: { value: '해요체', source: VoiceValueSource.MEASURED, unknown: false },
  },
  axes: { involvement: 2 },
}

describe('the 말투 tab', () => {
  // Change 04 A1.
  it('renders the profile and none of the other tabs’ panels', async () => {
    renderAppAt('/voice', { user: { id: 'alice' }, voice: { structured: LEARNED } })

    expect(await screen.findByRole('heading', { level: 1, name: '말투' })).toBeInTheDocument()
    expect(screen.getByText('현재 말투 프로필')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '검증 시작' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '복원' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('문체 규칙')).not.toBeInTheDocument()
    expect(screen.queryByText('학습 샘플')).not.toBeInTheDocument()
  })

  // Change 04 A4: the three detail lists belong to the tabs that display them.
  it('issues no version, confirmation or validation request on mount', async () => {
    const calls: string[] = []
    renderAppAt('/voice', { user: { id: 'alice' }, calls, voice: { structured: LEARNED } })

    await screen.findByText('현재 말투 프로필')
    await waitFor(() => expect(calls).toContain('GetVoiceProfile'))
    expect(calls).not.toContain('ListVoiceProfileVersions')
    expect(calls).not.toContain('ListRuleConfirmations')
    expect(calls).not.toContain('ListVoiceProfileValidations')
  })

  // Change 04 A11, frontend half: an axis the analysis never answered is not a measurement.
  it('shows an unanswered axis as 알 수 없음 rather than 0', async () => {
    renderAppAt('/voice', { user: { id: 'alice' }, voice: { structured: LEARNED } })

    const axes = (await screen.findByText('여섯 성향 (-3~3)')).closest('section')!
    expect(within(axes).getByText('관여도').nextElementSibling).toHaveTextContent('2')
    expect(within(axes).getByText('서사성').nextElementSibling).toHaveTextContent('알 수 없음')
    expect(within(axes).queryByText('0')).not.toBeInTheDocument()
  })
})

describe('the voice tab row', () => {
  // Change 04 A2 / A3.
  it('gives every tab an address and marks the current one', async () => {
    const { router } = renderAppAt('/voice', { user: { id: 'alice' } })

    const tabs = within(await screen.findByRole('navigation', { name: '말투 설정' })).getAllByRole(
      'link',
    )
    expect(tabs.map((tab) => tab.getAttribute('href'))).toEqual([
      '/voice',
      '/voice/versions',
      '/voice/import',
      '/voice/rules',
      '/voice/validations',
    ])
    expect(tabs[0]).toHaveAttribute('aria-current', 'page')
    // Change 04 A5, the mechanical half: the row scrolls instead of wrapping or crushing its five
    // Korean labels, and every tab keeps the 44px floor.
    expect(screen.getByRole('navigation', { name: '말투 설정' })).toHaveClass('overflow-x-auto')
    tabs.forEach((tab) => {
      expect(tab).toHaveClass('min-h-11')
      expect(tab).toHaveClass('whitespace-nowrap')
    })

    await userEvent.setup().click(tabs[3])
    await waitFor(() => expect(router.state.location.pathname).toBe('/voice/rules'))
    expect(await screen.findByRole('heading', { level: 1, name: '대조 규칙' })).toBeInTheDocument()

    router.history.back()
    await waitFor(() => expect(router.state.location.pathname).toBe('/voice'))
  })

  it.each([
    ['/voice/versions', '버전 기록'],
    ['/voice/import', '기존 글 가져오기'],
    ['/voice/rules', '대조 규칙'],
    ['/voice/validations', '프로필 검증'],
  ])('renders %s as its own screen on reload', async (path, heading) => {
    const { router } = renderAppAt(path, { user: { id: 'alice' } })

    expect(await screen.findByRole('heading', { level: 1, name: heading })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe(path)
    expect(screen.queryByText('현재 말투 프로필')).not.toBeInTheDocument()
  })
})

describe('the 기존 글 가져오기 tab', () => {
  it('resumes polling the active analysis exposed by the profile', async () => {
    const calls: string[] = []
    renderAppAt('/voice/import', {
      user: { id: 'alice' },
      calls,
      voice: { activeJobId: 'voice-job' },
      jobs: {
        jobs: [
          {
            id: 'voice-job',
            kind: 'analyze_voice',
            status: 'running',
            stage: 'analyze',
            progressDone: 0,
            progressTotal: 1,
          },
        ],
      },
    })

    expect(await screen.findByText('문체 분석 중')).toBeInTheDocument()
    await waitFor(() => expect(calls).toContain('GetGeneration'))
  })

  it('refreshes the profile when the resumed analysis is already done', async () => {
    renderAppAt('/voice/import', {
      user: { id: 'alice' },
      voice: {
        activeJobId: 'voice-job',
        styleguideAfterAnalysis: '# 종결어미\n~다를 자주 사용',
      },
      jobs: {
        jobs: [
          {
            id: 'voice-job',
            kind: 'analyze_voice',
            status: 'done',
            stage: 'analyze',
            progressDone: 1,
            progressTotal: 1,
          },
        ],
      },
    })

    await waitFor(() =>
      expect(screen.getByLabelText('문체 규칙')).toHaveValue('# 종결어미\n~다를 자주 사용'),
    )
  })

  it('persists an edited styleguide across leaving and reopening the route', async () => {
    const calls: string[] = []
    const app = renderAppAt('/voice/import', {
      user: { id: 'alice' },
      calls,
      voice: { styleguide: '기존 규칙' },
    })
    const user = userEvent.setup()
    const editor = await screen.findByLabelText('문체 규칙')
    await user.clear(editor)
    await user.type(editor, '수정한 규칙')
    await user.click(within(editor.closest('section')!).getByRole('button', { name: '저장' }))
    await waitFor(() => expect(calls).toContain('UpdateVoiceProfile'))

    await app.router.navigate({ to: '/posts' })
    await screen.findByRole('heading', { name: '내 글' })
    app.queryClient.removeQueries({ queryKey: voiceProfileQueryKey(app.transport, 'alice') })
    await app.router.navigate({ to: '/voice/import' })

    expect(await screen.findByLabelText('문체 규칙')).toHaveValue('수정한 규칙')
  })
})

import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { voiceProfileQueryKey } from '@/entities/voice-profile'
import { renderAppAt } from '@/test/app'

describe('VoicePage', () => {
  it('resumes polling the active analysis exposed by the profile', async () => {
    const calls: string[] = []
    renderAppAt('/voice', {
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
    renderAppAt('/voice', {
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
    const app = renderAppAt('/voice', {
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
    await app.router.navigate({ to: '/voice' })

    expect(await screen.findByLabelText('문체 규칙')).toHaveValue('수정한 규칙')
  })
})

import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'
import { observedClipFixture } from '@/test/clip-observations'

describe('observations in the clip workspace', () => {
  it('keeps observations available on all three steps without invoking generation', async () => {
    const calls: string[] = []
    const project = observedClipFixture()
    renderAppAt('/clips/project', { user: { id: 'alice' }, clips: { projects: [project], calls } })
    const user = userEvent.setup()
    expect(await screen.findByRole('heading', { name: 'AI가 관찰한 내용' })).toBeVisible()
    const tabs = within(screen.getByRole('tablist', { name: '클립 단계' }))
    for (const name of ['클립 생성', '클립 다듬기', '클립 완성']) {
      await user.click(tabs.getByRole('tab', { name: new RegExp(name) }))
      expect(screen.getAllByRole('heading', { name: 'AI가 관찰한 내용' })).toHaveLength(1)
      await user.click(screen.getByRole('button', { name: '관찰 구간 2개 자세히 보기' }))
      expect(screen.getByText('1번 컷에 사용 · 원본 0:03.500–0:10')).toBeVisible()
    }
    expect(calls).not.toContain('StartClipGeneration')
    expect(calls).not.toContain('QuoteClipGeneration')
    expect(calls).not.toContain('CreateClipSourceBatch')
  })

  it.each(['queued', 'running', 'failed'])(
    'identifies previous observations during a %s regeneration',
    async (status) => {
      renderAppAt('/clips/project', {
        user: { id: 'alice' },
        clips: {
          projects: [
            {
              ...observedClipFixture(),
              latestJob: { id: 'new-run', kind: 'generate_clip', status, stage: 'analyze' },
            },
          ],
        },
        jobs: { jobs: [{ id: 'new-run', kind: 'generate_clip', status, stage: 'analyze' }] },
      })
      expect(await screen.findByText(/이전 생성의 관찰 결과예요/)).toBeVisible()
      await userEvent.click(screen.getByRole('button', { name: /자세히 보기/ }))
      expect(screen.getByText('맛있어요')).toBeVisible()
    },
  )
})

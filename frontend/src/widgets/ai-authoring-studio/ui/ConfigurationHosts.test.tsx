import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

describe('configuration authoring entrypoints', () => {
  it.each([
    {
      route: '/templates/new',
      title: '새 템플릿',
      manual: true,
      choice: 'AI와 템플릿 만들기',
      goal: '글을 어떤 순서로 풀어 쓰고 싶으세요?',
    },
    {
      route: '/video-templates/new',
      title: '새 영상 템플릿',
      manual: true,
      choice: 'AI와 영상 템플릿 만들기',
      goal: '영상이 어떤 순서로 이어지면 좋을까요?',
    },
    {
      route: '/guidelines',
      title: '지침',
      trigger: 'AI로 지침 만들기',
      goal: '글을 쓸 때 어떤 점을 지키면 좋을까요?',
    },
    {
      route: '/video-guidelines',
      title: '영상 지침',
      trigger: 'AI로 영상 지침 만들기',
      goal: '영상 속 문구를 어떻게 쓰면 좋을까요?',
    },
    {
      route: '/voices',
      title: '말투',
      trigger: 'AI 말투 추천받기',
      goal: '어떤 말투로 이야기하고 싶으세요?',
    },
  ])('opens $route as a readable AI draft without starting or saving work', async (entry) => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt(entry.route, { user: { id: 'alice' }, calls, existingSetup: true })
    await screen.findByRole('heading', { level: 1, name: entry.title })
    if (entry.trigger) {
      const triggers = await screen.findAllByRole('button', { name: entry.trigger })
      const primary = triggers.find((button) => !button.closest('details'))!
      expect(primary).toBeInTheDocument()
      expect(calls).not.toContain('GetLatestAuthoringSession')
      await user.click(primary)
      const dialog = await screen.findByRole('dialog')
      expect(await within(dialog).findByRole('heading', { name: entry.goal })).toBeVisible()
    } else {
      if (entry.choice) await user.click(await screen.findByRole('button', { name: entry.choice }))
      expect(await screen.findByRole('heading', { name: entry.goal })).toBeVisible()
      expect(screen.queryByLabelText('템플릿 이름')).not.toBeInTheDocument()
      expect(screen.queryByLabelText('이름')).not.toBeInTheDocument()
      expect(screen.queryByLabelText('템플릿 이름')).not.toBeInTheDocument()
    }
    if (entry.route === '/voices')
      await waitFor(() => expect(calls).toContain('GetLatestAuthoringSession'))
    else await waitFor(() => expect(calls).toContain('ListAuthoringSummaries'))
    expect(
      calls.filter((name) => /^(Start|Generate|Analyze|Adopt|Create|SaveAuthoring)/.test(name)),
    ).toEqual([])
    expect(calls).not.toContain('GetLatestWritingVoiceCandidates')
  })
})

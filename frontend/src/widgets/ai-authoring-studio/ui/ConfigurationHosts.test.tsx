import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

describe('configuration authoring entrypoints', () => {
  it('opens /templates/new as the guided wizard without starting or saving work', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/templates/new', { user: { id: 'alice' }, calls, existingSetup: true })
    expect(
      await screen.findByRole('heading', { level: 2, name: '어떤 체험을 글로 남기나요?' }),
    ).toBeVisible()
    await user.click(screen.getByRole('button', { name: /^맛집/ }))
    expect(await screen.findByRole('heading', { level: 2, name: '어떤 식당인가요?' })).toBeVisible()
    await user.click(screen.getByRole('button', { name: '이전' }))
    expect(
      await screen.findByRole('heading', { level: 2, name: '어떤 체험을 글로 남기나요?' }),
    ).toBeVisible()
    expect(
      calls.filter((name) => /^(Start|Generate|Analyze|Adopt|Create|SaveAuthoring)/.test(name)),
    ).toEqual([])
    expect(calls).not.toContain('GetLatestWritingVoiceCandidates')
  })

  it('opens video-template creation as a wizard without creating an authoring session', async () => {
    const calls: string[] = []
    renderAppAt('/video-templates/new', { user: { id: 'alice' }, calls, existingSetup: true })
    expect(
      await screen.findByRole('heading', { name: '어떤 체험을 영상으로 남기나요?' }),
    ).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'AI와 영상 템플릿 만들기' }),
    ).not.toBeInTheDocument()
    expect(
      calls.filter((name) => /^(Start|Generate|Create|Adopt|SaveAuthoring)/.test(name)),
    ).toEqual([])
  })

  it.each([
    {
      route: '/guidelines',
      title: '지침',
      trigger: '새 지침',
      goal: '글을 쓸 때 어떤 점을 지키면 좋을까요?',
    },
    {
      route: '/video-guidelines',
      title: '영상 지침',
      trigger: '새 지침',
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
    // 새 지침 is the section's creation button, a link to its own `?new=1` (THEME-67).
    const triggers = await screen.findAllByRole(entry.route === '/voices' ? 'button' : 'link', {
      name: entry.trigger,
    })
    const primary = triggers.find((button) => !button.closest('details'))!
    expect(primary).toBeInTheDocument()
    expect(calls).not.toContain('GetLatestAuthoringSession')
    await user.click(primary)
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByRole('heading', { name: entry.goal })).toBeVisible()
    if (entry.route === '/voices')
      await waitFor(() => expect(calls).toContain('GetLatestAuthoringSession'))
    else await waitFor(() => expect(calls).toContain('ListAuthoringSummaries'))
    expect(
      calls.filter((name) => /^(Start|Generate|Analyze|Adopt|Create|SaveAuthoring)/.test(name)),
    ).toEqual([])
    expect(calls).not.toContain('GetLatestWritingVoiceCandidates')
  })
})

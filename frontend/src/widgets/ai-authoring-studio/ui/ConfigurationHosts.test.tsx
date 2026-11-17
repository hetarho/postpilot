import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

describe('configuration authoring entrypoints', () => {
  // A template is made by talking to the AI (owner 2026-11-17): /templates/new opens the
  // conversation, asking what the owner posts, with no wizard step.
  it('opens /templates/new as a conversation without starting or saving work', async () => {
    const calls: string[] = []
    renderAppAt('/templates/new', { user: { id: 'alice' }, calls, existingSetup: true })
    const log = await screen.findByRole('log', { name: '템플릿 대화' })
    expect(within(log).getByText(/어떤 글을 올리세요\?/)).toBeVisible()
    expect(screen.getByRole('textbox', { name: '무엇을 올리는지' })).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { level: 2, name: '어떤 체험을 글로 남기나요?' }),
    ).not.toBeInTheDocument()
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
      way: 'AI와 함께 만들기',
      goal: '글을 쓸 때 어떤 점을 지키면 좋을까요?',
    },
    {
      route: '/video-guidelines',
      title: '영상 지침',
      trigger: '새 지침',
      way: 'AI와 함께 만들기',
      goal: '영상 속 문구를 어떻게 쓰면 좋을까요?',
    },
    {
      route: '/voices',
      title: '말투',
      trigger: '새 말투 만들기',
      way: 'AI 말투 추천받기',
      goal: '어떤 말투로 이야기하고 싶으세요?',
    },
  ])('opens $route as a readable AI draft without starting or saving work', async (entry) => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt(entry.route, { user: { id: 'alice' }, calls, existingSetup: true })
    await screen.findByRole('heading', { level: 1, name: entry.title })
    // The section's one creation button; its AI way opens from it.
    const primary = await screen.findByRole('button', { name: entry.trigger })
    expect(calls).not.toContain('GetLatestAuthoringSession')
    await user.click(primary)
    await user.click(await screen.findByRole('menuitem', { name: new RegExp('^' + entry.way) }))
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

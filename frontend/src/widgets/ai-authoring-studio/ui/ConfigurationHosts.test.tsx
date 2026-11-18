import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

describe('configuration authoring entrypoints', () => {
  // A template is made by talking to the AI from scratch: /templates/new opens the conversation
  // on its first question, and nothing is created or started until the owner asks for a draft.
  it('opens /templates/new as the AI conversation without starting or saving work', async () => {
    const calls: string[] = []
    renderAppAt('/templates/new', { user: { id: 'alice' }, calls, existingSetup: true })
    expect(await screen.findByRole('textbox', { name: '어떤 글인지 말해 주세요' })).toBeVisible()
    expect(screen.queryByRole('heading', { name: '어떤 체험을 글로 남기나요?' })).toBeNull()
    expect(
      calls.filter((name) => /^(Start|Generate|Analyze|Adopt|Create|SaveAuthoring)/.test(name)),
    ).toEqual([])
    expect(calls).not.toContain('GetLatestWritingVoiceCandidates')
  })

  it('opens video-template creation as the AI conversation without creating a session', async () => {
    const calls: string[] = []
    renderAppAt('/video-templates/new', { user: { id: 'alice' }, calls, existingSetup: true })
    expect(await screen.findByRole('textbox', { name: '어떤 영상인지 말해 주세요' })).toBeVisible()
    expect(
      calls.filter((name) => /^(Start|Generate|Create|Adopt|SaveAuthoring)/.test(name)),
    ).toEqual([])
  })

  it.each([
    {
      route: '/guidelines',
      title: '지침',
      menu: '새 지침',
      way: /AI와 대화로 만들기/,
      goal: '글을 쓸 때 어떤 점을 지키면 좋을까요?',
    },
    {
      route: '/video-guidelines',
      title: '영상 지침',
      menu: '새 지침',
      way: /AI와 대화로 만들기/,
      goal: '영상 속 문구를 어떻게 쓰면 좋을까요?',
    },
    {
      route: '/voices',
      title: '말투',
      menu: '새 말투',
      way: /AI와 대화로 만들기/,
      goal: '어떤 말투로 이야기하고 싶으세요?',
    },
  ])('opens $route as a readable AI draft without starting or saving work', async (entry) => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt(entry.route, { user: { id: 'alice' }, calls, existingSetup: true })
    await screen.findByRole('heading', { level: 1, name: entry.title })
    // The ways to make one wait behind the screen's one 새 … menu in its heading row.
    await user.click(await screen.findByRole('button', { name: entry.menu }))
    expect(calls).not.toContain('GetLatestAuthoringSession')
    await user.click(await screen.findByRole('menuitem', { name: entry.way }))
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

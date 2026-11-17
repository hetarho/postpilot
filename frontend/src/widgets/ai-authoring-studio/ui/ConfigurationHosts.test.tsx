import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

describe('configuration authoring entrypoints', () => {
  // Guide: a new template is made by talking to the AI, from scratch — for posts and for videos.
  it.each([
    ['/templates/new', /어떤 글을 올리세요\?/, '어떤 글을 올리는지 말해 주세요'],
    ['/video-templates/new', /어떤 영상을 만드세요\?/, '어떤 글을 올리는지 말해 주세요'],
  ])(
    'opens %s as a conversation without starting or saving work',
    async (route, greeting, field) => {
      const user = userEvent.setup()
      const calls: string[] = []
      renderAppAt(route, { user: { id: 'alice' }, calls, existingSetup: true })
      expect(await screen.findByText(greeting)).toBeVisible()
      const input = screen.getByRole('textbox', { name: field })
      const starter = screen
        .getAllByRole('button')
        .find((button) => /순서로요$/.test(button.textContent ?? ''))!
      await user.click(starter)
      expect(input).toHaveValue(starter.textContent)
      expect(screen.queryByRole('heading', { name: '어떤 체험을 글로 남기나요?' })).toBeNull()
      expect(screen.queryByRole('heading', { name: '어떤 체험을 영상으로 남기나요?' })).toBeNull()
      expect(
        calls.filter((name) => /^(Start|Generate|Analyze|Adopt|Create|SaveAuthoring)/.test(name)),
      ).toEqual([])
      expect(calls).not.toContain('GetLatestWritingVoiceCandidates')
    },
  )

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
      trigger: '새 말투',
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
    // 새 말투's ways of making one open from it; the AI's is one of them.
    if (entry.route === '/voices')
      await user.click(await screen.findByRole('menuitem', { name: /AI와 대화로 만들기/ }))
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

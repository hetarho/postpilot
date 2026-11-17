import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

describe('configuration authoring entrypoints', () => {
  // A new template is made by talking: the page opens on the AI's question and one composer —
  // no wizard step, nothing to drag — and opening it starts and saves nothing.
  it('opens /templates/new as a conversation without starting or saving work', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/templates/new', { user: { id: 'alice' }, calls, existingSetup: true })
    const log = await screen.findByRole('log', { name: '글 구성 만들기 대화' })
    expect(within(log).getByText(/어떤 글을 올리세요\?/)).toBeVisible()
    expect(
      screen.queryByRole('heading', { level: 2, name: '어떤 체험을 글로 남기나요?' }),
    ).not.toBeInTheDocument()
    const composer = screen.getByRole('textbox', { name: 'AI에게 보낼 말' })
    await user.type(composer, '동네 빵집 소개')
    expect(composer).toHaveValue('동네 빵집 소개')
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(
      calls.filter((name) => /^(Start|Generate|Analyze|Adopt|Create|SaveAuthoring)/.test(name)),
    ).toEqual([])
    expect(calls).not.toContain('GetLatestWritingVoiceCandidates')
  })

  // A video template is made the same way: the AI's question and one composer, no wizard.
  it('opens /video-templates/new as a conversation without starting or saving work', async () => {
    const calls: string[] = []
    renderAppAt('/video-templates/new', { user: { id: 'alice' }, calls, existingSetup: true })
    const log = await screen.findByRole('log')
    expect(within(log).getByText(/어떤 영상을 만드세요\?/)).toBeVisible()
    expect(
      screen.queryByRole('heading', { name: '어떤 체험을 영상으로 남기나요?' }),
    ).not.toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'AI에게 보낼 말' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(
      calls.filter((name) => /^(Start|Generate|Create|Adopt|SaveAuthoring)/.test(name)),
    ).toEqual([])
  })

  it.each([
    {
      route: '/guidelines',
      title: '지침',
      trigger: '새 지침 만드는 방법',
      goal: '글을 쓸 때 어떤 점을 지키면 좋을까요?',
    },
    {
      route: '/video-guidelines',
      title: '영상 지침',
      trigger: '새 지침 만드는 방법',
      goal: '영상 속 문구를 어떻게 쓰면 좋을까요?',
    },
    {
      route: '/voices',
      title: '말투',
      trigger: '새 말투 만드는 방법',
      goal: '어떤 말투로 이야기하고 싶으세요?',
    },
  ])('opens $route as a readable AI draft without starting or saving work', async (entry) => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt(entry.route, { user: { id: 'alice' }, calls, existingSetup: true })
    await screen.findByRole('heading', { level: 1, name: entry.title })
    // The ways to make one open from the section's one creation in the dock.
    const trigger = await screen.findByRole('button', { name: entry.trigger })
    expect(calls).not.toContain('GetLatestAuthoringSession')
    await user.click(trigger)
    await user.click(await screen.findByRole('menuitem', { name: 'AI와 대화로 만들기' }))
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

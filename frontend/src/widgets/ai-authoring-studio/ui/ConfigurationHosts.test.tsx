import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

describe('configuration authoring entrypoints', () => {
  it.each([
    { route: '/templates/new', title: '새 템플릿', manual: true },
    { route: '/video-templates/new', title: '새 영상 템플릿', manual: true },
    { route: '/guidelines', title: '지침', trigger: 'AI로 지침 만들기' },
    { route: '/video-guidelines', title: '영상 지침', trigger: 'AI로 영상 지침 만들기' },
    { route: '/voices', title: '말투', trigger: 'AI 말투 추천받기' },
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
      expect(await within(dialog).findByRole('button', { name: '8가지 추천받기' })).toBeVisible()
    } else {
      expect(await screen.findByRole('button', { name: '8가지 추천받기' })).toBeVisible()
      expect(screen.getByRole('button', { name: '직접 편집' })).toBeEnabled()
      expect(screen.queryByLabelText('이름')).not.toBeInTheDocument()
      expect(screen.queryByLabelText('템플릿 이름')).not.toBeInTheDocument()
    }
    await waitFor(() => expect(calls).toContain('GetLatestAuthoringSession'))
    expect(
      calls.filter((name) => /^(Start|Generate|Analyze|Adopt|Create|SaveAuthoring)/.test(name)),
    ).toEqual([])
    expect(calls).not.toContain('GetLatestWritingVoiceCandidates')
  })
})

import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { emptyClipProject, type ClipProject } from '@/entities/clip-project'
import { initializeI18n } from '@/app/providers/i18n'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeClipsOptions } from '@/test/clips'
import { ClipUploadText } from './ClipUploadText'

const model = { providerId: 'provider', modelId: 'writer' }
const project: ClipProject = {
  ...emptyClipProject(),
  id: 'clip',
  language: 'ko',
  channel: 'naver_clip',
  campaign: true,
  createdAt: '2026-10-15T01:00:00Z',
  updatedAt: '2026-10-15T02:00:00Z',
  editPlanRevision: 1,
  renderedPlanRevision: 1,
}
const stored = (extra = {}) => ({
  projectId: 'clip',
  channel: 'naver_clip',
  body: '다시 찾고 싶은 곳',
  tags: ['광고주', '방문'],
  edited: false,
  revision: 1,
  updatedAt: project.updatedAt,
  ...extra,
})
function mount(options: FakeClipsOptions = {}, value = project) {
  initializeI18n('ko')
  const calls: string[] = [],
    quotes: unknown[] = [],
    starts: unknown[] = [],
    saves: unknown[] = []
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    clips: {
      projects: [value],
      calls,
      uploadTextQuotes: quotes,
      uploadTextStarts: starts,
      uploadTextSaves: saves,
      ...options,
    },
    jobs: {
      jobs: [
        {
          id: 'clip-upload-text-job',
          kind: 'clip_upload_text',
          status: 'running',
          stage: 'upload_text',
          clipProjectId: 'clip',
        },
      ],
    },
  })
  const client = createTestQueryClient()
  const beforeWrite = vi.fn(async () => {})
  const props = { ownerId: 'alice', project: value, writeModel: model, beforeWrite }
  const view = render(<ClipUploadText {...props} />, { wrapper: withProviders(transport, client) })
  return {
    ...view,
    props,
    client,
    calls,
    quotes,
    starts,
    saves,
    beforeWrite,
    user: userEvent.setup(),
  }
}
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  initializeI18n('ko')
})

describe('clip upload text', () => {
  it('only writes after explicit quote approval, with one write call and no video work', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const view = mount({ uploadTextStartGate: gate })
    const make = await screen.findByRole('button', { name: '업로드 문구 만들기' })
    expect(view.quotes).toEqual([])
    expect(view.starts).toEqual([])
    await view.user.click(make)
    const approve = await screen.findByRole('button', { name: '최대 4 크레딧 · 문구 작성' })
    expect(view.starts).toEqual([])
    await view.user.click(screen.getByRole('button', { name: '요금 자세히' }))
    expect(screen.getByText('업로드 문구 작성 1회')).toBeVisible()
    await view.user.dblClick(approve)
    await waitFor(() => expect(view.starts).toHaveLength(1))
    expect(view.quotes).toHaveLength(1)
    expect(view.quotes[0]).toMatchObject({
      projectId: 'clip',
      channel: 'naver_clip',
      writeModel: model,
      confirmEdited: false,
    })
    expect(view.starts[0]).toMatchObject({
      writeModel: model,
      approvedMaxCredits: 4,
      cancellationPolicyVersion: 1,
    })
    expect(
      view.calls.some((call) => /Render|Source|StartClipGeneration|StartClipStoryline/u.test(call)),
    ).toBe(false)
    await act(async () => release())
  })

  it('copies disclosure only in the displayed output, tags separately, and edits the canonical body', async () => {
    const rows = [stored()]
    const view = mount({ uploadTexts: rows })
    const clipboard = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    await view.user.click(await screen.findByRole('button', { name: '본문 복사' }))
    expect(clipboard).toHaveBeenLastCalledWith(
      '대가표시_배너_또는_문구_대가표시\n\n다시 찾고 싶은 곳',
    )
    expect(await screen.findByText('복사됨')).toBeVisible()
    await view.user.click(screen.getByRole('button', { name: '태그 복사' }))
    expect(clipboard).toHaveBeenLastCalledWith('#광고주 #방문')
    await view.user.click(screen.getByRole('button', { name: '문구 수정' }))
    expect(screen.getByRole('button', { name: '인스타 클립' })).toBeDisabled()
    const body = screen.getByRole('textbox', { name: '본문' })
    expect(body).toHaveValue('다시 찾고 싶은 곳')
    await view.user.clear(body)
    await view.user.type(body, '제가 고친 문장')
    await view.user.click(screen.getByRole('button', { name: '문구 저장' }))
    await waitFor(() =>
      expect(screen.queryByRole('textbox', { name: '본문' })).not.toBeInTheDocument(),
    )
    expect(view.saves[0]).toMatchObject({
      body: '제가 고친 문장',
      expectedRevision: 1,
      tags: ['광고주', '방문'],
    })
    expect(rows[0]).toMatchObject({ body: '제가 고친 문장', edited: true, revision: 2 })
    expect(view.starts).toEqual([])
  })

  it('confirms edited replacement before even quoting and lets the other channel use the same finished video', async () => {
    const finished = {
      ...project,
      finalized: { at: project.updatedAt, resultId: 'video', planRevision: 1 },
    }
    const view = mount({ uploadTexts: [stored({ edited: true })] }, finished)
    await view.user.click(await screen.findByRole('button', { name: '다시 쓰기' }))
    const dialog = await screen.findByRole('dialog')
    expect(view.quotes).toEqual([])
    await view.user.click(within(dialog).getByRole('button', { name: '예상 크레딧 확인' }))
    await screen.findByRole('button', { name: '최대 4 크레딧 · 문구 작성' })
    expect(view.quotes[0]).toMatchObject({ confirmEdited: true })
    await view.user.click(screen.getByRole('button', { name: '인스타 클립' }))
    expect(
      screen.queryByRole('button', { name: '최대 4 크레딧 · 문구 작성' }),
    ).not.toBeInTheDocument()
    await view.user.click(screen.getByRole('button', { name: '인스타 클립 문구도 만들기' }))
    await view.user.click(await screen.findByRole('button', { name: '최대 4 크레딧 · 문구 작성' }))
    await waitFor(() => expect(view.starts).toHaveLength(1))
    expect(view.starts[0]).toMatchObject({ channel: 'instagram_clip', confirmEdited: false })
    expect(view.calls.some((call) => /Render|Source|StartClipGeneration/u.test(call))).toBe(false)
  })

  it('selects a manual copy field after refusal and never revives stale feedback across channel switches', async () => {
    const view = mount({
      uploadTexts: [stored(), stored({ channel: 'instagram_clip', body: '인스타 문장' })],
    })
    const clipboard = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockRejectedValue(new Error('denied'))
    await view.user.click(await screen.findByRole('button', { name: '본문 복사' }))
    const fallback = (await screen.findByRole('textbox', {
      name: '직접 복사할 문구',
    })) as HTMLTextAreaElement
    expect(fallback).toHaveFocus()
    expect(fallback.selectionEnd).toBe(fallback.value.length)
    let release!: () => void
    clipboard.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          release = resolve
        }),
    )
    await view.user.click(screen.getByRole('button', { name: '본문 복사' }))
    await view.user.click(screen.getByRole('button', { name: '인스타 클립' }))
    await act(async () => release())
    await view.user.click(screen.getByRole('button', { name: '네이버 클립' }))
    expect(screen.queryByText('복사됨')).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: '직접 복사할 문구' })).not.toBeInTheDocument()
  })

  it('discards a quote that returns after its channel was left', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const view = mount({ uploadTextQuoteGate: gate })
    await view.user.click(await screen.findByRole('button', { name: '업로드 문구 만들기' }))
    await waitFor(() => expect(view.quotes).toHaveLength(1))
    await view.user.click(screen.getByRole('button', { name: '인스타 클립' }))
    await act(async () => release())
    expect(
      screen.queryByRole('button', { name: '최대 4 크레딧 · 문구 작성' }),
    ).not.toBeInTheDocument()
    expect(view.starts).toEqual([])
  })

  it('refuses copying an older body that no longer fits after campaign is enabled', async () => {
    mount({ uploadTexts: [stored({ body: '가'.repeat(300) })] })
    expect(await screen.findByRole('button', { name: '본문 복사' })).toBeDisabled()
    expect(screen.getByRole('alert')).toHaveTextContent('채널 한도')
    expect(screen.getByRole('button', { name: '문구 수정' })).toBeEnabled()
  })
  it('serializes copies so a late body cannot overwrite newer tags, with one bounded confirmation', async () => {
    const view = mount({ uploadTexts: [stored()] })
    let release!: () => void
    const clipboard = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockImplementationOnce(
        () =>
          new Promise<void>((resolve) => {
            release = resolve
          }),
      )
      .mockResolvedValue(undefined)
    await view.user.click(await screen.findByRole('button', { name: '본문 복사' }))
    await view.user.click(screen.getByRole('button', { name: '태그 복사' }))
    expect(clipboard).toHaveBeenCalledTimes(1)
    await act(async () => release())
    await waitFor(() => expect(clipboard).toHaveBeenCalledTimes(2))
    expect(clipboard).toHaveBeenLastCalledWith('#광고주 #방문')
    expect(await screen.findByText('복사됨')).toBeVisible()
    vi.useFakeTimers()
    try {
      // A new copy schedules the same shared confirmation dwell.
      await act(async () => {
        screen.getByRole('button', { name: '태그 복사' }).click()
      })
      await act(async () => {
        vi.advanceTimersByTime(1499)
      })
      expect(screen.getByText('복사됨')).toBeVisible()
      await act(async () => {
        vi.advanceTimersByTime(1)
      })
      expect(screen.queryByText('복사됨')).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('copies English disclosure with Instagram hashtags within one caption and omits it while off', async () => {
    const instagram = { ...project, language: 'en' as const, channel: 'instagram_clip' as const }
    const view = mount(
      { uploadTexts: [stored({ channel: 'instagram_clip', body: 'My visit' })] },
      instagram,
    )
    const clipboard = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    await view.user.click(await screen.findByRole('button', { name: '본문과 해시태그 함께 복사' }))
    expect(clipboard).toHaveBeenLastCalledWith(
      'disclosure_banner_or_text_disclosure\n\nMy visit\n\n#광고주 #방문',
    )
    view.rerender(<ClipUploadText {...view.props} project={{ ...instagram, campaign: false }} />)
    await view.user.click(screen.getByRole('button', { name: '본문 복사' }))
    expect(clipboard).toHaveBeenLastCalledWith('My visit')
  })
  it.each(['본문 복사', '본문과 해시태그 함께 복사'])(
    'dismisses a focused fallback and returns focus to %s on a content change',
    async (button) => {
      const view = mount(
        { uploadTexts: [stored({ channel: 'instagram_clip' })] },
        { ...project, channel: 'instagram_clip' },
      )
      vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
      await view.user.click(await screen.findByRole('button', { name: button }))
      expect(await screen.findByRole('textbox', { name: '직접 복사할 문구' })).toHaveFocus()
      view.rerender(
        <ClipUploadText {...view.props} project={{ ...view.props.project, campaign: false }} />,
      )
      expect(screen.queryByRole('textbox', { name: '직접 복사할 문구' })).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: button })).toHaveFocus()
    },
  )
})

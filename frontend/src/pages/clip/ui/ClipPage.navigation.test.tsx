import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import type { Transport } from '@connectrpc/connect'
import { initializeI18n } from '@/app/providers/i18n'
import { rememberClipEntry, readClipReturnContext } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'
import { renderAppAt } from '@/test/app'
import { createFakeAuthTransport } from '@/test/session'
import type { FakeClipProject } from '@/test/clips'

afterEach(() => {
  vi.restoreAllMocks()
  discardClipDraftQueues()
  sessionStorage.clear()
  initializeI18n('ko')
})
const project: FakeClipProject = {
  id: 'saved',
  title: '제주 여행',
  videoTemplateId: '',
  ratio: 'vertical',
  targetDurationMs: 15000,
  disclosure: 'ad',
}
it('keeps home creation through explicit minting and reload, then confirmed deletion returns home without AI or cancellation', async () => {
  const calls: string[] = []
  const user = userEvent.setup()
  const view = renderAppAt('/clips/new', { user: { id: 'alice' }, clips: { calls } })
  expect(await screen.findByRole('link', { name: '만들기로 돌아가기' })).toHaveAttribute(
    'href',
    '/',
  )
  await user.type(await screen.findByLabelText('클립 제목'), '바다 여행')
  await user.click(screen.getByRole('button', { name: '클립 만들기' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/clips/clip-1'))
  expect(readClipReturnContext('alice', 'clip-1')?.path).toBe('/')
  expect(screen.getByRole('link', { name: '만들기로 돌아가기' })).toHaveAttribute('href', '/')
  view.unmount()
  const reopened = renderAppAt('/clips/clip-1', { transport: view.transport })
  await screen.findByLabelText('클립 제목')
  expect(screen.getByText('바다 여행')).toBeVisible()
  await user.click(screen.getByRole('button', { name: '삭제' }))
  await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '삭제' }))
  await waitFor(() => expect(reopened.router.state.location.pathname).toBe('/'))
  expect(readClipReturnContext('alice', 'clip-1')).toBeUndefined()
  expect(calls.filter((call) => call === 'CreateClipProject')).toHaveLength(1)
  expect(calls.filter((call) => call === 'DeleteClipProject')).toHaveLength(1)
  expect(calls.filter((call) => /Start|Cancel/.test(call))).toEqual([])
})
it('opens from filtered history and returns to the same filters and scroll after reload', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(window, 'scrollY', 'get').mockReturnValue(720)
  const view = renderAppAt('/clips?q=제주&status=draft', {
    user: { id: 'alice' },
    clips: { projects: [project], calls },
  })
  await user.click(await screen.findByRole('link', { name: /제주 여행.*초안/ }))
  await screen.findByLabelText('클립 제목')
  expect(readClipReturnContext('alice', 'saved')).toMatchObject({
    path: '/clips',
    filters: { q: '제주', status: 'draft' },
    scrollY: 720,
  })
  view.unmount()
  const reopened = renderAppAt('/clips/saved', { transport: view.transport })
  await screen.findByLabelText('클립 제목')
  const back = await screen.findByRole('link', { name: '작업 내역으로 돌아가기' })
  expect(back).toHaveAttribute('href', '/clips?q=%EC%A0%9C%EC%A3%BC&status=draft')
  await user.click(back)
  await waitFor(() => expect(reopened.router.state.location.pathname).toBe('/clips'))
  expect(reopened.router.state.location.search).toMatchObject({ q: '제주', status: 'draft' })
  await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 720))
  expect(calls.filter((call) => /Create|Start|Cancel|Update/.test(call))).toEqual([])
})
it.each(['ko', 'en'] as const)(
  'a running server job keeps its contextual return and stable workspace through resize in %s without spending or cancelling',
  async (language) => {
    initializeI18n(language)
    rememberClipEntry('alice', {
      path: '/',
      section: 'creation',
      filters: {},
      scrollY: 0,
      targetId: 'saved',
    })
    const calls: string[] = []
    const job = {
      id: 'render-job',
      kind: 'render_clip',
      clipProjectId: 'saved',
      status: 'running',
      stage: 'render_wait',
      canCancel: true,
    }
    const view = renderAppAt('/clips/saved', {
      user: { id: 'alice' },
      clips: { projects: [{ ...project, latestJob: job }], calls },
      jobs: { jobs: [job], calls },
    })
    const label = language === 'ko' ? '만들기로 돌아가기' : 'Back to creation'
    await screen.findByRole('link', { name: label })
    expect(await screen.findByText(project.title)).toBeVisible()
    const status = await screen.findByRole('progressbar')
    await act(async () => {
      window.dispatchEvent(new Event('resize'))
    })
    expect(status).toBeInTheDocument()
    await userEvent.click(screen.getByRole('link', { name: label }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe('/'))
    expect(calls.filter((call) => /Create|Start|Cancel|Update/.test(call))).toEqual([])
  },
)
it('a direct or foreign-owner record uses named history without adopting another owner creation context', async () => {
  rememberClipEntry('alice', {
    path: '/',
    section: 'creation',
    filters: {},
    scrollY: 0,
    targetId: 'saved',
  })
  renderAppAt('/clips/saved', {
    user: { id: 'bob' },
    clips: { projects: [{ ...project, ownerId: 'bob' }] },
  })
  const back = await screen.findByRole('link', { name: '작업 내역으로 돌아가기' })
  expect(back).toHaveAttribute('href', '/clips')
  expect(screen.queryByRole('link', { name: '만들기로 돌아가기' })).not.toBeInTheDocument()
})
it('freezes the creation origin and ignores late mint navigation after the owner workspace closes', async () => {
  rememberClipEntry('alice', {
    path: '/clips',
    section: 'clips',
    filters: { q: '기존 맥락' },
    scrollY: 360,
  })
  let release!: () => void
  const gate = new Promise<void>((done) => {
    release = done
  })
  const calls: string[] = []
  const base = createFakeAuthTransport({
    user: { id: 'alice' },
    existingSetup: true,
    clips: { calls },
  })
  let held = false
  const transport: Transport = {
    ...base,
    unary: async (...args) => {
      const response = await base.unary(...args)
      if (args[0].name === 'CreateClipProject') {
        held = true
        await gate
      }
      return response
    },
  }
  const view = renderAppAt('/clips/new', { transport })
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('클립 제목'), '늦게 저장되는 영상')
  await user.click(screen.getByRole('button', { name: '클립 만들기' }))
  await waitFor(() => expect(calls).toContain('CreateClipProject'))
  await waitFor(() => expect(held).toBe(true))
  view.unmount()
  rememberClipEntry('alice', { path: '/', section: 'creation', filters: {}, scrollY: 0 })
  await act(async () => {
    release()
    await gate
  })
  await waitFor(() =>
    expect(readClipReturnContext('alice', 'clip-1')).toMatchObject({
      path: '/clips',
      filters: { q: '기존 맥락' },
    }),
  )
  expect(view.router.state.location.pathname).toBe('/clips/new')
  expect(readClipReturnContext('alice')?.path).toBe('/')
})

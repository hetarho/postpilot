import { Blob as NodeBlob, File as NodeFile } from 'node:buffer'
import { webcrypto } from 'node:crypto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { readClipReturnContext, type ClipProjectDraft } from '@/entities/clip-project'
import { discardClipSourceSessions } from '@/features/upload-clip-sources'
import { discardClipDraftQueues } from '@/features/edit-clip-project'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'
import { readVideoMetadata } from '@/shared/lib/media'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { renderAppAt } from '@/test/app'
import { USER, openStep, resetEditorTest } from '@/test/editor'
import {
  finalizedPostRow,
  publishedPostRow,
  type FakeDraftSave,
  type FakePostRow,
} from '@/test/posts'
import type { FakeClipsOptions } from '@/test/clips'

vi.mock('@/shared/lib/media', async (original) => ({
  ...(await original<object>()),
  readVideoMetadata: vi.fn(),
}))
vi.mock('@/shared/lib/upload', async (original) => ({
  ...(await original<object>()),
  putBlobWithProgress: vi.fn(),
}))

const SLUG = '20260820-seongsu'
const ACTION = '이 체험으로 클립 만들기'
function fetchedVideo(content: string) {
  const response = new Response()
  // Node's Response and jsdom's Blob belong to different realms. Use the same native
  // Blob/File pair so the download's bytes, rather than "[object Blob]", enter the manifest.
  Object.defineProperty(response, 'blob', {
    value: async () => new NodeBlob([content], { type: 'video/mp4' }),
  })
  return response
}
beforeEach(() => {
  vi.stubGlobal('File', NodeFile)
  vi.stubGlobal('crypto', webcrypto)
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: RequestInfo | URL) => fetchedVideo(String(url))),
  )
  vi.mocked(readVideoMetadata).mockResolvedValue({ durationMs: 1000, width: 1920, height: 1080 })
  vi.mocked(putBlobWithProgress).mockResolvedValue(undefined)
})
afterEach(() => {
  resetEditorTest()
  clearCaret()
  discardClipSourceSessions()
  discardClipDraftQueues()
  sessionStorage.clear()
  vi.resetAllMocks()
})

function mount(row: Partial<FakePostRow> = {}, clips: FakeClipsOptions = {}) {
  const writes: ClipProjectDraft[] = []
  const requests: unknown[] = []
  const calls: string[] = []
  const draftSaves: FakeDraftSave[] = []
  const materials: Array<{ slug: string; title: string; memo: string }> = []
  const post = finalizedPostRow({
    slug: SLUG,
    title: '성수 카페',
    memo: '친구와 나눈 이야기',
    campaign: true,
    ...row,
  })
  const view = renderAppAt(`/posts/${SLUG}`, {
    user: USER,
    posts: { posts: [post], draftSaves, draftMaterials: materials },
    clips: { projectWrites: writes, sourceRequests: requests, calls, ...clips },
  })
  return { ...view, writes, requests, calls, draftSaves, materials, post }
}
async function dialog(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: ACTION }))
  return screen.findByRole('dialog', { name: ACTION })
}
async function confirm(user: ReturnType<typeof userEvent.setup>) {
  const panel = await dialog(user)
  await user.click(within(panel).getByRole('button', { name: '클립 만들기' }))
}
const video = (filename: string, index = 0) => ({
  id: `video-${index}`,
  filename,
  durationMs: 1000n,
  width: 1920,
  height: 1080,
  contentType: 'video/mp4',
  viewUrl: `https://post-videos.test/${filename}`,
})
const arrived = (view: ReturnType<typeof mount>) =>
  waitFor(() => expect(view.router.state.location.pathname).toBe('/clips/clip-1'), {
    timeout: 5000,
  })

describe('making a clip from a finished post', () => {
  it('createClipFromPost links the post', async () => {
    const user = userEvent.setup()
    const view = mount({
      campaign: false,
      advertiserGuide: {
        text: '성수 카페를 소개해 주세요',
        keywords: ['성수 카페'],
        hashtags: ['성수카페'],
        minChars: 1500,
        minPhotos: 10,
      },
    })
    const panel = await dialog(user)
    expect(view.writes).toEqual([])
    await user.click(within(panel).getByRole('tab', { name: '인스타 클립' }))
    await user.click(within(panel).getByRole('button', { name: '클립 만들기' }))
    await arrived(view)
    expect(view.writes[0]).toMatchObject({
      linkedPostSlug: SLUG,
      title: '성수 카페',
      instruction: '친구와 나눈 이야기',
      channel: 'instagram_clip',
      ratio: 'vertical',
      campaign: false,
      advertiserGuide: {
        text: '성수 카페를 소개해 주세요',
        keywords: ['성수 카페'],
        hashtags: ['성수카페'],
      },
    })
    expect(view.writes[0].advertiserGuide?.minChars).toBeUndefined()
    expect(view.writes[0].advertiserGuide?.minPhotos).toBeUndefined()
    expect(view.calls.filter((name) => name === 'CreateClipProject')).toHaveLength(1)
    expect(view.calls.filter((name) => name === 'UpdateClipProject')).toHaveLength(0)
    expect(view.calls.some((name) => name.startsWith('Start'))).toBe(false)
    expect(view.draftSaves).toEqual([])
    expect(readClipReturnContext(USER.id, 'clip-1')).toMatchObject({
      targetId: 'clip-1',
      path: '/insta-clip',
      filters: { channel: 'instagram_clip' },
    })
    await waitFor(() => expect(view.router.state.location.state.clipImport).toBeUndefined())
  })

  it('cancels without creating a clip or uploading sources', async () => {
    const user = userEvent.setup()
    const view = mount({ videos: [video('one.mp4')] })
    const panel = await dialog(user)
    await user.click(within(panel).getByRole('tab', { name: '인스타 클립' }))
    await user.click(within(panel).getByRole('button', { name: '취소' }))
    expect(view.writes).toEqual([])
    expect(view.calls).not.toContain('CreateClipProject')
    expect(fetch).not.toHaveBeenCalled()
    expect(view.router.state.location.pathname).toBe(`/posts/${SLUG}`)
  })

  it('cuts a long memo at the instruction bound and says so', async () => {
    const user = userEvent.setup()
    const memo = '😀'.repeat(1001) + '끝'
    const view = mount({ memo })
    const panel = await dialog(user)
    expect(within(panel).getByText(/앞 1000자/)).toHaveTextContent(/나머지 메모는 글에 그대로/)
    await user.click(within(panel).getByRole('button', { name: '클립 만들기' }))
    await arrived(view)
    expect(view.writes[0].instruction).toBe('😀'.repeat(1000))
    expect(Array.from(view.writes[0].instruction!)).toHaveLength(1000)
    expect(view.post.memo).toBe(memo)
    expect(view.draftSaves).toEqual([])
  })

  it('carries no photos', async () => {
    const user = userEvent.setup()
    const view = mount({
      images: [{ id: 'photo', filename: 'only-photo.jpg', viewUrl: 'https://photos.test/private' }],
    })
    await confirm(user)
    await arrived(view)
    expect(view.calls).not.toContain('CreateClipSourceBatch')
    expect(fetch).not.toHaveBeenCalled()
    expect(JSON.stringify(view.writes)).not.toContain('only-photo.jpg')
    expect(view.draftSaves).toEqual([])
  })

  it('offers the action on published posts and needs canonical content', async () => {
    const user = userEvent.setup()
    const published = mount(publishedPostRow({ slug: SLUG, title: '발행한 글' }))
    await confirm(user)
    await arrived(published)
    expect(published.writes[0].title).toBe('발행한 글')
    expect(published.draftSaves).toEqual([])
    published.unmount()
    mount({ status: 'draft', content: undefined, finalizedRevision: 0n, finalizedAt: '' })
    await openStep(user, '글 완성')
    expect(screen.queryByRole('button', { name: ACTION })).not.toBeInTheDocument()
  })

  it('imports fetched videos through the real pipeline and keeps the arrived originals', async () => {
    const user = userEvent.setup()
    const view = mount(
      { videos: [video('good.mp4'), video('bad.mp4', 1)] },
      { importConfirmationFailures: ['bad.mp4'] },
    )
    vi.mocked(putBlobWithProgress).mockImplementation(async (_url, _headers, file) => {
      if ('name' in file && file.name === 'bad.mp4') throw new Error('transfer failed')
    })
    await confirm(user)
    await arrived(view)
    const notice = await screen.findByText(/가져오지 못한 영상:/)
    expect(notice).toHaveTextContent('bad.mp4')
    expect(notice).not.toHaveTextContent('good.mp4')
    expect(await screen.findByRole('button', { name: 'good.mp4 선택' })).toBeInTheDocument()
    expect(putBlobWithProgress).toHaveBeenCalledTimes(2)
    expect(await vi.mocked(putBlobWithProgress).mock.calls[0][2].text()).toBe(
      'https://post-videos.test/good.mp4',
    )
    expect(readVideoMetadata).toHaveBeenCalledWith(expect.any(NodeFile), expect.any(AbortSignal))
    expect(view.requests[0]).toMatchObject({
      sources: [
        {
          filename: 'good.mp4',
          bytes: BigInt('https://post-videos.test/good.mp4'.length),
          fingerprint: expect.stringMatching(/^[0-9a-f]{64}$/u),
        },
        { filename: 'bad.mp4', fingerprint: expect.stringMatching(/^[0-9a-f]{64}$/u) },
      ],
    })
    expect(view.calls).not.toContain('DiscardClipSourceBatch')
    expect(view.calls.some((name) => name.startsWith('Start'))).toBe(false)
    expect(
      JSON.stringify(view.requests, (_key, value) =>
        typeof value === 'bigint' ? value.toString() : value,
      ),
    ).not.toContain('post-videos.test')
    expect(view.draftSaves).toEqual([])
  })

  it('names videos over the source limit without downloading them', async () => {
    const user = userEvent.setup()
    const view = mount({ videos: Array.from({ length: 21 }, (_, i) => video(`${i}.mp4`, i)) })
    await confirm(user)
    await arrived(view)
    expect(await screen.findByText(/가져오기 한도를 넘은 영상:/)).toHaveTextContent('20.mp4')
    expect(fetch).toHaveBeenCalledTimes(20)
    expect(view.calls.filter((name) => name === 'CreateClipSourceBatch')).toHaveLength(1)
    expect((view.requests[0] as { sources: unknown[] }).sources).toHaveLength(20)
  })

  it('flushes the post and carries its latest memo and campaign settings', async () => {
    const user = userEvent.setup()
    const view = mount()
    await openStep(user, '글 생성')
    await user.clear(screen.getByLabelText('메모'))
    await user.type(screen.getByLabelText('메모'), '방금 쓴 메모')
    await user.click(screen.getByRole('switch', { name: '체험단 글' }))
    await openStep(user, '글 완성')
    await confirm(user)
    await arrived(view)
    expect(view.writes[0]).toMatchObject({ instruction: '방금 쓴 메모', campaign: false })
    expect(view.materials.some((save) => save.memo === '방금 쓴 메모')).toBe(true)
  })

  it('does not navigate back after the owner leaves during a transfer', async () => {
    const user = userEvent.setup()
    const view = mount({ videos: [video('waiting.mp4')] })
    let release!: (response: Response) => void
    let signal: AbortSignal | undefined
    vi.mocked(fetch).mockImplementation(
      async (_url, options) =>
        new Promise<Response>((resolve) => {
          release = resolve
          signal = options?.signal ?? undefined
        }),
    )
    await confirm(user)
    await waitFor(() => expect(release).toBeTypeOf('function'))
    await act(() => view.router.navigate({ to: '/naver-blog' }))
    expect(signal?.aborted).toBe(true)
    await act(async () => {
      release(fetchedVideo('late video'))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    expect(view.router.state.location.pathname).toBe('/naver-blog')
    expect(view.calls.filter((name) => name === 'CreateClipProject')).toHaveLength(1)
    expect(view.calls).not.toContain('CreateClipSourceBatch')
    expect(view.calls).not.toContain('DiscardClipSourceBatch')
  })

  it.each([
    { ownerId: 'another-owner', projectId: 'kept' },
    { ownerId: USER.id, projectId: 'another-clip' },
  ])('keeps arrival filenames scoped to their owner and clip: %o', async (identity) => {
    const view = renderAppAt('/clips', {
      user: USER,
      clips: {
        projects: [
          {
            id: 'kept',
            title: '기존 클립',
            videoTemplateId: '',
            ratio: 'vertical',
            targetDurationMs: 0,
            disclosure: '',
          },
        ],
      },
    })
    await act(() =>
      view.router.navigate({
        to: '/clips/$clipId',
        params: { clipId: 'kept' },
        state: {
          clipImport: {
            ...identity,
            problems: [{ filename: 'private.mp4', reason: 'read' }],
            uncertain: true,
          },
        },
      }),
    )
    await screen.findByRole('textbox', { name: '클립 제목' })
    expect(screen.queryByText(/private.mp4/)).not.toBeInTheDocument()
    await waitFor(() => expect(view.router.state.location.state.clipImport).toBeUndefined())
  })
})

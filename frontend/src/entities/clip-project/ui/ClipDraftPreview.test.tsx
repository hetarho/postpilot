import { webcrypto } from 'node:crypto'
import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ClipService, PrepareClipPreviewResponseSchema } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import type { ClipEditPlan } from '../model/edit-plan'
import { ClipDraftPreview } from './ClipDraftPreview'

const draft: ClipEditPlan = {
  durationMs: 19800,
  hook: '',
  cuts: [
    {
      id: 'a',
      sourceId: 'a',
      fingerprint: 'a',
      startMs: 2000,
      endMs: 12000,
      transitionMs: 0,
      copies: [],
      chips: [],
      volumePermille: 1000,
      focal: { x: 0, y: 0.5 },
    },
    {
      id: 'b',
      sourceId: 'b',
      fingerprint: 'b',
      startMs: 3000,
      endMs: 13000,
      transitionMs: 200,
      copies: [],
      chips: [],
      volumePermille: 500,
    },
  ],
}
const sources = draft.cuts.map((c) => ({
  id: c.id,
  fingerprint: c.fingerprint,
  filename: `${c.id}.mp4`,
  durationMs: 30000,
  width: 1920,
  height: 1080,
}))
beforeEach(() => {
  vi.stubGlobal('crypto', webcrypto)
  vi.stubGlobal('MediaError', { MEDIA_ERR_SRC_NOT_SUPPORTED: 4, MEDIA_ERR_DECODE: 3 })
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = vi.fn(() => 'blob:glyph')
      static revokeObjectURL = vi.fn()
    },
  )
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function mount(
  ratio = 'vertical',
  access = vi.fn(async (fp: string, refresh?: boolean) => {
    void refresh
    return `blob:${fp}`
  }),
) {
  const prepare = vi.fn(async (hash: string) =>
    create(PrepareClipPreviewResponseSchema, {
      draftHash: hash,
      canvasWidth: 1080,
      canvasHeight: 1920,
      nextOffset: -1,
      assets: [
        {
          key: 'glyph',
          instanceId: 'fixed',
          png: new Uint8Array([1]),
          width: 1,
          height: 1,
          startMs: 0,
          endMs: 19800,
        },
      ],
    }),
  )
  const transport = createRouterTransport((router) =>
    router.service(ClipService, { prepareClipPreview: (req) => prepare(req.draftHash) }),
  )
  const view = render(
    <TransportProvider transport={transport}>
      <ClipDraftPreview
        projectId="project"
        revision={1}
        plan={draft}
        ratio={ratio}
        sources={sources}
        resolvePlayback={access}
      />
    </TransportProvider>,
  )
  return { ...view, access, prepare }
}

it.each(['vertical', 'horizontal', 'square'])(
  'prepares %s current draft overlays with at most two source players',
  async (ratio) => {
    const view = mount(ratio)
    await screen.findByText('현재 편집 내용의 미리보기예요.')
    expect(view.container.querySelectorAll('video')).toHaveLength(2)
    expect(view.container.querySelectorAll('img')).toHaveLength(1)
    expect(view.access).toHaveBeenCalledWith('a')
    expect(view.access).toHaveBeenCalledWith('b')
    view.unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:glyph')
  },
)

it('keeps the missing original state separate from successful text preparation', async () => {
  mount(
    'vertical',
    vi.fn(async () => {
      throw connectAppError('CLIP_SOURCE_EXPIRED', Code.FailedPrecondition)
    }),
  )
  expect(
    await screen.findByText(
      '원본 보관 시간이 지났어요. 같은 원본을 다시 선택하면 미리 볼 수 있어요.',
    ),
  ).toBeVisible()
  expect(await screen.findByText('현재 편집 내용의 미리보기예요.')).toBeVisible()
})

it('refreshes a failed signed URL once and keeps codec failure explicit', async () => {
  const view = mount()
  await waitFor(() => expect(view.container.querySelectorAll('video')).toHaveLength(2))
  const first = view.container.querySelector('video')!
  fireEvent.error(first)
  await waitFor(() => expect(view.access).toHaveBeenCalledWith('a', true))
  fireEvent.error(first)
  expect(
    await screen.findByText('원본을 재생하지 못했어요. 글과 시간은 계속 수정할 수 있어요.'),
  ).toBeVisible()
  expect(view.access.mock.calls.filter(([, refresh]) => refresh)).toHaveLength(1)
  view.unmount()
  const next = mount()
  await waitFor(() => expect(next.container.querySelector('video')).not.toBeNull())
  const codec = next.container.querySelector('video')!
  Object.defineProperty(codec, 'error', { value: { code: 4 } })
  fireEvent.error(codec)
  expect(
    await screen.findByText(
      '이 브라우저에서는 원본 형식을 재생할 수 없어요. 글과 시간은 계속 수정할 수 있어요.',
    ),
  ).toBeVisible()
})

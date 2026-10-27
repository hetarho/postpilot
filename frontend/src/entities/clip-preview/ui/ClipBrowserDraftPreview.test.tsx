import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { BrowserCompositionSnapshot } from '../model/browser-composition'
import { ClipBrowserDraftPreview } from './ClipBrowserDraftPreview'

const failure = vi.hoisted(() => ({ code: '' }))
vi.mock('../lib/create-preview-worker', () => ({
  BrowserPreviewWorker: class {
    initialize = async () => {
      if (failure.code) throw new Error(failure.code)
    }
    prepare = (signal: AbortSignal, progress: (done: number, total: number) => void) => {
      progress(1, 3)
      return new Promise<void>((_resolve, reject) => {
        signal.addEventListener('abort', () => reject(signal.reason), { once: true })
      })
    }
    cancel = vi.fn()
    dispose = vi.fn()
  },
}))
afterEach(cleanup)
it('shows scene preparation and keeps it alive through playhead changes before enabling playback', async () => {
  failure.code = ''
  const plan = clipTimelineFixture().plan
  const snapshot = {
    ownerId: 'owner',
    projectId: 'project',
    versions: {},
    sources: [],
    snapshotFingerprint: 'current',
    frameCount: 594,
    plan,
  } as unknown as BrowserCompositionSnapshot
  const props = {
    ratio: 'vertical',
    plan,
    local: { snapshot, source: vi.fn(), audioSource: vi.fn() },
    preview: {
      ready: true,
      updating: false,
      assets: [],
      canvasWidth: 1080,
      canvasHeight: 1920,
      onRetry: vi.fn(),
    },
  }
  const view = render(<ClipBrowserDraftPreview {...props} timeMs={0} />)
  expect(await screen.findByText('미리보기 장면을 확인하고 있어요. 1/3')).toBeInTheDocument()
  view.rerender(<ClipBrowserDraftPreview {...props} timeMs={1000} />)
  expect(screen.getByText('미리보기 장면을 확인하고 있어요. 1/3')).toBeInTheDocument()
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})
it.each([
  ['CLIP_SOURCE_CODEC_UNSUPPORTED', '원본 형식을 지원하는 브라우저나 기기에서 열어 주세요.'],
  ['CLIP_SOURCE_MEMORY_LIMIT', '원본을 미리 보는 데 필요한 메모리가 부족해요.'],
  ['CLIP_PREVIEW_TIMEOUT', '원본을 읽는 데 시간이 오래 걸려 미리보기가 멈췄어요.'],
])(
  'identifies %s and preserves the retry action without starting an export',
  async (code, message) => {
    failure.code = code
    const plan = clipTimelineFixture().plan
    const snapshot = {
      ownerId: 'owner',
      projectId: 'project',
      versions: {},
      sources: [],
      snapshotFingerprint: 'current',
      frameCount: 594,
      plan,
    } as unknown as BrowserCompositionSnapshot
    render(
      <ClipBrowserDraftPreview
        ratio="vertical"
        plan={plan}
        local={{ snapshot, source: vi.fn(), audioSource: vi.fn() }}
        preview={{
          ready: true,
          updating: false,
          assets: [],
          canvasWidth: 1080,
          canvasHeight: 1920,
          onRetry: vi.fn(),
        }}
      />,
    )
    expect(await screen.findByText(new RegExp(message))).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '미리보기 다시 준비' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: /서버.*승인/ })).not.toBeInTheDocument()
  },
)

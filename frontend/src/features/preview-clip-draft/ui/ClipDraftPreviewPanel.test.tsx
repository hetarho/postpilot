import { useState } from 'react'
import { webcrypto } from 'node:crypto'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { ClipDraftPreviewPanel } from './ClipDraftPreviewPanel'

const integration = vi.hoisted(() => ({
  active: false,
  hold: false,
  position: 0,
  running: false,
  plays: [] as number[],
  preparations: [] as Array<() => void>,
}))
vi.mock('@/entities/clip-plan', async (original) => ({
  ...(await original<typeof import('@/entities/clip-plan')>()),
  useClipSpeechCalls: () => ({ load: vi.fn() }),
}))
vi.mock('@/entities/clip-preview/lib/create-preview-worker', () => ({
  BrowserPreviewWorker: class {
    fingerprint = ''
    initialize = async (snapshot: { snapshotFingerprint: string }) => {
      this.fingerprint = snapshot.snapshotFingerprint
    }
    prepare = async () => {
      if (integration.hold)
        await new Promise<void>((resolve) => integration.preparations.push(resolve))
    }
    render = async (request: { frame: number; flow: boolean }) => ({
      ...request,
      fingerprint: this.fingerprint,
      displayed: [],
      bitmap: { close: vi.fn() },
      resources: { liveFrames: 0 },
      backgroundMeasured: true,
    })
    cancel = vi.fn()
    dispose = vi.fn()
  },
}))
vi.mock('@/entities/clip-preview/model/composition-playback', () => ({
  BrowserCompositionPlayback: class {
    get timeMs() {
      return integration.position
    }
    get running() {
      return integration.running
    }
    play = async (position: number) => {
      integration.plays.push(position)
      integration.position = position
      integration.running = true
      return true
    }
    pause = () => {
      integration.running = false
    }
    setMuted = vi.fn()
    dispose = () => {
      integration.running = false
    }
  },
}))

const received: Array<{ timeMs: number; onTimeChange: (ms: number) => void }> = []
vi.mock('@/entities/clip-preview', async (original) => {
  const actual = await original<typeof import('@/entities/clip-preview')>()
  return {
    ...actual,
    ClipDraftPreview: (props: { timeMs: number; onTimeChange: (ms: number) => void }) => {
      received.push(props)
      if (integration.active)
        return (
          <actual.ClipDraftPreview
            {...(props as unknown as Parameters<typeof actual.ClipDraftPreview>[0])}
          />
        )
      return <button onClick={() => props.onTimeChange(props.timeMs + 1000)}>advance frame</button>
    },
  }
})

afterEach(() => {
  cleanup()
  received.length = 0
  integration.active = false
  integration.hold = false
  integration.position = 0
  integration.running = false
  integration.plays.length = 0
  integration.preparations.length = 0
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

it('keeps the frame clock callback while playback advances', () => {
  render(
    <ClipDraftPreviewPanel
      ownerId="owner"
      projectId="project"
      revision={1}
      plan={{ durationMs: 2000, cuts: [] } as ClipEditPlan}
      ratio="vertical"
      sources={[]}
      resolvePlayback={async () => ''}
    />,
  )
  const first = received.at(-1)!.onTimeChange
  fireEvent.click(screen.getByRole('button', { name: 'advance frame' }))
  expect(received.at(-1)!.timeMs).toBe(1000)
  expect(received.at(-1)!.onTimeChange).toBe(first)
  fireEvent.click(screen.getByRole('button', { name: 'advance frame' }))
  expect(received.at(-1)!.timeMs).toBe(2000)
  expect(received.at(-1)!.onTimeChange).toBe(first)
})

it('the actual hook/panel/player continues an exact-draft ACK after a pre-Play caption edit', async () => {
  integration.active = true
  vi.stubGlobal('crypto', webcrypto)
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    drawImage: vi.fn(),
    clearRect: vi.fn(),
  } as unknown as ReturnType<HTMLCanvasElement['getContext']>)
  const fingerprint = 'a'.repeat(64)
  const resolvePlayback = vi.fn(async () => 'https://owned.test/original')
  const draft: ClipEditPlan = {
    nativeComposition: true,
    durationMs: 15000,
    cuts: [
      {
        id: 'cut',
        sourceId: 'source',
        fingerprint,
        startMs: 0,
        endMs: 15000,
        transitionMs: 0,
        copies: [],
        volumePermille: 1000,
        playbackRatePermille: 1000,
      },
    ],
    elements: [
      {
        instanceId: 'caption',
        elementId: 'caption',
        cutId: '',
        kind: 'fixed',
        role: 'caption',
        text: '첫 자막',
        rows: [],
        style: 'bold',
        position: 'auto',
        align: 'center',
        basis: 'output-start',
        startMs: 1000,
        endMs: 6000,
        resolvedStartMs: 1000,
        resolvedEndMs: 6000,
        pace: 'steady',
        accent: '',
        keyword: '',
        groupId: '',
        itemId: '',
      },
    ],
  }
  const sources = [
    {
      id: 'source',
      fingerprint,
      filename: 'source.mp4',
      durationMs: 15000,
      width: 320,
      height: 180,
      hasAudio: false,
      allowedRatePermille: [1000],
    },
  ]
  function Editing() {
    const [plan, setPlan] = useState(draft),
      [revision, setRevision] = useState(1)
    return (
      <>
        <button
          onClick={() =>
            setPlan({
              ...plan,
              elements: plan.elements!.map((element) => ({ ...element, text: '저장할 자막' })),
            })
          }
        >
          edit caption
        </button>
        <button onClick={() => setRevision(2)}>held ACK</button>
        <ClipDraftPreviewPanel
          ownerId="owner"
          projectId="project"
          projectRevision={revision}
          revision={revision}
          plan={plan}
          sources={sources}
          ratio="vertical"
          design={{ hideDisclosure: true }}
          resolvePlayback={resolvePlayback}
        />
      </>
    )
  }
  const view = render(<Editing />)
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  fireEvent.click(screen.getByRole('button', { name: 'edit caption' }))
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  await waitFor(() => expect(integration.running).toBe(true))
  integration.position = 2533
  integration.hold = true
  fireEvent.click(screen.getByRole('button', { name: 'held ACK' }))
  await waitFor(() => expect(integration.preparations).toHaveLength(1))
  expect(integration.running).toBe(false)
  expect(screen.getByRole('button', { name: '일시 정지' })).toBeEnabled()
  await act(async () => {
    integration.hold = false
    integration.preparations.splice(0).forEach((finish) => finish())
  })
  await waitFor(() => expect(integration.plays).toEqual([0, 2533]))
  expect(resolvePlayback).toHaveBeenCalledOnce()
  expect(integration.running).toBe(true)
  expect(received.at(-1)!.timeMs).toBe(2533)
  view.unmount()
  expect(integration.running).toBe(false)
})

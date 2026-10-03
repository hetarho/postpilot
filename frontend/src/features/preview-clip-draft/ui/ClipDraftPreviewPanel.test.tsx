import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { ClipDraftPreviewPanel } from './ClipDraftPreviewPanel'

const received: Array<{ timeMs: number; onTimeChange: (ms: number) => void }> = []
vi.mock('@/entities/clip-preview', () => ({
  ClipDraftPreview: (props: { timeMs: number; onTimeChange: (ms: number) => void }) => {
    received.push(props)
    return <button onClick={() => props.onTimeChange(props.timeMs + 1000)}>advance frame</button>
  },
}))
vi.mock('../model/useClipDraftPreview', () => ({
  useClipDraftPreview: () => ({ ready: true, assets: [] }),
}))
afterEach(() => {
  cleanup()
  received.length = 0
})

it('keeps the frame clock callback while playback advances', () => {
  render(
    <ClipDraftPreviewPanel
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

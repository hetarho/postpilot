import { useReducer, useState } from 'react'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { clipTimelineReducer, createClipTimeline } from '@/entities/clip-plan'
import { clipTimelineFixture } from '@/test/clip-editing'
import { ClipCorrectionWorkspace, type ClipEditorPreviewProps } from './ClipCorrectionWorkspace'

vi.mock('@/entities/clip-preview', async (original) => ({
  ...(await original<typeof import('@/entities/clip-preview')>()),
  useClipLocalCaptionPreview: () => ({}),
  useClipLocalStyleSamples: () => ({}),
}))

const initialHeight = window.innerHeight
afterEach(() => {
  cleanup()
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: initialHeight })
})

function Preview({ timeMs, maxHeight, corner }: ClipEditorPreviewProps) {
  const [flow, setFlow] = useState(false)
  return (
    <div>
      <div data-testid="frame" style={{ maxHeight }}>
        <video data-testid="media" />
        {corner}
      </div>
      <button onClick={() => setFlow(!flow)}>{flow ? 'flow' : 'video'}</button>
      <output aria-label="preview time">{timeMs}</output>
    </div>
  )
}

function Harness() {
  const state = clipTimelineFixture()
  const [timeline, dispatch] = useReducer(clipTimelineReducer, state.plan, createClipTimeline)
  const correction = {
    timeline,
    draft: timeline.plan,
    dispatch,
    change: vi.fn(),
    moveCut: vi.fn(),
    validation: { valid: true, cuts: [], elements: [] },
  } as unknown as Parameters<typeof ClipCorrectionWorkspace>[0]['correction']
  return (
    <ClipCorrectionWorkspace
      project={{ id: 'project', state }}
      correction={correction}
      render={{}}
      footage={{ localSources: [] }}
      slots={{ preview: (props) => <Preview {...props} /> }}
      disabled={false}
    />
  )
}

it('expands and compacts the same preview while retaining its mode, selection and time', async () => {
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: 800 })
  render(<Harness />)
  await userEvent.click(screen.getByRole('button', { name: '컷 2' }))
  await userEvent.click(screen.getByRole('button', { name: 'video' }))
  const media = screen.getByTestId('media')
  const frame = screen.getByTestId('frame')
  expect(frame).toHaveStyle({ maxHeight: '240px' })
  expect(screen.getByLabelText('preview time')).toHaveTextContent('9800')
  await userEvent.click(screen.getByRole('button', { name: '미리보기 크게 보기' }))
  expect(frame).toHaveStyle({ maxHeight: '480px' })
  expect(screen.getByRole('button', { name: '미리보기 작게 보기' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  expect(screen.getByTestId('media')).toBe(media)
  expect(screen.getByRole('button', { name: 'flow' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '컷 2' })).toHaveAttribute('aria-pressed', 'true')
  expect(screen.getByLabelText('preview time')).toHaveTextContent('9800')
  act(() => {
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 400 })
    fireEvent(window, new Event('resize'))
  })
  expect(frame).toHaveStyle({ maxHeight: '140px' })
  await userEvent.click(screen.getByRole('button', { name: '미리보기 작게 보기' }))
  expect(frame).toHaveStyle({ maxHeight: '80px' })
  expect(screen.getByTestId('media')).toBe(media)
  expect(screen.getByRole('button', { name: 'flow' })).toBeInTheDocument()
})

import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { clipNarrationFixture } from '@/test/clip-editing'
import { ClipTextControls } from './ClipTextControls'

afterEach(cleanup)

it('offers a narration caption its own text and absolute times, and no placement', async () => {
  const plan = clipNarrationFixture().plan
  const change = vi.fn()
  render(
    <ClipTextControls
      plan={plan}
      text={plan.elements!.find((text) => text.instanceId === 'narration-2')!}
      change={change}
      invalid={false}
    />,
  )
  // Absolute output times, not a cut's offsets.
  const start = screen.getByLabelText('자막 시작 (전체 기준)')
  expect(start).toHaveValue(8)
  fireEvent.change(start, { target: { value: '9' } })
  expect(change).toHaveBeenCalledWith(
    { type: 'text', id: 'narration-2', patch: { startMs: 9000, endMs: 12000 } },
    'narration-2:start',
  )
  expect(screen.getByLabelText('자막 원문')).toHaveValue('컷을 건너가는 자막')
  // Removal is the item sheet's pinned footer, not this block's last control (CLIP-53).
  expect(screen.queryByRole('button', { name: '문구 삭제' })).not.toBeInTheDocument()
  // The server places a caption and the project sets its pace and accent.
  for (const label of ['위치', '정렬', '강조색', '자막 속도', '기준']) {
    expect(screen.queryByLabelText(label)).not.toBeInTheDocument()
  }
})

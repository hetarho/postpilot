import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { ClipTextControls } from './ClipTextControls'

afterEach(cleanup)
it('reuses one localized fallback line and removes it after the owner-edited response', () => {
  const state = clipTimelineFixture()
  const text = { ...state.plan.elements![0], fallbackReason: 'shorter_copy' }
  const props = {
    plan: state.plan,
    text,
    styles: state.copyStyles,
    change: vi.fn(),
    invalid: false,
    language: 'ko' as const,
  }
  const notice = {
    code: 'shorter_copy',
    elementId: text.elementId,
    cutId: text.cutId,
    action: 'repair',
  }
  const view = render(<ClipTextControls {...props} notices={[notice]} />)
  expect(screen.getAllByText('같은 내용을 담은 더 짧은 문구를 사용했어요.')).toHaveLength(1)
  expect(screen.getByLabelText('자막 원문')).toBeEnabled()
  expect(screen.queryByText(/shorter_copy/)).not.toBeInTheDocument()
  view.rerender(
    <ClipTextControls
      {...props}
      text={{ ...text, text: '직접 쓴 문구', fallbackReason: '' }}
      notices={[]}
    />,
  )
  expect(screen.queryByText('같은 내용을 담은 더 짧은 문구를 사용했어요.')).not.toBeInTheDocument()
})

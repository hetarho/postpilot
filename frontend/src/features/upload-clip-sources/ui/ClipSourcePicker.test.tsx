import type { ComponentProps } from 'react'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ClipSelectionError } from '../model/manifest'
import { ClipSourcePicker } from './ClipSourcePicker'

function fixture(): ComponentProps<typeof ClipSourcePicker>['upload'] {
  return {
    phase: 'uploading',
    ensurePlayback: vi.fn(async () => 'blob:test'),
    refreshRetained: vi.fn(async () => {}),
    entries: ['one.mp4', 'two.mp4', 'three.mp4'].map((filename, index) => ({
      file: new File(['video'], filename),
      metadata: {
        filename,
        fingerprint: filename,
        durationMs: 10000,
        width: 1920,
        height: 1080,
        contentType: 'video/mp4',
        bytes: 100,
      },
      previewURL: `blob:${filename}`,
      percent: index === 0 ? 100 : 25,
      confirmed: index === 0,
    })),
    select: vi.fn(),
    cancel: vi.fn(),
    beginAttempt: vi.fn(),
    markOwned: vi.fn(),
    rejectAttempt: vi.fn(),
    finishAttempt: vi.fn(),
  }
}

describe('horizontal clip source picker', () => {
  it('keeps only the selected full player and switches it by keyboard', async () => {
    const user = userEvent.setup()
    const { container, rerender } = render(<ClipSourcePicker upload={fixture()} />)
    expect(screen.getByRole('list', { name: '선택한 원본 영상' })).toHaveClass('overflow-x-auto')
    expect(container.querySelectorAll('video[controls]')).toHaveLength(1)
    expect(screen.getByLabelText('one.mp4')).toHaveAttribute('src', 'blob:one.mp4')
    const second = screen.getByRole('button', { name: 'two.mp4 선택' })
    expect(within(second).getByText('업로드 25%')).toBeVisible()
    second.focus()
    await user.keyboard('{Enter}')
    expect(second).toHaveAttribute('aria-pressed', 'true')
    expect(container.querySelectorAll('video[controls]')).toHaveLength(1)
    expect(screen.getByLabelText('two.mp4', { selector: 'video' })).toHaveAttribute(
      'src',
      'blob:two.mp4',
    )
    expect(screen.getByRole('progressbar', { name: 'two.mp4' })).toHaveAttribute(
      'aria-valuenow',
      '25',
    )
    const upload = fixture()
    upload.entries = [upload.entries[0]!]
    rerender(<ClipSourcePicker upload={upload} />)
    expect(screen.getByLabelText('one.mp4')).toBeInTheDocument()
  })

  it('keeps failures outside the scroller and terminal summaries without any source pixels', () => {
    const upload = fixture()
    upload.error = new ClipSelectionError('duplicate')
    const { container, rerender } = render(<ClipSourcePicker upload={upload} />)
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('같은 영상이 중복 선택됐어요.')
    expect(screen.getByRole('list', { name: '선택한 원본 영상' })).not.toContainElement(alert)
    rerender(
      <ClipSourcePicker
        upload={{
          ...upload,
          phase: 'finished',
          entries: [],
          summaries: [{ filename: 'one.mp4', status: 'done' }],
        }}
      />,
    )
    expect(container.querySelectorAll('video')).toHaveLength(0)
    expect(screen.getByRole('list', { name: '처리가 끝난 원본 목록' })).toHaveClass(
      'overflow-x-auto',
    )
    expect(screen.getByText(/one.mp4 · 처리 완료/)).toBeVisible()
  })
})

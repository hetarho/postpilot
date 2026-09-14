import type { ComponentProps } from 'react'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ClipSelectionError } from '../model/manifest'
import { ClipSourcePicker } from './ClipSourcePicker'

function fixture(): ComponentProps<typeof ClipSourcePicker>['upload'] {
  return {
    phase: 'uploading',
    acceptSoundBatch: vi.fn(),
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

it.each(['local', 'retained'])(
  'offers read-only %s playback during production without upload or cancellation',
  async (kind) => {
    const upload = fixture()
    upload.phase = 'owned'
    upload.entries = upload.entries.map((entry) => ({
      ...entry,
      ...(kind === 'retained'
        ? { file: undefined, previewURL: 'https://private.test/' + entry.metadata.filename }
        : {}),
    }))
    const { container } = render(<ClipSourcePicker upload={upload} processing readOnly />)
    await screen.findByRole('heading', { name: '업로드한 원본' })
    expect(container.querySelector('input[type=file]')).toBeNull()
    expect(screen.queryByRole('button', { name: '선택 취소' })).not.toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'two.mp4 선택' }))
    const player = screen.getByLabelText('two.mp4', { selector: 'video' })
    expect(player).toHaveAttribute('src', upload.entries[1].previewURL)
    expect(player).not.toHaveAttribute('autoplay')
    expect(container.querySelectorAll('video[controls]')).toHaveLength(1)
    expect(upload.ensurePlayback).toHaveBeenCalledWith('two.mp4')
    expect(upload.select).not.toHaveBeenCalled()
    expect(upload.cancel).not.toHaveBeenCalled()
    expect(upload.beginAttempt).not.toHaveBeenCalled()
  },
)

it('distinguishes loading and missing originals and surfaces playback failures during production', async () => {
  const upload = fixture()
  upload.entries = []
  let finish!: () => void
  upload.refreshRetained = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        finish = resolve
      }),
  )
  const { rerender } = render(<ClipSourcePicker upload={upload} processing readOnly />)
  expect(screen.getByText('원본 미리보기를 불러오는 중이에요.')).toBeVisible()
  await act(async () => finish())
  expect(screen.getByText('지금 표시할 원본 미리보기가 없어요.')).toBeVisible()
  const retained = fixture()
  retained.entries = [
    { ...retained.entries[0], file: undefined, previewURL: '', playbackError: 'expired' },
  ]
  rerender(<ClipSourcePicker upload={retained} processing readOnly />)
  expect(screen.getByText('원본 미리보기를 불러올 수 없어요.')).toBeVisible()
  rerender(<ClipSourcePicker upload={fixture()} processing readOnly />)
  fireEvent.error(screen.getByLabelText('one.mp4', { selector: 'video' }))
  expect(screen.getByText('원본 미리보기를 불러올 수 없어요.')).toBeVisible()
})

it('opens an available original when the first retained entry is missing', async () => {
  const upload = fixture()
  upload.entries = upload.entries.map((entry, i) =>
    i
      ? entry
      : {
          ...entry,
          file: undefined,
          previewURL: '',
          availability: 'missing',
          playbackError: 'missing',
        },
  )
  render(<ClipSourcePicker upload={upload} processing readOnly />)
  expect(await screen.findByLabelText('two.mp4', { selector: 'video' })).toHaveAttribute(
    'src',
    'blob:two.mp4',
  )
  expect(upload.ensurePlayback).not.toHaveBeenCalledWith('one.mp4')
})

it('keeps source selection/playback independent of the sibling keyboard sound switch and reports retry', async () => {
  const upload = fixture()
  upload.phase = 'ready'
  upload.entries = upload.entries.map((entry, i) => ({
    ...entry,
    batchId: 'batch',
    sourceId: String(i),
    current: true,
    confirmed: true,
    availability: 'available',
    retentionExpiresAt: '2099-01-01T00:00:00Z',
    retainOriginalAudio: i === 1,
  }))
  const sound = {
    value: (s: { retainOriginalAudio: boolean }) => s.retainOriginalAudio,
    change: vi.fn(),
    retry: vi.fn(),
    failed: false,
  }
  const { rerender, container } = render(<ClipSourcePicker upload={upload} sound={sound} />)
  const second = screen.getByRole('switch', { name: 'two.mp4 원본 소리 유지' })
  expect(second).toBeChecked()
  expect(second.closest('button')).toBeNull()
  expect(screen.getByRole('switch', { name: 'one.mp4 원본 소리 유지' })).not.toBeChecked()
  second.focus()
  await userEvent.keyboard(' ')
  expect(sound.change).toHaveBeenCalledWith(
    expect.objectContaining({ sourceId: '1', batchId: 'batch', fingerprint: 'two.mp4' }),
    false,
  )
  expect(screen.getByRole('button', { name: 'one.mp4 선택' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  expect(container.querySelector('video[controls]')).toHaveAttribute('src', 'blob:one.mp4')
  rerender(<ClipSourcePicker upload={upload} sound={{ ...sound, failed: true }} />)
  await userEvent.click(screen.getByRole('button', { name: '소리 설정 다시 저장' }))
  expect(sound.retry).toHaveBeenCalledOnce()
  rerender(<ClipSourcePicker upload={upload} sound={sound} readOnly />)
  expect(screen.queryByRole('switch')).not.toBeInTheDocument()
})

it.each(['running', 'unavailable', 'not-current', 'expired'])(
  'refuses the %s source sound control',
  async (reason) => {
    const upload = fixture()
    upload.phase = 'ready'
    upload.entries = [
      {
        ...upload.entries[0],
        batchId: 'batch',
        sourceId: 'source',
        current: reason !== 'not-current',
        confirmed: true,
        availability: reason === 'unavailable' ? 'missing' : 'available',
        retentionExpiresAt: reason === 'expired' ? '2000-01-01T00:00:00Z' : '2099-01-01T00:00:00Z',
      },
    ]
    const sound = {
      value: () => false,
      change: vi.fn(),
      retry: vi.fn(),
      disabled: reason === 'running',
    }
    render(<ClipSourcePicker upload={upload} sound={sound} />)
    expect(screen.getByRole('switch')).toBeDisabled()
    await userEvent.click(screen.getByRole('switch'))
    expect(sound.change).not.toHaveBeenCalled()
  },
)

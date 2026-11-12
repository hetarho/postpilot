import { render, screen, within } from '@testing-library/react'
import { expect, it } from 'vitest'
import type { LocalClipSource } from '@/features/upload-clip-sources'
import { ClipPreparationSources } from './ClipPreparationSources'

const source = (filename: string, extra: Partial<LocalClipSource> = {}): LocalClipSource => ({
  metadata: {
    filename,
    fingerprint: filename,
    durationMs: 2000,
    bytes: 4,
    width: 10,
    height: 10,
    contentType: 'video/mp4',
  },
  confirmed: true,
  previewURL: '',
  percent: 100,
  ...extra,
})

it('reads current order, pending audio and item choices, distinguishing pending and expired originals', () => {
  const first = source('first.mp4', { confirmed: false })
  first.metadata.durationMs = 12345.678
  const entries = [
    source('second.mp4'),
    first,
    source('expired.mp4', { retentionExpiresAt: '2000-01-01T00:00:00Z' }),
  ]
  const value = {
    upload: { entries },
    sound: { value: ({ fingerprint }: { fingerprint: string }) => fingerprint === 'second.mp4' },
    binding: {
      items: [{ value: 'item', label: '메뉴 · 파스타' }],
      value: ({ fingerprint }: { fingerprint: string }) =>
        fingerprint === 'second.mp4' ? 'item' : '',
    },
  }
  const view = render(
    <ClipPreparationSources
      sources={value}
      ready={false}
      now={Date.parse('2026-11-11T00:00:00Z')}
    />,
  )
  expect(screen.getByText('3개 · 준비 완료 1개')).toBeVisible()
  const rows = screen.getAllByRole('listitem')
  expect(rows[0]).toHaveTextContent('1. second.mp4 · 2초 · 준비 완료 · 원본 소리 유지')
  expect(within(rows[0]).getByText('항목 연결: 메뉴 · 파스타')).toBeVisible()
  // A probed fractional duration reads in tenths of a second.
  expect(rows[1]).toHaveTextContent('2. first.mp4 · 12.3초 · 업로드·준비 대기 · 원본 소리 제외')
  expect(rows[2]).toHaveTextContent('원본을 다시 선택해 주세요.')
  view.rerender(
    <ClipPreparationSources
      sources={{ ...value, upload: { entries: [] } }}
      ready={false}
      now={Date.parse('2026-11-11T00:00:00Z')}
    />,
  )
  expect(screen.getByText('소스에서 원본 영상을 선택해 주세요.')).toBeVisible()
  expect(screen.queryByRole('listitem')).toBeNull()
})

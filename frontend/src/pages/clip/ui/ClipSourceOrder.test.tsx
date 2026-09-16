import { afterEach, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ClipSourceBatchSchema } from '@/shared/api/gen/postpilot/v1/clip_pb'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeClipsOptions } from '@/test/clips'
import { discardClipDraftQueues } from '@/features/edit-clip-project'

const template = {
  id: 'template',
  name: '여행',
  informationFields: [],
  cutGuidance: '',
  accent: '' as const,
  preset: 'restaurant' as const,
}
const project = {
  id: 'owned',
  title: '고기',
  videoTemplateId: template.id,
  ratio: 'vertical' as const,
  targetDurationMs: 30000,
  answers: [],
  disclosure: 'ad' as const,
  cta: '' as const,
}
const batch = create(ClipSourceBatchSchema, {
  id: 'retained',
  projectId: 'owned',
  current: true,
  state: 'ready',
  expiresAt: '2099-01-01T00:00:00Z',
  sources: ['one', 'two', 'three'].map((name, i) => ({
    id: `src-${i}`,
    state: 'ready',
    availability: 'available',
    retentionExpiresAt: '2099-01-01T00:00:00Z',
    metadata: {
      filename: `${name}.mp4`,
      durationMs: 10000 + i,
      fingerprint: `fp-${i}`,
      contentType: 'video/mp4',
      bytes: 5n,
    },
  })),
})
afterEach(() => discardClipDraftQueues())

// The owner arranges the footage in ① and the writer follows that order when no
// instruction says otherwise (CLIP-136). The drag has a keyboard and touch path,
// which is what these two buttons are (CLIP-55).
it('moves a source without a pointer, announces where it went and saves the whole order', async () => {
  const user = userEvent.setup()
  const orders: NonNullable<FakeClipsOptions['sourceOrders']> = []
  renderAppAt('/clips/owned', {
    user: { id: 'alice' },
    providers: { models: [] },
    clips: {
      templates: [template],
      projects: [project],
      retainedBatches: [batch],
      sourceOrders: orders,
    },
  })
  const strip = await screen.findByRole('list', { name: '선택한 원본 영상' })
  expect(
    within(strip)
      .getAllByRole('button', { name: /선택$/ })
      .map((tile) => tile.getAttribute('aria-label')),
  ).toEqual(['one.mp4 선택', 'two.mp4 선택', 'three.mp4 선택'])
  // The first tile cannot move earlier and the last cannot move later.
  expect(within(strip).getByRole('button', { name: 'one.mp4을 앞으로 옮기기' })).toBeDisabled()
  expect(within(strip).getByRole('button', { name: 'three.mp4을 뒤로 옮기기' })).toBeDisabled()
  await user.click(within(strip).getByRole('button', { name: 'one.mp4을 뒤로 옮기기' }))
  await waitFor(() => expect(orders).toHaveLength(1))
  // The WHOLE batch travels, in its new order.
  expect(orders[0]).toEqual(['src-1', 'src-0', 'src-2'])
  expect(screen.getByText('one.mp4 · 2 / 3 번째로 옮겼어요')).toHaveAttribute('aria-live', 'polite')
  // The strip then shows what the server holds.
  await waitFor(() =>
    expect(
      within(screen.getByRole('list', { name: '선택한 원본 영상' }))
        .getAllByRole('button', { name: /선택$/ })
        .map((tile) => tile.getAttribute('aria-label')),
    ).toEqual(['two.mp4 선택', 'one.mp4 선택', 'three.mp4 선택']),
  )
})

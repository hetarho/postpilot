import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ClipQuoteApproval } from './ClipQuoteApproval'
import type { ClipQuote } from '../model/types'

const quote: ClipQuote = {
  quoteId: 'quote',
  maxCredits: 20,
  expiresAt: '2026-09-30T15:00:00Z',
  binding: 'binding',
  cancellationPolicy: { version: 1, numerator: 1, denominator: 2, rounding: 'ceil' },
  calls: [
    { label: 'observe', calls: 2 },
    { label: 'flow', calls: 1 },
    { label: 'narration', calls: 1 },
  ],
}

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

const approval = (extra: Partial<Parameters<typeof ClipQuoteApproval>[0]> = {}) =>
  render(
    <ClipQuoteApproval
      quote={quote}
      quoting={false}
      expired={false}
      balance={{ credits: 100, unlimited: true, renewsAt: '2026-09-30T15:00:00Z' }}
      approveLabel="최대 20 크레딧 · 승인하고 생성"
      onRefresh={() => {}}
      onApprove={() => {}}
      {...extra}
    />,
  )

// The panel is docked, so on a phone its explanations covered the step behind it. It folds by
// default — but what the approval itself must state cannot fold: the ceiling rides the action,
// the cancellation rule sits beside it (CLIP-19, CLIP-81), and a refusal must not be hidden
// behind a control the owner has no reason to press.
it('folds the breakdown while the approval keeps saying what it charges for', async () => {
  approval({
    error: {
      reason: 'CLIP_COMPOSITION_INVALID',
      params: {
        element_id: 'extra',
        line: '1',
        reason: 'answer_limit',
        label: '기타 정보',
        max: '18',
        actual: '105',
      },
    },
  })
  expect(screen.getByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })).toBeInTheDocument()
  expect(screen.getByText(/취소/)).toBeVisible()
  expect(screen.getByText(/기타 정보/)).toBeVisible()
  expect(screen.queryByText(/청구 상한/)).not.toBeInTheDocument()
  expect(screen.queryByRole('list', { name: '작성 호출' })).not.toBeInTheDocument()

  await userEvent.click(screen.getByRole('button', { name: '요금 자세히' }))
  expect(screen.getByText(/청구 상한/)).toBeVisible()
  expect(screen.getByRole('list', { name: '작성 호출' })).toBeVisible()
  expect(screen.getByText(/마스터는 크레딧을 차감하지/)).toBeVisible()

  await userEvent.click(screen.getByRole('button', { name: '요금 접기' }))
  expect(screen.queryByRole('list', { name: '작성 호출' })).not.toBeInTheDocument()
})

// CDS-81: a sequence-rendered caption costs one rasterisation per output frame,
// so the approval says how many there are and what they add — and says it
// plainly when a clip has none. It refuses nothing: the button is unchanged
// (CLIP-145), and rendering costs no credits at all (CLIP-20).
it('states what the frame-by-frame captions add to the render, without gating on it', () => {
  approval({
    quote: {
      ...quote,
      sequenceCaptions: {
        fromPlan: true,
        captions: 3,
        frames: 180,
        addedRenderMs: 5400,
        selectedStyles: 2,
      },
    },
  })
  expect(screen.getByText('프레임마다 그리는 자막 3개 · 출력이 약 5초 길어져요')).toBeVisible()
  expect(screen.getByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })).toBeEnabled()
})

// Before the first generation there is no plan, so the styles are known and the
// captions are not — the surface says exactly that rather than an estimate.
it('counts the selected styles while no plan exists, and says none where every style is static', () => {
  approval({
    quote: {
      ...quote,
      sequenceCaptions: {
        fromPlan: false,
        captions: 0,
        frames: 0,
        addedRenderMs: 0,
        selectedStyles: 2,
      },
    },
  })
  expect(screen.getByText(/프레임마다 그리는 스타일 2개/)).toBeVisible()
  cleanup()
  approval({
    quote: {
      ...quote,
      sequenceCaptions: {
        fromPlan: true,
        captions: 0,
        frames: 0,
        addedRenderMs: 0,
        selectedStyles: 0,
      },
    },
  })
  expect(screen.getByText('프레임마다 그리는 자막 없음 · 출력 시간이 늘지 않아요')).toBeVisible()
})

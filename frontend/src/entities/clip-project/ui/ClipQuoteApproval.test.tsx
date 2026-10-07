import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
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
  expect(screen.queryByText(/마스터는 크레딧을 차감하지/)).not.toBeInTheDocument()

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
  expect(screen.getByText(/^효과 자막 3개.*서버 예상 추가 시간 약 5초/)).toBeVisible()
  expect(screen.getByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })).toBeEnabled()
})

// The selected styles bound work categories, never device-independent duration.
it.each(['ko', 'en'] as const)(
  'states reusable drawing and device-dependent effects before narration (%s)',
  async (language) => {
    initializeI18n(language)
    const onApprove = vi.fn()
    approval({
      onApprove,
      quote: {
        ...quote,
        sequenceCaptions: {
          fromPlan: false,
          captions: 0,
          frames: 450,
          addedRenderMs: 13500,
          selectedStyles: 2,
        },
      },
    })
    expect(
      screen.getByText(
        language === 'ko'
          ? /^선택한 스타일의 가장 복잡한 효과 기준.*서버 예상 추가 시간 약 14초.*기기와 효과/
          : /^Most demanding selected effects.*about 14s.*depends on the device/,
      ),
    ).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }))
    expect(onApprove).toHaveBeenCalledOnce()
  },
)

it.each([false, true])('states reusable drawing for static captions (fromPlan=%s)', (fromPlan) => {
  approval({
    quote: {
      ...quote,
      sequenceCaptions: { fromPlan, captions: 0, frames: 0, addedRenderMs: 0, selectedStyles: 0 },
    },
  })
  expect(screen.getByText('자막 그림 재사용 · 복잡한 프레임 효과 없음')).toBeVisible()
  expect(screen.getByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })).toBeEnabled()
})

it('keeps approval available for a large sequence estimate', async () => {
  const onApprove = vi.fn()
  approval({
    onApprove,
    quote: {
      ...quote,
      sequenceCaptions: {
        fromPlan: true,
        captions: 100,
        frames: 3600,
        addedRenderMs: 108000,
        selectedStyles: 13,
      },
    },
  })
  expect(screen.getByText(/^효과 자막 100개.*서버 예상 추가 시간 약 108초/)).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }))
  expect(onApprove).toHaveBeenCalledOnce()
})

// QUOTA-65, QUOTA-68: a quote carries no rate for anyone, so the approval states credits only,
// for the operator (an unlimited balance) as for an owner.
it.each([true, false])('names no exchange rate (unlimited balance: %s)', (unlimited) => {
  approval({ balance: { credits: 100, unlimited, renewsAt: '2026-09-30T15:00:00Z' } })
  expect(screen.queryByText(/환율|원\/USD/)).not.toBeInTheDocument()
})

// QUOTA-68: an unlimited (master) balance reads the quote as a customer does, with no
// exemption notice beside it.
it('shows an unlimited balance no exemption notice', () => {
  approval()
  expect(screen.queryByText(/마스터|차감하지 않아요/)).not.toBeInTheDocument()
})

it('shows an unlimited balance no exemption notice in English', () => {
  initializeI18n('en')
  approval()
  expect(screen.queryByText(/not debited|exempt/i)).not.toBeInTheDocument()
})

import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import type { GenerationJob } from '@/entities/generation-job'
import { ClipCreditSettlement } from './ClipCreditSettlement'

it('shows confirmed debit, original-expiry return and later compensation as separate facts', () => {
  render(
    <ClipCreditSettlement
      job={{ id: 'fault', kind: 'generate_clip', status: 'failed' } as GenerationJob}
      accounting={{
        jobId: 'fault',
        status: 'settled',
        settled: true,
        confirmedChargeCredits: 7,
        finalChargeCredits: 7,
        refundCredits: 33,
        compensationCredits: 4,
        compensationExpiresAt: '2026-10-07T12:00:00Z',
        netDebitCredits: 3,
      }}
    />,
  )
  const settlement = screen.getByRole('region', { name: '이번 작업의 크레딧' })
  expect(settlement).toHaveTextContent('확인된 AI 사용 차감 7 크레딧')
  expect(settlement).toHaveTextContent('사용하지 않은 예약 33 크레딧은 원래 만료 시각대로 반환')
  expect(settlement).toHaveTextContent('별도 오류 보상 +4 크레딧')
  expect(settlement).toHaveTextContent('순 크레딧 영향 3 차감')
  expect(settlement).not.toHaveTextContent(/원\/USD|환율/)
})

// QUOTA-65: no caller is sent a rate, so the settlement speaks in credits only.
it('shows the settlement without any exchange rate', () => {
  render(
    <ClipCreditSettlement
      job={{ id: 'done', kind: 'generate_clip', status: 'done' } as GenerationJob}
      accounting={{
        jobId: 'done',
        status: 'settled',
        settled: true,
        confirmedChargeCredits: 7,
        finalChargeCredits: 7,
        refundCredits: 33,
        netDebitCredits: 7,
      }}
    />,
  )
  const settlement = screen.getByRole('region', { name: '이번 작업의 크레딧' })
  expect(settlement).toHaveTextContent('확인된 AI 사용 차감 7 크레딧')
  expect(settlement).not.toHaveTextContent(/원\/USD|환율/)
})

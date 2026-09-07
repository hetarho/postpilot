import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'

describe('BillingMethodFailPage', () => {
  it('keeps the provider message out of the primary localized refusal', async () => {
    renderAppAt('/billing/method/fail?code=PAY_PROCESS_CANCELED&message=Raw%20provider%20copy', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
    })

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('카드 등록을 완료하지 못했습니다. 다시 시도해 주세요.')
    expect(alert).not.toHaveTextContent('Raw provider copy')
    expect(screen.getByText('기술 세부 정보')).toBeInTheDocument()
    expect(screen.getByText('PAY_PROCESS_CANCELED: Raw provider copy')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '결제 관리로 돌아가기' })).toHaveAttribute(
      'href',
      '/billing',
    )
  })
})

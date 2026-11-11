import { useState } from 'react'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { EMPTY_GUIDE } from '@/entities/advertiser-guide'
import { CampaignMaterial } from './CampaignMaterial'
import { CampaignTracker } from './CampaignTracker'

afterEach(cleanup)
it('lets the owner edit a candidate before confirming it and dismiss another without applying it', async () => {
  const user = userEvent.setup()
  function Host() {
    const [guide, setGuide] = useState({ ...EMPTY_GUIDE, text: '사진 8장 이상\n#성수' })
    return (
      <CampaignMaterial guide={guide} readOnly={false} refusal={undefined} onChange={setGuide} />
    )
  }
  render(<Host />)
  await user.click(screen.getByRole('button', { name: '사진 최소 8장 고쳐서 추가' }))
  const input = screen.getByRole('textbox', { name: '사진 최소 8장 수정' })
  await user.clear(input)
  await user.type(input, '7')
  expect(screen.getByLabelText('최소 사진 수')).toHaveValue('')
  await user.click(screen.getByRole('button', { name: '확인해서 추가' }))
  expect(screen.getByLabelText('최소 사진 수')).toHaveValue('7')
  expect(screen.queryByRole('button', { name: '사진 최소 8장 추가' })).toBeNull()
  await user.click(screen.getByRole('button', { name: '#성수 제안 지우기' }))
  expect(screen.queryByRole('button', { name: '#성수 추가' })).toBeNull()
  expect(screen.queryByRole('button', { name: '#성수 지우기' })).toBeNull()
})
it('updates input coverage live without treating a draft character count as finished copy', () => {
  const props = {
    guide: {
      ...EMPTY_GUIDE,
      keywords: ['성수 카페'],
      minChars: 1500,
      minPhotos: 8,
      hashtags: ['성수'],
    },
    photoCount: 2,
    title: '',
    memo: '',
    step: 'details' as const,
    onEdit: () => {},
  }
  const { rerender } = render(
    <CampaignTracker {...props} answers={[{ text: '', enabled: true }]} />,
  )
  expect(screen.getByRole('status')).toHaveTextContent('사진 2 / 8장')
  expect(screen.getByRole('status')).toHaveTextContent('입력한 키워드 0 / 1')
  rerender(<CampaignTracker {...props} answers={[{ text: '성수카페 방문', enabled: true }]} />)
  expect(screen.getByRole('status')).toHaveTextContent('입력한 키워드 1 / 1')
  rerender(
    <CampaignTracker
      {...props}
      step="review"
      answers={[{ text: '성수카페 방문', enabled: false }]}
    />,
  )
  expect(screen.getByText('아직 입력하지 않은 키워드: 성수 카페')).toBeInTheDocument()
  expect(screen.getByText('완성 글에서 1500자 이상인지 확인해요.')).toBeInTheDocument()
  expect(screen.getByText(/부족해도 생성할 수 있어요/)).toBeInTheDocument()
})

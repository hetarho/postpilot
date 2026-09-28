import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import type { DefaultGuideline } from '../model/types'
import { DefaultGuidelineDetail } from './DefaultGuidelineDetail'

afterEach(() => cleanup())

const NAMING: DefaultGuideline = {
  key: 'naming',
  enabled: true,
  name: '메모의 이름으로',
  text: '가게와 메뉴는 메모에 적힌 이름으로 쓰세요.',
  koreanTargetOnly: false,
  memoriesOnly: false,
}

// GUIDE-47: the text is the server's word for word, and a default with no target says nothing more.
it('shows the text word for word and the action it is given', () => {
  render(<DefaultGuidelineDetail guideline={NAMING} action={<button>적용 안함</button>} />)
  expect(screen.getByText('가게와 메뉴는 메모에 적힌 이름으로 쓰세요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '적용 안함' })).toBeInTheDocument()
  expect(screen.queryByText('한국어 글에만 적용돼요')).not.toBeInTheDocument()
  expect(screen.queryByText('기억 사용을 켠 글에만 적용돼요')).not.toBeInTheDocument()
})

it('names the runs a targeted default reaches', () => {
  render(
    <DefaultGuidelineDetail
      guideline={{ ...NAMING, koreanTargetOnly: true, memoriesOnly: true }}
    />,
  )
  expect(screen.getByText('한국어 글에만 적용돼요')).toBeInTheDocument()
  expect(screen.getByText('기억 사용을 켠 글에만 적용돼요')).toBeInTheDocument()
})

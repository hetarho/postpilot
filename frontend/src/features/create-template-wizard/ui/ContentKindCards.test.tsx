import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { SectionDraft } from '../model/wizard-machine'
import { ContentKindCards } from './ContentKindCards'

const section: SectionDraft = {
  id: 'info',
  name: '매장 정보',
  help: '주소와 영업시간',
  checked: true,
  heading: '📍 매장 정보',
  content: 'write',
  question: '주소와 영업시간',
  required: false,
  literal: '지도는 아래에',
  photos: 0,
  repeatable: false,
  repeat: false,
}

// TMPL-74: each kind is a card drawing 글 쓸 때 and 완성된 글 from this section.
describe('the content-kind cards', () => {
  it('offers the three kinds as radios described in plain words', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<ContentKindCards section={section} onChange={onChange} />)
    const group = screen.getByRole('group', { name: '이 자리에 들어갈 것' })
    const write = within(group).getByRole('radio', { name: 'AI가 쓰는 글' })
    expect(write).toBeChecked()
    expect(write).toHaveAccessibleDescription('내가 남긴 메모와 사진으로 AI가 이 부분을 써요.')
    await user.click(within(group).getByRole('radio', { name: '직접 입력' }))
    expect(onChange).toHaveBeenCalledWith('ask')
  })

  it('draws both moments from the section itself', () => {
    render(<ContentKindCards section={section} onChange={() => {}} />)
    expect(screen.getAllByText('글 쓸 때')).toHaveLength(3)
    expect(screen.getAllByText('완성된 글')).toHaveLength(3)
    expect(screen.getAllByText('📍 매장 정보')).toHaveLength(3)
    // 직접 입력: the question's field while writing, the typed answer in the post.
    expect(screen.getByText('주소와 영업시간')).toBeInTheDocument()
    expect(screen.getByText('여기에 직접 적어요')).toBeInTheDocument()
    expect(screen.getByText('적은 내용 그대로')).toBeInTheDocument()
    // 고정 문구: the owner's own text, as it is.
    expect(screen.getByText('지도는 아래에')).toBeInTheDocument()
  })
})

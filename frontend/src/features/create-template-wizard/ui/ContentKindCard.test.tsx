import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { SectionContent } from '@/entities/template'
import type { SectionDraft } from '../model/wizard-machine'
import { ContentKindCard } from './ContentKindCard'

const section = (content: SectionContent): SectionDraft => ({
  id: 'info',
  name: '매장 정보',
  help: '주소와 영업시간',
  checked: true,
  heading: '📍 매장 정보',
  content,
  question: '주소와 영업시간',
  required: false,
  literal: '지도는 아래에',
  photos: 0,
  repeatable: false,
  repeat: false,
})

const inputs = {
  ask: (
    <label>
      물어볼 질문
      <input />
    </label>
  ),
  literal: (
    <label>
      적을 문구
      <textarea />
    </label>
  ),
}

// TMPL-74: one card; radios across its top switch its drawings and its inputs.
describe('the content-kind card', () => {
  it('names the kinds by what happens and reports the choice', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<ContentKindCard section={section('write')} onChange={onChange} inputs={inputs} />)
    const group = screen.getByRole('group', { name: '이 자리에 들어갈 것' })
    expect(
      within(group)
        .getAllByRole('radio')
        .map((radio) => radio.getAttribute('value')),
    ).toEqual(['write', 'ask', 'literal'])
    expect(within(group).getByRole('radio', { name: 'AI가 전부 자동으로 작성' })).toBeChecked()
    expect(group).toHaveAccessibleDescription('내가 남긴 메모와 사진으로 AI가 이 부분을 써요.')
    await user.click(within(group).getByRole('radio', { name: '질문을 바탕으로 작성' }))
    expect(onChange).toHaveBeenCalledWith('ask')
  })

  it('draws only the chosen kind and holds its inputs', () => {
    const { rerender } = render(
      <ContentKindCard section={section('write')} onChange={() => {}} inputs={inputs} />,
    )
    expect(screen.getAllByText('글 쓸 때')).toHaveLength(1)
    expect(screen.getByText('메모와 사진만 남겨요')).toBeInTheDocument()
    expect(screen.queryByLabelText('물어볼 질문')).not.toBeInTheDocument()

    rerender(<ContentKindCard section={section('ask')} onChange={() => {}} inputs={inputs} />)
    expect(screen.getByText('여기에 직접 적어요')).toBeInTheDocument()
    expect(screen.getByText('내 답을 바탕으로')).toBeInTheDocument()
    const group = screen.getByRole('group', { name: '이 자리에 들어갈 것' })
    expect(within(group).getByLabelText('물어볼 질문')).toBeInTheDocument()
    expect(group).toHaveAccessibleDescription(
      '글을 쓸 때 이 질문에 답하면, 그 답을 바탕으로 AI가 이 부분을 써요.',
    )

    rerender(<ContentKindCard section={section('literal')} onChange={() => {}} inputs={inputs} />)
    expect(screen.getByText('지도는 아래에')).toBeInTheDocument()
    expect(within(group).getByLabelText('적을 문구')).toBeInTheDocument()
    expect(screen.queryByLabelText('물어볼 질문')).not.toBeInTheDocument()
  })
})

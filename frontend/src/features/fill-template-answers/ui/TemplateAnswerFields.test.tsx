import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { TemplateAnswerFields } from './TemplateAnswerFields'
import type { AnswerField } from '../model/answers'

const FIELDS: AnswerField[] = [
  {
    label: '방문일',
    flavor: 'verbatim',
    prompt: '',
    required: false,
    text: '2026-03-01',
    enabled: true,
  },
  {
    label: '총평 별점',
    flavor: 'write',
    prompt: '직접 준 점수와 이유',
    required: false,
    text: '4.5점',
    enabled: false,
  },
]

function renderFields(fields: AnswerField[] = FIELDS, onChange = vi.fn()) {
  initializeI18n('ko')
  render(<TemplateAnswerFields fields={fields} onChange={onChange} />)
  return onChange
}

describe('the template’s data fields in ①', () => {
  it('renders one titled field per ask, in the order it was given', () => {
    renderFields()
    const boxes = screen.getAllByRole('textbox')
    expect(boxes).toHaveLength(2)
    expect(screen.getByLabelText('방문일')).toHaveValue('2026-03-01')
    expect(screen.getByLabelText('총평 별점')).toHaveValue('4.5점')
    expect(screen.getByText('직접 준 점수와 이유')).toBeInTheDocument()
  })

  // Off is not empty: the text stays readable so the switch is a decision the author can take
  // back (POST-62).
  it('greys an excluded field while keeping its text', () => {
    renderFields()
    const excluded = screen.getByLabelText('총평 별점')
    expect(excluded).toBeDisabled()
    expect(excluded).toHaveValue('4.5점')
    expect(screen.getByText('이 칸은 글에서 빠져요.')).toBeInTheDocument()
    // The included one is untouched.
    expect(screen.getByLabelText('방문일')).toBeEnabled()
  })

  it('reports a switch and a keystroke by label', async () => {
    const onChange = renderFields()
    await userEvent.click(screen.getByRole('switch', { name: '총평 별점 넣기' }))
    expect(onChange).toHaveBeenCalledWith('총평 별점', { enabled: true })

    onChange.mockClear()
    await userEvent.type(screen.getByLabelText('방문일'), '!')
    expect(onChange).toHaveBeenCalledWith('방문일', { text: '2026-03-01!' })
  })

  it('marks required fields, hides their exclusion switch and points out blanks', () => {
    initializeI18n('ko')
    render(
      <TemplateAnswerFields
        fields={[
          {
            label: '방문 계기',
            flavor: 'write',
            prompt: '그날 직접 겪은 상황',
            required: true,
            text: '',
            enabled: true,
          },
        ]}
        onChange={vi.fn()}
        showRequiredErrors
      />,
    )
    const answer = screen.getByRole('textbox', { name: /방문 계기/ })
    expect(answer).toHaveAttribute('required')
    expect(answer).toHaveAttribute('aria-invalid', 'true')
    expect(answer).toHaveAttribute('data-required-answer-missing', 'true')
    expect(answer).toHaveAccessibleDescription(
      '그날 직접 겪은 상황 글을 만들려면 이 내용을 입력해 주세요.',
    )
    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    expect(screen.getByText('그날 직접 겪은 상황')).toBeInTheDocument()
    expect(screen.getByText(/필수 입력란을 확인해 주세요: 방문 계기/)).toBeInTheDocument()
    expect(screen.getByText('글을 만들려면 이 내용을 입력해 주세요.')).toBeInTheDocument()
  })

  it('keeps a previously excluded required answer editable and offers to restore it', async () => {
    const onChange = vi.fn()
    initializeI18n('ko')
    render(
      <TemplateAnswerFields
        fields={[
          {
            label: '방문 계기',
            flavor: 'write',
            prompt: '직접 가게 된 이유',
            required: true,
            text: '간판을 보고 들어갔다',
            enabled: false,
          },
        ]}
        onChange={onChange}
        showRequiredErrors
      />,
    )
    const answer = screen.getByRole('textbox', { name: /방문 계기/ })
    expect(answer).toBeEnabled()
    expect(answer).toHaveAttribute('aria-invalid', 'true')
    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    expect(screen.getByText('이 답변은 이전에 글에서 제외됐어요.')).toBeInTheDocument()
    expect(
      screen.getByText('기존 답변을 다시 사용하거나 내용을 수정해 주세요.'),
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '기존 답변 사용하기' }))
    expect(onChange).toHaveBeenCalledWith('방문 계기', { enabled: true })
  })

  // An empty section would be a question with no question in it.
  it('renders nothing at all when there is no field', () => {
    const { container } = (() => {
      initializeI18n('ko')
      return render(<TemplateAnswerFields fields={[]} onChange={() => {}} />)
    })()
    expect(container).toBeEmptyDOMElement()
  })
})

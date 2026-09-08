import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { TemplateAnswerFields } from './TemplateAnswerFields'
import type { AnswerField } from '../model/answers'

const FIELDS: AnswerField[] = [
  { label: '방문일', flavor: 'verbatim', text: '2026-03-01', enabled: true },
  { label: '총평 별점', flavor: 'write', text: '4.5점', enabled: false },
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

  // An empty section would be a question with no question in it.
  it('renders nothing at all when there is no field', () => {
    const { container } = (() => {
      initializeI18n('ko')
      return render(<TemplateAnswerFields fields={[]} onChange={() => {}} />)
    })()
    expect(container).toBeEmptyDOMElement()
  })
})

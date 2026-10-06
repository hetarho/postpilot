import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ChoiceButton } from './ChoiceButton'

describe('ChoiceButton', () => {
  it('names the choice and associates its explanation without submitting a form', async () => {
    const user = userEvent.setup()
    const choose = vi.fn()
    render(
      <ChoiceButton
        title="내가 쓴 글로 만들기"
        description="직접 쓴 글을 붙여넣어 주세요."
        onClick={choose}
      />,
    )
    const button = screen.getByRole('button', { name: '내가 쓴 글로 만들기' })
    expect(button).toHaveAccessibleDescription('직접 쓴 글을 붙여넣어 주세요.')
    expect(button).toHaveAttribute('type', 'button')
    await user.tab()
    await user.keyboard('{Enter}')
    expect(choose).toHaveBeenCalledTimes(1)
  })

  it('keeps its accessible name and prevents a duplicate action while pending', async () => {
    const user = userEvent.setup()
    const choose = vi.fn()
    render(
      <ChoiceButton
        title="질문에 답하기"
        description="일상적인 상황을 글로 알려 주세요."
        pending
        onClick={choose}
      />,
    )
    const button = screen.getByRole('button', { name: '질문에 답하기' })
    expect(button).toBeDisabled()
    expect(button).toHaveAttribute('aria-busy', 'true')
    await user.click(button)
    expect(choose).not.toHaveBeenCalled()
  })

  it('exposes an explicitly selected comparison choice', () => {
    render(<ChoiceButton title="편안한 말투" description="친구에게 이야기하듯 써요." selected />)
    expect(screen.getByRole('button', { name: '편안한 말투', pressed: true })).toBeInTheDocument()
  })
})

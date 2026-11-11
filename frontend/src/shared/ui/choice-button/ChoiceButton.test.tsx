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

  // On a phone the tick was a row of its own above the chosen title, so choosing made one card a
  // row taller and the other a row shorter, and the cards under them jolted with each tap.
  it('marks the choice without changing the height of either card', () => {
    const { rerender } = render(
      <>
        <ChoiceButton title="체험단" description="광고주가 부탁한 내용을 먼저 봐요." selected />
        <ChoiceButton title="내 돈으로" description="내 경험으로 써요." selected={false} />
      </>,
    )
    const chosen = screen.getByRole('button', { name: '체험단', pressed: true })
    const other = screen.getByRole('button', { name: '내 돈으로', pressed: false })
    expect(chosen).toHaveClass('ring-2', 'ring-stroke-accent')
    expect(other).not.toHaveClass('ring-2')
    // The narrow tick is laid over the corner the title always leaves free.
    const tick = chosen.querySelector('.absolute.top-4.right-4')
    expect(tick).not.toBeNull()
    expect(tick).toHaveClass('@md:hidden')
    for (const title of [screen.getByText('체험단'), screen.getByText('내 돈으로')])
      expect(title).toHaveClass('pr-7', '@md:pr-0')

    rerender(
      <>
        <ChoiceButton
          title="체험단"
          description="광고주가 부탁한 내용을 먼저 봐요."
          selected={false}
        />
        <ChoiceButton title="내 돈으로" description="내 경험으로 써요." selected />
      </>,
    )
    expect(screen.getByRole('button', { name: '체험단' }).querySelector('.absolute')).toBeNull()
    expect(screen.getByText('체험단')).toHaveClass('pr-7')
  })

  it('exposes an explicitly selected comparison choice', () => {
    render(<ChoiceButton title="편안한 말투" description="친구에게 이야기하듯 써요." selected />)
    expect(screen.getByRole('button', { name: '편안한 말투', pressed: true })).toBeInTheDocument()
  })
})

it('shares peer guidance once without creating a dangling description or changing keyboard activation', async () => {
  const choose = vi.fn()
  const user = userEvent.setup()
  render(
    <>
      <p id="factor-guidance">한 가지 조건만 바꾸어 비교합니다.</p>
      <ChoiceButton title="말투" selected aria-describedby="factor-guidance" onClick={choose} />
      <ChoiceButton title="구성" />
    </>,
  )
  const voice = screen.getByRole('button', { name: '말투', pressed: true })
  expect(voice).toHaveAccessibleDescription('한 가지 조건만 바꾸어 비교합니다.')
  expect(screen.getByRole('button', { name: '구성' })).not.toHaveAttribute('aria-describedby')
  expect(screen.getAllByText('한 가지 조건만 바꾸어 비교합니다.')).toHaveLength(1)
  voice.focus()
  await user.keyboard('{Enter}')
  expect(choose).toHaveBeenCalledTimes(1)
})

import { afterEach, describe, expect, it, vi } from 'vitest'
import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { formatGuide } from '../model/guide'
import { TEMPLATE_LIMITS } from '../model/types'
import { TemplateSource } from './TemplateSource'

const originalClipboard = navigator.clipboard

function setClipboard(value: Pick<Clipboard, 'writeText'> | Clipboard | undefined) {
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value })
}

afterEach(() => {
  setClipboard(originalClipboard)
  vi.restoreAllMocks()
})

/** Controlled, through a real parent, so what the assertions read is what a caller would save. */
function Source({ initial = '' }: { initial?: string }) {
  const [body, setBody] = useState(initial)
  return (
    <>
      <TemplateSource value={body} onChange={setBody} failure={null} />
      <output data-testid="body">{body}</output>
    </>
  )
}

const body = () => screen.getByTestId('body').textContent ?? ''

describe('the source editor', () => {
  // TEMPLATE-42/8/19: what is pasted is what is stored. No trim, no normalization, on input or
  // on the way out — an outside AI's body has to survive this field byte for byte.
  it('keeps what is typed byte for byte, outer whitespace included', async () => {
    const user = userEvent.setup()
    render(<Source />)

    const field = screen.getByLabelText('원문')
    await user.click(field)
    // Leading spaces and a trailing newline: exactly what a paste from a chat window carries.
    await user.paste('  <write>인트로</write>\n')

    expect(body()).toBe('  <write>인트로</write>\n')
    expect(field).toHaveValue('  <write>인트로</write>\n')
  })

  it('shows the failure at its line, in words, and marks the field invalid', () => {
    render(
      <TemplateSource
        value="<write>닫히지 않음"
        onChange={() => {}}
        failure={{ line: 1, reason: 'unclosed_tag' }}
      />,
    )

    const field = screen.getByLabelText('원문')
    expect(field).toHaveAttribute('aria-invalid', 'true')
    const message = screen.getByRole('alert')
    expect(message).toHaveTextContent('1번째 줄: 닫히지 않았어요')
    expect(field).toHaveAttribute('aria-describedby', message.id)
  })

  it('shows no failure and no invalid state when the body parses', () => {
    render(<TemplateSource value="<write>인트로</write>" onChange={() => {}} failure={null} />)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByLabelText('원문')).not.toHaveAttribute('aria-invalid')
  })

  it('copies the body and confirms it', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
    setClipboard({ writeText })
    render(<Source initial="<write>인트로</write>" />)

    await user.click(screen.getByRole('button', { name: '원문 복사' }))
    expect(writeText).toHaveBeenCalledWith('<write>인트로</write>')
    expect(await screen.findByText('복사했어요')).toBeInTheDocument()
  })

  // TEMPLATE-41: the guide is the thing an outside AI is handed, so copying it is the point.
  it('copies the format guide and confirms it', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn<Clipboard['writeText']>().mockResolvedValue(undefined)
    setClipboard({ writeText })
    render(<Source />)

    await user.click(screen.getByRole('button', { name: '형식 안내 복사' }))
    expect(writeText).toHaveBeenCalledWith(formatGuide())
    expect(await screen.findByText('복사했어요')).toBeInTheDocument()
  })

  // A clipboard the browser refuses is exactly when the user needs the text selected instead.
  it('falls back to selecting the body when the clipboard is unavailable', async () => {
    const user = userEvent.setup()
    setClipboard(undefined)
    render(<Source initial="<write>인트로</write>" />)

    await user.click(screen.getByRole('button', { name: '원문 복사' }))

    const field = screen.getByLabelText('원문')
    expect(field).toHaveFocus()
    expect((field as HTMLTextAreaElement).selectionEnd).toBe('<write>인트로</write>'.length)
    expect(await screen.findByText(/복사가 막혀 있어요/)).toBeInTheDocument()
    expect(screen.queryByText('복사했어요')).not.toBeInTheDocument()
  })

  // The guide's fallback field lives inside a `<details>`, and a selection inside a collapsed
  // element is a selection nobody can see — so the disclosure opens with it.
  it('opens the disclosure and selects the guide when the clipboard is unavailable', async () => {
    const user = userEvent.setup()
    setClipboard(undefined)
    render(<Source />)

    await user.click(screen.getByRole('button', { name: '형식 안내 복사' }))

    const guideField = screen.getByLabelText('형식 안내 보기')
    expect(guideField).toHaveFocus()
    expect(guideField.closest('details')).toHaveAttribute('open')
    expect(await screen.findByText(/복사가 막혀 있어요/)).toBeInTheDocument()
  })

  // What the disclosure shows and what the button copies are one string.
  it('shows the same guide it copies', () => {
    render(<Source />)
    expect(screen.getByLabelText('형식 안내 보기')).toHaveValue(formatGuide())
  })

  // The same counter the name and description fields carry, over the body's own ceiling.
  it('counts what is left against the body ceiling', async () => {
    const user = userEvent.setup()
    render(<Source />)

    expect(screen.getByText(`${TEMPLATE_LIMITS.body}자 남음`)).toBeInTheDocument()
    await user.click(screen.getByLabelText('원문'))
    await user.paste('가나다')
    expect(screen.getByText(`${TEMPLATE_LIMITS.body - 3}자 남음`)).toBeInTheDocument()
  })
})
